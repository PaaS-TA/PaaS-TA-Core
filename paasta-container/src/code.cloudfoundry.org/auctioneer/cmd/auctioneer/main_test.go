package main_test

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net"
	"os"
	"os/exec"
	"time"

	"code.cloudfoundry.org/auctioneer"
	"code.cloudfoundry.org/auctioneer/cmd/auctioneer/config"
	"code.cloudfoundry.org/bbs"
	"code.cloudfoundry.org/bbs/models"
	"code.cloudfoundry.org/bbs/models/test/model_helpers"
	"code.cloudfoundry.org/clock"
	"code.cloudfoundry.org/durationjson"
	"code.cloudfoundry.org/localip"
	"code.cloudfoundry.org/locket"
	locketconfig "code.cloudfoundry.org/locket/cmd/locket/config"
	locketrunner "code.cloudfoundry.org/locket/cmd/locket/testrunner"
	"code.cloudfoundry.org/locket/lock"
	locketmodels "code.cloudfoundry.org/locket/models"
	"code.cloudfoundry.org/rep"
	"github.com/hashicorp/consul/api"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	. "github.com/onsi/gomega/gexec"
	"github.com/tedsuo/ifrit"
	"github.com/tedsuo/ifrit/ginkgomon"
)

var dummyAction = &models.RunAction{
	User: "me",
	Path: "cat",
	Args: []string{"/tmp/file"},
}

var exampleDesiredLRP = models.DesiredLRP{
	ProcessGuid: "process-guid",
	DiskMb:      1,
	MemoryMb:    1,
	MaxPids:     1,
	RootFs:      linuxRootFSURL,
	Action:      models.WrapAction(dummyAction),
	Domain:      "test",
	Instances:   2,
}

func exampleTaskDefinition() *models.TaskDefinition {
	taskDef := model_helpers.NewValidTaskDefinition()
	taskDef.RootFs = linuxRootFSURL
	taskDef.Action = models.WrapAction(dummyAction)
	taskDef.PlacementTags = nil
	return taskDef
}

var _ = Describe("Auctioneer", func() {
	var (
		auctioneerConfig config.AuctioneerConfig

		runner            *ginkgomon.Runner
		auctioneerProcess ifrit.Process

		auctioneerClient auctioneer.Client

		locketRunner  ifrit.Runner
		locketProcess ifrit.Process
		locketAddress string
	)

	BeforeEach(func() {
		auctioneerConfig = config.AuctioneerConfig{
			BBSAddress:        bbsURL.String(),
			ListenAddress:     auctioneerLocation,
			LockRetryInterval: durationjson.Duration(time.Second),
			ConsulCluster:     consulRunner.ConsulCluster(),
			UUID:              "auctioneer-boshy-bosh",
		}
		auctioneerClient = auctioneer.NewClient("http://" + auctioneerLocation)
	})

	JustBeforeEach(func() {
		configFile, err := ioutil.TempFile("", "auctioneer-config")
		Expect(err).NotTo(HaveOccurred())

		encoder := json.NewEncoder(configFile)
		err = encoder.Encode(&auctioneerConfig)
		Expect(err).NotTo(HaveOccurred())

		runner = ginkgomon.New(ginkgomon.Config{
			Name: "auctioneer",
			Command: exec.Command(
				auctioneerPath,
				"-config", configFile.Name(),
			),
			StartCheck: "auctioneer.started",
			Cleanup: func() {
				os.RemoveAll(configFile.Name())
			},
		})
	})

	AfterEach(func() {
		ginkgomon.Interrupt(locketProcess)
		ginkgomon.Interrupt(auctioneerProcess)
	})

	Context("when the bbs is down", func() {
		BeforeEach(func() {
			ginkgomon.Interrupt(bbsProcess)
		})

		It("starts", func() {
			auctioneerProcess = ginkgomon.Invoke(runner)
			Consistently(runner).ShouldNot(Exit())
		})
	})

	Context("when the auctioneer starts up", func() {
		It("registers itself as a service and registers a TTL Healthcheck", func() {
			auctioneerProcess = ginkgomon.Invoke(runner)

			client := consulRunner.NewClient()
			services, err := client.Agent().Services()
			Expect(err).NotTo(HaveOccurred())
			Expect(services).To(HaveKeyWithValue("auctioneer", &api.AgentService{
				ID:      "auctioneer",
				Service: "auctioneer",
				Port:    auctioneerServerPort,
				Address: "",
			}))

			checks, err := client.Agent().Checks()
			Expect(err).NotTo(HaveOccurred())
			Expect(checks).To(HaveKeyWithValue("service:auctioneer", &api.AgentCheck{
				Node:        "0",
				CheckID:     "service:auctioneer",
				Name:        "Service 'auctioneer' check",
				Status:      "passing",
				Notes:       "",
				Output:      "",
				ServiceID:   "auctioneer",
				ServiceName: "auctioneer",
			}))
		})

		Context("when a debug address is specified", func() {
			BeforeEach(func() {
				auctioneerConfig.DebugAddress = fmt.Sprintf("0.0.0.0:%d", 1234+GinkgoParallelNode())
			})

			It("starts the debug server", func() {
				auctioneerProcess = ginkgomon.Invoke(runner)

				_, err := net.Dial("tcp", auctioneerConfig.DebugAddress)
				Expect(err).NotTo(HaveOccurred())
			})
		})
	})

	Context("when a start auction message arrives", func() {
		It("should start the process running on reps of the appropriate stack", func() {
			auctioneerProcess = ginkgomon.Invoke(runner)

			err := auctioneerClient.RequestLRPAuctions(logger, []*auctioneer.LRPStartRequest{{
				ProcessGuid: exampleDesiredLRP.ProcessGuid,
				Domain:      exampleDesiredLRP.Domain,
				Indices:     []int{0},
				Resource: rep.Resource{
					MemoryMB: 5,
					DiskMB:   5,
				},
				PlacementConstraint: rep.PlacementConstraint{
					RootFs: exampleDesiredLRP.RootFs,
				},
			}})
			Expect(err).NotTo(HaveOccurred())

			err = auctioneerClient.RequestLRPAuctions(logger, []*auctioneer.LRPStartRequest{{
				ProcessGuid: exampleDesiredLRP.ProcessGuid,
				Domain:      exampleDesiredLRP.Domain,
				Indices:     []int{1},
				Resource: rep.Resource{
					MemoryMB: 5,
					DiskMB:   5,
				},
				PlacementConstraint: rep.PlacementConstraint{
					RootFs: exampleDesiredLRP.RootFs,
				},
			}})
			Expect(err).NotTo(HaveOccurred())
			Eventually(linuxCell.LRPs).Should(HaveLen(2))
			Expect(dotNetCell.LRPs()).To(BeEmpty())
		})
	})

	Context("when exceeding max inflight container counts", func() {
		BeforeEach(func() {
			auctioneerConfig.StartingContainerCountMaximum = 1
		})

		It("should only start up to the max inflight processes", func() {
			auctioneerProcess = ginkgomon.Invoke(runner)

			err := auctioneerClient.RequestLRPAuctions(logger, []*auctioneer.LRPStartRequest{{
				ProcessGuid: exampleDesiredLRP.ProcessGuid,
				Domain:      exampleDesiredLRP.Domain,
				Indices:     []int{0},
				Resource: rep.Resource{
					MemoryMB: 5,
					DiskMB:   5,
				},
				PlacementConstraint: rep.PlacementConstraint{
					RootFs: exampleDesiredLRP.RootFs,
				},
			}})

			Expect(err).NotTo(HaveOccurred())

			err = auctioneerClient.RequestLRPAuctions(logger, []*auctioneer.LRPStartRequest{{
				ProcessGuid: exampleDesiredLRP.ProcessGuid,
				Domain:      exampleDesiredLRP.Domain,
				Indices:     []int{1},
				Resource: rep.Resource{
					MemoryMB: 5,
					DiskMB:   5,
				},
			}})
			Expect(err).NotTo(HaveOccurred())

			Eventually(linuxCell.LRPs).Should(HaveLen(1))
		})
	})

	Context("when a task message arrives", func() {
		Context("when there are sufficient resources to start the task", func() {
			It("should start the task running on reps of the appropriate stack", func() {
				auctioneerProcess = ginkgomon.Invoke(runner)

				taskDef := exampleTaskDefinition()
				taskDef.DiskMb = 1
				taskDef.MemoryMb = 1
				taskDef.MaxPids = 1
				err := bbsClient.DesireTask(logger, "guid", "domain", taskDef)
				Expect(err).NotTo(HaveOccurred())

				Eventually(linuxCell.Tasks).Should(HaveLen(1))
				Expect(dotNetCell.Tasks()).To(BeEmpty())
			})
		})

		Context("when there are insufficient resources to start the task", func() {
			var taskDef *models.TaskDefinition

			BeforeEach(func() {
				taskDef = exampleTaskDefinition()
				taskDef.DiskMb = 1000
				taskDef.MemoryMb = 1000
				taskDef.MaxPids = 1000
			})

			It("should not place the tasks and mark the task as failed in the BBS", func() {
				auctioneerProcess = ginkgomon.Invoke(runner)

				err := bbsClient.DesireTask(logger, "task-guid", "domain", taskDef)
				Expect(err).NotTo(HaveOccurred())

				Consistently(linuxCell.Tasks).Should(BeEmpty())
				Consistently(dotNetCell.Tasks).Should(BeEmpty())

				Eventually(func() []*models.Task {
					return getTasksByState(bbsClient, models.Task_Completed)
				}).Should(HaveLen(1))

				completedTasks := getTasksByState(bbsClient, models.Task_Completed)
				completedTask := completedTasks[0]
				Expect(completedTask.TaskGuid).To(Equal("task-guid"))
				Expect(completedTask.Failed).To(BeTrue())
				Expect(completedTask.FailureReason).To(Equal("insufficient resources: disk, memory"))
			})
		})
	})

	Context("when the auctioneer loses the consul lock", func() {
		It("exits with an error", func() {
			auctioneerProcess = ginkgomon.Invoke(runner)

			consulRunner.Reset()

			Eventually(runner.ExitCode, 3).Should(Equal(1))
		})
	})

	Context("when the auctioneer cannot acquire the consul lock on startup", func() {
		var (
			task                       *rep.Task
			competingAuctioneerProcess ifrit.Process
		)

		JustBeforeEach(func() {
			task = &rep.Task{
				TaskGuid: "task-guid",
				Domain:   "test",
				Resource: rep.Resource{
					MemoryMB: 124,
					DiskMB:   456,
				},
				PlacementConstraint: rep.PlacementConstraint{
					RootFs: "some-rootfs",
				},
			}

			competingAuctioneerLock := locket.NewLock(logger, consulClient, locket.LockSchemaPath("auctioneer_lock"), []byte{}, clock.NewClock(), 500*time.Millisecond, 10*time.Second)
			competingAuctioneerProcess = ifrit.Invoke(competingAuctioneerLock)

			auctioneerProcess = ifrit.Background(runner)
		})

		AfterEach(func() {
			ginkgomon.Kill(competingAuctioneerProcess)
		})

		It("should not advertise its presence, and should not be reachable", func() {
			Consistently(func() error {
				return auctioneerClient.RequestTaskAuctions(logger, []*auctioneer.TaskStartRequest{
					&auctioneer.TaskStartRequest{*task},
				})
			}).Should(HaveOccurred())
		})

		It("should eventually come up in the event that the lock is released", func() {
			ginkgomon.Kill(competingAuctioneerProcess)

			Eventually(func() error {
				return auctioneerClient.RequestTaskAuctions(logger, []*auctioneer.TaskStartRequest{
					&auctioneer.TaskStartRequest{*task},
				})
			}).ShouldNot(HaveOccurred())
		})
	})

	Context("when the auctioneer is configured to grab the lock from the sql locking server", func() {
		var (
			task                       *rep.Task
			competingAuctioneerProcess ifrit.Process
		)

		BeforeEach(func() {
			task = &rep.Task{
				TaskGuid: "task-guid",
				Domain:   "test",
				Resource: rep.Resource{
					MemoryMB: 124,
					DiskMB:   456,
				},
				PlacementConstraint: rep.PlacementConstraint{
					RootFs: "some-rootfs",
				},
			}

			locketPort, err := localip.LocalPort()
			Expect(err).NotTo(HaveOccurred())
			locketAddress = fmt.Sprintf("localhost:%d", locketPort)

			locketRunner = locketrunner.NewLocketRunner(locketBinPath, func(cfg *locketconfig.LocketConfig) {
				cfg.ConsulCluster = consulRunner.ConsulCluster()
				cfg.DatabaseConnectionString = sqlRunner.ConnectionString()
				cfg.DatabaseDriver = sqlRunner.DriverName()
				cfg.ListenAddress = locketAddress
			})
			locketProcess = ginkgomon.Invoke(locketRunner)

			auctioneerConfig.ClientLocketConfig = locketrunner.ClientLocketConfig()
			auctioneerConfig.LocketAddress = locketAddress
		})

		JustBeforeEach(func() {
			auctioneerProcess = ifrit.Background(runner)
		})

		AfterEach(func() {
			ginkgomon.Interrupt(auctioneerProcess)
		})

		It("acquires the lock and becomes active", func() {
			Eventually(func() error {
				return auctioneerClient.RequestTaskAuctions(logger, []*auctioneer.TaskStartRequest{
					&auctioneer.TaskStartRequest{*task},
				})
			}).ShouldNot(HaveOccurred())
		})

		It("uses the configured UUID as the owner", func() {
			locketClient, err := locket.NewClient(logger, auctioneerConfig.ClientLocketConfig)
			Expect(err).NotTo(HaveOccurred())

			var lock *locketmodels.FetchResponse
			Eventually(func() error {
				lock, err = locketClient.Fetch(context.Background(), &locketmodels.FetchRequest{
					Key: "auctioneer",
				})
				return err
			}).ShouldNot(HaveOccurred())

			Expect(lock.Resource.Owner).To(Equal(auctioneerConfig.UUID))
		})

		Context("and the locking server becomes unreachable after grabbing the lock", func() {
			It("exits", func() {
				ginkgomon.Interrupt(locketProcess)
				Eventually(auctioneerProcess.Wait()).Should(Receive())
			})
		})

		Context("when the consul lock is not required", func() {
			BeforeEach(func() {
				auctioneerConfig.SkipConsulLock = true

				competingAuctioneerLock := locket.NewLock(logger, consulClient, locket.LockSchemaPath("auctioneer_lock"), []byte{}, clock.NewClock(), 500*time.Millisecond, 10*time.Second)
				competingAuctioneerProcess = ifrit.Invoke(competingAuctioneerLock)
			})

			AfterEach(func() {
				ginkgomon.Interrupt(competingAuctioneerProcess)
			})

			It("only grabs the sql lock and starts succesfully", func() {
				Eventually(func() error {
					return auctioneerClient.RequestTaskAuctions(logger, []*auctioneer.TaskStartRequest{
						&auctioneer.TaskStartRequest{*task},
					})
				}).ShouldNot(HaveOccurred())
			})
		})

		Context("when the lock is not available", func() {
			var competingProcess ifrit.Process

			BeforeEach(func() {
				locketClient, err := locket.NewClient(logger, auctioneerConfig.ClientLocketConfig)
				Expect(err).NotTo(HaveOccurred())

				lockIdentifier := &locketmodels.Resource{
					Key:      "auctioneer",
					Owner:    "Your worst enemy.",
					Value:    "Something",
					TypeCode: locketmodels.LOCK,
				}

				clock := clock.NewClock()
				competingRunner := lock.NewLockRunner(
					logger,
					locketClient,
					lockIdentifier,
					locket.DefaultSessionTTLInSeconds,
					clock,
					locket.RetryInterval,
				)
				competingProcess = ginkgomon.Invoke(competingRunner)
			})

			AfterEach(func() {
				ginkgomon.Interrupt(competingProcess)
			})

			It("starts but does not accept auctions", func() {
				Consistently(func() error {
					return auctioneerClient.RequestTaskAuctions(logger, []*auctioneer.TaskStartRequest{
						&auctioneer.TaskStartRequest{*task},
					})
				}).Should(HaveOccurred())
			})

			Context("and continues to be unavailable", func() {
				It("exits", func() {
					Eventually(auctioneerProcess.Wait(), locket.DefaultSessionTTL*2).Should(Receive())
				})
			})

			Context("and the lock becomes available", func() {
				JustBeforeEach(func() {
					Eventually(runner.Buffer()).Should(gbytes.Say(
						"failed-to-acquire-lock"))
					ginkgomon.Interrupt(competingProcess)
				})

				It("acquires the lock and becomes active", func() {
					Eventually(func() error {
						return auctioneerClient.RequestTaskAuctions(logger, []*auctioneer.TaskStartRequest{
							&auctioneer.TaskStartRequest{*task},
						})
					}, 2*time.Second).ShouldNot(HaveOccurred())
				})
			})
		})

		Context("and the locket address is invalid", func() {
			BeforeEach(func() {
				auctioneerConfig.LocketAddress = "{{{}}}}{{{{"
			})

			It("exits with an error", func() {
				Eventually(auctioneerProcess.Wait()).Should(Receive())
			})
		})

		Context("and the UUID is not present", func() {
			BeforeEach(func() {
				auctioneerConfig.UUID = ""
			})

			It("exits with an error", func() {
				Eventually(auctioneerProcess.Wait()).Should(Receive())
			})
		})

		Context("when neither lock is configured", func() {
			BeforeEach(func() {
				auctioneerConfig.LocketAddress = ""
				auctioneerConfig.SkipConsulLock = true
			})

			It("exits with an error", func() {
				Eventually(auctioneerProcess.Wait()).Should(Receive())
			})
		})
	})

	Context("when the auctioneer is configured with TLS options", func() {
		var caCertFile, serverCertFile, serverKeyFile string

		BeforeEach(func() {
			caCertFile = "fixtures/green-certs/ca.crt"
			serverCertFile = "fixtures/green-certs/server.crt"
			serverKeyFile = "fixtures/green-certs/server.key"

			auctioneerConfig.CACertFile = caCertFile
			auctioneerConfig.ServerCertFile = serverCertFile
			auctioneerConfig.ServerKeyFile = serverKeyFile
		})

		JustBeforeEach(func() {
			auctioneerProcess = ifrit.Background(runner)
		})

		AfterEach(func() {
			ginkgomon.Interrupt(auctioneerProcess)
		})

		Context("when invalid values for the certificates are supplied", func() {
			BeforeEach(func() {
				auctioneerConfig.CACertFile = caCertFile
				auctioneerConfig.ServerCertFile = "invalid-certs/server.cr"
				auctioneerConfig.ServerKeyFile = serverKeyFile
			})

			It("fails", func() {
				Eventually(runner.Buffer()).Should(gbytes.Say(
					"invalid-tls-config"))
				Eventually(runner.ExitCode()).ShouldNot(Equal(0))
			})
		})

		Context("when invalid combinations of the certificates are supplied", func() {
			Context("when the server cert file isn't specified", func() {
				BeforeEach(func() {
					auctioneerConfig.CACertFile = caCertFile
					auctioneerConfig.ServerCertFile = ""
					auctioneerConfig.ServerKeyFile = serverKeyFile
				})

				It("fails", func() {
					Eventually(runner.Buffer()).Should(gbytes.Say(
						"invalid-tls-config"))
					Eventually(runner.ExitCode()).ShouldNot(Equal(0))
				})
			})

			Context("when the server cert file and server key file aren't specified", func() {
				BeforeEach(func() {
					auctioneerConfig.CACertFile = caCertFile
					auctioneerConfig.ServerCertFile = ""
					auctioneerConfig.ServerKeyFile = ""
				})

				It("fails", func() {
					Eventually(runner.Buffer()).Should(gbytes.Say(
						"invalid-tls-config"))
					Eventually(runner.ExitCode()).ShouldNot(Equal(0))
				})
			})

			Context("when the server key file isn't specified", func() {
				BeforeEach(func() {
					auctioneerConfig.CACertFile = caCertFile
					auctioneerConfig.ServerCertFile = serverCertFile
					auctioneerConfig.ServerKeyFile = ""
				})

				It("fails", func() {
					Eventually(runner.Buffer()).Should(gbytes.Say(
						"invalid-tls-config"))
					Eventually(runner.ExitCode()).ShouldNot(Equal(0))
				})
			})
		})

		Context("when the server key and the CA cert don't match", func() {
			BeforeEach(func() {
				auctioneerConfig.CACertFile = caCertFile
				auctioneerConfig.ServerCertFile = serverCertFile
				auctioneerConfig.ServerKeyFile = "fixtures/blue-certs/server.key"
			})

			It("fails", func() {
				Eventually(runner.Buffer()).Should(gbytes.Say(
					"invalid-tls-config"))
				Eventually(runner.ExitCode()).ShouldNot(Equal(0))
			})
		})

		Context("when correct TLS options are supplied", func() {
			It("starts", func() {
				Eventually(auctioneerProcess.Ready()).Should(BeClosed())
				Consistently(runner).ShouldNot(Exit())
			})

			It("responds successfully to a TLS client", func() {
				Eventually(auctioneerProcess.Ready()).Should(BeClosed())

				secureAuctioneerClient, err := auctioneer.NewSecureClient("https://"+auctioneerLocation, caCertFile, serverCertFile, serverKeyFile, false)
				Expect(err).NotTo(HaveOccurred())

				err = secureAuctioneerClient.RequestLRPAuctions(logger, nil)
				Expect(err).NotTo(HaveOccurred())
			})
		})
	})

	Context("Auctioneer Client", func() {
		var client auctioneer.Client

		JustBeforeEach(func() {
			auctioneerProcess = ginkgomon.Invoke(runner)
		})

		Context("when the auctioneer is configured with TLS", func() {
			BeforeEach(func() {
				auctioneerConfig.CACertFile = "fixtures/green-certs/ca.crt"
				auctioneerConfig.ServerCertFile = "fixtures/green-certs/server.crt"
				auctioneerConfig.ServerKeyFile = "fixtures/green-certs/server.key"
			})

			Context("and the auctioneer client is not configured with TLS", func() {
				BeforeEach(func() {
					client = auctioneer.NewClient("http://" + auctioneerLocation)
				})

				It("does not work", func() {
					err := client.RequestLRPAuctions(logger, []*auctioneer.LRPStartRequest{})
					Expect(err).To(HaveOccurred())

					err = client.RequestTaskAuctions(logger, []*auctioneer.TaskStartRequest{})
					Expect(err).To(HaveOccurred())
				})
			})

			Context("and the auctioneer client is configured with tls", func() {
				BeforeEach(func() {
					var err error
					client, err = auctioneer.NewSecureClient(
						"https://"+auctioneerLocation,
						"fixtures/green-certs/ca.crt",
						"fixtures/green-certs/client.crt",
						"fixtures/green-certs/client.key",
						true,
					)
					Expect(err).NotTo(HaveOccurred())
				})

				It("works", func() {
					err := client.RequestLRPAuctions(logger, []*auctioneer.LRPStartRequest{})
					Expect(err).NotTo(HaveOccurred())

					err = client.RequestTaskAuctions(logger, []*auctioneer.TaskStartRequest{})
					Expect(err).NotTo(HaveOccurred())
				})
			})
		})

		Context("when the auctioneer is not configured with TLS", func() {
			Context("and the auctioneer client is not configured with TLS", func() {
				BeforeEach(func() {
					client = auctioneer.NewClient("http://" + auctioneerLocation)
				})

				It("works", func() {
					err := client.RequestLRPAuctions(logger, []*auctioneer.LRPStartRequest{})
					Expect(err).NotTo(HaveOccurred())

					err = client.RequestTaskAuctions(logger, []*auctioneer.TaskStartRequest{})
					Expect(err).NotTo(HaveOccurred())
				})
			})

			Context("and the auctioneer client is configured with TLS", func() {
				Context("and the client requires tls", func() {
					BeforeEach(func() {
						var err error
						client, err = auctioneer.NewSecureClient(
							"https://"+auctioneerLocation,
							"fixtures/green-certs/ca.crt",
							"fixtures/green-certs/client.crt",
							"fixtures/green-certs/client.key",
							true,
						)
						Expect(err).NotTo(HaveOccurred())
					})

					It("does not work", func() {
						err := client.RequestLRPAuctions(logger, []*auctioneer.LRPStartRequest{})
						Expect(err).To(HaveOccurred())

						err = client.RequestTaskAuctions(logger, []*auctioneer.TaskStartRequest{})
						Expect(err).To(HaveOccurred())
					})
				})

				Context("and the client does not require tls", func() {
					BeforeEach(func() {
						var err error
						client, err = auctioneer.NewSecureClient(
							"https://"+auctioneerLocation,
							"fixtures/green-certs/ca.crt",
							"fixtures/green-certs/client.crt",
							"fixtures/green-certs/client.key",
							false,
						)
						Expect(err).NotTo(HaveOccurred())
					})

					It("falls back to http and does work", func() {
						err := client.RequestLRPAuctions(logger, []*auctioneer.LRPStartRequest{})
						Expect(err).NotTo(HaveOccurred())

						err = client.RequestTaskAuctions(logger, []*auctioneer.TaskStartRequest{})
						Expect(err).NotTo(HaveOccurred())
					})
				})
			})
		})
	})
})

func getTasksByState(client bbs.InternalClient, state models.Task_State) []*models.Task {
	tasks, err := client.Tasks(logger)
	Expect(err).NotTo(HaveOccurred())

	filteredTasks := make([]*models.Task, 0)
	for _, task := range tasks {
		if task.State == state {
			filteredTasks = append(filteredTasks, task)
		}
	}

	return filteredTasks
}
