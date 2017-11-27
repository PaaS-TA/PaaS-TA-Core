package containerstore_test

import (
	"bytes"
	"errors"
	"fmt"
	"io/ioutil"
	"net"
	"os"
	"sync"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/tedsuo/ifrit"
	"github.com/tedsuo/ifrit/ginkgomon"

	"code.cloudfoundry.org/bbs/models"
	"code.cloudfoundry.org/clock/fakeclock"
	"code.cloudfoundry.org/executor"
	"code.cloudfoundry.org/executor/depot/containerstore"
	"code.cloudfoundry.org/executor/depot/containerstore/containerstorefakes"
	"code.cloudfoundry.org/executor/depot/transformer/faketransformer"
	"code.cloudfoundry.org/garden"
	mfakes "code.cloudfoundry.org/go-loggregator/testhelpers/fakes/v1"
	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/volman"
	"code.cloudfoundry.org/volman/volmanfakes"

	eventfakes "code.cloudfoundry.org/executor/depot/event/fakes"
	"code.cloudfoundry.org/garden/gardenfakes"
	"code.cloudfoundry.org/garden/server"
)

var _ = Describe("Container Store", func() {
	var (
		containerConfig containerstore.ContainerConfig
		containerStore  containerstore.ContainerStore

		iNodeLimit    uint64
		maxCPUShares  uint64
		ownerName     string
		totalCapacity executor.ExecutorResources

		containerGuid string

		metricMap     map[string]struct{}
		metricMapLock sync.RWMutex

		gardenClient      *gardenfakes.FakeClient
		gardenContainer   *gardenfakes.FakeContainer
		megatron          *faketransformer.FakeTransformer
		dependencyManager *containerstorefakes.FakeDependencyManager
		credManager       *containerstorefakes.FakeCredManager
		volumeManager     *volmanfakes.FakeManager

		clock            *fakeclock.FakeClock
		eventEmitter     *eventfakes.FakeHub
		fakeMetronClient *mfakes.FakeIngressClient
	)

	var pollForComplete = func(guid string) func() bool {
		return func() bool {
			container, err := containerStore.Get(logger, guid)
			Expect(err).NotTo(HaveOccurred())
			return container.State == executor.StateCompleted
		}
	}

	var pollForRunning = func(guid string) func() bool {
		return func() bool {
			container, err := containerStore.Get(logger, guid)
			Expect(err).NotTo(HaveOccurred())
			return container.State == executor.StateRunning
		}
	}

	getMetrics := func() map[string]struct{} {
		metricMapLock.Lock()
		defer metricMapLock.Unlock()
		m := make(map[string]struct{}, len(metricMap))
		for k, v := range metricMap {
			m[k] = v
		}
		return m
	}

	BeforeEach(func() {
		metricMap = map[string]struct{}{}
		gardenContainer = &gardenfakes.FakeContainer{}
		gardenClient = &gardenfakes.FakeClient{}
		dependencyManager = &containerstorefakes.FakeDependencyManager{}
		credManager = &containerstorefakes.FakeCredManager{}
		volumeManager = &volmanfakes.FakeManager{}
		clock = fakeclock.NewFakeClock(time.Now())
		eventEmitter = &eventfakes.FakeHub{}

		credManager.RunnerReturns(ifrit.RunFunc(func(signals <-chan os.Signal, ready chan<- struct{}) error {
			close(ready)
			<-signals
			return nil
		}))

		iNodeLimit = 64
		maxCPUShares = 100
		ownerName = "test-owner"
		totalCapacity = executor.NewExecutorResources(1024*10, 1024*10, 10)

		containerGuid = "container-guid"

		megatron = &faketransformer.FakeTransformer{}

		fakeMetronClient = new(mfakes.FakeIngressClient)

		containerConfig = containerstore.ContainerConfig{
			OwnerName:              ownerName,
			INodeLimit:             iNodeLimit,
			MaxCPUShares:           maxCPUShares,
			ReapInterval:           20 * time.Millisecond,
			ReservedExpirationTime: 20 * time.Millisecond,
		}

		containerStore = containerstore.New(
			containerConfig,
			&totalCapacity,
			gardenClient,
			dependencyManager,
			volumeManager,
			credManager,
			clock,
			eventEmitter,
			megatron,
			"/var/vcap/data/cf-system-trusted-certs",
			fakeMetronClient,
			"/var/vcap/packages/healthcheck",
		)

		fakeMetronClient.SendDurationStub = func(name string, value time.Duration) error {
			metricMapLock.Lock()
			defer metricMapLock.Unlock()
			metricMap[name] = struct{}{}
			return nil
		}
	})

	Describe("Reserve", func() {
		var (
			containerTags     executor.Tags
			containerResource executor.Resource
			req               *executor.AllocationRequest
		)

		BeforeEach(func() {
			containerTags = executor.Tags{
				"Foo": "bar",
			}
			containerResource = executor.Resource{
				MemoryMB:   1024,
				DiskMB:     1024,
				RootFSPath: "/foo/bar",
			}

			req = &executor.AllocationRequest{
				Guid:     containerGuid,
				Tags:     containerTags,
				Resource: containerResource,
			}
		})

		It("returns a populated container", func() {
			container, err := containerStore.Reserve(logger, req)
			Expect(err).NotTo(HaveOccurred())

			Expect(container.Guid).To(Equal(containerGuid))
			Expect(container.Tags).To(Equal(containerTags))
			Expect(container.Resource).To(Equal(containerResource))
			Expect(container.State).To(Equal(executor.StateReserved))
			Expect(container.AllocatedAt).To(Equal(clock.Now().UnixNano()))
		})

		It("tracks the container", func() {
			container, err := containerStore.Reserve(logger, req)
			Expect(err).NotTo(HaveOccurred())

			found, err := containerStore.Get(logger, container.Guid)
			Expect(err).NotTo(HaveOccurred())
			Expect(found).To(Equal(container))
		})

		It("emits a reserved container event", func() {
			container, err := containerStore.Reserve(logger, req)
			Expect(err).NotTo(HaveOccurred())

			Eventually(eventEmitter.EmitCallCount).Should(Equal(1))

			event := eventEmitter.EmitArgsForCall(0)
			Expect(event).To(Equal(executor.ContainerReservedEvent{
				RawContainer: container,
			}))
		})

		It("decrements the remaining capacity", func() {
			_, err := containerStore.Reserve(logger, req)
			Expect(err).NotTo(HaveOccurred())

			remainingCapacity := containerStore.RemainingResources(logger)
			Expect(remainingCapacity.MemoryMB).To(Equal(totalCapacity.MemoryMB - req.MemoryMB))
			Expect(remainingCapacity.DiskMB).To(Equal(totalCapacity.DiskMB - req.DiskMB))
			Expect(remainingCapacity.Containers).To(Equal(totalCapacity.Containers - 1))
		})

		Context("when the container guid is already reserved", func() {
			BeforeEach(func() {
				_, err := containerStore.Reserve(logger, req)
				Expect(err).NotTo(HaveOccurred())
			})

			It("fails with container guid not available", func() {
				_, err := containerStore.Reserve(logger, req)
				Expect(err).To(Equal(executor.ErrContainerGuidNotAvailable))
			})
		})

		Context("when there are not enough remaining resources available", func() {
			BeforeEach(func() {
				req.Resource.MemoryMB = totalCapacity.MemoryMB + 1
			})

			It("returns an error", func() {
				_, err := containerStore.Reserve(logger, req)
				Expect(err).To(Equal(executor.ErrInsufficientResourcesAvailable))
			})
		})
	})

	Describe("Initialize", func() {
		var (
			req     *executor.RunRequest
			runInfo executor.RunInfo
			runTags executor.Tags
		)

		BeforeEach(func() {
			runInfo = executor.RunInfo{
				CPUWeight:      2,
				StartTimeoutMs: 50000,
				Privileged:     true,
			}

			runTags = executor.Tags{
				"Beep": "Boop",
			}

			req = &executor.RunRequest{
				Guid:    containerGuid,
				RunInfo: runInfo,
				Tags:    runTags,
			}
		})

		Context("when the container has not been reserved", func() {
			It("returns a container not found error", func() {
				err := containerStore.Initialize(logger, req)
				Expect(err).To(Equal(executor.ErrContainerNotFound))
			})
		})

		Context("when the conatiner is reserved", func() {
			BeforeEach(func() {
				allocationReq := &executor.AllocationRequest{
					Guid: containerGuid,
					Tags: executor.Tags{},
				}

				_, err := containerStore.Reserve(logger, allocationReq)
				Expect(err).NotTo(HaveOccurred())
			})

			It("populates the container with info from the run request", func() {
				err := containerStore.Initialize(logger, req)
				Expect(err).NotTo(HaveOccurred())

				container, err := containerStore.Get(logger, req.Guid)
				Expect(err).NotTo(HaveOccurred())
				Expect(container.State).To(Equal(executor.StateInitializing))
				Expect(container.RunInfo).To(Equal(runInfo))
				Expect(container.Tags).To(Equal(runTags))
			})
		})

		Context("when the container exists but is not reserved", func() {
			BeforeEach(func() {
				allocationReq := &executor.AllocationRequest{
					Guid: containerGuid,
					Tags: executor.Tags{},
				}

				_, err := containerStore.Reserve(logger, allocationReq)
				Expect(err).NotTo(HaveOccurred())

				err = containerStore.Initialize(logger, req)
				Expect(err).NotTo(HaveOccurred())
			})

			It("returns an invalid state tranistion error", func() {
				err := containerStore.Initialize(logger, req)
				Expect(err).To(Equal(executor.ErrInvalidTransition))
			})
		})
	})

	Describe("Create", func() {
		var (
			resource      executor.Resource
			allocationReq *executor.AllocationRequest
		)

		BeforeEach(func() {
			resource = executor.Resource{
				MemoryMB:   1024,
				DiskMB:     1024,
				MaxPids:    1024,
				RootFSPath: "/foo/bar",
			}

			allocationReq = &executor.AllocationRequest{
				Guid: containerGuid,
				Tags: executor.Tags{
					"Foo": "Bar",
				},
				Resource: resource,
			}
		})

		Context("when the container is initializing", func() {
			var (
				externalIP, internalIP string
				runReq                 *executor.RunRequest
			)

			BeforeEach(func() {
				externalIP = "6.6.6.6"
				internalIP = "7.7.7.7"
				env := []executor.EnvironmentVariable{
					{Name: "foo", Value: "bar"},
					{Name: "beep", Value: "booop"},
				}

				runInfo := executor.RunInfo{
					Privileged:     true,
					CPUWeight:      50,
					StartTimeoutMs: 99000,
					CachedDependencies: []executor.CachedDependency{
						{Name: "artifact", From: "https://example.com", To: "/etc/foo", CacheKey: "abc", LogSource: "source"},
					},
					LogConfig: executor.LogConfig{
						Guid:       "log-guid",
						Index:      1,
						SourceName: "test-source",
					},
					MetricsConfig: executor.MetricsConfig{
						Guid:  "metric-guid",
						Index: 1,
					},
					Env: env,
					TrustedSystemCertificatesPath: "",
					Network: &executor.Network{
						Properties: map[string]string{
							"some-key":       "some-value",
							"some-other-key": "some-other-value",
						},
					},
				}

				runReq = &executor.RunRequest{
					Guid:    containerGuid,
					RunInfo: runInfo,
				}

				gardenContainer.InfoReturns(garden.ContainerInfo{ExternalIP: externalIP, ContainerIP: internalIP}, nil)
				gardenClient.CreateReturns(gardenContainer, nil)
			})

			JustBeforeEach(func() {
				_, err := containerStore.Reserve(logger, allocationReq)
				Expect(err).NotTo(HaveOccurred())

				err = containerStore.Initialize(logger, runReq)
				Expect(err).NotTo(HaveOccurred())
			})

			It("sets the container state to created", func() {
				_, err := containerStore.Create(logger, containerGuid)
				Expect(err).NotTo(HaveOccurred())

				container, err := containerStore.Get(logger, containerGuid)
				Expect(err).NotTo(HaveOccurred())
				Expect(container.State).To(Equal(executor.StateCreated))
			})

			It("creates the container in garden with correct image parameters", func() {
				_, err := containerStore.Create(logger, containerGuid)
				Expect(err).NotTo(HaveOccurred())

				Expect(gardenClient.CreateCallCount()).To(Equal(1))
				containerSpec := gardenClient.CreateArgsForCall(0)
				Expect(containerSpec.Handle).To(Equal(containerGuid))
				Expect(containerSpec.Image.URI).To(Equal(resource.RootFSPath))
				Expect(containerSpec.Privileged).To(Equal(true))
			})

			Context("when setting image credentials", func() {
				BeforeEach(func() {
					runReq.RunInfo.ImageUsername = "some-username"
					runReq.RunInfo.ImagePassword = "some-password"
				})

				It("creates the container in garden with correct image credentials", func() {
					_, err := containerStore.Create(logger, containerGuid)
					Expect(err).NotTo(HaveOccurred())

					Expect(gardenClient.CreateCallCount()).To(Equal(1))
					containerSpec := gardenClient.CreateArgsForCall(0)
					Expect(containerSpec.Image.URI).To(Equal(resource.RootFSPath))
					Expect(containerSpec.Image.Username).To(Equal("some-username"))
					Expect(containerSpec.Image.Password).To(Equal("some-password"))
				})
			})

			It("creates the container in garden with the correct limits", func() {
				_, err := containerStore.Create(logger, containerGuid)
				Expect(err).NotTo(HaveOccurred())

				Expect(gardenClient.CreateCallCount()).To(Equal(1))
				containerSpec := gardenClient.CreateArgsForCall(0)
				Expect(containerSpec.Limits.Memory.LimitInBytes).To(BeEquivalentTo(resource.MemoryMB * 1024 * 1024))

				Expect(containerSpec.Limits.Disk.Scope).To(Equal(garden.DiskLimitScopeExclusive))
				Expect(containerSpec.Limits.Disk.ByteHard).To(BeEquivalentTo(resource.DiskMB * 1024 * 1024))
				Expect(containerSpec.Limits.Disk.InodeHard).To(Equal(iNodeLimit))

				Expect(int(containerSpec.Limits.Pid.Max)).To(Equal(resource.MaxPids))

				expectedCPUShares := uint64(float64(maxCPUShares) * float64(runReq.CPUWeight) / 100.0)
				Expect(containerSpec.Limits.CPU.LimitInShares).To(Equal(expectedCPUShares))
			})

			It("downloads the correct cache dependencies", func() {
				_, err := containerStore.Create(logger, containerGuid)
				Expect(err).NotTo(HaveOccurred())
				Expect(dependencyManager.DownloadCachedDependenciesCallCount()).To(Equal(1))
				_, mounts, _ := dependencyManager.DownloadCachedDependenciesArgsForCall(0)
				Expect(mounts).To(Equal(runReq.CachedDependencies))
			})

			It("creates the container in garden with the correct limits", func() {
				expectedMount := garden.BindMount{
					SrcPath: "foo",
					DstPath: "/etc/foo",
					Mode:    garden.BindMountModeRO,
					Origin:  garden.BindMountOriginHost,
				}
				dependencyManager.DownloadCachedDependenciesReturns(containerstore.BindMounts{
					GardenBindMounts: []garden.BindMount{expectedMount},
				}, nil)

				_, err := containerStore.Create(logger, containerGuid)
				Expect(err).NotTo(HaveOccurred())

				Expect(gardenClient.CreateCallCount()).To(Equal(1))
				containerSpec := gardenClient.CreateArgsForCall(0)
				Expect(containerSpec.BindMounts).To(ContainElement(expectedMount))
			})

			It("creates the container with the correct properties", func() {
				_, err := containerStore.Create(logger, containerGuid)
				Expect(err).NotTo(HaveOccurred())

				Expect(gardenClient.CreateCallCount()).To(Equal(1))
				containerSpec := gardenClient.CreateArgsForCall(0)

				Expect(containerSpec.Properties).To(Equal(garden.Properties{
					containerstore.ContainerOwnerProperty: ownerName,
					"network.some-key":                    "some-value",
					"network.some-other-key":              "some-other-value",
				}))
			})

			Context("if the network is not set", func() {
				BeforeEach(func() {
					runReq.RunInfo.Network = nil
				})
				It("sets the owner property", func() {
					_, err := containerStore.Create(logger, containerGuid)
					Expect(err).NotTo(HaveOccurred())

					containerSpec := gardenClient.CreateArgsForCall(0)

					Expect(containerSpec.Properties).To(Equal(garden.Properties{
						containerstore.ContainerOwnerProperty: ownerName,
					}))
				})
			})

			It("creates the container with the correct environment", func() {
				_, err := containerStore.Create(logger, containerGuid)
				Expect(err).NotTo(HaveOccurred())

				Expect(gardenClient.CreateCallCount()).To(Equal(1))
				containerSpec := gardenClient.CreateArgsForCall(0)

				expectedEnv := []string{}
				for _, envVar := range runReq.Env {
					expectedEnv = append(expectedEnv, envVar.Name+"="+envVar.Value)
				}
				Expect(containerSpec.Env).To(Equal(expectedEnv))
			})

			It("sets the correct external and internal ip", func() {
				container, err := containerStore.Create(logger, containerGuid)
				Expect(err).NotTo(HaveOccurred())
				Expect(container.ExternalIP).To(Equal(externalIP))
				Expect(container.InternalIP).To(Equal(internalIP))
			})

			It("emits metrics after creating the container", func() {
				_, err := containerStore.Create(logger, containerGuid)
				Expect(err).NotTo(HaveOccurred())
				Eventually(getMetrics).Should(HaveKey(containerstore.GardenContainerCreationDuration))
				Eventually(getMetrics).Should(HaveKey(containerstore.GardenContainerCreationSucceededDuration))
			})

			It("sends a log after creating the container", func() {
				_, err := containerStore.Create(logger, containerGuid)
				Expect(err).NotTo(HaveOccurred())
				Eventually(fakeMetronClient.SendAppLogCallCount()).Should(Equal(2))
				Eventually(func() string {
					_, msg, _, _ := fakeMetronClient.SendAppLogArgsForCall(1)
					return msg
				}).Should(ContainSubstring("Successfully created container"))
			})

			It("generates container credential directory", func() {
				_, err := containerStore.Create(logger, containerGuid)
				Expect(err).NotTo(HaveOccurred())

				Expect(credManager.CreateCredDirCallCount()).To(Equal(1))
				_, container := credManager.CreateCredDirArgsForCall(0)
				Expect(container.Guid).To(Equal(containerGuid))
			})

			It("bind mounts the healthcheck", func() {
				_, err := containerStore.Create(logger, containerGuid)
				Expect(err).NotTo(HaveOccurred())

				Expect(gardenClient.CreateCallCount()).To(Equal(1))
				containerSpec := gardenClient.CreateArgsForCall(0)
				Expect(containerSpec.BindMounts).To(ContainElement(garden.BindMount{
					SrcPath: "/var/vcap/packages/healthcheck",
					DstPath: "/etc/cf-assets/healthcheck",
					Mode:    garden.BindMountModeRO,
					Origin:  garden.BindMountOriginHost,
				}))
			})

			Context("when credential mounts are configured", func() {
				var (
					expectedBindMount garden.BindMount
				)

				BeforeEach(func() {
					expectedBindMount = garden.BindMount{SrcPath: "hpath1", DstPath: "cpath1", Mode: garden.BindMountModeRO, Origin: garden.BindMountOriginHost}
					envVariables := []executor.EnvironmentVariable{
						{Name: "CF_INSTANCE_CERT", Value: "some-cert"},
					}
					credManager.CreateCredDirReturns([]garden.BindMount{expectedBindMount}, envVariables, nil)
				})

				It("mounts the credential directory into the container", func() {
					_, err := containerStore.Create(logger, containerGuid)
					Expect(err).NotTo(HaveOccurred())
					Expect(gardenClient.CreateCallCount()).To(Equal(1))
					Expect(gardenClient.CreateArgsForCall(0).BindMounts).To(ContainElement(expectedBindMount))
				})

				It("add the instance identity environment variables to the container", func() {
					_, err := containerStore.Create(logger, containerGuid)
					Expect(err).NotTo(HaveOccurred())
					Expect(gardenClient.CreateCallCount()).To(Equal(1))
					Expect(gardenClient.CreateArgsForCall(0).Env).To(ContainElement("CF_INSTANCE_CERT=some-cert"))
				})

				Context("when failing to create credential directory on host", func() {
					BeforeEach(func() {
						credManager.CreateCredDirReturns(nil, nil, errors.New("failed to create dir"))
					})

					It("fails fast and completes the container", func() {
						_, err := containerStore.Create(logger, containerGuid)
						Expect(err).To(HaveOccurred())

						container, err := containerStore.Get(logger, containerGuid)
						Expect(err).NotTo(HaveOccurred())
						Expect(container.State).To(Equal(executor.StateCompleted))
						Expect(container.RunResult.Failed).To(BeTrue())
						Expect(container.RunResult.FailureReason).To(Equal(containerstore.CredDirFailed))
					})
				})
			})

			Context("when there are volume mounts configured", func() {
				BeforeEach(func() {
					someConfig := map[string]interface{}{"some-config": "interface"}
					runReq.RunInfo.VolumeMounts = []executor.VolumeMount{
						executor.VolumeMount{ContainerPath: "cpath1", Mode: executor.BindMountModeRW, Driver: "some-driver", VolumeId: "some-volume", Config: someConfig},
						executor.VolumeMount{ContainerPath: "cpath2", Mode: executor.BindMountModeRO, Driver: "some-other-driver", VolumeId: "some-other-volume", Config: someConfig},
					}

					count := 0
					volumeManager.MountStub = // first call mounts at a different point than second call
						func(lager.Logger, string, string, map[string]interface{}) (volman.MountResponse, error) {
							defer func() { count = count + 1 }()
							if count == 0 {
								return volman.MountResponse{Path: "hpath1"}, nil
							}
							return volman.MountResponse{Path: "hpath2"}, nil
						}
				})

				It("mounts the correct volumes via the volume manager", func() {
					_, err := containerStore.Create(logger, containerGuid)
					Expect(err).NotTo(HaveOccurred())
					Expect(volumeManager.MountCallCount()).To(Equal(2))

					_, driverName, volumeId, config := volumeManager.MountArgsForCall(0)
					Expect(driverName).To(Equal(runReq.VolumeMounts[0].Driver))
					Expect(volumeId).To(Equal(runReq.VolumeMounts[0].VolumeId))
					Expect(config).To(Equal(runReq.VolumeMounts[0].Config))

					_, driverName, volumeId, config = volumeManager.MountArgsForCall(1)
					Expect(driverName).To(Equal(runReq.VolumeMounts[1].Driver))
					Expect(volumeId).To(Equal(runReq.VolumeMounts[1].VolumeId))
					Expect(config).To(Equal(runReq.VolumeMounts[1].Config))
				})

				It("correctly maps container and host directories in garden", func() {
					_, err := containerStore.Create(logger, containerGuid)
					Expect(err).NotTo(HaveOccurred())
					Expect(gardenClient.CreateCallCount()).To(Equal(1))

					expectedBindMount1 := garden.BindMount{SrcPath: "hpath1", DstPath: "cpath1", Mode: garden.BindMountModeRW}
					Expect(gardenClient.CreateArgsForCall(0).BindMounts).To(ContainElement(expectedBindMount1))
					expectedBindMount2 := garden.BindMount{SrcPath: "hpath2", DstPath: "cpath2", Mode: garden.BindMountModeRO}
					Expect(gardenClient.CreateArgsForCall(0).BindMounts).To(ContainElement(expectedBindMount2))
				})

				Context("when it errors on mount", func() {
					BeforeEach(func() {
						volumeManager.MountReturns(volman.MountResponse{Path: "host-path"}, errors.New("some-error"))
					})

					It("fails fast and completes the container", func() {
						_, err := containerStore.Create(logger, containerGuid)
						Expect(err).To(HaveOccurred())
						Expect(volumeManager.MountCallCount()).To(Equal(1))

						container, err := containerStore.Get(logger, containerGuid)
						Expect(err).NotTo(HaveOccurred())
						Expect(container.State).To(Equal(executor.StateCompleted))
						Expect(container.RunResult.Failed).To(BeTrue())
						Expect(container.RunResult.FailureReason).To(Equal(containerstore.VolmanMountFailed))
					})
				})
			})

			Context("when there are trusted system certificates", func() {
				Context("and the desired LRP has a certificates path", func() {
					var mounts []garden.BindMount

					BeforeEach(func() {
						runReq.RunInfo.TrustedSystemCertificatesPath = "/etc/cf-system-certificates"
						mounts = []garden.BindMount{
							{
								SrcPath: "/var/vcap/data/cf-system-trusted-certs",
								DstPath: runReq.RunInfo.TrustedSystemCertificatesPath,
								Mode:    garden.BindMountModeRO,
								Origin:  garden.BindMountOriginHost,
							},
						}
					})

					It("creates a bind mount", func() {
						_, err := containerStore.Create(logger, containerGuid)
						Expect(err).NotTo(HaveOccurred())

						Expect(gardenClient.CreateCallCount()).To(Equal(1))
						gardenContainerSpec := gardenClient.CreateArgsForCall(0)
						Expect(gardenContainerSpec.BindMounts).To(ContainElement(mounts[0]))
					})
				})

				Context("and the desired LRP does not have a certificates path", func() {
					It("does not create a bind mount", func() {
						_, err := containerStore.Create(logger, containerGuid)
						Expect(err).NotTo(HaveOccurred())

						Expect(gardenClient.CreateCallCount()).To(Equal(1))
						gardenContainerSpec := gardenClient.CreateArgsForCall(0)
						Expect(gardenContainerSpec.BindMounts).NotTo(ContainElement(garden.BindMount{
							SrcPath: "/var/vcap/data/cf-system-trusted-certs",
							DstPath: runReq.RunInfo.TrustedSystemCertificatesPath,
							Mode:    garden.BindMountModeRO,
							Origin:  garden.BindMountOriginHost,
						}))
					})
				})
			})

			Context("when downloading bind mounts fails", func() {
				BeforeEach(func() {
					dependencyManager.DownloadCachedDependenciesReturns(containerstore.BindMounts{}, errors.New("no"))
				})

				It("transitions to a completed state", func() {
					_, err := containerStore.Create(logger, containerGuid)
					Expect(err).To(HaveOccurred())

					container, err := containerStore.Get(logger, containerGuid)
					Expect(err).NotTo(HaveOccurred())
					Expect(container.State).To(Equal(executor.StateCompleted))
					Expect(container.RunResult.Failed).To(BeTrue())
					Expect(container.RunResult.FailureReason).To(Equal(containerstore.DownloadCachedDependenciesFailed))
				})
			})

			Context("when egress rules are requested", func() {
				BeforeEach(func() {
					egressRules := []*models.SecurityGroupRule{
						{
							Protocol:     "icmp",
							Destinations: []string{"1.1.1.1"},
							IcmpInfo: &models.ICMPInfo{
								Type: 2,
								Code: 10,
							},
						},
						{
							Protocol:     "icmp",
							Destinations: []string{"1.1.1.2"},
							IcmpInfo: &models.ICMPInfo{
								Type: 3,
								Code: 10,
							},
						},
					}
					runReq.EgressRules = egressRules
				})

				It("calls NetOut for each egress rule", func() {
					_, err := containerStore.Create(logger, containerGuid)
					Expect(err).NotTo(HaveOccurred())

					Expect(gardenClient.CreateCallCount()).To(Equal(1))
					containerSpec := gardenClient.CreateArgsForCall(0)
					Expect(containerSpec.NetOut).To(HaveLen(2))
					icmpCode := garden.ICMPCode(10)
					Expect(containerSpec.NetOut).To(ContainElement(garden.NetOutRule{
						Protocol: garden.Protocol(3),
						Networks: []garden.IPRange{
							{
								Start: net.ParseIP("1.1.1.1"),
								End:   net.ParseIP("1.1.1.1"),
							},
						},
						ICMPs: &garden.ICMPControl{
							Type: 2,
							Code: &icmpCode,
						},
					}))
					Expect(containerSpec.NetOut).To(ContainElement(garden.NetOutRule{
						Protocol: garden.Protocol(3),
						Networks: []garden.IPRange{
							{
								Start: net.ParseIP("1.1.1.2"),
								End:   net.ParseIP("1.1.1.2"),
							},
						},
						ICMPs: &garden.ICMPControl{
							Type: 3,
							Code: &icmpCode,
						},
					}))
				})

				Context("when a egress rule is not valid", func() {
					BeforeEach(func() {
						egressRule := &models.SecurityGroupRule{
							Protocol: "tcp",
						}
						runReq.EgressRules = append(runReq.EgressRules, egressRule)
					})

					It("returns an error", func() {
						_, err := containerStore.Create(logger, containerGuid)
						Expect(err).To(HaveOccurred())

						Expect(gardenClient.CreateCallCount()).To(Equal(0))
					})
				})
			})

			Context("when ports are requested", func() {
				BeforeEach(func() {
					portMapping := []executor.PortMapping{
						{ContainerPort: 8080},
						{ContainerPort: 9090},
					}
					runReq.Ports = portMapping

					gardenClient.CreateStub = func(spec garden.ContainerSpec) (garden.Container, error) {
						gardenContainer.InfoStub = func() (garden.ContainerInfo, error) {
							info := garden.ContainerInfo{}
							info.MappedPorts = []garden.PortMapping{}
							for _, netIn := range spec.NetIn {
								switch netIn.ContainerPort {
								case 8080:
									info.MappedPorts = append(info.MappedPorts, garden.PortMapping{HostPort: 16000, ContainerPort: 8080})
								case 9090:
									info.MappedPorts = append(info.MappedPorts, garden.PortMapping{HostPort: 32000, ContainerPort: 9090})
								default:
									return info, errors.New("failed-net-in")
								}
							}
							return info, nil
						}
						return gardenContainer, nil
					}
				})

				It("calls NetIn on the container for each port", func() {
					_, err := containerStore.Create(logger, containerGuid)
					Expect(err).NotTo(HaveOccurred())

					containerSpec := gardenClient.CreateArgsForCall(0)
					Expect(containerSpec.NetIn).To(HaveLen(2))
					Expect(containerSpec.NetIn).To(ContainElement(garden.NetIn{
						HostPort: 0, ContainerPort: 8080,
					}))
					Expect(containerSpec.NetIn).To(ContainElement(garden.NetIn{
						HostPort: 0, ContainerPort: 9090,
					}))
				})

				It("saves the actual port mappings on the container", func() {
					container, err := containerStore.Create(logger, containerGuid)
					Expect(err).NotTo(HaveOccurred())

					Expect(container.Ports[0].ContainerPort).To(BeEquivalentTo(8080))
					Expect(container.Ports[0].HostPort).To(BeEquivalentTo(16000))
					Expect(container.Ports[1].ContainerPort).To(BeEquivalentTo(9090))
					Expect(container.Ports[1].HostPort).To(BeEquivalentTo(32000))

					fetchedContainer, err := containerStore.Get(logger, containerGuid)
					Expect(err).NotTo(HaveOccurred())
					Expect(fetchedContainer).To(Equal(container))
				})
			})

			Context("when a total disk scope is request", func() {
				BeforeEach(func() {
					runReq.DiskScope = executor.TotalDiskLimit
				})

				It("creates the container with the correct disk scope", func() {
					_, err := containerStore.Create(logger, containerGuid)
					Expect(err).NotTo(HaveOccurred())

					Expect(gardenClient.CreateCallCount()).To(Equal(1))
					containerSpec := gardenClient.CreateArgsForCall(0)
					Expect(containerSpec.Limits.Disk.Scope).To(Equal(garden.DiskLimitScopeTotal))
				})
			})

			Context("when creating the container fails", func() {
				BeforeEach(func() {
					gardenClient.CreateReturns(nil, errors.New("boom!"))
				})

				It("returns an error", func() {
					_, err := containerStore.Create(logger, containerGuid)
					Expect(err).To(Equal(errors.New("boom!")))
				})

				It("transitions to a completed state", func() {
					_, err := containerStore.Create(logger, containerGuid)
					Expect(err).To(Equal(errors.New("boom!")))

					container, err := containerStore.Get(logger, containerGuid)
					Expect(err).NotTo(HaveOccurred())
					Expect(container.State).To(Equal(executor.StateCompleted))
					Expect(container.RunResult.Failed).To(BeTrue())
					Expect(container.RunResult.FailureReason).To(Equal(containerstore.ContainerInitializationFailedMessage))
				})

				It("emits a metric after failing to create the container", func() {
					_, err := containerStore.Create(logger, containerGuid)
					Expect(err).To(HaveOccurred())
					Eventually(getMetrics).Should(HaveKey(containerstore.GardenContainerCreationFailedDuration))
				})
			})

			Context("when requesting the container info for the created container fails", func() {
				BeforeEach(func() {
					gardenContainer.InfoStub = func() (garden.ContainerInfo, error) {
						if gardenContainer.InfoCallCount() == 1 {
							return garden.ContainerInfo{}, nil
						}
						return garden.ContainerInfo{}, errors.New("could not obtain info")
					}
				})

				It("returns an error", func() {
					_, err := containerStore.Create(logger, containerGuid)
					Expect(err).To(HaveOccurred())

					Expect(gardenClient.DestroyCallCount()).To(Equal(1))
					Expect(gardenClient.DestroyArgsForCall(0)).To(Equal(containerGuid))
				})

				It("transitions to a completed state", func() {
					_, err := containerStore.Create(logger, containerGuid)
					Expect(err).To(HaveOccurred())

					container, err := containerStore.Get(logger, containerGuid)
					Expect(err).NotTo(HaveOccurred())
					Expect(container.State).To(Equal(executor.StateCompleted))
					Expect(container.RunResult.Failed).To(BeTrue())
					Expect(container.RunResult.FailureReason).To(Equal(containerstore.ContainerInitializationFailedMessage))
				})
			})
		})

		Context("when the container does not exist", func() {
			It("returns a conatiner not found error", func() {
				_, err := containerStore.Create(logger, "bogus-guid")
				Expect(err).To(Equal(executor.ErrContainerNotFound))
			})
		})

		Context("when the container is not initializing", func() {
			BeforeEach(func() {
				_, err := containerStore.Reserve(logger, allocationReq)
				Expect(err).NotTo(HaveOccurred())
			})

			It("returns an invalid state transition error", func() {
				_, err := containerStore.Create(logger, containerGuid)
				Expect(err).To(Equal(executor.ErrInvalidTransition))
			})
		})
	})

	Describe("Run", func() {
		var (
			allocationReq *executor.AllocationRequest
			runProcess    *gardenfakes.FakeProcess
		)

		BeforeEach(func() {
			allocationReq = &executor.AllocationRequest{
				Guid: containerGuid,
			}

			runProcess = &gardenfakes.FakeProcess{}

			gardenContainer.RunReturns(runProcess, nil)
			gardenClient.CreateReturns(gardenContainer, nil)
			gardenClient.LookupReturns(gardenContainer, nil)
		})

		Context("when it is in the created state", func() {
			var (
				runReq *executor.RunRequest
			)

			BeforeEach(func() {
				runAction := &models.Action{
					RunAction: &models.RunAction{
						Path: "/foo/bar",
					},
				}

				runReq = &executor.RunRequest{
					Guid: containerGuid,
					RunInfo: executor.RunInfo{
						Action: runAction,
					},
				}
			})

			JustBeforeEach(func() {
				_, err := containerStore.Reserve(logger, allocationReq)
				Expect(err).NotTo(HaveOccurred())

				err = containerStore.Initialize(logger, runReq)
				Expect(err).NotTo(HaveOccurred())

				_, err = containerStore.Create(logger, containerGuid)
				Expect(err).NotTo(HaveOccurred())
			})

			Context("while the cred manager is still setting up", func() {
				var (
					finishSetup           chan struct{}
					containerRunnerCalled chan struct{}
				)

				BeforeEach(func() {
					finishSetup = make(chan struct{})
					containerRunnerCalled = make(chan struct{})
					credManager.RunnerReturns(ifrit.RunFunc(func(signals <-chan os.Signal, ready chan<- struct{}) error {
						<-finishSetup
						close(ready)
						<-signals
						return nil
					}))

					megatron.StepsRunnerReturns(ifrit.RunFunc(func(signals <-chan os.Signal, ready chan<- struct{}) error {
						close(containerRunnerCalled)
						return nil
					}), nil)
				})

				AfterEach(func() {
					close(finishSetup)
					Expect(containerStore.Destroy(logger, containerGuid)).To(Succeed())
				})

				It("does not start the container while cred manager is setting up", func() {
					go containerStore.Run(logger, containerGuid)
					Consistently(containerRunnerCalled).ShouldNot(BeClosed())
				})
			})

			Context("when the runner fails the initial credential generation", func() {
				BeforeEach(func() {
					credManager.RunnerReturns(ifrit.RunFunc(func(signals <-chan os.Signal, ready chan<- struct{}) error {
						return errors.New("BOOOM")
					}))
				})

				It("destroys the container and returns an error", func() {
					err := containerStore.Run(logger, containerGuid)
					Expect(err).NotTo(HaveOccurred())

					Eventually(func() executor.State {
						container, err := containerStore.Get(logger, containerGuid)
						Expect(err).NotTo(HaveOccurred())
						return container.State
					}).Should(Equal(executor.StateCompleted))
					container, _ := containerStore.Get(logger, containerGuid)
					Expect(container.RunResult.Failed).To(BeTrue())
					// make sure the error message is at the end so that
					// FailureReasonSanitizer can properly map the error messages
					Expect(container.RunResult.FailureReason).To(MatchRegexp("BOOOM$"))
				})

				It("tranistions immediately to Completed state", func() {
					err := containerStore.Run(logger, containerGuid)
					Expect(err).NotTo(HaveOccurred())

					Eventually(func() executor.State {
						container, err := containerStore.Get(logger, containerGuid)
						Expect(err).NotTo(HaveOccurred())
						return container.State
					}).Should(Equal(executor.StateCompleted))

					Eventually(func() []string {
						var events []string
						for i := 0; i < eventEmitter.EmitCallCount(); i++ {
							event := eventEmitter.EmitArgsForCall(i)
							events = append(events, string(event.EventType()))
						}
						return events
					}).Should(ConsistOf("container_reserved", "container_complete"))
				})
			})

			Context("when instance credential is ready", func() {
				var (
					containerRunnerCalled   chan struct{}
					credManagerRunnerCalled chan struct{}
				)

				BeforeEach(func() {
					credManagerRunnerCalled = make(chan struct{})
					called := credManagerRunnerCalled
					credManager.RunnerReturns(ifrit.RunFunc(func(signals <-chan os.Signal, ready chan<- struct{}) error {
						close(ready)
						close(called)
						<-signals
						return nil
					}))

					containerRunnerCalled = make(chan struct{})

					megatron.StepsRunnerReturns(ifrit.RunFunc(func(signals <-chan os.Signal, ready chan<- struct{}) error {
						close(containerRunnerCalled)
						close(ready)
						<-signals
						return nil
					}), nil)
				})

				AfterEach(func() {
					containerStore.Destroy(logger, containerGuid)
				})

				Context("when the runner fails subsequent credential generation", func() {
					BeforeEach(func() {
						credManager.RunnerReturns(ifrit.RunFunc(func(signals <-chan os.Signal, ready chan<- struct{}) error {
							close(ready)
							select {
							case <-containerRunnerCalled:
								return errors.New("BOOOM")
							case <-signals:
								return nil
							}
						}))
					})

					It("destroys the container and returns an error", func() {
						err := containerStore.Run(logger, containerGuid)
						Expect(err).NotTo(HaveOccurred())

						Eventually(func() executor.State {
							container, err := containerStore.Get(logger, containerGuid)
							Expect(err).NotTo(HaveOccurred())
							return container.State
						}).Should(Equal(executor.StateCompleted))
						container, _ := containerStore.Get(logger, containerGuid)
						Expect(container.RunResult.Failed).To(BeTrue())
						// make sure the error message is at the end so that
						// FailureReasonSanitizer can properly map the error messages
						Expect(container.RunResult.FailureReason).To(MatchRegexp("BOOOM$"))
					})
				})

				Context("when the action runs indefinitely", func() {
					var readyChan chan struct{}
					BeforeEach(func() {
						readyChan = make(chan struct{})
						var testRunner ifrit.RunFunc = func(signals <-chan os.Signal, ready chan<- struct{}) error {
							readyChan <- struct{}{}
							close(ready)
							<-signals
							return nil
						}
						megatron.StepsRunnerReturns(testRunner, nil)
					})

					It("performs the step", func() {
						err := containerStore.Run(logger, containerGuid)
						Expect(err).NotTo(HaveOccurred())

						Expect(megatron.StepsRunnerCallCount()).To(Equal(1))
						Eventually(readyChan).Should(Receive())
					})

					It("sets the container state to running once the healthcheck passes, and emits a running event", func() {
						err := containerStore.Run(logger, containerGuid)
						Expect(err).NotTo(HaveOccurred())

						container, err := containerStore.Get(logger, containerGuid)
						Expect(err).NotTo(HaveOccurred())
						Expect(container.State).To(Equal(executor.StateCreated))

						Eventually(readyChan).Should(Receive())

						Eventually(func() executor.State {
							container, err := containerStore.Get(logger, containerGuid)
							Expect(err).NotTo(HaveOccurred())
							return container.State
						}).Should(Equal(executor.StateRunning))

						container, err = containerStore.Get(logger, containerGuid)
						Expect(err).NotTo(HaveOccurred())

						Eventually(eventEmitter.EmitCallCount).Should(Equal(2))
						event := eventEmitter.EmitArgsForCall(1)
						Expect(event).To(Equal(executor.ContainerRunningEvent{RawContainer: container}))
					})
				})

				Context("when the action exits", func() {
					Context("successfully", func() {
						var (
							completeChan chan struct{}
						)

						BeforeEach(func() {
							completeChan = make(chan struct{})

							var testRunner ifrit.RunFunc = func(signals <-chan os.Signal, ready chan<- struct{}) error {
								close(ready)
								<-completeChan
								return nil
							}
							megatron.StepsRunnerReturns(testRunner, nil)
						})

						It("sets its state to completed", func() {
							err := containerStore.Run(logger, containerGuid)
							Expect(err).NotTo(HaveOccurred())

							close(completeChan)

							Eventually(pollForComplete(containerGuid)).Should(BeTrue())

							container, err := containerStore.Get(logger, containerGuid)
							Expect(err).NotTo(HaveOccurred())
							Expect(container.State).To(Equal(executor.StateCompleted))
						})

						It("emits a container completed event", func() {
							err := containerStore.Run(logger, containerGuid)
							Expect(err).NotTo(HaveOccurred())

							Eventually(pollForRunning(containerGuid)).Should(BeTrue())
							close(completeChan)
							Eventually(pollForComplete(containerGuid)).Should(BeTrue())

							Expect(eventEmitter.EmitCallCount()).To(Equal(3))

							container, err := containerStore.Get(logger, containerGuid)
							Expect(err).NotTo(HaveOccurred())

							emittedEvents := []executor.Event{}
							for i := 0; i < eventEmitter.EmitCallCount(); i++ {
								emittedEvents = append(emittedEvents, eventEmitter.EmitArgsForCall(i))
							}
							Expect(emittedEvents).To(ContainElement(executor.ContainerCompleteEvent{
								RawContainer: container,
							}))
						})

						It("sets the result on the container", func() {
							err := containerStore.Run(logger, containerGuid)
							Expect(err).NotTo(HaveOccurred())

							close(completeChan)

							Eventually(pollForComplete(containerGuid)).Should(BeTrue())

							container, err := containerStore.Get(logger, containerGuid)
							Expect(err).NotTo(HaveOccurred())
							Expect(container.RunResult.Failed).To(Equal(false))
							Expect(container.RunResult.Stopped).To(Equal(false))
						})
					})

					Context("unsuccessfully", func() {
						BeforeEach(func() {
							var testRunner ifrit.RunFunc = func(signals <-chan os.Signal, ready chan<- struct{}) error {
								close(ready)
								return errors.New("BOOOOM!!!!")
							}
							megatron.StepsRunnerReturns(testRunner, nil)
						})

						It("sets the run result on the container", func() {
							err := containerStore.Run(logger, containerGuid)
							Expect(err).NotTo(HaveOccurred())

							Eventually(pollForComplete(containerGuid)).Should(BeTrue())

							container, err := containerStore.Get(logger, containerGuid)
							Expect(err).NotTo(HaveOccurred())
							Expect(container.RunResult.Failed).To(Equal(true))
							// make sure the error message is at the end so that
							// FailureReasonSanitizer can properly map the error messages
							Expect(container.RunResult.FailureReason).To(MatchRegexp("BOOOOM!!!!$"))
							Expect(container.RunResult.Stopped).To(Equal(false))
						})
					})
				})

				Context("when the transformer fails to generate steps", func() {
					BeforeEach(func() {
						megatron.StepsRunnerReturns(nil, errors.New("defeated by the auto bots"))
					})

					It("returns an error", func() {
						err := containerStore.Run(logger, containerGuid)
						Expect(err).To(HaveOccurred())
					})
				})
			})
		})

		Context("when the container does not exist", func() {
			It("returns an ErrContainerNotFound error", func() {
				err := containerStore.Run(logger, containerGuid)
				Expect(err).To(Equal(executor.ErrContainerNotFound))
			})
		})

		Context("When the container is not in the created state", func() {
			JustBeforeEach(func() {
				_, err := containerStore.Reserve(logger, allocationReq)
				Expect(err).NotTo(HaveOccurred())
			})

			It("returns a transition error", func() {
				err := containerStore.Run(logger, containerGuid)
				Expect(err).To(Equal(executor.ErrInvalidTransition))
			})
		})
	})

	Describe("Stop", func() {
		var finishRun chan struct{}
		var runReq *executor.RunRequest
		BeforeEach(func() {
			finishRun = make(chan struct{})
			ifritFinishRun := finishRun
			var testRunner ifrit.RunFunc = func(signals <-chan os.Signal, ready chan<- struct{}) error {
				<-signals
				ifritFinishRun <- struct{}{}
				return nil
			}
			runInfo := executor.RunInfo{
				LogConfig: executor.LogConfig{
					Guid:       containerGuid,
					Index:      1,
					SourceName: "test-source",
				},
			}
			runReq = &executor.RunRequest{Guid: containerGuid, RunInfo: runInfo}
			gardenClient.CreateReturns(gardenContainer, nil)
			megatron.StepsRunnerReturns(testRunner, nil)
		})

		JustBeforeEach(func() {
			_, err := containerStore.Reserve(logger, &executor.AllocationRequest{Guid: containerGuid})
			Expect(err).NotTo(HaveOccurred())

			err = containerStore.Initialize(logger, runReq)
			Expect(err).NotTo(HaveOccurred())

			_, err = containerStore.Create(logger, containerGuid)
			Expect(err).NotTo(HaveOccurred())
		})

		Context("when the container has processes associated with it", func() {
			JustBeforeEach(func() {
				err := containerStore.Run(logger, containerGuid)
				Expect(err).NotTo(HaveOccurred())
			})

			It("sets stopped to true on the run result", func() {
				err := containerStore.Stop(logger, containerGuid)
				Expect(err).NotTo(HaveOccurred())

				Eventually(finishRun).Should(Receive())

				container, err := containerStore.Get(logger, containerGuid)
				Expect(err).NotTo(HaveOccurred())
				Expect(container.RunResult.Stopped).To(BeTrue())
			})

			It("logs that the container is stopping", func() {
				err := containerStore.Stop(logger, containerGuid)
				Expect(err).NotTo(HaveOccurred())
				Expect(fakeMetronClient.SendAppLogCallCount()).To(Equal(3))
				appId, msg, sourceType, sourceInstance := fakeMetronClient.SendAppLogArgsForCall(2)
				Expect(appId).To(Equal(containerGuid))
				Expect(sourceType).To(Equal("test-source"))
				Expect(msg).To(Equal(fmt.Sprintf("Stopping instance %s", containerGuid)))
				Expect(sourceInstance).To(Equal("1"))
			})
		})

		Context("when the container does not have processes associated with it", func() {
			It("transitions to the completed state", func() {
				err := containerStore.Stop(logger, containerGuid)
				Expect(err).NotTo(HaveOccurred())

				container, err := containerStore.Get(logger, containerGuid)
				Expect(err).NotTo(HaveOccurred())

				Expect(container.RunResult.Stopped).To(BeTrue())
				Expect(container.State).To(Equal(executor.StateCompleted))
			})
		})

		Context("when the container does not exist", func() {
			It("returns an ErrContainerNotFound", func() {
				err := containerStore.Stop(logger, "")
				Expect(err).To(Equal(executor.ErrContainerNotFound))
			})
		})
	})

	Describe("Destroy", func() {
		var resource executor.Resource
		var expectedMounts containerstore.BindMounts
		var runReq *executor.RunRequest

		BeforeEach(func() {
			runInfo := executor.RunInfo{
				LogConfig: executor.LogConfig{
					Guid:       containerGuid,
					Index:      1,
					SourceName: "test-source",
				},
			}
			runReq = &executor.RunRequest{Guid: containerGuid, RunInfo: runInfo}
			gardenClient.CreateReturns(gardenContainer, nil)
			resource = executor.NewResource(1024, 2048, 1024, "foobar")
			expectedMounts = containerstore.BindMounts{
				CacheKeys: []containerstore.BindMountCacheKey{
					{CacheKey: "cache-key", Dir: "foo"},
				},
			}
			dependencyManager.DownloadCachedDependenciesReturns(expectedMounts, nil)
		})

		JustBeforeEach(func() {
			_, err := containerStore.Reserve(logger, &executor.AllocationRequest{Guid: containerGuid, Resource: resource})
			Expect(err).NotTo(HaveOccurred())

			err = containerStore.Initialize(logger, runReq)
			Expect(err).NotTo(HaveOccurred())

			_, err = containerStore.Create(logger, containerGuid)
			Expect(err).NotTo(HaveOccurred())
		})

		Context("when there are volumes mounted", func() {
			BeforeEach(func() {
				someConfig := map[string]interface{}{"some-config": "interface"}
				runReq.RunInfo.VolumeMounts = []executor.VolumeMount{
					executor.VolumeMount{ContainerPath: "cpath1", Driver: "some-driver", VolumeId: "some-volume", Config: someConfig},
					executor.VolumeMount{ContainerPath: "cpath2", Driver: "some-other-driver", VolumeId: "some-other-volume", Config: someConfig},
				}
				count := 0
				volumeManager.MountStub = // first call mounts at a different point than second call
					func(lager.Logger, string, string, map[string]interface{}) (volman.MountResponse, error) {
						defer func() { count = count + 1 }()
						if count == 0 {
							return volman.MountResponse{Path: "hpath1"}, nil
						}
						return volman.MountResponse{Path: "hpath2"}, nil
					}
			})

			It("removes mounted volumes on the host machine", func() {
				err := containerStore.Destroy(logger, containerGuid)
				Expect(err).NotTo(HaveOccurred())
				Expect(volumeManager.UnmountCallCount()).To(Equal(2))

				_, driverId, volumeId := volumeManager.UnmountArgsForCall(0)
				Expect(driverId).To(Equal(runReq.RunInfo.VolumeMounts[0].Driver))
				Expect(volumeId).To(Equal(runReq.RunInfo.VolumeMounts[0].VolumeId))

				_, driverId, volumeId = volumeManager.UnmountArgsForCall(1)
				Expect(driverId).To(Equal(runReq.RunInfo.VolumeMounts[1].Driver))
				Expect(volumeId).To(Equal(runReq.RunInfo.VolumeMounts[1].VolumeId))
			})

			Context("when we fail to release cache dependencies", func() {
				BeforeEach(func() {
					dependencyManager.ReleaseCachedDependenciesReturns(errors.New("oh noes!"))
				})
				It("still attempts to unmount our volumes", func() {
					err := containerStore.Destroy(logger, containerGuid)
					Expect(err).To(HaveOccurred())
					Expect(volumeManager.UnmountCallCount()).To(Equal(2))

					_, driverId, volumeId := volumeManager.UnmountArgsForCall(0)
					Expect(driverId).To(Equal(runReq.RunInfo.VolumeMounts[0].Driver))
					Expect(volumeId).To(Equal(runReq.RunInfo.VolumeMounts[0].VolumeId))

					_, driverId, volumeId = volumeManager.UnmountArgsForCall(1)
					Expect(driverId).To(Equal(runReq.RunInfo.VolumeMounts[1].Driver))
					Expect(volumeId).To(Equal(runReq.RunInfo.VolumeMounts[1].VolumeId))
				})
			})
			Context("when an volume unmount fails", func() {
				BeforeEach(func() {
					volumeManager.UnmountReturns(errors.New("oh noes!"))
				})

				It("still attempts to unmount the remaining volumes", func() {
					err := containerStore.Destroy(logger, containerGuid)
					Expect(err).To(HaveOccurred())
					Expect(volumeManager.UnmountCallCount()).To(Equal(2))
				})
			})
		})

		It("removes downloader cache references", func() {
			err := containerStore.Destroy(logger, containerGuid)
			Expect(err).NotTo(HaveOccurred())
			Expect(dependencyManager.ReleaseCachedDependenciesCallCount()).To(Equal(1))
			_, keys := dependencyManager.ReleaseCachedDependenciesArgsForCall(0)
			Expect(keys).To(Equal(expectedMounts.CacheKeys))
		})

		It("destroys the container", func() {
			err := containerStore.Destroy(logger, containerGuid)
			Expect(err).NotTo(HaveOccurred())

			Expect(gardenClient.DestroyCallCount()).To(Equal(1))
			Expect(gardenClient.DestroyArgsForCall(0)).To(Equal(containerGuid))

			_, err = containerStore.Get(logger, containerGuid)
			Expect(err).To(Equal(executor.ErrContainerNotFound))
		})

		It("emits a metric after destroying the container", func() {
			err := containerStore.Destroy(logger, containerGuid)
			Expect(err).NotTo(HaveOccurred())

			Eventually(getMetrics).Should(HaveKey(containerstore.GardenContainerDestructionSucceededDuration))
		})

		It("frees the containers resources", func() {
			err := containerStore.Destroy(logger, containerGuid)
			Expect(err).NotTo(HaveOccurred())

			remainingResources := containerStore.RemainingResources(logger)
			Expect(remainingResources).To(Equal(totalCapacity))
		})

		Context("when destroying the garden container fails", func() {
			var destroyErr error
			BeforeEach(func() {
				destroyErr = errors.New("failed to destroy garden container")
			})

			JustBeforeEach(func() {
				gardenClient.DestroyReturns(destroyErr)
			})

			Context("because the garden container does not exist", func() {
				BeforeEach(func() {
					destroyErr = garden.ContainerNotFoundError{}
				})

				It("does not return an error", func() {
					err := containerStore.Destroy(logger, containerGuid)
					Expect(err).NotTo(HaveOccurred())
				})
			})

			Context("because the garden container is already being destroyed", func() {
				BeforeEach(func() {
					destroyErr = server.ErrConcurrentDestroy
				})

				It("does not return an error", func() {
					err := containerStore.Destroy(logger, containerGuid)
					Expect(err).NotTo(HaveOccurred())
				})

				It("logs the container is destroyed", func() {
					err := containerStore.Destroy(logger, containerGuid)
					Expect(err).NotTo(HaveOccurred())
					Expect(fakeMetronClient.SendAppLogCallCount()).To(Equal(4))
					appId, msg, sourceType, sourceInstance := fakeMetronClient.SendAppLogArgsForCall(2)
					Expect(appId).To(Equal(containerGuid))
					Expect(sourceType).To(Equal("test-source"))
					Expect(msg).To(Equal("Destroying container"))
					Expect(sourceInstance).To(Equal("1"))

					appId, msg, sourceType, sourceInstance = fakeMetronClient.SendAppLogArgsForCall(3)
					Expect(appId).To(Equal(containerGuid))
					Expect(sourceType).To(Equal("test-source"))
					Expect(msg).To(Equal("Successfully destroyed container"))
					Expect(sourceInstance).To(Equal("1"))
				})
			})

			Context("for unknown reason", func() {
				It("returns an error", func() {
					err := containerStore.Destroy(logger, containerGuid)
					Expect(err).To(Equal(destroyErr))
				})

				It("emits a metric after failing to destroy the container", func() {
					err := containerStore.Destroy(logger, containerGuid)
					Expect(err).To(Equal(destroyErr))
					Eventually(getMetrics).Should(HaveKey(containerstore.GardenContainerDestructionFailedDuration))
				})

				It("does remove the container from the container store", func() {
					err := containerStore.Destroy(logger, containerGuid)
					Expect(err).To(Equal(destroyErr))

					Expect(gardenClient.DestroyCallCount()).To(Equal(1))
					Expect(gardenClient.DestroyArgsForCall(0)).To(Equal(containerGuid))

					_, err = containerStore.Get(logger, containerGuid)
					Expect(err).To(Equal(executor.ErrContainerNotFound))
				})

				It("frees the containers resources", func() {
					err := containerStore.Destroy(logger, containerGuid)
					Expect(err).To(Equal(destroyErr))

					remainingResources := containerStore.RemainingResources(logger)
					Expect(remainingResources).To(Equal(totalCapacity))
				})
			})
		})

		Context("when the container does not exist", func() {
			It("returns a ErrContainerNotFound", func() {
				err := containerStore.Destroy(logger, "")
				Expect(err).To(Equal(executor.ErrContainerNotFound))
			})
		})

		Context("when there is a stopped process associated with the container", func() {
			var (
				finishRun                 chan struct{}
				credManagerRunnerSignaled chan struct{}
				destroyed                 chan struct{}
			)

			BeforeEach(func() {
				credManagerRunnerSignaled = make(chan struct{})
				finishRun = make(chan struct{})
				finishRunIfrit := finishRun
				var testRunner ifrit.RunFunc = func(signals <-chan os.Signal, ready chan<- struct{}) error {
					close(ready)
					<-signals
					<-finishRunIfrit
					return nil
				}

				signaled := credManagerRunnerSignaled
				megatron.StepsRunnerReturns(testRunner, nil)
				credManager.RunnerReturns(ifrit.RunFunc(func(signals <-chan os.Signal, ready chan<- struct{}) error {
					close(ready)
					<-signals
					close(signaled)
					return nil
				}))
			})

			JustBeforeEach(func() {
				err := containerStore.Run(logger, containerGuid)
				Expect(err).NotTo(HaveOccurred())
				Eventually(pollForRunning(containerGuid)).Should(BeTrue())
				err = containerStore.Stop(logger, containerGuid)
				Expect(err).NotTo(HaveOccurred())
				destroyed = make(chan struct{})
				go func(ch chan struct{}) {
					containerStore.Destroy(logger, containerGuid)
					close(ch)
				}(destroyed)
			})

			It("cancels the process", func() {
				Consistently(destroyed).ShouldNot(Receive())
				close(finishRun)
				Eventually(destroyed).Should(BeClosed())
			})

			It("logs that the container is stopping once", func() {
				close(finishRun)
				Eventually(destroyed).Should(BeClosed())

				Expect(fakeMetronClient.SendAppLogCallCount()).To(Equal(5))
				appId, msg, sourceType, sourceInstance := fakeMetronClient.SendAppLogArgsForCall(2)
				Expect(appId).To(Equal(containerGuid))
				Expect(sourceType).To(Equal("test-source"))
				Expect(msg).To(Equal(fmt.Sprintf("Stopping instance %s", containerGuid)))
				Expect(sourceInstance).To(Equal("1"))
			})
		})

		Context("when there is a running process associated with the container", func() {
			var (
				finishRun                 chan struct{}
				credManagerRunnerSignaled chan struct{}
				destroyed                 chan struct{}
			)

			BeforeEach(func() {
				credManagerRunnerSignaled = make(chan struct{})
				finishRun = make(chan struct{})
				finishRunIfrit := finishRun
				var testRunner ifrit.RunFunc = func(signals <-chan os.Signal, ready chan<- struct{}) error {
					close(ready)
					<-signals
					<-finishRunIfrit
					return nil
				}

				signaled := credManagerRunnerSignaled
				megatron.StepsRunnerReturns(testRunner, nil)
				credManager.RunnerReturns(ifrit.RunFunc(func(signals <-chan os.Signal, ready chan<- struct{}) error {
					close(ready)
					<-signals
					close(signaled)
					return nil
				}))
			})

			JustBeforeEach(func() {
				err := containerStore.Run(logger, containerGuid)
				Expect(err).NotTo(HaveOccurred())
				Eventually(pollForRunning(containerGuid)).Should(BeTrue())
				destroyed = make(chan struct{})
				go func(ch chan struct{}) {
					containerStore.Destroy(logger, containerGuid)
					close(ch)
				}(destroyed)
			})

			It("cancels the process", func() {
				Consistently(destroyed).ShouldNot(Receive())
				close(finishRun)
				Eventually(destroyed).Should(BeClosed())
			})

			It("signals the cred manager runner", func() {
				close(finishRun)
				Eventually(credManagerRunnerSignaled).Should(BeClosed())
			})

			It("logs that the container is stopping", func() {
				close(finishRun)
				Eventually(destroyed).Should(BeClosed())
				Expect(fakeMetronClient.SendAppLogCallCount()).To(Equal(5))
				appId, msg, sourceType, sourceInstance := fakeMetronClient.SendAppLogArgsForCall(2)
				Expect(appId).To(Equal(containerGuid))
				Expect(sourceType).To(Equal("test-source"))
				Expect(msg).To(Equal(fmt.Sprintf("Stopping instance %s", containerGuid)))
				Expect(sourceInstance).To(Equal("1"))
			})
		})
	})

	Describe("Get", func() {
		BeforeEach(func() {
			_, err := containerStore.Reserve(logger, &executor.AllocationRequest{Guid: containerGuid})
			Expect(err).NotTo(HaveOccurred())
		})

		It("returns the specified container", func() {
			container, err := containerStore.Get(logger, containerGuid)
			Expect(err).NotTo(HaveOccurred())

			Expect(container.Guid).To(Equal(containerGuid))
		})

		Context("when the container does not exist", func() {
			It("returns an ErrContainerNotFound", func() {
				_, err := containerStore.Get(logger, "")
				Expect(err).To(Equal(executor.ErrContainerNotFound))
			})
		})
	})

	Describe("List", func() {
		var container1, container2 executor.Container

		BeforeEach(func() {
			_, err := containerStore.Reserve(logger, &executor.AllocationRequest{
				Guid: containerGuid,
			})
			Expect(err).NotTo(HaveOccurred())

			err = containerStore.Initialize(logger, &executor.RunRequest{
				Guid: containerGuid,
			})
			Expect(err).NotTo(HaveOccurred())

			_, err = containerStore.Reserve(logger, &executor.AllocationRequest{
				Guid: containerGuid + "2",
			})
			Expect(err).NotTo(HaveOccurred())

			container1, err = containerStore.Get(logger, containerGuid)
			Expect(err).NotTo(HaveOccurred())

			container2, err = containerStore.Get(logger, containerGuid+"2")
			Expect(err).NotTo(HaveOccurred())
		})

		It("returns the list of known containers", func() {
			containers := containerStore.List(logger)
			Expect(containers).To(HaveLen(2))
			Expect(containers).To(ContainElement(container1))
			Expect(containers).To(ContainElement(container2))
		})
	})

	reserveContainer := func(guid string) {
		resource := executor.Resource{
			MemoryMB:   10,
			DiskMB:     10,
			RootFSPath: "/foo/bar",
		}
		tags := executor.Tags{}
		_, err := containerStore.Reserve(logger, &executor.AllocationRequest{Guid: guid, Tags: tags, Resource: resource})
		Expect(err).NotTo(HaveOccurred())
	}

	initializeContainer := func(guid string) {
		runInfo := executor.RunInfo{
			CPUWeight:          2,
			StartTimeoutMs:     50000,
			Privileged:         true,
			CachedDependencies: []executor.CachedDependency{},
			LogConfig: executor.LogConfig{
				Guid:       "log-guid",
				Index:      1,
				SourceName: "test-source",
			},
			MetricsConfig: executor.MetricsConfig{
				Guid:  "metric-guid",
				Index: 1,
			},
			Env: []executor.EnvironmentVariable{},
			TrustedSystemCertificatesPath: "",
			Network: &executor.Network{
				Properties: map[string]string{},
			},
		}

		req := &executor.RunRequest{
			Guid:    guid,
			RunInfo: runInfo,
			Tags:    executor.Tags{},
		}

		err := containerStore.Initialize(logger, req)
		Expect(err).ToNot(HaveOccurred())
	}

	Describe("Metrics", func() {
		var (
			containerGuid1, containerGuid2, containerGuid3, containerGuid4 string
			containerGuid5, containerGuid6                                 string
		)

		BeforeEach(func() {
			containerGuid1 = "container-guid-1"
			containerGuid2 = "container-guid-2"
			containerGuid3 = "container-guid-3"
			containerGuid4 = "container-guid-4"
			containerGuid5 = "container-guid-5"
			containerGuid6 = "container-guid-6"

			reserveContainer(containerGuid1)
			reserveContainer(containerGuid2)
			reserveContainer(containerGuid3)
			reserveContainer(containerGuid4)
			reserveContainer(containerGuid5)
			reserveContainer(containerGuid6)

			initializeContainer(containerGuid1)
			initializeContainer(containerGuid2)
			initializeContainer(containerGuid3)
			initializeContainer(containerGuid4)

			gardenContainer.InfoReturns(garden.ContainerInfo{ExternalIP: "6.6.6.6"}, nil)
			gardenClient.CreateReturns(gardenContainer, nil)
			_, err := containerStore.Create(logger, containerGuid1)
			Expect(err).NotTo(HaveOccurred())
			_, err = containerStore.Create(logger, containerGuid2)
			Expect(err).ToNot(HaveOccurred())
			_, err = containerStore.Create(logger, containerGuid3)
			Expect(err).ToNot(HaveOccurred())
			_, err = containerStore.Create(logger, containerGuid4)
			Expect(err).ToNot(HaveOccurred())

			bulkMetrics := map[string]garden.ContainerMetricsEntry{
				containerGuid1: garden.ContainerMetricsEntry{
					Metrics: garden.Metrics{
						MemoryStat: garden.ContainerMemoryStat{
							TotalUsageTowardLimit: 1024,
						},
						DiskStat: garden.ContainerDiskStat{
							ExclusiveBytesUsed: 2048,
						},
						CPUStat: garden.ContainerCPUStat{
							Usage: 5000000000,
						},
					},
				},
				containerGuid2: garden.ContainerMetricsEntry{
					Metrics: garden.Metrics{
						MemoryStat: garden.ContainerMemoryStat{
							TotalUsageTowardLimit: 512,
						},
						DiskStat: garden.ContainerDiskStat{
							ExclusiveBytesUsed: 128,
						},
						CPUStat: garden.ContainerCPUStat{
							Usage: 1000000,
						},
					},
				},
				containerGuid4: garden.ContainerMetricsEntry{
					Err: &garden.Error{Err: errors.New("no-metrics-here")},
					Metrics: garden.Metrics{
						MemoryStat: garden.ContainerMemoryStat{
							TotalUsageTowardLimit: 512,
						},
						DiskStat: garden.ContainerDiskStat{
							ExclusiveBytesUsed: 128,
						},
						CPUStat: garden.ContainerCPUStat{
							Usage: 1000000,
						},
					},
				},
				"BOGUS-GUID": garden.ContainerMetricsEntry{},
			}
			gardenClient.BulkMetricsReturns(bulkMetrics, nil)
		})

		It("returns metrics for all known containers in the running and created state", func() {
			metrics, err := containerStore.Metrics(logger)
			Expect(err).NotTo(HaveOccurred())
			containerSpec1 := gardenClient.CreateArgsForCall(0)
			containerSpec2 := gardenClient.CreateArgsForCall(1)

			Expect(gardenClient.BulkMetricsCallCount()).To(Equal(1))
			Expect(gardenClient.BulkMetricsArgsForCall(0)).To(ConsistOf(
				containerGuid1, containerGuid2, containerGuid3, containerGuid4,
			))

			Expect(metrics).To(HaveLen(2))

			container1Metrics, ok := metrics[containerGuid1]
			Expect(ok).To(BeTrue())
			Expect(container1Metrics.MemoryUsageInBytes).To(BeEquivalentTo(1024))
			Expect(container1Metrics.DiskUsageInBytes).To(BeEquivalentTo(2048))
			Expect(container1Metrics.MemoryLimitInBytes).To(BeEquivalentTo(containerSpec1.Limits.Memory.LimitInBytes))
			Expect(container1Metrics.DiskLimitInBytes).To(BeEquivalentTo(containerSpec1.Limits.Disk.ByteHard))
			Expect(container1Metrics.TimeSpentInCPU).To(Equal(5 * time.Second))

			container2Metrics, ok := metrics[containerGuid2]
			Expect(ok).To(BeTrue())
			Expect(container2Metrics.MemoryUsageInBytes).To(BeEquivalentTo(512))
			Expect(container2Metrics.DiskUsageInBytes).To(BeEquivalentTo(128))
			Expect(container2Metrics.MemoryLimitInBytes).To(BeEquivalentTo(containerSpec2.Limits.Memory.LimitInBytes))
			Expect(container2Metrics.DiskLimitInBytes).To(BeEquivalentTo(containerSpec2.Limits.Disk.ByteHard))
			Expect(container2Metrics.TimeSpentInCPU).To(Equal(1 * time.Millisecond))
		})

		Context("when fetching bulk metrics fails", func() {
			BeforeEach(func() {
				gardenClient.BulkMetricsReturns(nil, errors.New("failed-bulk-metrics"))
			})

			It("returns an error", func() {
				_, err := containerStore.Metrics(logger)
				Expect(err).To(Equal(errors.New("failed-bulk-metrics")))
			})
		})
	})

	Describe("GetFiles", func() {
		BeforeEach(func() {
			gardenClient.CreateReturns(gardenContainer, nil)
			gardenContainer.StreamOutReturns(ioutil.NopCloser(bytes.NewReader([]byte("this is the stream"))), nil)
		})

		JustBeforeEach(func() {
			_, err := containerStore.Reserve(logger, &executor.AllocationRequest{Guid: containerGuid})
			Expect(err).NotTo(HaveOccurred())
		})

		Context("when the container has a corresponding garden container", func() {
			JustBeforeEach(func() {
				err := containerStore.Initialize(logger, &executor.RunRequest{Guid: containerGuid})
				Expect(err).NotTo(HaveOccurred())

				_, err = containerStore.Create(logger, containerGuid)
				Expect(err).NotTo(HaveOccurred())
			})

			It("calls streamout on the garden client", func() {
				stream, err := containerStore.GetFiles(logger, containerGuid, "/path/to/file")
				Expect(err).NotTo(HaveOccurred())

				Expect(gardenContainer.StreamOutCallCount()).To(Equal(1))
				streamOutSpec := gardenContainer.StreamOutArgsForCall(0)
				Expect(streamOutSpec.Path).To(Equal("/path/to/file"))
				Expect(streamOutSpec.User).To(Equal("root"))

				output := make([]byte, len("this is the stream"))
				_, err = stream.Read(output)
				Expect(err).NotTo(HaveOccurred())
				Expect(output).To(Equal([]byte("this is the stream")))
			})
		})

		Context("when the container does not have a corresponding garden container", func() {
			It("returns an error", func() {
				_, err := containerStore.GetFiles(logger, containerGuid, "/path")
				Expect(err).To(Equal(executor.ErrContainerNotFound))
			})
		})

		Context("when the container does not exist", func() {
			It("returns ErrContainerNotFound", func() {
				_, err := containerStore.GetFiles(logger, "", "/stuff")
				Expect(err).To(Equal(executor.ErrContainerNotFound))
			})
		})
	})

	Describe("RegistryPruner", func() {
		var (
			expirationTime time.Duration
			process        ifrit.Process
			resource       executor.Resource
		)

		BeforeEach(func() {
			resource = executor.NewResource(512, 512, 1024, "")
			req := executor.NewAllocationRequest("forever-reserved", &resource, nil)

			_, err := containerStore.Reserve(logger, &req)
			Expect(err).NotTo(HaveOccurred())

			resource = executor.NewResource(512, 512, 1024, "")
			req = executor.NewAllocationRequest("eventually-initialized", &resource, nil)

			_, err = containerStore.Reserve(logger, &req)
			Expect(err).NotTo(HaveOccurred())

			runReq := executor.NewRunRequest("eventually-initialized", &executor.RunInfo{}, executor.Tags{})
			err = containerStore.Initialize(logger, &runReq)
			Expect(err).NotTo(HaveOccurred())

			expirationTime = 20 * time.Millisecond

			pruner := containerStore.NewRegistryPruner(logger)
			process = ginkgomon.Invoke(pruner)
		})

		AfterEach(func() {
			ginkgomon.Interrupt(process)
		})

		Context("when the elapsed time is less than expiration period", func() {
			BeforeEach(func() {
				clock.Increment(expirationTime / 2)
			})

			It("still has all the containers in the list", func() {
				Consistently(func() []executor.Container {
					return containerStore.List(logger)
				}).Should(HaveLen(2))

				resources := containerStore.RemainingResources(logger)
				expectedResources := totalCapacity.Copy()
				expectedResources.Subtract(&resource)
				expectedResources.Subtract(&resource)
				Expect(resources).To(Equal(expectedResources))
			})
		})

		Context("when the elapsed time is more than expiration period", func() {
			BeforeEach(func() {
				clock.Increment(2 * expirationTime)
			})

			It("completes only RESERVED containers from the list", func() {
				Eventually(func() executor.State {
					container, err := containerStore.Get(logger, "forever-reserved")
					Expect(err).NotTo(HaveOccurred())
					return container.State
				}).Should(Equal(executor.StateCompleted))

				Consistently(func() executor.State {
					container, err := containerStore.Get(logger, "eventually-initialized")
					Expect(err).NotTo(HaveOccurred())
					return container.State
				}).ShouldNot(Equal(executor.StateCompleted))
			})
		})
	})

	Describe("ContainerReaper", func() {
		var (
			containerGuid1, containerGuid2, containerGuid3 string
			containerGuid4, containerGuid5, containerGuid6 string
			process                                        ifrit.Process
			extraGardenContainer                           *gardenfakes.FakeContainer
		)

		BeforeEach(func() {
			gardenClient.CreateReturns(gardenContainer, nil)

			containerGuid1 = "container-guid-1"
			containerGuid2 = "container-guid-2"
			containerGuid3 = "container-guid-3"
			containerGuid4 = "container-guid-4"
			containerGuid5 = "container-guid-5"
			containerGuid6 = "container-guid-6"

			// Reserve
			_, err := containerStore.Reserve(logger, &executor.AllocationRequest{Guid: containerGuid1})
			Expect(err).NotTo(HaveOccurred())
			_, err = containerStore.Reserve(logger, &executor.AllocationRequest{Guid: containerGuid2})
			Expect(err).NotTo(HaveOccurred())
			_, err = containerStore.Reserve(logger, &executor.AllocationRequest{Guid: containerGuid3})
			Expect(err).NotTo(HaveOccurred())
			_, err = containerStore.Reserve(logger, &executor.AllocationRequest{Guid: containerGuid4})
			Expect(err).NotTo(HaveOccurred())
			_, err = containerStore.Reserve(logger, &executor.AllocationRequest{Guid: containerGuid5})
			Expect(err).NotTo(HaveOccurred())
			_, err = containerStore.Reserve(logger, &executor.AllocationRequest{Guid: containerGuid6})
			Expect(err).NotTo(HaveOccurred())

			// Initialize
			err = containerStore.Initialize(logger, &executor.RunRequest{Guid: containerGuid2})
			Expect(err).NotTo(HaveOccurred())
			err = containerStore.Initialize(logger, &executor.RunRequest{Guid: containerGuid3})
			Expect(err).NotTo(HaveOccurred())
			err = containerStore.Initialize(logger, &executor.RunRequest{Guid: containerGuid4})
			Expect(err).NotTo(HaveOccurred())
			err = containerStore.Initialize(logger, &executor.RunRequest{Guid: containerGuid5})
			Expect(err).NotTo(HaveOccurred())
			err = containerStore.Initialize(logger, &executor.RunRequest{Guid: containerGuid6})
			Expect(err).NotTo(HaveOccurred())

			// Create Containers
			_, err = containerStore.Create(logger, containerGuid3)
			Expect(err).NotTo(HaveOccurred())
			_, err = containerStore.Create(logger, containerGuid4)
			Expect(err).NotTo(HaveOccurred())
			_, err = containerStore.Create(logger, containerGuid5)
			Expect(err).NotTo(HaveOccurred())

			// Stop One of the containers
			err = containerStore.Stop(logger, containerGuid6)
			Expect(err).NotTo(HaveOccurred())

			Eventually(eventEmitter.EmitCallCount).Should(Equal(7))

			extraGardenContainer = &gardenfakes.FakeContainer{}
			extraGardenContainer.HandleReturns("foobar")
			gardenContainer.HandleReturns(containerGuid3)
			gardenContainers := []garden.Container{gardenContainer, extraGardenContainer}
			gardenClient.ContainersReturns(gardenContainers, nil)
		})

		JustBeforeEach(func() {
			reaper := containerStore.NewContainerReaper(logger)
			process = ginkgomon.Invoke(reaper)
		})

		AfterEach(func() {
			ginkgomon.Interrupt(process)
		})

		It("marks containers completed that no longer have corresponding garden containers", func() {
			initialEmitCallCount := eventEmitter.EmitCallCount()

			clock.WaitForWatcherAndIncrement(30 * time.Millisecond)

			Eventually(func() executor.State {
				container, err := containerStore.Get(logger, containerGuid4)
				Expect(err).NotTo(HaveOccurred())
				return container.State
			}).Should(Equal(executor.StateCompleted))

			Eventually(func() executor.State {
				container, err := containerStore.Get(logger, containerGuid5)
				Expect(err).NotTo(HaveOccurred())
				return container.State
			}).Should(Equal(executor.StateCompleted))

			Eventually(eventEmitter.EmitCallCount).Should(Equal(initialEmitCallCount + 2))

			container4, err := containerStore.Get(logger, containerGuid4)
			Expect(err).NotTo(HaveOccurred())
			container5, err := containerStore.Get(logger, containerGuid5)
			Expect(err).NotTo(HaveOccurred())

			var events []executor.Event
			events = append(events, eventEmitter.EmitArgsForCall(initialEmitCallCount))
			events = append(events, eventEmitter.EmitArgsForCall(initialEmitCallCount+1))

			Expect(events).To(ContainElement(executor.ContainerCompleteEvent{RawContainer: container4}))
			Expect(events).To(ContainElement(executor.ContainerCompleteEvent{RawContainer: container5}))

			Expect(gardenClient.ContainersCallCount()).To(Equal(2))

			properties := gardenClient.ContainersArgsForCall(0)
			Expect(properties[containerstore.ContainerOwnerProperty]).To(Equal(ownerName))
			properties = gardenClient.ContainersArgsForCall(1)
			Expect(properties[containerstore.ContainerOwnerProperty]).To(Equal(ownerName))

			clock.WaitForWatcherAndIncrement(30 * time.Millisecond)

			Eventually(gardenClient.ContainersCallCount).Should(Equal(4))
		})

		Context("when listing containers in garden fails", func() {
			BeforeEach(func() {
				gardenClient.ContainersReturns([]garden.Container{}, errors.New("failed-to-list"))
			})

			It("logs the failure and continues", func() {
				clock.Increment(30 * time.Millisecond)
				Eventually(logger).Should(gbytes.Say("failed-to-fetch-containers"))

				Consistently(func() []executor.Container {
					return containerStore.List(logger)
				}).Should(HaveLen(6))
			})
		})

		Context("when destroying the extra container fails", func() {
			BeforeEach(func() {
				gardenClient.DestroyReturns(errors.New("failed-to-destroy"))
			})

			It("logs the error and continues", func() {
				clock.Increment(30 * time.Millisecond)
				Eventually(logger).Should(gbytes.Say("failed-to-destroy-container"))
			})
		})
	})
})
