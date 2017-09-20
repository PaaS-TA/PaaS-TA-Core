package backend_test

import (
	"encoding/json"
	"fmt"
	"time"

	"code.cloudfoundry.org/bbs/models"
	"code.cloudfoundry.org/dockerapplifecycle"
	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/lager/lagertest"
	"code.cloudfoundry.org/runtimeschema/cc_messages"
	"code.cloudfoundry.org/stager/backend"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("DockerBackend", func() {
	var (
		docker backend.Backend
		logger lager.Logger
		config backend.Config
	)

	BeforeEach(func() {
		config = backend.Config{
			TaskDomain:         "config-task-domain",
			StagerURL:          "http://staging-url.com",
			FileServerURL:      "http://file-server.com",
			CCUploaderURL:      "http://cc-uploader.com",
			DockerStagingStack: "penguin",
			InsecureDockerRegistries: []string{
				"http://registry-1.com",
				"http://registry-2.com",
			},
			Lifecycles: map[string]string{
				"penguin":                "penguin-compiler",
				"rabbit_hole":            "rabbit-hole-compiler",
				"compiler_with_full_url": "http://the-full-compiler-url",
				"compiler_with_bad_url":  "ftp://the-bad-compiler-url",
				"docker":                 "docker_lifecycle/docker_app_lifecycle.tgz",
			},
			Sanitizer: func(msg string) *cc_messages.StagingError {
				return &cc_messages.StagingError{Message: msg + " was totally sanitized"}
			},
		}

		logger = lagertest.NewTestLogger("test")
		docker = backend.NewDockerBackend(config, logger)
	})

	Describe("BuildBackend", func() {
		var (
			stagingRequest cc_messages.StagingRequestFromCC
			dockerImageUrl string
			appID          string
			dockerUser     string
			dockerPassword string
			dockerEmail    string
			memoryMb       int32
			diskMb         int32
			timeout        int
		)

		BeforeEach(func() {
			dockerImageUrl = "busybox"
			appID = "app-id"
			memoryMb = 2048
			diskMb = 3072
			timeout = 900
		})

		AfterEach(func() {
			dockerUser = ""
			dockerPassword = ""
			dockerEmail = ""
		})

		JustBeforeEach(func() {
			rawJsonBytes, err := json.Marshal(cc_messages.DockerStagingData{
				DockerImageUrl:    dockerImageUrl,
				DockerLoginServer: "",
				DockerUser:        dockerUser,
				DockerPassword:    dockerPassword,
				DockerEmail:       dockerEmail,
			})
			Expect(err).NotTo(HaveOccurred())

			lifecycleData := json.RawMessage(rawJsonBytes)
			stagingRequest = cc_messages.StagingRequestFromCC{
				AppId:           appID,
				LogGuid:         "log-guid",
				FileDescriptors: 512,
				MemoryMB:        int(memoryMb),
				DiskMB:          int(diskMb),
				Environment: []*models.EnvironmentVariable{
					{"VCAP_APPLICATION", "foo"},
					{"VCAP_SERVICES", "bar"},
				},
				EgressRules: []*models.SecurityGroupRule{
					{
						Protocol:     "TCP",
						Destinations: []string{"0.0.0.0/0"},
						PortRange:    &models.PortRange{Start: 80, End: 443},
					},
				},
				Timeout:            timeout,
				Lifecycle:          "docker",
				LifecycleData:      &lifecycleData,
				CompletionCallback: "https://api.cc.com/v1/staging/some-staging-guid/droplet_completed",
			}
		})

		It("returns the task domain", func() {
			_, _, domain, err := docker.BuildRecipe("staging-guid", stagingRequest)
			Expect(err).NotTo(HaveOccurred())
			Expect(domain).To(Equal("config-task-domain"))
		})

		It("returns the task guid", func() {
			_, guid, _, err := docker.BuildRecipe("staging-guid", stagingRequest)
			Expect(err).NotTo(HaveOccurred())
			Expect(guid).To(Equal("staging-guid"))
		})

		It("sets the task LogGuid", func() {
			taskDef, _, _, err := docker.BuildRecipe("staging-guid", stagingRequest)
			Expect(err).NotTo(HaveOccurred())
			Expect(taskDef.LogGuid).To(Equal("log-guid"))
		})

		It("sets the task LogSource", func() {
			taskDef, _, _, err := docker.BuildRecipe("staging-guid", stagingRequest)
			Expect(err).NotTo(HaveOccurred())
			Expect(taskDef.LogSource).To(Equal(backend.TaskLogSource))
		})

		It("sets the task ResultFile", func() {
			taskDef, _, _, err := docker.BuildRecipe("staging-guid", stagingRequest)
			Expect(err).NotTo(HaveOccurred())
			Expect(taskDef.ResultFile).To(Equal("/tmp/docker-result/result.json"))
		})

		It("sets the task Privileged as false by default", func() {
			taskDef, _, _, err := docker.BuildRecipe("staging-guid", stagingRequest)
			Expect(err).NotTo(HaveOccurred())
			Expect(taskDef.Privileged).To(BeFalse())
		})

		It("sets the LegacyDownloadUser", func() {
			taskDef, _, _, err := docker.BuildRecipe("staging-guid", stagingRequest)
			Expect(err).NotTo(HaveOccurred())
			Expect(taskDef.LegacyDownloadUser).To(Equal("vcap"))
		})

		It("sets the task Annotation", func() {
			taskDef, _, _, err := docker.BuildRecipe("staging-guid", stagingRequest)
			Expect(err).NotTo(HaveOccurred())

			var annotation cc_messages.StagingTaskAnnotation
			err = json.Unmarshal([]byte(taskDef.Annotation), &annotation)
			Expect(err).NotTo(HaveOccurred())

			Expect(annotation).To(Equal(cc_messages.StagingTaskAnnotation{
				Lifecycle:          "docker",
				CompletionCallback: "https://api.cc.com/v1/staging/some-staging-guid/droplet_completed",
			}))
		})

		It("sets the task CachedDependencies", func() {
			taskDef, _, _, err := docker.BuildRecipe("staging-guid", stagingRequest)
			Expect(err).NotTo(HaveOccurred())

			actions := actionsFromTaskDef(taskDef)
			Expect(actions).To(HaveLen(1))

			cachedDependencies := taskDef.CachedDependencies
			Expect(cachedDependencies).To(HaveLen(1))

			dockerCachedDependency := models.CachedDependency{
				From:     "http://file-server.com/v1/static/docker_lifecycle/docker_app_lifecycle.tgz",
				To:       "/tmp/docker_app_lifecycle",
				CacheKey: "docker-lifecycle",
			}

			Expect(*cachedDependencies[0]).To(Equal(dockerCachedDependency))
		})

		It("sets the task RunAction", func() {
			taskDef, _, _, err := docker.BuildRecipe("staging-guid", stagingRequest)
			Expect(err).NotTo(HaveOccurred())

			fileDescriptorLimit := uint64(512)
			runAction := models.EmitProgressFor(
				&models.RunAction{
					Path: "/tmp/docker_app_lifecycle/builder",
					Args: []string{
						"-outputMetadataJSONFilename", "/tmp/docker-result/result.json",
						"-dockerRef", "busybox",
						"-insecureDockerRegistries", "http://registry-1.com,http://registry-2.com",
					},
					Env: []*models.EnvironmentVariable{
						{
							Name:  "VCAP_APPLICATION",
							Value: "foo",
						},
						{
							Name:  "VCAP_SERVICES",
							Value: "bar",
						},
					},
					ResourceLimits: &models.ResourceLimits{
						Nofile: &fileDescriptorLimit,
					},
					User: "vcap",
				},
				"Staging...",
				"Staging Complete",
				"Staging Failed",
			)

			actions := actionsFromTaskDef(taskDef)
			Expect(actions).To(HaveLen(1))
			Expect(actions[0].GetEmitProgressAction()).To(Equal(runAction))
		})

		It("sets the task MemoryMb", func() {
			taskDef, _, _, err := docker.BuildRecipe("staging-guid", stagingRequest)
			Expect(err).NotTo(HaveOccurred())

			Expect(taskDef.MemoryMb).To(Equal(memoryMb))
		})

		It("sets the task DiskMb", func() {
			taskDef, _, _, err := docker.BuildRecipe("staging-guid", stagingRequest)
			Expect(err).NotTo(HaveOccurred())
			Expect(taskDef.DiskMb).To(Equal(diskMb))
		})

		It("sets the task EgressRules", func() {
			taskDef, _, _, err := docker.BuildRecipe("staging-guid", stagingRequest)
			Expect(err).NotTo(HaveOccurred())

			egressRules := []*models.SecurityGroupRule{
				{
					Protocol:     "TCP",
					Destinations: []string{"0.0.0.0/0"},
					PortRange:    &models.PortRange{Start: 80, End: 443},
				},
			}

			Expect(taskDef.EgressRules).To(Equal(egressRules))
		})

		It("sets the task RootFS to the configured Docker staging stack", func() {
			taskDef, _, _, err := docker.BuildRecipe("staging-guid", stagingRequest)
			Expect(err).NotTo(HaveOccurred())

			Expect(taskDef.RootFs).To(Equal(models.PreloadedRootFS("penguin")))
		})

		It("sets the task CompletionCallbackURL", func() {
			taskDef, _, _, err := docker.BuildRecipe("staging-guid", stagingRequest)
			Expect(err).NotTo(HaveOccurred())

			Expect(taskDef.CompletionCallbackUrl).To(Equal(fmt.Sprintf("%s/v1/staging/%s/completed", "http://staging-url.com", "staging-guid")))
		})

		It("sets the task TrustedSystemCertificatesPath", func() {
			taskDef, _, _, err := docker.BuildRecipe("staging-guid", stagingRequest)
			Expect(err).NotTo(HaveOccurred())

			Expect(taskDef.TrustedSystemCertificatesPath).To(Equal(backend.TrustedSystemCertificatesPath))
		})

		Context("with a missing app id", func() {
			BeforeEach(func() {
				appID = ""
			})

			It("returns an error", func() {
				_, _, _, err := docker.BuildRecipe("staging-guid", stagingRequest)
				Expect(err).To(Equal(backend.ErrMissingAppId))
			})
		})

		Context("with a missing docker image url", func() {
			BeforeEach(func() {
				dockerImageUrl = ""
			})

			It("returns an error", func() {
				_, _, _, err := docker.BuildRecipe("staging-guid", stagingRequest)
				Expect(err).To(Equal(backend.ErrMissingDockerImageUrl))
			})
		})

		Context("with password and email but no user", func() {
			BeforeEach(func() {
				dockerPassword = "password"
				dockerEmail = "email@example.com"
			})

			It("returns an error", func() {
				_, _, _, err := docker.BuildRecipe("staging-guid", stagingRequest)
				Expect(err).To(Equal(backend.ErrMissingDockerCredentials))
			})
		})

		Context("with user and email but no password", func() {
			BeforeEach(func() {
				dockerUser = "user"
				dockerEmail = "email@example.com"
			})

			It("returns an error", func() {
				_, _, _, err := docker.BuildRecipe("staging-guid", stagingRequest)
				Expect(err).To(Equal(backend.ErrMissingDockerCredentials))
			})
		})

		Context("with user and password but no email", func() {
			BeforeEach(func() {
				dockerUser = "user"
				dockerPassword = "password"
			})

			It("returns an error", func() {
				_, _, _, err := docker.BuildRecipe("staging-guid", stagingRequest)
				Expect(err).To(Equal(backend.ErrMissingDockerCredentials))
			})
		})

		Context("when the docker lifecycle is missing", func() {
			BeforeEach(func() {
				delete(config.Lifecycles, "docker")
			})

			It("returns an error", func() {
				_, _, _, err := docker.BuildRecipe("staging-guid", stagingRequest)
				Expect(err).To(Equal(backend.ErrNoCompilerDefined))
			})
		})

		Context("when the docker lifecycle is empty", func() {
			BeforeEach(func() {
				config.Lifecycles["docker"] = ""
			})

			It("returns an error", func() {
				_, _, _, err := docker.BuildRecipe("staging-guid", stagingRequest)
				Expect(err).To(Equal(backend.ErrNoCompilerDefined))
			})
		})

		Context("when a positive timeout is specified in the staging request from CC", func() {
			BeforeEach(func() {
				timeout = 5
			})

			It("passes the timeout along", func() {
				taskDef, _, _, err := docker.BuildRecipe("staging-guid", stagingRequest)
				Expect(err).NotTo(HaveOccurred())

				timeoutAction := taskDef.Action.GetTimeoutAction()
				Expect(timeoutAction).NotTo(BeNil())
				Expect(timeoutAction.TimeoutMs).To(Equal(int64(time.Duration(timeout) * time.Second / 1000000)))
			})
		})

		Context("when a 0 timeout is specified in the staging request from CC", func() {
			BeforeEach(func() {
				timeout = 0
			})

			It("uses the default timeout", func() {
				taskDef, _, _, err := docker.BuildRecipe("staging-guid", stagingRequest)
				Expect(err).NotTo(HaveOccurred())

				timeoutAction := taskDef.Action.GetTimeoutAction()
				Expect(timeoutAction).NotTo(BeNil())
				Expect(timeoutAction.TimeoutMs).To(Equal(int64(backend.DefaultStagingTimeout / 1000000)))
			})
		})

		Context("when a negative timeout is specified in the staging request from CC", func() {
			BeforeEach(func() {
				timeout = -3
			})

			It("uses the default timeout", func() {
				taskDef, _, _, err := docker.BuildRecipe("staging-guid", stagingRequest)
				Expect(err).NotTo(HaveOccurred())

				timeoutAction := taskDef.Action.GetTimeoutAction()
				Expect(timeoutAction).NotTo(BeNil())
				Expect(timeoutAction.TimeoutMs).To(Equal(int64(backend.DefaultStagingTimeout / 1000000)))
			})
		})
	})

	Describe("BuildStagingResponse", func() {
		var (
			response          cc_messages.StagingResponseForCC
			failureReason     string
			buildError        error
			stagingResultJson []byte
			stagingResult     dockerapplifecycle.StagingResult
		)

		BeforeEach(func() {
			stagingResult = dockerapplifecycle.NewStagingResult(
				dockerapplifecycle.ProcessTypes{"a": "b"},
				dockerapplifecycle.LifecycleMetadata{
					DockerImage: "cloudfoundry/diego-docker-app",
				},
				"metadata",
			)
			var err error
			stagingResultJson, err = json.Marshal(stagingResult)
			Expect(err).NotTo(HaveOccurred())
		})

		Context("with a successful task response", func() {
			BeforeEach(func() {
				taskResponse := &models.TaskCallbackResponse{
					Failed:        false,
					FailureReason: failureReason,
					Result:        string(stagingResultJson),
				}

				response, buildError = docker.BuildStagingResponse(taskResponse)
				Expect(buildError).NotTo(HaveOccurred())
			})

			It("populates a staging response correctly", func() {
				result := json.RawMessage(stagingResultJson)
				Expect(response).To(Equal(cc_messages.StagingResponseForCC{
					Result: &result,
				}))
			})

			Context("with a failed task response", func() {
				BeforeEach(func() {
					taskResponse := &models.TaskCallbackResponse{
						Failed:        true,
						FailureReason: "some-failure-reason",
						Result:        string(stagingResultJson),
					}

					response, buildError = docker.BuildStagingResponse(taskResponse)
					Expect(buildError).NotTo(HaveOccurred())
				})

				It("populates a staging response correctly", func() {
					Expect(response).To(Equal(cc_messages.StagingResponseForCC{
						Error: &cc_messages.StagingError{Message: "some-failure-reason was totally sanitized"},
					}))
				})
			})
		})
	})
})
