package backend_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"code.cloudfoundry.org/bbs/models"
	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/runtimeschema/cc_messages"
	"code.cloudfoundry.org/stager/backend"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/ghttp"
)

var _ = Describe("DockerBackend", func() {
	const (
		dockerRegistryPort = uint32(8080)
		dockerRegistryHost = "docker-registry.service.cf.internal"
	)

	var (
		validDockerRegistryAddress = fmt.Sprintf("%s:%d", dockerRegistryHost, dockerRegistryPort)
	)

	newConsulCluster := func(ips []string) *ghttp.Server {
		server := ghttp.NewServer()
		type service struct {
			Address string
		}
		services := []service{}
		for i, _ := range ips {
			services = append(services, service{Address: ips[i]})
		}

		payload, err := json.Marshal(services)
		Expect(err).NotTo(HaveOccurred())

		server.AppendHandlers(
			ghttp.CombineHandlers(
				ghttp.VerifyRequest("GET", "/v1/catalog/service/docker-registry"),
				http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
					w.Write([]byte(payload))
				}),
			),
		)

		return server
	}

	setupDockerBackend := func(
		dockerRegistryAddress string,
		insecureDockerRegistries []string,
		consulCluster *ghttp.Server,
	) backend.Backend {
		config := backend.Config{
			FileServerURL:            "http://file-server.com",
			CCUploaderURL:            "http://cc-uploader.com",
			ConsulCluster:            consulCluster.URL(),
			DockerRegistryAddress:    dockerRegistryAddress,
			InsecureDockerRegistries: insecureDockerRegistries,
			Lifecycles: map[string]string{
				"docker": "docker_lifecycle/docker_app_lifecycle.tgz",
			},
		}

		logger := lager.NewLogger("fakelogger")
		logger.RegisterSink(lager.NewWriterSink(GinkgoWriter, lager.DEBUG))

		return backend.NewDockerBackend(config, logger)
	}

	setupStagingRequest := func(dockerImageCachingEnabled bool, loginServer, user, password, email string) cc_messages.StagingRequestFromCC {
		rawJsonBytes, err := json.Marshal(cc_messages.DockerStagingData{
			DockerImageUrl:    "busybox",
			DockerLoginServer: loginServer,
			DockerUser:        user,
			DockerPassword:    password,
			DockerEmail:       email,
		})
		Expect(err).NotTo(HaveOccurred())
		lifecycleData := json.RawMessage(rawJsonBytes)

		Expect(err).NotTo(HaveOccurred())

		stagingRequest := cc_messages.StagingRequestFromCC{
			AppId:           "bunny",
			FileDescriptors: 512,
			MemoryMB:        512,
			DiskMB:          512,
			Timeout:         512,
			LifecycleData:   &lifecycleData,
			EgressRules: []*models.SecurityGroupRule{
				{
					Protocol:     "TCP",
					Destinations: []string{"0.0.0.0/0"},
					PortRange:    &models.PortRange{Start: 80, End: 443},
				},
			},
		}

		if dockerImageCachingEnabled {
			stagingRequest.Environment = append(stagingRequest.Environment, &models.EnvironmentVariable{
				Name:  "DIEGO_DOCKER_CACHE",
				Value: "true",
			})
		}

		return stagingRequest
	}

	Context("when docker registry is running", func() {
		var dockerCachedDependency = models.CachedDependency{
			From:     "http://file-server.com/v1/static/docker_lifecycle/docker_app_lifecycle.tgz",
			To:       "/tmp/docker_app_lifecycle",
			CacheKey: "docker-lifecycle",
		}

		fileDescriptorLimit := uint64(512)
		var mountCgroupsAction = models.EmitProgressFor(
			&models.RunAction{
				Path: "/tmp/docker_app_lifecycle/mount_cgroups",
				ResourceLimits: &models.ResourceLimits{
					Nofile: &fileDescriptorLimit,
				},
				User: "root",
			},
			"Preparing docker daemon...",
			"",
			"Failed to set up docker environment",
		)

		var (
			dockerBackend     backend.Backend
			dockerRegistryIPs []string
			consulCluster     *ghttp.Server
			stagingRequest    cc_messages.StagingRequestFromCC
		)

		BeforeEach(func() {
			dockerRegistryIPs = []string{"10.244.2.6", "10.244.2.7"}
			consulCluster = newConsulCluster(dockerRegistryIPs)
		})

		Context("user did not opt-in for docker image caching", func() {
			BeforeEach(func() {
				dockerBackend = setupDockerBackend(validDockerRegistryAddress, []string{}, consulCluster)
				stagingRequest = setupStagingRequest(false, "", "", "", "")
			})

			It("creates a cf-app-docker-staging Task with no additional egress rules", func() {
				taskDef, _, _, err := dockerBackend.BuildRecipe("staging-guid", stagingRequest)
				Expect(err).NotTo(HaveOccurred())
				Expect(taskDef.EgressRules).To(Equal(stagingRequest.EgressRules))
			})
		})

		Context("user opted-in for docker image caching", func() {
			BeforeEach(func() {
				stagingRequest = setupStagingRequest(true, "", "", "", "")
			})

			Context("when an invalid docker registry address is given", func() {
				BeforeEach(func() {
					dockerBackend = setupDockerBackend("://host:", []string{}, consulCluster)
				})

				It("returns an error", func() {
					_, _, _, err := dockerBackend.BuildRecipe("staging-guid", stagingRequest)
					Expect(err).To(Equal(backend.ErrInvalidDockerRegistryAddress))
				})
			})

			Context("and Docker Registry is secure", func() {
				BeforeEach(func() {
					dockerBackend = setupDockerBackend(validDockerRegistryAddress, []string{}, consulCluster)
				})

				It("runs as unprivileged", func() {
					taskDef, _, _, err := dockerBackend.BuildRecipe("staging-guid", stagingRequest)
					Expect(err).NotTo(HaveOccurred())

					Expect(taskDef.Privileged).To(BeFalse())
				})

				It("has an Action", func() {
					taskDef, _, _, err := dockerBackend.BuildRecipe("staging-guid", stagingRequest)
					Expect(err).NotTo(HaveOccurred())

					Expect(taskDef.Action).NotTo(BeNil())
				})

				It("has expected EgressRules", func() {
					taskDef, _, _, err := dockerBackend.BuildRecipe("staging-guid", stagingRequest)
					Expect(err).NotTo(HaveOccurred())

					expectedEgressRules := []*models.SecurityGroupRule{}
					for i, _ := range stagingRequest.EgressRules {
						expectedEgressRules = append(expectedEgressRules, stagingRequest.EgressRules[i])
					}

					for _, ip := range dockerRegistryIPs {
						expectedEgressRules = append(expectedEgressRules, &models.SecurityGroupRule{
							Protocol:     models.TCPProtocol,
							Destinations: []string{ip},
							Ports:        []uint32{dockerRegistryPort},
						})
					}

					Expect(taskDef.EgressRules).To(Equal(expectedEgressRules))
				})

				It("includes the expected Docker DownloadAction", func() {
					taskDef, _, _, err := dockerBackend.BuildRecipe("staging-guid", stagingRequest)
					Expect(err).NotTo(HaveOccurred())

					cachedDependencies := taskDef.CachedDependencies
					Expect(cachedDependencies).To(HaveLen(1))
					Expect(*cachedDependencies[0]).To(Equal(dockerCachedDependency))
				})

				It("includes mounting of the cgroups", func() {
					taskDef, _, _, err := dockerBackend.BuildRecipe("staging-guid", stagingRequest)
					Expect(err).NotTo(HaveOccurred())

					actions := actionsFromTaskDef(taskDef)
					Expect(actions).To(HaveLen(2))
					Expect(actions[0].GetEmitProgressAction()).To(Equal(mountCgroupsAction))
				})

				It("includes the expected Run action", func() {
					taskDef, _, _, err := dockerBackend.BuildRecipe("staging-guid", stagingRequest)
					Expect(err).NotTo(HaveOccurred())

					actions := actionsFromTaskDef(taskDef)
					Expect(actions).To(HaveLen(2))

					fileDescriptorLimit := uint64(512)
					internalRunAction := models.RunAction{
						Path: "/tmp/docker_app_lifecycle/builder",
						Args: []string{
							"-outputMetadataJSONFilename", "/tmp/docker-result/result.json",
							"-dockerRef", "busybox",
							"-cacheDockerImage",
							"-dockerRegistryHost", dockerRegistryHost,
							"-dockerRegistryPort", fmt.Sprintf("%d", dockerRegistryPort),
							"-dockerRegistryIPs", strings.Join(dockerRegistryIPs, ","),
						},
						Env: []*models.EnvironmentVariable{
							&models.EnvironmentVariable{Name: "DIEGO_DOCKER_CACHE", Value: "true"},
						},
						ResourceLimits: &models.ResourceLimits{
							Nofile: &fileDescriptorLimit,
						},
						User: "root",
					}
					expectedRunAction := models.EmitProgressFor(
						&internalRunAction,
						"Staging...",
						"Staging Complete",
						"Staging Failed",
					)

					Expect(actions[1].GetEmitProgressAction()).To(Equal(expectedRunAction))
				})
			})

			Context("and Docker Registry is insecure", func() {
				BeforeEach(func() {
					dockerBackend = setupDockerBackend(validDockerRegistryAddress, []string{validDockerRegistryAddress, "http://insecure-registry.com"}, consulCluster)
				})

				It("runs as unprivileged", func() {
					taskDef, _, _, err := dockerBackend.BuildRecipe("staging-guid", stagingRequest)
					Expect(err).NotTo(HaveOccurred())

					Expect(taskDef.Privileged).To(BeFalse())
				})

				It("has an Action", func() {
					taskDef, _, _, err := dockerBackend.BuildRecipe("staging-guid", stagingRequest)
					Expect(err).NotTo(HaveOccurred())

					Expect(taskDef.Action).NotTo(BeNil())
				})

				It("has expected EgressRules", func() {
					taskDef, _, _, err := dockerBackend.BuildRecipe("staging-guid", stagingRequest)
					Expect(err).NotTo(HaveOccurred())

					expectedEgressRules := []*models.SecurityGroupRule{}
					for i, _ := range stagingRequest.EgressRules {
						expectedEgressRules = append(expectedEgressRules, stagingRequest.EgressRules[i])
					}

					for _, ip := range dockerRegistryIPs {
						expectedEgressRules = append(expectedEgressRules, &models.SecurityGroupRule{
							Protocol:     models.TCPProtocol,
							Destinations: []string{ip},
							Ports:        []uint32{dockerRegistryPort},
						})
					}

					Expect(taskDef.EgressRules).To(Equal(expectedEgressRules))
				})

				It("includes the expected Docker DownloadAction", func() {
					taskDef, _, _, err := dockerBackend.BuildRecipe("staging-guid", stagingRequest)
					Expect(err).NotTo(HaveOccurred())

					cachedDependencies := taskDef.CachedDependencies
					Expect(cachedDependencies).To(HaveLen(1))
					Expect(*cachedDependencies[0]).To(Equal(dockerCachedDependency))
				})

				It("includes mounting of the cgroups", func() {
					taskDef, _, _, err := dockerBackend.BuildRecipe("staging-guid", stagingRequest)
					Expect(err).NotTo(HaveOccurred())

					actions := actionsFromTaskDef(taskDef)
					Expect(actions).To(HaveLen(2))
					Expect(actions[0].GetEmitProgressAction()).To(Equal(mountCgroupsAction))
				})

				It("includes the expected Run action", func() {
					taskDef, _, _, err := dockerBackend.BuildRecipe("staging-guid", stagingRequest)
					Expect(err).NotTo(HaveOccurred())

					actions := actionsFromTaskDef(taskDef)
					Expect(actions).To(HaveLen(2))

					fileDescriptorLimit := uint64(512)
					internalRunAction := models.RunAction{
						Path: "/tmp/docker_app_lifecycle/builder",
						Args: []string{
							"-outputMetadataJSONFilename", "/tmp/docker-result/result.json",
							"-dockerRef", "busybox",
							"-insecureDockerRegistries", strings.Join([]string{validDockerRegistryAddress, "http://insecure-registry.com"}, ","),
							"-cacheDockerImage",
							"-dockerRegistryHost", dockerRegistryHost,
							"-dockerRegistryPort", fmt.Sprintf("%d", dockerRegistryPort),
							"-dockerRegistryIPs", strings.Join(dockerRegistryIPs, ","),
						},
						Env: []*models.EnvironmentVariable{
							&models.EnvironmentVariable{Name: "DIEGO_DOCKER_CACHE", Value: "true"},
						},
						ResourceLimits: &models.ResourceLimits{
							Nofile: &fileDescriptorLimit,
						},
						User: "root",
					}
					expectedRunAction := models.EmitProgressFor(
						&internalRunAction,
						"Staging...",
						"Staging Complete",
						"Staging Failed",
					)

					Expect(actions[1].GetEmitProgressAction()).To(Equal(expectedRunAction))
				})
			})

			Context("and credentials are provided", func() {
				var (
					loginServer = "http://loginServer.com"
					user        = "user"
					password    = "password"
					email       = "email@example.com"
				)

				BeforeEach(func() {
					stagingRequest = setupStagingRequest(true, loginServer, user, password, email)
					dockerBackend = setupDockerBackend(validDockerRegistryAddress, []string{}, consulCluster)
				})

				It("runs as unprivileged", func() {
					taskDef, _, _, err := dockerBackend.BuildRecipe("staging-guid", stagingRequest)
					Expect(err).NotTo(HaveOccurred())

					Expect(taskDef.Privileged).To(BeFalse())
				})

				It("has an Action", func() {
					taskDef, _, _, err := dockerBackend.BuildRecipe("staging-guid", stagingRequest)
					Expect(err).NotTo(HaveOccurred())

					Expect(taskDef.Action).NotTo(BeNil())
				})

				It("has expected EgressRules", func() {
					taskDef, _, _, err := dockerBackend.BuildRecipe("staging-guid", stagingRequest)
					Expect(err).NotTo(HaveOccurred())

					expectedEgressRules := []*models.SecurityGroupRule{}
					for i, _ := range stagingRequest.EgressRules {
						expectedEgressRules = append(expectedEgressRules, stagingRequest.EgressRules[i])
					}

					for _, ip := range dockerRegistryIPs {
						expectedEgressRules = append(expectedEgressRules, &models.SecurityGroupRule{
							Protocol:     models.TCPProtocol,
							Destinations: []string{ip},
							Ports:        []uint32{dockerRegistryPort},
						})
					}

					Expect(taskDef.EgressRules).To(Equal(expectedEgressRules))
				})

				It("includes the expected Docker DownloadAction", func() {
					taskDef, _, _, err := dockerBackend.BuildRecipe("staging-guid", stagingRequest)
					Expect(err).NotTo(HaveOccurred())

					cachedDependencies := taskDef.CachedDependencies
					Expect(cachedDependencies).To(HaveLen(1))
					Expect(*cachedDependencies[0]).To(Equal(dockerCachedDependency))
				})

				It("includes mounting of the cgroups", func() {
					taskDef, _, _, err := dockerBackend.BuildRecipe("staging-guid", stagingRequest)
					Expect(err).NotTo(HaveOccurred())

					actions := actionsFromTaskDef(taskDef)
					Expect(actions).To(HaveLen(2))
					Expect(actions[0].GetEmitProgressAction()).To(Equal(mountCgroupsAction))
				})

				It("includes the expected Run action", func() {
					taskDef, _, _, err := dockerBackend.BuildRecipe("staging-guid", stagingRequest)
					Expect(err).NotTo(HaveOccurred())

					actions := actionsFromTaskDef(taskDef)
					Expect(actions).To(HaveLen(2))

					fileDescriptorLimit := uint64(512)
					internalRunAction := models.RunAction{
						Path: "/tmp/docker_app_lifecycle/builder",
						Args: []string{
							"-outputMetadataJSONFilename", "/tmp/docker-result/result.json",
							"-dockerRef", "busybox",
							"-cacheDockerImage",
							"-dockerRegistryHost", dockerRegistryHost,
							"-dockerRegistryPort", fmt.Sprintf("%d", dockerRegistryPort),
							"-dockerRegistryIPs", strings.Join(dockerRegistryIPs, ","),
							"-dockerLoginServer", loginServer,
							"-dockerUser", user,
							"-dockerPassword", password,
							"-dockerEmail", email,
						},
						Env: []*models.EnvironmentVariable{
							&models.EnvironmentVariable{Name: "DIEGO_DOCKER_CACHE", Value: "true"},
						},
						ResourceLimits: &models.ResourceLimits{
							Nofile: &fileDescriptorLimit,
						},
						User: "root",
					}
					expectedRunAction := models.EmitProgressFor(
						&internalRunAction,
						"Staging...",
						"Staging Complete",
						"Staging Failed",
					)
					Expect(actions[1].GetEmitProgressAction()).To(Equal(expectedRunAction))
				})
			})
		})
	})

	Context("when Docker Registry is not running", func() {
		var (
			docker         backend.Backend
			stagingRequest cc_messages.StagingRequestFromCC
		)

		BeforeEach(func() {
			docker = setupDockerBackend(validDockerRegistryAddress, []string{}, newConsulCluster([]string{}))
		})

		Context("and user opted-in for docker image caching", func() {
			BeforeEach(func() {
				stagingRequest = setupStagingRequest(true, "", "", "", "")
			})

			It("errors", func() {
				_, _, _, err := docker.BuildRecipe("staging-guid", stagingRequest)
				Expect(err).To(HaveOccurred())
				Expect(err).To(Equal(backend.ErrMissingDockerRegistry))
			})
		})

		Context("and user did not opt-in for docker image caching", func() {
			BeforeEach(func() {
				stagingRequest = setupStagingRequest(false, "", "", "", "")
			})

			It("does not error", func() {
				_, _, _, err := docker.BuildRecipe("staging-guid", stagingRequest)
				Expect(err).NotTo(HaveOccurred())
			})
		})
	})
})
