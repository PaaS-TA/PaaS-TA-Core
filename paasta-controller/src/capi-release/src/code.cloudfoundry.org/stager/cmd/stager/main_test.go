package main_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"strconv"
	"strings"

	"code.cloudfoundry.org/bbs/models"
	"code.cloudfoundry.org/bbs/models/test/model_helpers"
	"code.cloudfoundry.org/buildpackapplifecycle"
	"code.cloudfoundry.org/runtimeschema/cc_messages/flags"
	"code.cloudfoundry.org/stager"
	"code.cloudfoundry.org/stager/cmd/stager/testrunner"
	"code.cloudfoundry.org/stager/diego_errors"
	"github.com/gogo/protobuf/proto"
	"github.com/hashicorp/consul/api"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/gexec"
	"github.com/onsi/gomega/ghttp"
	"github.com/tedsuo/rata"
)

var _ = Describe("Stager", func() {
	var (
		fakeBBS *ghttp.Server
		fakeCC  *ghttp.Server

		requestGenerator *rata.RequestGenerator
		httpClient       *http.Client
		stagerPort       int

		callbackURL string
	)

	BeforeEach(func() {
		stagerPort = 8888 + GinkgoParallelNode()
		listenAddress := fmt.Sprintf("127.0.0.1:%d", stagerPort)
		stagerURL := fmt.Sprintf("http://%s", listenAddress)
		callbackURL = stagerURL + "/v1/staging/my-task-guid/completed"

		fakeBBS = ghttp.NewServer()
		fakeCC = ghttp.NewServer()

		runner = testrunner.New(testrunner.Config{
			StagerBin:          stagerPath,
			ListenAddress:      listenAddress,
			TaskCallbackURL:    stagerURL,
			BBSURL:             fakeBBS.URL(),
			CCBaseURL:          fakeCC.URL(),
			DockerStagingStack: "docker-staging-stack",
			ConsulCluster:      consulRunner.URL(),
		})

		requestGenerator = rata.NewRequestGenerator(stagerURL, stager.Routes)
		httpClient = http.DefaultClient
	})

	AfterEach(func() {
		runner.Stop()
	})

	Context("when started", func() {
		BeforeEach(func() {
			runner.Start(
				"-lifecycle", "buildpack/linux:lifecycle.zip",
				"-lifecycle", "docker:docker/lifecycle.tgz",
			)
			Eventually(runner.Session()).Should(gbytes.Say("Listening for staging requests!"))
		})

		Describe("when a buildpack staging request is received", func() {
			It("desires a staging task via the API", func() {
				fakeBBS.RouteToHandler("POST", "/v1/tasks/desire.r2", func(w http.ResponseWriter, req *http.Request) {
					var desireTaskRequest models.DesireTaskRequest
					data, err := ioutil.ReadAll(req.Body)
					Expect(err).NotTo(HaveOccurred())

					err = desireTaskRequest.Unmarshal(data)
					Expect(err).NotTo(HaveOccurred())

					Expect(desireTaskRequest.TaskDefinition.MemoryMb).To(Equal(int32(1024)))
					Expect(desireTaskRequest.TaskDefinition.DiskMb).To(Equal(int32(128)))
					Expect(desireTaskRequest.TaskDefinition.CompletionCallbackUrl).To(Equal(callbackURL))
				})

				req, err := requestGenerator.CreateRequest(stager.StageRoute, rata.Params{"staging_guid": "my-task-guid"}, strings.NewReader(`{
					"app_id":"my-app-guid",
					"file_descriptors":3,
					"memory_mb" : 1024,
					"disk_mb" : 128,
					"environment" : [],
					"lifecycle": "buildpack",
					"lifecycle_data": {
					  "buildpacks" : [],
						"stack":"linux",
					  "app_bits_download_uri":"http://example.com/app_bits"
					}
				}`))
				Expect(err).NotTo(HaveOccurred())
				req.Header.Set("Content-Type", "application/json")

				resp, err := httpClient.Do(req)
				Expect(err).NotTo(HaveOccurred())
				Expect(resp.StatusCode).To(Equal(http.StatusAccepted))

				Eventually(fakeBBS.ReceivedRequests).Should(HaveLen(1))
				Consistently(runner.Session()).ShouldNot(gexec.Exit())
			})
		})

		Describe("when a docker staging request is received", func() {
			It("desires a staging task via the API", func() {
				fakeBBS.RouteToHandler("POST", "/v1/tasks/desire.r2", func(w http.ResponseWriter, req *http.Request) {
					var desireTaskRequest models.DesireTaskRequest
					data, err := ioutil.ReadAll(req.Body)
					Expect(err).NotTo(HaveOccurred())

					err = desireTaskRequest.Unmarshal(data)
					Expect(err).NotTo(HaveOccurred())

					Expect(desireTaskRequest.TaskDefinition.MemoryMb).To(Equal(int32(1024)))
					Expect(desireTaskRequest.TaskDefinition.DiskMb).To(Equal(int32(128)))
					Expect(desireTaskRequest.TaskDefinition.CompletionCallbackUrl).To(Equal(callbackURL))
				})

				req, err := requestGenerator.CreateRequest(stager.StageRoute, rata.Params{"staging_guid": "my-task-guid"}, strings.NewReader(`{
					"app_id":"my-app-guid",
					"file_descriptors":3,
					"memory_mb" : 1024,
					"disk_mb" : 128,
					"environment" : [],
					"lifecycle": "docker",
					"lifecycle_data": {
					  "docker_image":"http://docker.docker/docker"
					}
				}`))
				Expect(err).NotTo(HaveOccurred())
				req.Header.Set("Content-Type", "application/json")

				resp, err := httpClient.Do(req)
				Expect(err).NotTo(HaveOccurred())
				Expect(resp.StatusCode).To(Equal(http.StatusAccepted))

				Eventually(fakeBBS.ReceivedRequests).Should(HaveLen(1))
				Consistently(runner.Session()).ShouldNot(gexec.Exit())
			})
		})

		Describe("when a stop staging request is recevied", func() {
			BeforeEach(func() {
				taskDef := model_helpers.NewValidTaskDefinition()
				taskDef.Annotation = `{"lifecycle": "whatever"}`
				task := &models.Task{
					TaskDefinition: taskDef,
					TaskGuid:       "the-task-guid",
				}
				taskResponse := models.TaskResponse{
					Task:  task,
					Error: nil,
				}

				fakeBBS.RouteToHandler("POST", "/v1/tasks/get_by_task_guid.r2", func(w http.ResponseWriter, req *http.Request) {
					var taskByGuidRequest models.TaskByGuidRequest
					data, err := ioutil.ReadAll(req.Body)
					Expect(err).NotTo(HaveOccurred())

					err = taskByGuidRequest.Unmarshal(data)
					Expect(err).NotTo(HaveOccurred())

					Expect(taskByGuidRequest.TaskGuid).To(Equal("the-task-guid"))
					writeResponse(w, &taskResponse)
				})

				fakeBBS.RouteToHandler("POST", "/v1/tasks/cancel", func(w http.ResponseWriter, req *http.Request) {
					var taskGuidRequest models.TaskByGuidRequest
					data, err := ioutil.ReadAll(req.Body)
					Expect(err).NotTo(HaveOccurred())

					err = taskGuidRequest.Unmarshal(data)
					Expect(err).NotTo(HaveOccurred())

					Expect(taskGuidRequest.TaskGuid).To(Equal("the-task-guid"))
				})
			})

			It("cancels the staging task via the API", func() {
				req, err := requestGenerator.CreateRequest(stager.StopStagingRoute, rata.Params{"staging_guid": "the-task-guid"}, nil)
				Expect(err).NotTo(HaveOccurred())
				req.Header.Set("Content-Type", "application/json")

				resp, err := httpClient.Do(req)
				Eventually(fakeBBS.ReceivedRequests).Should(HaveLen(2))
				Expect(err).NotTo(HaveOccurred())
				Expect(resp.StatusCode).To(Equal(http.StatusAccepted))

				Eventually(fakeBBS.ReceivedRequests).Should(HaveLen(2))
				Consistently(runner.Session()).ShouldNot(gexec.Exit())
			})
		})

		Describe("when a staging task completes", func() {
			var (
				taskJSON []byte
				err      error
			)

			JustBeforeEach(func() {
				req, err := requestGenerator.CreateRequest(stager.StagingCompletedRoute, rata.Params{"staging_guid": "the-task-guid"}, bytes.NewReader(taskJSON))
				Expect(err).NotTo(HaveOccurred())

				req.Header.Set("Content-Type", "application/json")

				resp, err := httpClient.Do(req)
				Expect(err).NotTo(HaveOccurred())
				Expect(resp.StatusCode).To(Equal(http.StatusOK))
			})

			Context("successfully", func() {
				Context("for a docker lifecycle", func() {
					BeforeEach(func() {
						fakeCC.AppendHandlers(
							ghttp.CombineHandlers(
								ghttp.VerifyRequest("POST", "/internal/staging/the-task-guid/completed"),
								ghttp.VerifyContentType("application/json"),
								ghttp.VerifyJSON(`{
								"result": {
									"execution_metadata": "metadata",
									"process_types": {"a": "b"},
									"lifecycle_metadata": {
										"docker_image": "http://docker.docker/docker"
									}
								}
							}`),
							),
						)

						taskJSON, err = json.Marshal(&models.TaskCallbackResponse{
							TaskGuid: "the-task-guid",
							Failed:   false,
							Annotation: `{
							"lifecycle": "docker"
						}`,
							Result: `{
							"execution_metadata": "metadata",
							"process_types": {"a": "b"},
							"lifecycle_metadata": {
								"docker_image": "http://docker.docker/docker"
							}
						}`,
						})
						Expect(err).NotTo(HaveOccurred())
					})

					It("POSTs to the CC that staging is complete", func() {
						Eventually(fakeCC.ReceivedRequests).Should(HaveLen(1))
					})
				})

				Context("for a buildpack lifecycle", func() {
					BeforeEach(func() {
						fakeCC.AppendHandlers(
							ghttp.CombineHandlers(
								ghttp.VerifyRequest("POST", "/internal/staging/the-task-guid/completed"),
								ghttp.VerifyContentType("application/json"),
								ghttp.VerifyJSON(`{
								"result": {
									"process_types": {"a": "b"},
									"lifecycle_metadata": {
										"buildpack_key": "buildpack-key",
										"detected_buildpack": "detected-buildpack"
									},
									"execution_metadata": "metadata"
								}
							}`),
							),
						)

						taskJSON, err = json.Marshal(models.TaskCallbackResponse{
							TaskGuid: "the-task-guid",
							Annotation: `{
							"lifecycle": "buildpack"
						}`,
							Result: `{
							"process_types": {"a": "b"},
							"lifecycle_metadata": {
								"buildpack_key": "buildpack-key",
								"detected_buildpack": "detected-buildpack"
							},
							"execution_metadata": "metadata"
						}`,
						})
						Expect(err).NotTo(HaveOccurred())

					})

					It("POSTs to the CC that staging is complete", func() {
						Eventually(fakeCC.ReceivedRequests).Should(HaveLen(1))
					})
				})
			})

			Context("when staging returns with 'insufficient resources'", func() {
				BeforeEach(func() {
					taskJSON, err = json.Marshal(models.TaskCallbackResponse{
						TaskGuid: "the-task-guid",
						Annotation: `{
							"lifecycle": "buildpack"
						}`,
						Failed:        true,
						FailureReason: diego_errors.INSUFFICIENT_RESOURCES_MESSAGE,
					})

					fakeCC.AppendHandlers(
						ghttp.CombineHandlers(
							ghttp.VerifyRequest("POST", "/internal/staging/the-task-guid/completed"),
							ghttp.VerifyContentType("application/json"),
							ghttp.VerifyJSON(`{
								"error": {
									"id": "InsufficientResources",
									"message": "insufficient resources"
								}
							}`),
						),
					)
				})

				It("POSTs to CC that staging fails", func() {
					Eventually(fakeCC.ReceivedRequests).Should(HaveLen(1))
				})
			})

			Context("when buildpack detection fails", func() {
				BeforeEach(func() {
					taskJSON, err = json.Marshal(models.TaskCallbackResponse{
						TaskGuid: "the-task-guid",
						Annotation: `{
							"lifecycle": "buildpack"
						}`,
						Failed:        true,
						FailureReason: "nope " + strconv.Itoa(buildpackapplifecycle.DETECT_FAIL_CODE),
					})

					fakeCC.AppendHandlers(
						ghttp.CombineHandlers(
							ghttp.VerifyRequest("POST", "/internal/staging/the-task-guid/completed"),
							ghttp.VerifyContentType("application/json"),
							ghttp.VerifyJSON(`{
								"error": {
									"id": "NoAppDetectedError",
									"message": "staging failed"
								}
							}`),
						),
					)
				})

				It("POSTs to CC that detection failed", func() {
					Eventually(fakeCC.ReceivedRequests).Should(HaveLen(1))
				})
			})

			Context("when buildpack compile fails", func() {
				BeforeEach(func() {
					taskJSON, err = json.Marshal(models.TaskCallbackResponse{
						TaskGuid: "the-task-guid",
						Annotation: `{
							"lifecycle": "buildpack"
						}`,
						Failed:        true,
						FailureReason: "nope " + strconv.Itoa(buildpackapplifecycle.COMPILE_FAIL_CODE),
					})

					fakeCC.AppendHandlers(
						ghttp.CombineHandlers(
							ghttp.VerifyRequest("POST", "/internal/staging/the-task-guid/completed"),
							ghttp.VerifyContentType("application/json"),
							ghttp.VerifyJSON(`{
								"error": {
									"id": "BuildpackCompileFailed",
									"message": "staging failed"
								}
							}`),
						),
					)
				})

				It("POSTs to CC that compile failed", func() {
					Eventually(fakeCC.ReceivedRequests).Should(HaveLen(1))
				})
			})

			Context("when buildpack release fails", func() {
				BeforeEach(func() {
					taskJSON, err = json.Marshal(models.TaskCallbackResponse{
						TaskGuid: "the-task-guid",
						Annotation: `{
							"lifecycle": "buildpack"
						}`,
						Failed:        true,
						FailureReason: "nope " + strconv.Itoa(buildpackapplifecycle.RELEASE_FAIL_CODE),
					})

					fakeCC.AppendHandlers(
						ghttp.CombineHandlers(
							ghttp.VerifyRequest("POST", "/internal/staging/the-task-guid/completed"),
							ghttp.VerifyContentType("application/json"),
							ghttp.VerifyJSON(`{
								"error": {
									"id": "BuildpackReleaseFailed",
									"message": "staging failed"
								}
							}`),
						),
					)
				})

				It("POSTs to CC that release failed", func() {
					Eventually(fakeCC.ReceivedRequests).Should(HaveLen(1))
				})
			})
		})
	})

	Context("when started with -insecureDockerRegistry", func() {
		BeforeEach(func() {
			runner.Start("-lifecycle", "linux:lifecycle.zip", "-insecureDockerRegistry", "http://b.c", "-insecureDockerRegistry", "http://a.b")
			Eventually(runner.Session()).Should(gbytes.Say("Listening for staging requests!"))
		})

		It("starts successfully", func() {
			Consistently(runner.Session()).ShouldNot(gexec.Exit())
		})
	})

	Describe("service registration", func() {
		BeforeEach(func() {
			runner.Start("-lifecycle", "linux:lifecycle.zip")
			Eventually(runner.Session()).Should(gbytes.Say("Listening for staging requests!"))
		})

		It("registers itself with consul", func() {
			client := consulRunner.NewClient()
			services, err := client.Agent().Services()

			Expect(err).NotTo(HaveOccurred())

			Expect(services).Should(HaveKeyWithValue("stager",
				&api.AgentService{
					ID:      "stager",
					Service: "stager",
					Port:    stagerPort,
				}))
		})

		It("registers a TTL healthcheck", func() {
			client := consulRunner.NewClient()
			checks, err := client.Agent().Checks()

			Expect(err).NotTo(HaveOccurred())

			Expect(checks).Should(HaveKeyWithValue("service:stager",
				&api.AgentCheck{
					Node:        "0",
					CheckID:     "service:stager",
					Name:        "Service 'stager' check",
					Status:      "passing",
					ServiceID:   "stager",
					ServiceName: "stager",
				}))
		})
	})

	Describe("-consulCluster arg", func() {
		Context("when started with an invalid -consulCluster arg", func() {
			BeforeEach(func() {
				runner.Config.ConsulCluster = "://noscheme:8500"
				runner.Start("-lifecycle", "linux:lifecycle.zip")
			})

			It("logs and errors", func() {
				Eventually(runner.Session().ExitCode()).ShouldNot(Equal(0))
				Eventually(runner.Session()).Should(gbytes.Say("Error parsing consul agent URL"))
			})
		})
	})

	Describe("-dockerRegistryAddress arg", func() {
		Context("when started with a valid -dockerRegistryAddress arg", func() {
			BeforeEach(func() {
				runner.Start("-lifecycle", "linux:lifecycle.zip",
					"-dockerRegistryAddress", "docker-registry.service.cf.internal:8080")
				Eventually(runner.Session()).Should(gbytes.Say("Listening for staging requests!"))
			})

			It("starts successfully", func() {
				Consistently(runner.Session()).ShouldNot(gexec.Exit())
			})
		})

		Context("when started with an invalid -dockerRegistryAddress arg", func() {
			BeforeEach(func() {
				runner.Start("-lifecycle", "linux:lifecycle.zip",
					"-dockerRegistryAddress", "://noscheme:8500")
			})

			It("logs and errors", func() {
				Eventually(runner.Session().ExitCode()).ShouldNot(Equal(0))
				Eventually(runner.Session()).Should(gbytes.Say("Error parsing Docker Registry address"))
			})
		})
	})

	Describe("-lifecycles arg", func() {
		Context("when started with an invalid -lifecycles arg", func() {
			BeforeEach(func() {
				runner.Start("-lifecycle", "invalid form")
			})

			It("logs and errors", func() {
				Eventually(runner.Session().ExitCode()).ShouldNot(Equal(0))
				Eventually(runner.Session().Err).Should(gbytes.Say(flags.ErrLifecycleFormatInvalid.Error()))
			})
		})
	})

	Describe("-stagingTaskCallbackURL arg", func() {
		Context("when started with an invalid -stagingTaskCallbackURL arg", func() {
			BeforeEach(func() {
				runner.Start("-stagingTaskCallbackURL", `://localhost:8080`)
			})

			It("logs and errors", func() {
				Eventually(runner.Session().ExitCode()).ShouldNot(Equal(0))
				Eventually(runner.Session()).Should(gbytes.Say("Invalid staging task callback url"))
			})
		})
	})

	Describe("-listenAddress arg", func() {
		Context("when started with an invalid -listenAddress arg with no :", func() {
			BeforeEach(func() {
				runner.Config.ListenAddress = "portless"
				runner.Start()
			})

			It("logs and errors", func() {
				Eventually(runner.Session().ExitCode()).ShouldNot(Equal(0))
				Eventually(runner.Session()).Should(gbytes.Say("missing port in address"))
			})
		})

		Context("when started with an invalid -listenAddress arg with invalid port", func() {
			BeforeEach(func() {
				runner.Config.ListenAddress = "127.0.0.1:onehundred"
				runner.Start()
			})

			It("logs and errors", func() {
				Eventually(runner.Session().ExitCode()).ShouldNot(Equal(0))
				Eventually(runner.Session()).Should(gbytes.Say("stager.failed-invalid-listen-port"))
			})
		})
	})

})

func writeResponse(w http.ResponseWriter, message proto.Message) {
	responseBytes, err := proto.Marshal(message)
	if err != nil {
		panic("Unable to encode Proto: " + err.Error())
	}

	w.Header().Set("Content-Length", strconv.Itoa(len(responseBytes)))
	w.Header().Set("Content-Type", "application/x-protobuf")
	w.WriteHeader(http.StatusOK)

	w.Write(responseBytes)
}
