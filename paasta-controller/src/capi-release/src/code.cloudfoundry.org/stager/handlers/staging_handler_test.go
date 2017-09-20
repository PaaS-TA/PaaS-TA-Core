package handlers_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"

	"code.cloudfoundry.org/bbs/fake_bbs"
	"code.cloudfoundry.org/bbs/models"
	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/lager/lagertest"
	"code.cloudfoundry.org/runtimeschema/cc_messages"
	"code.cloudfoundry.org/stager/backend"
	"code.cloudfoundry.org/stager/backend/fake_backend"
	"code.cloudfoundry.org/stager/handlers"
	fake_metric_sender "github.com/cloudfoundry/dropsonde/metric_sender/fake"
	"github.com/cloudfoundry/dropsonde/metrics"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
)

var _ = Describe("StagingHandler", func() {

	var (
		fakeMetricSender *fake_metric_sender.FakeMetricSender

		logger          lager.Logger
		fakeDiegoClient *fake_bbs.FakeClient
		fakeBackend     *fake_backend.FakeBackend

		responseRecorder *httptest.ResponseRecorder
		handler          handlers.StagingHandler
	)

	BeforeEach(func() {
		logger = lagertest.NewTestLogger("test")

		fakeMetricSender = fake_metric_sender.NewFakeMetricSender()
		metrics.Initialize(fakeMetricSender, nil)

		fakeBackend = &fake_backend.FakeBackend{}
		fakeBackend.BuildRecipeReturns(&models.TaskDefinition{}, "", "", nil)

		fakeDiegoClient = &fake_bbs.FakeClient{}

		responseRecorder = httptest.NewRecorder()
		handler = handlers.NewStagingHandler(logger, map[string]backend.Backend{"fake-backend": fakeBackend}, fakeDiegoClient)
	})

	Describe("Stage", func() {
		var (
			stagingRequestJson []byte
		)

		JustBeforeEach(func() {
			req, err := http.NewRequest("PUT", "/v1/staging/a-staging-guid", bytes.NewReader(stagingRequestJson))
			Expect(err).NotTo(HaveOccurred())

			req.Form = url.Values{":staging_guid": {"a-staging-guid"}}

			handler.Stage(responseRecorder, req)
		})

		Context("when a staging request is received for a registered backend", func() {
			var stagingRequest cc_messages.StagingRequestFromCC

			BeforeEach(func() {
				stagingRequest = cc_messages.StagingRequestFromCC{
					AppId:     "myapp",
					Lifecycle: "fake-backend",
				}

				var err error
				stagingRequestJson, err = json.Marshal(stagingRequest)
				Expect(err).NotTo(HaveOccurred())
			})

			It("increments the counter to track arriving staging messages", func() {
				Expect(fakeMetricSender.GetCounter("StagingStartRequestsReceived")).To(Equal(uint64(1)))
			})

			It("returns an Accepted response", func() {
				Expect(responseRecorder.Code).To(Equal(http.StatusAccepted))
			})

			It("builds a staging recipe", func() {
				Expect(fakeBackend.BuildRecipeCallCount()).To(Equal(1))

				guid, request := fakeBackend.BuildRecipeArgsForCall(0)
				Expect(guid).To(Equal("a-staging-guid"))
				Expect(request).To(Equal(stagingRequest))
			})

			Context("when the recipe was built successfully", func() {
				var fakeTaskDef = &models.TaskDefinition{Annotation: "test annotation"}
				BeforeEach(func() {
					fakeBackend.BuildRecipeReturns(fakeTaskDef, "a-guid", "a-domain", nil)
				})

				It("creates a task on Diego", func() {
					Expect(fakeDiegoClient.DesireTaskCallCount()).To(Equal(1))
					_, _, _, resultingTaskDef := fakeDiegoClient.DesireTaskArgsForCall(0)
					Expect(resultingTaskDef).To(Equal(fakeTaskDef))
				})

				Context("when the task has already been created", func() {
					BeforeEach(func() {
						fakeDiegoClient.DesireTaskReturns(models.NewError(models.Error_ResourceExists, "ok, this task already exists"))
					})

					It("does not log a failure", func() {
						Expect(logger).NotTo(gbytes.Say("staging-failed"))
					})
				})

				Context("create task fails for any other reason", func() {
					var desireError error

					BeforeEach(func() {
						desireError = errors.New("some task create error")
						fakeDiegoClient.DesireTaskReturns(desireError)
					})

					It("logs the failure", func() {
						Expect(logger).To(gbytes.Say("staging-failed"))
					})

					It("returns an internal service error status code", func() {
						Expect(responseRecorder.Code).To(Equal(http.StatusInternalServerError))
					})

					Context("when the response builder succeeds", func() {
						var responseForCC cc_messages.StagingResponseForCC

						BeforeEach(func() {
							responseForCC = cc_messages.StagingResponseForCC{
								Error: backend.SanitizeErrorMessage(desireError.Error()),
							}
						})

						It("returns the cloud controller error response", func() {
							var response cc_messages.StagingResponseForCC
							decoder := json.NewDecoder(responseRecorder.Body)
							err := decoder.Decode(&response)
							Expect(err).NotTo(HaveOccurred())

							Expect(response).To(Equal(responseForCC))
						})
					})
				})
			})

			Context("when the recipe failed to be built", func() {
				var buildRecipeError error

				BeforeEach(func() {
					buildRecipeError = errors.New("some build recipe error")
					fakeBackend.BuildRecipeReturns(&models.TaskDefinition{}, "", "", buildRecipeError)
				})

				It("logs the failure", func() {
					Expect(logger).To(gbytes.Say("recipe-building-failed"))
				})

				It("returns an internal service error status code", func() {
					Expect(responseRecorder.Code).To(Equal(http.StatusInternalServerError))
				})

				Context("when the response builder succeeds", func() {
					var responseForCC cc_messages.StagingResponseForCC

					BeforeEach(func() {
						responseForCC = cc_messages.StagingResponseForCC{
							Error: backend.SanitizeErrorMessage(buildRecipeError.Error()),
						}
					})

					It("returns the cloud controller error response", func() {
						var response cc_messages.StagingResponseForCC
						decoder := json.NewDecoder(responseRecorder.Body)
						err := decoder.Decode(&response)
						Expect(err).NotTo(HaveOccurred())

						Expect(response).To(Equal(responseForCC))
					})
				})
			})
		})

		Describe("bad requests", func() {
			Context("when the request fails to unmarshal", func() {
				BeforeEach(func() {
					stagingRequestJson = []byte(`bad-json`)
				})

				It("returns bad request", func() {
					Expect(responseRecorder.Code).To(Equal(http.StatusBadRequest))
				})
			})

			Context("when a staging request is received for an unknown backend", func() {
				BeforeEach(func() {
					stagingRequest := cc_messages.StagingRequestFromCC{
						AppId:     "myapp",
						Lifecycle: "unknown-backend",
					}

					var err error
					stagingRequestJson, err = json.Marshal(stagingRequest)
					Expect(err).NotTo(HaveOccurred())
				})

				It("returns a Not Found response", func() {
					Expect(responseRecorder.Code).To(Equal(http.StatusNotFound))
				})
			})

			Context("when a malformed staging request is received", func() {
				BeforeEach(func() {
					stagingRequestJson = []byte(`bogus-request`)
				})

				It("returns a BadRequest error", func() {
					Expect(responseRecorder.Code).To(Equal(http.StatusBadRequest))
				})
			})
		})
	})

	Describe("StopStaging", func() {
		BeforeEach(func() {
			stagingTask := &models.Task{
				TaskGuid:       "a-staging-guid",
				TaskDefinition: &models.TaskDefinition{Annotation: `{"lifecycle": "fake-backend"}`},
			}

			fakeDiegoClient.TaskByGuidReturns(stagingTask, nil)
		})

		JustBeforeEach(func() {
			req, err := http.NewRequest("POST", "/v1/staging/a-staging-guid", nil)
			Expect(err).NotTo(HaveOccurred())

			req.Form = url.Values{":staging_guid": {"a-staging-guid"}}

			handler.StopStaging(responseRecorder, req)
		})

		Context("when receiving a stop staging request", func() {
			It("retrieves the current staging task by guid", func() {
				Expect(fakeDiegoClient.TaskByGuidCallCount()).To(Equal(1))
				_, task := fakeDiegoClient.TaskByGuidArgsForCall(0)
				Expect(task).To(Equal("a-staging-guid"))
			})

			Context("when an in-flight staging task is not found", func() {
				BeforeEach(func() {
					fakeDiegoClient.TaskByGuidReturns(&models.Task{}, models.ErrResourceNotFound)
				})

				It("returns StatusNotFound", func() {
					Expect(responseRecorder.Code).To(Equal(http.StatusNotFound))
				})
			})

			Context("when retrieving the current task fails", func() {
				BeforeEach(func() {
					fakeDiegoClient.TaskByGuidReturns(&models.Task{}, errors.New("boom"))
				})

				It("returns StatusInternalServerError", func() {
					Expect(responseRecorder.Code).To(Equal(http.StatusInternalServerError))
				})
			})

			Context("when retrieving the current task is sucessful", func() {
				Context("when the task annotation fails to unmarshal", func() {
					BeforeEach(func() {
						stagingTask := &models.Task{
							TaskGuid:       "a-staging-guid",
							TaskDefinition: &models.TaskDefinition{Annotation: `"fake-backend"}`},
						}

						fakeDiegoClient.TaskByGuidReturns(stagingTask, nil)
					})

					It("returns StatusInternalServerError", func() {
						Expect(responseRecorder.Code).To(Equal(http.StatusInternalServerError))
					})
				})

				It("increments the counter to track arriving stop staging messages", func() {
					Expect(fakeMetricSender.GetCounter("StagingStopRequestsReceived")).To(Equal(uint64(1)))
				})

				It("cancels the Diego task", func() {
					Expect(fakeDiegoClient.CancelTaskCallCount()).To(Equal(1))
					_, task := fakeDiegoClient.CancelTaskArgsForCall(0)
					Expect(task).To(Equal("a-staging-guid"))
				})

				It("returns an Accepted response", func() {
					Expect(responseRecorder.Code).To(Equal(http.StatusAccepted))
				})

			})
		})
	})
})
