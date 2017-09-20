package handlers_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"time"

	"code.cloudfoundry.org/bbs/models"
	"code.cloudfoundry.org/clock/fakeclock"
	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/runtimeschema/cc_messages"
	"code.cloudfoundry.org/stager/backend"
	"code.cloudfoundry.org/stager/backend/fake_backend"
	"code.cloudfoundry.org/stager/cc_client"
	"code.cloudfoundry.org/stager/cc_client/fakes"
	"code.cloudfoundry.org/stager/handlers"
	"github.com/cloudfoundry/dropsonde/metric_sender/fake"
	"github.com/cloudfoundry/dropsonde/metrics"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("StagingCompletedHandler", func() {
	var (
		logger lager.Logger

		fakeCCClient        *fakes.FakeCcClient
		fakeBackend         *fake_backend.FakeBackend
		backendResponse     cc_messages.StagingResponseForCC
		backendError        error
		fakeClock           *fakeclock.FakeClock
		metricSender        *fake.FakeMetricSender
		stagingDurationNano time.Duration

		responseRecorder *httptest.ResponseRecorder
		handler          handlers.CompletionHandler
	)

	BeforeEach(func() {
		logger = lager.NewLogger("fakelogger")
		logger.RegisterSink(lager.NewWriterSink(GinkgoWriter, lager.DEBUG))

		stagingDurationNano = 900900
		metricSender = fake.NewFakeMetricSender()
		metrics.Initialize(metricSender, nil)

		fakeCCClient = &fakes.FakeCcClient{}
		fakeBackend = &fake_backend.FakeBackend{}
		backendError = nil

		fakeClock = fakeclock.NewFakeClock(time.Now())

		responseRecorder = httptest.NewRecorder()
		handler = handlers.NewStagingCompletionHandler(logger, fakeCCClient, map[string]backend.Backend{"fake": fakeBackend}, fakeClock)
	})

	JustBeforeEach(func() {
		fakeBackend.BuildStagingResponseReturns(backendResponse, backendError)
	})

	postTask := func(task *models.TaskCallbackResponse) *http.Request {
		taskJSON, err := json.Marshal(task)
		Expect(err).NotTo(HaveOccurred())

		request, err := http.NewRequest("POST", fmt.Sprintf("/v1/staging/%s/completed", task.TaskGuid), bytes.NewReader(taskJSON))
		Expect(err).NotTo(HaveOccurred())

		request.Form = url.Values{":staging_guid": {task.TaskGuid}}

		return request
	}

	Context("when a staging task completes", func() {
		var taskResponse *models.TaskCallbackResponse
		var annotationJson []byte

		BeforeEach(func() {
			var err error
			annotationJson, err = json.Marshal(cc_messages.StagingTaskAnnotation{
				Lifecycle: "fake",
			})
			Expect(err).NotTo(HaveOccurred())
		})

		JustBeforeEach(func() {
			createdAt := fakeClock.Now().UnixNano()
			fakeClock.Increment(stagingDurationNano)

			taskResponse = &models.TaskCallbackResponse{
				TaskGuid:  "the-task-guid",
				CreatedAt: createdAt,
				Result: `{
					"buildpack_key":"buildpack-key",
					"detected_buildpack":"Some Buildpack",
					"execution_metadata":"{\"start_command\":\"./some-start-command\"}",
					"detected_start_command":{"web":"./some-start-command"}
				}`,
				Annotation: string(annotationJson),
			}

			handler.StagingComplete(responseRecorder, postTask(taskResponse))
		})

		It("passes the task response to the matching response builder", func() {
			Eventually(fakeBackend.BuildStagingResponseCallCount()).Should(Equal(1))
			Expect(fakeBackend.BuildStagingResponseArgsForCall(0)).To(Equal(taskResponse))
		})

		Context("when the guid in the url does not match the task guid", func() {
			BeforeEach(func() {
				taskJSON, err := json.Marshal(taskResponse)
				Expect(err).NotTo(HaveOccurred())

				req, err := http.NewRequest("POST", "/v1/staging/an-invalid-guid/completed", bytes.NewReader(taskJSON))
				Expect(err).NotTo(HaveOccurred())

				req.Form = url.Values{":staging_guid": {"an-invalid-guid"}}

				handler.StagingComplete(responseRecorder, req)
			})

			It("returns StatusBadRequest", func() {
				Expect(responseRecorder.Code).To(Equal(http.StatusBadRequest))
			})
		})

		Describe("staging task annotation", func() {
			Context("when the annotation is missing", func() {
				BeforeEach(func() {
					annotationJson = []byte("")
				})

				It("returns bad request", func() {
					Expect(responseRecorder.Code).To(Equal(http.StatusBadRequest))
				})

				It("does not post staging complete to the CC", func() {
					Expect(fakeCCClient.StagingCompleteCallCount()).To(Equal(0))
				})
			})

			Context("when the annotation is invalid JSON", func() {
				BeforeEach(func() {
					annotationJson = []byte(",goo")
				})

				It("returns bad request", func() {
					Expect(responseRecorder.Code).To(Equal(http.StatusBadRequest))
				})

				It("does not post staging complete to the CC", func() {
					Expect(fakeCCClient.StagingCompleteCallCount()).To(Equal(0))
				})
			})

			Context("when lifecycle is missing from the annotation", func() {
				BeforeEach(func() {
					annotationJson = []byte(`{
						"task_id": "the-task-id",
						"app_id": "the-app-id"
					}`)
				})

				It("returns not found", func() {
					Expect(responseRecorder.Code).To(Equal(http.StatusNotFound))
				})

				It("does not post staging complete to the CC", func() {
					Expect(fakeCCClient.StagingCompleteCallCount()).To(Equal(0))
				})
			})
		})

		Context("when the response builder returns an error", func() {
			BeforeEach(func() {
				backendError = errors.New("build error")
			})

			It("returns a 400", func() {
				Expect(responseRecorder.Code).To(Equal(http.StatusBadRequest))
			})
		})

		Context("when the response builder does not return an error", func() {
			var backendResponseJson []byte

			BeforeEach(func() {
				backendResponse = cc_messages.StagingResponseForCC{}

				var err error
				backendResponseJson, err = json.Marshal(backendResponse)
				Expect(err).NotTo(HaveOccurred())
			})

			It("posts the response builder's result to CC", func() {
				Expect(fakeCCClient.StagingCompleteCallCount()).To(Equal(1))
				guid, payload, _ := fakeCCClient.StagingCompleteArgsForCall(0)
				Expect(guid).To(Equal("the-task-guid"))
				Expect(payload).To(Equal(backendResponseJson))
			})

			Context("when the CC request succeeds", func() {
				It("increments the staging success counter", func() {
					Expect(metricSender.GetCounter("StagingRequestsSucceeded")).To(BeEquivalentTo(1))
				})

				It("emits the time it took to stage succesfully", func() {
					Expect(metricSender.GetValue("StagingRequestSucceededDuration")).To(Equal(fake.Metric{
						Value: float64(stagingDurationNano),
						Unit:  "nanos",
					}))

				})

				It("returns a 200", func() {
					Expect(responseRecorder.Code).To(Equal(200))
				})
			})

			Context("when the CC request fails", func() {
				BeforeEach(func() {
					fakeCCClient.StagingCompleteReturns(&cc_client.BadResponseError{504})
				})

				It("responds with the status code that the CC returned", func() {
					Expect(responseRecorder.Code).To(Equal(504))
				})
			})

			Context("When an error occurs in making the CC request", func() {
				BeforeEach(func() {
					fakeCCClient.StagingCompleteReturns(errors.New("whoops"))
				})

				It("responds with a 503 error", func() {
					Expect(responseRecorder.Code).To(Equal(503))
				})

				It("does not update the staging counter", func() {
					Expect(metricSender.GetCounter("StagingRequestsSucceeded")).To(BeEquivalentTo(0))
				})

				It("does not update the staging duration", func() {
					Expect(metricSender.GetValue("StagingRequestSucceededDuration")).To(Equal(fake.Metric{}))
				})
			})
		})
	})

	Context("when a staging task fails", func() {
		var backendResponseJson []byte

		BeforeEach(func() {
			backendResponse = cc_messages.StagingResponseForCC{}

			var err error
			backendResponseJson, err = json.Marshal(backendResponse)
			Expect(err).NotTo(HaveOccurred())
		})

		JustBeforeEach(func() {
			createdAt := fakeClock.Now().UnixNano()
			fakeClock.Increment(stagingDurationNano)

			taskResponse := &models.TaskCallbackResponse{
				TaskGuid:      "the-task-guid",
				CreatedAt:     createdAt,
				Failed:        true,
				FailureReason: "because I said so",
				Result:        `{}`,
				Annotation: `{
					"lifecycle": "fake",
					"task_id": "the-task-id",
					"app_id": "the-app-id"
				}`,
			}

			handler.StagingComplete(responseRecorder, postTask(taskResponse))
		})

		It("posts the result to CC as an error", func() {
			Expect(fakeCCClient.StagingCompleteCallCount()).To(Equal(1))
			guid, payload, _ := fakeCCClient.StagingCompleteArgsForCall(0)
			Expect(guid).To(Equal("the-task-guid"))
			Expect(payload).To(Equal(backendResponseJson))
		})

		It("responds with a 200", func() {
			Expect(responseRecorder.Code).To(Equal(http.StatusOK))
		})

		It("increments the staging failed counter", func() {
			Expect(fakeCCClient.StagingCompleteCallCount()).To(Equal(1))
			Expect(metricSender.GetCounter("StagingRequestsFailed")).To(BeEquivalentTo(1))
		})

		It("emits the time it took to stage unsuccesfully", func() {
			Expect(metricSender.GetValue("StagingRequestFailedDuration")).To(Equal(fake.Metric{
				Value: 900900,
				Unit:  "nanos",
			}))

		})
	})

	Context("when a non-staging task is reported", func() {
		JustBeforeEach(func() {
			taskResponse := &models.TaskCallbackResponse{
				Failed:        true,
				FailureReason: "because I said so",
				Annotation:    `{}`,
				Result:        `{}`,
			}

			handler.StagingComplete(responseRecorder, postTask(taskResponse))
		})

		It("responds with a 404", func() {
			Expect(responseRecorder.Code).To(Equal(404))
		})
	})

	Context("when invalid JSON is posted instead of a task", func() {
		JustBeforeEach(func() {
			request, err := http.NewRequest("POST", "/v1/staging/an-invalid-guid/completed", strings.NewReader("{"))
			Expect(err).NotTo(HaveOccurred())

			handler.StagingComplete(responseRecorder, request)
		})

		It("responds with a 400", func() {
			Expect(responseRecorder.Code).To(Equal(400))
		})
	})
})
