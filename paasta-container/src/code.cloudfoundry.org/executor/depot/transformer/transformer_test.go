package transformer_test

import (
	"errors"
	"io/ioutil"
	"os"
	"sync"
	"sync/atomic"
	"time"

	"code.cloudfoundry.org/bbs/models"
	"code.cloudfoundry.org/clock/fakeclock"
	"code.cloudfoundry.org/executor"
	"code.cloudfoundry.org/executor/depot/log_streamer"
	"code.cloudfoundry.org/executor/depot/transformer"
	"code.cloudfoundry.org/garden"
	"code.cloudfoundry.org/garden/gardenfakes"
	mfakes "code.cloudfoundry.org/go-loggregator/testhelpers/fakes/v1"
	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/lager/lagertest"
	"code.cloudfoundry.org/workpool"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/tedsuo/ifrit"
	"github.com/tedsuo/ifrit/ginkgomon"
)

var _ = Describe("Transformer", func() {
	Describe("StepsRunner", func() {
		var (
			logger                      lager.Logger
			optimusPrime                transformer.Transformer
			container                   executor.Container
			logStreamer                 log_streamer.LogStreamer
			gardenContainer             *gardenfakes.FakeContainer
			clock                       *fakeclock.FakeClock
			fakeMetronClient            *mfakes.FakeIngressClient
			healthyMonitoringInterval   time.Duration
			unhealthyMonitoringInterval time.Duration
			healthCheckWorkPool         *workpool.WorkPool
			suppressExitStatusCode      bool
		)

		BeforeEach(func() {
			gardenContainer = &gardenfakes.FakeContainer{}
			fakeMetronClient = &mfakes.FakeIngressClient{}
			suppressExitStatusCode = false

			logger = lagertest.NewTestLogger("test-container-store")
			logStreamer = log_streamer.New("test", "test", 1, fakeMetronClient)

			healthyMonitoringInterval = 1 * time.Second
			unhealthyMonitoringInterval = 1 * time.Millisecond

			var err error
			healthCheckWorkPool, err = workpool.NewWorkPool(10)
			Expect(err).NotTo(HaveOccurred())

			clock = fakeclock.NewFakeClock(time.Now())

			optimusPrime = transformer.NewTransformer(
				nil, nil, nil, nil, nil, nil,
				os.TempDir(),
				false,
				healthyMonitoringInterval,
				unhealthyMonitoringInterval,
				healthCheckWorkPool,
				clock,
				[]string{"/post-setup/path", "-x", "argument"},
				"jim",
				false,
			)

			container = executor.Container{
				RunInfo: executor.RunInfo{
					Setup: &models.Action{
						RunAction: &models.RunAction{
							Path: "/setup/path",
						},
					},
					Action: &models.Action{
						RunAction: &models.RunAction{
							Path: "/action/path",
						},
					},
					Monitor: &models.Action{
						RunAction: &models.RunAction{
							Path: "/monitor/path",
						},
					},
				},
			}
		})

		Context("when there is no run action", func() {
			BeforeEach(func() {
				container.Action = nil
			})

			It("returns an error", func() {
				_, err := optimusPrime.StepsRunner(logger, container, gardenContainer, logStreamer)
				Expect(err).To(HaveOccurred())
			})
		})

		It("returns a step encapsulating setup, post-setup, monitor, and action", func() {
			setupReceived := make(chan struct{})
			postSetupReceived := make(chan struct{})
			monitorProcess := &gardenfakes.FakeProcess{}
			gardenContainer.RunStub = func(processSpec garden.ProcessSpec, processIO garden.ProcessIO) (garden.Process, error) {
				if processSpec.Path == "/setup/path" {
					setupReceived <- struct{}{}
				} else if processSpec.Path == "/post-setup/path" {
					postSetupReceived <- struct{}{}
				} else if processSpec.Path == "/monitor/path" {
					return monitorProcess, nil
				}
				return &gardenfakes.FakeProcess{}, nil
			}

			monitorProcess.WaitStub = func() (int, error) {
				if monitorProcess.WaitCallCount() == 1 {
					return 1, errors.New("boom")
				} else {
					return 0, nil
				}
			}

			runner, err := optimusPrime.StepsRunner(logger, container, gardenContainer, logStreamer)
			Expect(err).NotTo(HaveOccurred())

			process := ifrit.Background(runner)

			Eventually(gardenContainer.RunCallCount).Should(Equal(1))
			processSpec, _ := gardenContainer.RunArgsForCall(0)
			Expect(processSpec.Path).To(Equal("/setup/path"))
			Consistently(gardenContainer.RunCallCount).Should(Equal(1))

			<-setupReceived

			Eventually(gardenContainer.RunCallCount).Should(Equal(2))
			processSpec, _ = gardenContainer.RunArgsForCall(1)
			Expect(processSpec.Path).To(Equal("/post-setup/path"))
			Expect(processSpec.Args).To(Equal([]string{"-x", "argument"}))
			Expect(processSpec.User).To(Equal("jim"))
			Consistently(gardenContainer.RunCallCount).Should(Equal(2))

			<-postSetupReceived

			Eventually(gardenContainer.RunCallCount).Should(Equal(3))
			processSpec, _ = gardenContainer.RunArgsForCall(2)
			Expect(processSpec.Path).To(Equal("/action/path"))
			Consistently(gardenContainer.RunCallCount).Should(Equal(3))

			Consistently(process.Ready()).ShouldNot(Receive())

			clock.Increment(1 * time.Second)
			Eventually(gardenContainer.RunCallCount).Should(Equal(4))
			processSpec, _ = gardenContainer.RunArgsForCall(3)
			Expect(processSpec.Path).To(Equal("/monitor/path"))
			Consistently(process.Ready()).ShouldNot(Receive())

			clock.Increment(1 * time.Second)
			Eventually(gardenContainer.RunCallCount).Should(Equal(5))
			processSpec, processIO := gardenContainer.RunArgsForCall(4)
			Expect(processSpec.Path).To(Equal("/monitor/path"))
			Expect(container.Monitor.RunAction.GetSuppressLogOutput()).Should(BeFalse())
			Expect(processIO.Stdout).ShouldNot(Equal(ioutil.Discard))
			Eventually(process.Ready()).Should(BeClosed())

			process.Signal(os.Interrupt)
			clock.Increment(1 * time.Second)
			Eventually(process.Wait()).Should(Receive(nil))
		})

		makeProcess := func(waitCh chan int) *gardenfakes.FakeProcess {
			process := &gardenfakes.FakeProcess{}
			process.WaitStub = func() (int, error) {
				return <-waitCh, nil
			}
			return process
		}

		Describe("declarative healthchecks", func() {
			var (
				process          ifrit.Process
				readinessProcess *gardenfakes.FakeProcess
				readinessCh      chan int
				livenessProcess  *gardenfakes.FakeProcess
				livenessCh       chan int
				actionProcess    *gardenfakes.FakeProcess
				actionCh         chan int
				monitorProcess   *gardenfakes.FakeProcess
				monitorCh        chan int
				readinessIO      chan garden.ProcessIO
				livenessIO       chan garden.ProcessIO
				processLock      sync.Mutex
			)

			BeforeEach(func() {
				// get rid of race condition caused by read inside the RunStub
				processLock.Lock()
				defer processLock.Unlock()

				readinessIO = make(chan garden.ProcessIO, 1)
				livenessIO = make(chan garden.ProcessIO, 1)
				// make the race detector happy
				readinessIOCh := readinessIO
				livenessIOCh := livenessIO

				readinessCh = make(chan int)
				readinessProcess = makeProcess(readinessCh)

				livenessCh = make(chan int, 1)
				livenessProcess = makeProcess(livenessCh)

				actionCh = make(chan int, 1)
				actionProcess = makeProcess(actionCh)

				monitorCh = make(chan int)
				monitorProcess = makeProcess(monitorCh)

				healthcheckCallCount := int64(0)
				gardenContainer.RunStub = func(spec garden.ProcessSpec, io garden.ProcessIO) (process garden.Process, err error) {
					defer GinkgoRecover()
					// get rid of race condition caused by write inside the BeforeEach

					processLock.Lock()
					defer processLock.Unlock()

					switch spec.Path {
					case "/action/path":
						return actionProcess, nil
					case "/etc/cf-assets/healthcheck/healthcheck":
						oldCount := atomic.AddInt64(&healthcheckCallCount, 1)
						switch oldCount {
						case 1:
							readinessIOCh <- io
							return readinessProcess, nil
						case 2:
							livenessIOCh <- io
							return livenessProcess, nil
						}
					case "/monitor/path":
						return monitorProcess, nil
					}

					err = errors.New("")
					Fail("unexpected executable path: " + spec.Path)
					return
				}
				container = executor.Container{
					RunInfo: executor.RunInfo{
						Action: &models.Action{
							RunAction: &models.RunAction{
								Path: "/action/path",
							},
						},
						Monitor: &models.Action{
							RunAction: &models.RunAction{
								Path: "/monitor/path",
							},
						},
						CheckDefinition: &models.CheckDefinition{
							Checks: []*models.Check{
								&models.Check{
									HttpCheck: &models.HTTPCheck{
										Port:             5432,
										RequestTimeoutMs: 100,
										Path:             "/some/path",
									},
								},
							},
						},
					},
				}
			})

			JustBeforeEach(func() {
				runner, err := optimusPrime.StepsRunner(logger, container, gardenContainer, logStreamer)
				Expect(err).NotTo(HaveOccurred())

				process = ifrit.Background(runner)
			})

			AfterEach(func() {
				close(readinessCh)
				livenessCh <- 1 // the healthcheck in liveness mode can only exit by failing
				close(actionCh)
				close(monitorCh)
				ginkgomon.Interrupt(process)
			})

			Context("when they are enabled", func() {
				BeforeEach(func() {
					optimusPrime = transformer.NewTransformer(
						nil, nil, nil, nil, nil, nil,
						os.TempDir(),
						false,
						healthyMonitoringInterval,
						unhealthyMonitoringInterval,
						healthCheckWorkPool,
						clock,
						[]string{"/post-setup/path", "-x", "argument"},
						"jim",
						true,
					)

					container.StartTimeoutMs = 1000
				})

				AfterEach(func() {
					process.Signal(os.Kill)
				})

				Context("and no check definitions exist", func() {
					BeforeEach(func() {
						container.CheckDefinition = nil
					})

					JustBeforeEach(func() {
						clock.WaitForWatcherAndIncrement(unhealthyMonitoringInterval)
					})

					It("uses the monitor action", func() {
						Eventually(gardenContainer.RunCallCount, 5*time.Second).Should(Equal(2))
						paths := []string{}
						args := [][]string{}
						for i := 0; i < gardenContainer.RunCallCount(); i++ {
							spec, _ := gardenContainer.RunArgsForCall(i)
							paths = append(paths, spec.Path)
							args = append(args, spec.Args)
						}

						Expect(paths).To(ContainElement("/monitor/path"))
					})
				})

				Context("and an http check definition exists", func() {
					BeforeEach(func() {
						container.CheckDefinition = &models.CheckDefinition{
							Checks: []*models.Check{
								&models.Check{
									HttpCheck: &models.HTTPCheck{
										Port:             5432,
										RequestTimeoutMs: 100,
										Path:             "/some/path",
									},
								},
							},
						}
					})

					Context("and the starttimeout is set to 0", func() {
						BeforeEach(func() {
							container.StartTimeoutMs = 0
						})

						It("runs the healthcheck with readiness timeout set to 0", func() {
							Eventually(gardenContainer.RunCallCount).Should(Equal(2))
							paths := []string{}
							args := [][]string{}
							for i := 0; i < gardenContainer.RunCallCount(); i++ {
								spec, _ := gardenContainer.RunArgsForCall(i)
								paths = append(paths, spec.Path)
								args = append(args, spec.Args)
							}

							Expect(paths).To(ContainElement("/etc/cf-assets/healthcheck/healthcheck"))
							Expect(args).To(ContainElement([]string{
								"-port=5432",
								"-timeout=100ms",
								"-uri=/some/path",
								"-readiness-interval=1ms",
								"-readiness-timeout=0s",
							}))
						})
					})

					Context("and optional fields are missing", func() {
						BeforeEach(func() {
							container.CheckDefinition = &models.CheckDefinition{
								Checks: []*models.Check{
									&models.Check{
										HttpCheck: &models.HTTPCheck{
											Port: 5432,
										},
									},
								},
							}
						})

						It("uses sane defaults", func() {
							Eventually(gardenContainer.RunCallCount).Should(Equal(2))
							paths := []string{}
							args := [][]string{}
							for i := 0; i < gardenContainer.RunCallCount(); i++ {
								spec, _ := gardenContainer.RunArgsForCall(i)
								paths = append(paths, spec.Path)
								args = append(args, spec.Args)
							}

							Expect(paths).To(ContainElement("/etc/cf-assets/healthcheck/healthcheck"))
							Expect(args).To(ContainElement([]string{
								"-port=5432",
								"-timeout=1000ms",
								"-uri=/",
								"-readiness-interval=1ms",
								"-readiness-timeout=1s",
							}))
						})
					})

					It("uses the check definition", func() {
						Eventually(gardenContainer.RunCallCount).Should(Equal(2))
						paths := []string{}
						args := [][]string{}
						users := []string{}
						for i := 0; i < gardenContainer.RunCallCount(); i++ {
							spec, _ := gardenContainer.RunArgsForCall(i)
							paths = append(paths, spec.Path)
							args = append(args, spec.Args)
							users = append(users, spec.User)
						}

						Expect(paths).To(ContainElement("/etc/cf-assets/healthcheck/healthcheck"))
						Expect(args).To(ContainElement([]string{
							"-port=5432",
							"-timeout=100ms",
							"-uri=/some/path",
							"-readiness-interval=1ms", // 1ms
							"-readiness-timeout=1s",
						}))
						Expect(users).To(ContainElement("root"))
					})

					Context("when the readiness check times out", func() {
						JustBeforeEach(func() {
							By("waiting for the action and readiness check processes to start")
							var io garden.ProcessIO
							Eventually(readinessIO).Should(Receive(&io))
							_, err := io.Stdout.Write([]byte("readiness check failed"))
							Expect(err).NotTo(HaveOccurred())

							By("timing out the readiness check")
							Eventually(gardenContainer.RunCallCount).Should(Equal(2))

							Consistently(readinessProcess.SignalCallCount).Should(Equal(0))
							readinessCh <- 1
							Eventually(actionProcess.SignalCallCount).Should(Equal(1))
							actionCh <- 2
						})

						It("suppress the readiness check log", func() {
							Eventually(process.Wait()).Should(Receive(HaveOccurred()))
							Consistently(fakeMetronClient.SendAppLogCallCount).Should(Equal(2))
							_, msg0, _, _ := fakeMetronClient.SendAppLogArgsForCall(0)
							_, msg1, _, _ := fakeMetronClient.SendAppLogArgsForCall(1)
							Expect([]string{msg0, msg1}).To(ConsistOf("Starting health monitoring of container", "Exit status 2"))
						})

						It("logs the readiness check output on stderr", func() {
							Eventually(fakeMetronClient.SendAppErrorLogCallCount).Should(Equal(2))
							logLines := map[string]string{}
							_, msg, source, _ := fakeMetronClient.SendAppErrorLogArgsForCall(0)
							logLines[source] = msg
							_, msg, source, _ = fakeMetronClient.SendAppErrorLogArgsForCall(1)
							logLines[source] = msg
							Expect(logLines).To(Equal(map[string]string{
								"HEALTH": "readiness check failed",
								"test":   "Timed out after 1s: health check never passed.",
							}))
						})

						It("returns the readiness check output in the error", func() {
							Eventually(process.Wait()).Should(Receive(MatchError(ContainSubstring("Instance never healthy after 1s: readiness check failed"))))
						})
					})

					Context("when the readiness check passes", func() {
						JustBeforeEach(func() {
							readinessCh <- 0
						})

						It("starts the liveness check", func() {
							Eventually(gardenContainer.RunCallCount).Should(Equal(3))
							paths := []string{}
							args := [][]string{}
							for i := 0; i < gardenContainer.RunCallCount(); i++ {
								spec, _ := gardenContainer.RunArgsForCall(i)
								paths = append(paths, spec.Path)
								args = append(args, spec.Args)
							}

							Expect(paths).To(ContainElement("/etc/cf-assets/healthcheck/healthcheck"))
							Expect(args).To(ContainElement([]string{
								"-port=5432",
								"-timeout=100ms",
								"-uri=/some/path",
								"-liveness-interval=1s", // 1ms
							}))
						})

						Context("when the liveness check exits", func() {
							JustBeforeEach(func() {
								Eventually(gardenContainer.RunCallCount).Should(Equal(3))

								By("waiting the action and liveness check processes to start")
								var io garden.ProcessIO
								Eventually(livenessIO).Should(Receive(&io))
								_, err := io.Stdout.Write([]byte("liveness check failed"))
								Expect(err).NotTo(HaveOccurred())

								By("exiting the liveness check")
								livenessCh <- 1
								Eventually(actionProcess.SignalCallCount).Should(Equal(1))
								actionCh <- 2
							})

							It("logs the liveness check output on stderr", func() {
								Eventually(fakeMetronClient.SendAppErrorLogCallCount).Should(Equal(1))
								logLines := map[string]string{}
								_, msg, source, _ := fakeMetronClient.SendAppErrorLogArgsForCall(0)
								logLines[source] = msg
								Expect(logLines).To(Equal(map[string]string{
									"HEALTH": "liveness check failed",
								}))
							})

							It("returns the liveness check output in the error", func() {
								Eventually(process.Wait()).Should(Receive(MatchError(ContainSubstring("Instance became unhealthy: liveness check failed"))))
							})
						})
					})
				})

				Context("and a tcp check definition exists", func() {
					BeforeEach(func() {
						container.CheckDefinition = &models.CheckDefinition{
							Checks: []*models.Check{
								&models.Check{
									TcpCheck: &models.TCPCheck{
										Port:             5432,
										ConnectTimeoutMs: 100,
									},
								},
							},
						}
					})

					Context("and optional fields are missing", func() {
						BeforeEach(func() {
							container.CheckDefinition = &models.CheckDefinition{
								Checks: []*models.Check{
									&models.Check{
										TcpCheck: &models.TCPCheck{
											Port: 5432,
										},
									},
								},
							}
						})

						It("uses sane defaults", func() {
							Eventually(gardenContainer.RunCallCount).Should(Equal(2))
							paths := []string{}
							args := [][]string{}
							for i := 0; i < gardenContainer.RunCallCount(); i++ {
								spec, _ := gardenContainer.RunArgsForCall(i)
								paths = append(paths, spec.Path)
								args = append(args, spec.Args)
							}

							Expect(paths).To(ContainElement("/etc/cf-assets/healthcheck/healthcheck"))
							Expect(args).To(ContainElement([]string{
								"-port=5432",
								"-timeout=1000ms",
								"-readiness-interval=1ms",
								"-readiness-timeout=1s",
							}))
						})
					})

					It("uses the check definition", func() {
						Eventually(gardenContainer.RunCallCount).Should(Equal(2))
						paths := []string{}
						args := [][]string{}
						for i := 0; i < gardenContainer.RunCallCount(); i++ {
							spec, _ := gardenContainer.RunArgsForCall(i)
							paths = append(paths, spec.Path)
							args = append(args, spec.Args)
						}

						Expect(paths).To(ContainElement("/etc/cf-assets/healthcheck/healthcheck"))
						Expect(args).To(ContainElement([]string{
							"-port=5432",
							"-timeout=100ms",
							"-readiness-interval=1ms",
							"-readiness-timeout=1s",
						}))
					})
				})

				Context("logs", func() {
					BeforeEach(func() {
						container.CheckDefinition = &models.CheckDefinition{
							Checks: []*models.Check{
								&models.Check{
									HttpCheck: &models.HTTPCheck{
										Port:             5432,
										RequestTimeoutMs: 2000,
										Path:             "/some/path",
									},
								},
							},
						}
					})

					JustBeforeEach(func() {
						Eventually(fakeMetronClient.SendAppLogCallCount, 5).Should(BeNumerically(">=", 1))

						_, message, sourceName, _ := fakeMetronClient.SendAppLogArgsForCall(0)
						Expect(message).To(Equal("Starting health monitoring of container"))
						Expect(sourceName).To(Equal("test"))

						var io garden.ProcessIO
						Eventually(readinessIO).Should(Receive(&io))
						io.Stdout.Write([]byte("failed"))

						readinessCh <- 1
					})

					It("should default to HEALTH for log source", func() {
						Eventually(fakeMetronClient.SendAppErrorLogCallCount).Should(BeNumerically(">=", 1))
						_, _, sourceName, _ := fakeMetronClient.SendAppErrorLogArgsForCall(0)
						Expect(sourceName).To(Equal("HEALTH"))
					})

					Context("when log source defined", func() {
						BeforeEach(func() {
							container.CheckDefinition.LogSource = "healthcheck"
						})

						It("logs healthcheck errors with log source from check defintion", func() {
							Eventually(fakeMetronClient.SendAppErrorLogCallCount).Should(BeNumerically(">=", 1))
							_, _, sourceName, _ := fakeMetronClient.SendAppErrorLogArgsForCall(0)
							Expect(sourceName).To(Equal("healthcheck"))
						})
					})
				})

				Context("and multiple check definitions exists", func() {
					var (
						otherReadinessProcess *gardenfakes.FakeProcess
						otherReadinessCh      chan int
						otherLivenessProcess  *gardenfakes.FakeProcess
						otherLivenessCh       chan int
					)

					BeforeEach(func() {
						// get rid of race condition caused by read inside the RunStub
						processLock.Lock()
						defer processLock.Unlock()

						otherReadinessCh = make(chan int)
						otherReadinessProcess = makeProcess(otherReadinessCh)

						otherLivenessCh = make(chan int)
						otherLivenessProcess = makeProcess(otherLivenessCh)

						healthcheckCallCount := int64(0)
						gardenContainer.RunStub = func(spec garden.ProcessSpec, io garden.ProcessIO) (process garden.Process, err error) {
							defer GinkgoRecover()
							// get rid of race condition caused by write inside the BeforeEach
							processLock.Lock()
							defer processLock.Unlock()

							switch spec.Path {
							case "/action/path":
								return actionProcess, nil
							case "/etc/cf-assets/healthcheck/healthcheck":
								oldCount := atomic.AddInt64(&healthcheckCallCount, 1)
								switch oldCount {
								case 1:
									return readinessProcess, nil
								case 2:
									return otherReadinessProcess, nil
								case 3:
									return livenessProcess, nil
								case 4:
									return otherLivenessProcess, nil
								}
								return livenessProcess, nil
							case "/monitor/path":
								return monitorProcess, nil
							}

							err = errors.New("")
							Fail("unexpected executable path: " + spec.Path)
							return
						}

						container.CheckDefinition = &models.CheckDefinition{
							Checks: []*models.Check{
								&models.Check{
									TcpCheck: &models.TCPCheck{
										Port:             2222,
										ConnectTimeoutMs: 100,
									},
								},
								&models.Check{
									HttpCheck: &models.HTTPCheck{
										Port:             8080,
										RequestTimeoutMs: 100,
									},
								},
							},
						}
					})

					AfterEach(func() {
						close(otherReadinessCh)
						close(otherLivenessCh)
					})

					It("uses the check definition instead of the monitor action", func() {
						Eventually(gardenContainer.RunCallCount).Should(Equal(3))
						paths := []string{}
						args := [][]string{}
						for i := 0; i < gardenContainer.RunCallCount(); i++ {
							spec, _ := gardenContainer.RunArgsForCall(i)
							paths = append(paths, spec.Path)
							args = append(args, spec.Args)
						}

						Expect(paths).To(ContainElement("/etc/cf-assets/healthcheck/healthcheck"))
						Expect(args).To(ContainElement([]string{
							"-port=2222",
							"-timeout=100ms",
							"-readiness-interval=1ms",
							"-readiness-timeout=1s",
						}))
						Expect(args).To(ContainElement([]string{
							"-port=8080",
							"-timeout=100ms",
							"-uri=/",
							"-readiness-interval=1ms",
							"-readiness-timeout=1s",
						}))
					})

					Context("when one of the readiness checks finish", func() {
						JustBeforeEach(func() {
							Eventually(gardenContainer.RunCallCount).Should(Equal(3))
							readinessCh <- 0
						})

						It("waits for both healthchecks to pass", func() {
							Consistently(gardenContainer.RunCallCount).Should(Equal(3))
						})

						Context("and the other readiness check finish", func() {
							JustBeforeEach(func() {
								otherReadinessCh <- 0
							})

							It("starts the liveness checks", func() {
								Eventually(gardenContainer.RunCallCount).Should(Equal(5))
								paths := []string{}
								args := [][]string{}
								for i := 0; i < gardenContainer.RunCallCount(); i++ {
									spec, _ := gardenContainer.RunArgsForCall(i)
									paths = append(paths, spec.Path)
									args = append(args, spec.Args)
								}

								Expect(paths).To(ContainElement("/etc/cf-assets/healthcheck/healthcheck"))
								Expect(args).To(ContainElement([]string{
									"-port=2222",
									"-timeout=100ms",
									"-liveness-interval=1s", // 1ms
								}))
								Expect(args).To(ContainElement([]string{
									"-port=8080",
									"-timeout=100ms",
									"-uri=/",
									"-liveness-interval=1s", // 1ms
								}))
							})

							Context("when either liveness check exit", func() {
								JustBeforeEach(func() {
									Eventually(gardenContainer.RunCallCount).Should(Equal(5))
									livenessCh <- 1
								})

								It("signals the process and exit", func() {
									Eventually(otherLivenessProcess.SignalCallCount).ShouldNot(BeZero())
									otherLivenessCh <- 1

									Eventually(actionProcess.SignalCallCount).ShouldNot(BeZero())
									actionCh <- 0

									Eventually(process.Wait()).Should(Receive(HaveOccurred()))
								})
							})
						})
					})
				})
			})

			Context("when they are disabled", func() {
				BeforeEach(func() {
					suppressExitStatusCode = true
					optimusPrime = transformer.NewTransformer(
						nil, nil, nil, nil, nil, nil,
						os.TempDir(),
						false,
						healthyMonitoringInterval,
						unhealthyMonitoringInterval,
						healthCheckWorkPool,
						clock,
						[]string{"/post-setup/path", "-x", "argument"},
						"jim",
						false,
					)
				})

				It("ignores the check definition and use the MonitorAction", func() {
					clock.WaitForWatcherAndIncrement(unhealthyMonitoringInterval)
					Eventually(gardenContainer.RunCallCount).Should(Equal(2))
					paths := []string{}
					args := [][]string{}
					for i := 0; i < gardenContainer.RunCallCount(); i++ {
						spec, _ := gardenContainer.RunArgsForCall(i)
						paths = append(paths, spec.Path)
						args = append(args, spec.Args)
					}

					Expect(paths).To(ContainElement("/monitor/path"))
				})

				Context("and there is no monitor action", func() {
					BeforeEach(func() {
						container.Monitor = nil
					})

					It("does not run any healthchecks", func() {
						Eventually(gardenContainer.RunCallCount).Should(Equal(1))
						Consistently(gardenContainer.RunCallCount).Should(Equal(1))

						paths := []string{}
						for i := 0; i < gardenContainer.RunCallCount(); i++ {
							spec, _ := gardenContainer.RunArgsForCall(i)
							paths = append(paths, spec.Path)
						}

						Expect(paths).To(ContainElement("/action/path"))
					})
				})
			})
		})

		Context("when there is no setup", func() {
			BeforeEach(func() {
				container.Setup = nil
			})

			It("returns a codependent step for the action/monitor", func() {
				gardenContainer.RunReturns(&gardenfakes.FakeProcess{}, nil)

				runner, err := optimusPrime.StepsRunner(logger, container, gardenContainer, logStreamer)
				Expect(err).NotTo(HaveOccurred())

				process := ifrit.Background(runner)

				Eventually(gardenContainer.RunCallCount).Should(Equal(1))
				processSpec, _ := gardenContainer.RunArgsForCall(0)
				Expect(processSpec.Path).To(Equal("/action/path"))
				Consistently(gardenContainer.RunCallCount).Should(Equal(1))

				clock.Increment(1 * time.Second)
				Eventually(gardenContainer.RunCallCount).Should(Equal(2))
				processSpec, _ = gardenContainer.RunArgsForCall(1)
				Expect(processSpec.Path).To(Equal("/monitor/path"))
				Eventually(process.Ready()).Should(BeClosed())

				process.Signal(os.Interrupt)
				clock.Increment(1 * time.Second)
				Eventually(process.Wait()).Should(Receive(nil))
			})
		})

		Context("when there is no monitor", func() {
			BeforeEach(func() {
				container.Monitor = nil
			})

			It("does not run the monitor step and immediately says the healthcheck passed", func() {
				gardenContainer.RunReturns(&gardenfakes.FakeProcess{}, nil)

				runner, err := optimusPrime.StepsRunner(logger, container, gardenContainer, logStreamer)
				Expect(err).NotTo(HaveOccurred())

				process := ifrit.Background(runner)
				Eventually(process.Ready()).Should(BeClosed())

				Eventually(gardenContainer.RunCallCount).Should(Equal(3))
				processSpec, _ := gardenContainer.RunArgsForCall(2)
				Expect(processSpec.Path).To(Equal("/action/path"))
				Consistently(gardenContainer.RunCallCount).Should(Equal(3))
			})
		})

		Context("MonitorAction", func() {
			var (
				process ifrit.Process
			)

			JustBeforeEach(func() {
				runner, err := optimusPrime.StepsRunner(logger, container, gardenContainer, logStreamer)
				Expect(err).NotTo(HaveOccurred())
				process = ifrit.Background(runner)
			})

			AfterEach(func() {
				ginkgomon.Interrupt(process)
			})

			BeforeEach(func() {
				suppressExitStatusCode = true
				container.Setup = nil
				container.Monitor = &models.Action{
					ParallelAction: models.Parallel(
						&models.RunAction{
							Path:              "/monitor/path",
							SuppressLogOutput: true,
						},
						&models.RunAction{
							Path:              "/monitor/path",
							SuppressLogOutput: true,
						},
					),
				}
			})

			Context("SuppressLogOutput", func() {
				var (
					monitorCh, actionCh chan int
				)

				BeforeEach(func() {
					monitorCh = make(chan int, 2)
					actionCh = make(chan int, 1)

					gardenContainer.RunStub = func(processSpec garden.ProcessSpec, processIO garden.ProcessIO) (garden.Process, error) {
						switch processSpec.Path {
						case "/monitor/path":
							return makeProcess(monitorCh), nil
						case "/action/path":
							return makeProcess(actionCh), nil
						default:
							return &gardenfakes.FakeProcess{}, nil
						}
					}
				})

				AfterEach(func() {
					close(monitorCh)
					close(actionCh)
				})

				JustBeforeEach(func() {
					Eventually(gardenContainer.RunCallCount).Should(Equal(1))
					clock.Increment(1 * time.Second)
					Eventually(gardenContainer.RunCallCount).Should(Equal(3))
				})

				It("is ignored", func() {
					processSpec, processIO := gardenContainer.RunArgsForCall(1)
					Expect(processSpec.Path).To(Equal("/monitor/path"))
					Expect(container.Monitor.RunAction.GetSuppressLogOutput()).Should(BeFalse())
					Expect(processIO.Stdout).ShouldNot(Equal(ioutil.Discard))
					monitorCh <- 0
					monitorCh <- 0
					Eventually(process.Ready()).Should(BeClosed())
				})
			})

			Context("logs", func() {
				var (
					exitStatusCh    chan int
					monitorProcess1 *gardenfakes.FakeProcess
					monitorProcess2 *gardenfakes.FakeProcess
				)

				BeforeEach(func() {
					monitorProcess1 = &gardenfakes.FakeProcess{}
					monitorProcess2 = &gardenfakes.FakeProcess{}
					actionProcess := &gardenfakes.FakeProcess{}
					exitStatusCh = make(chan int)
					actionProcess.WaitStub = func() (int, error) {
						return <-exitStatusCh, nil
					}

					monitorProcessChan1 := make(chan *garden.ProcessIO, 4)
					monitorProcessChan2 := make(chan *garden.ProcessIO, 4)

					monitorProcess1.WaitStub = func() (int, error) {
						procIO := <-monitorProcessChan1

						if monitorProcess1.WaitCallCount() == 2 {
							procIO.Stdout.Write([]byte("healthcheck failed"))
							return 1, nil
						}
						return 0, nil
					}

					monitorProcess2.WaitStub = func() (int, error) {
						procIO := <-monitorProcessChan2

						if monitorProcess2.WaitCallCount() == 2 {
							procIO.Stdout.Write([]byte("healthcheck failed"))
							return 1, nil
						}
						return 0, nil
					}

					monitorProcessRun := uint32(0)

					gardenContainer.RunStub = func(processSpec garden.ProcessSpec, processIO garden.ProcessIO) (garden.Process, error) {
						if processSpec.Path == "/monitor/path" {
							if atomic.AddUint32(&monitorProcessRun, 1)%2 == 0 {
								monitorProcessChan1 <- &processIO
								return monitorProcess1, nil
							}
							monitorProcessChan2 <- &processIO
							return monitorProcess2, nil
						} else if processSpec.Path == "/action/path" {
							return actionProcess, nil
						}
						return &gardenfakes.FakeProcess{}, nil
					}
				})

				AfterEach(func() {
					Eventually(exitStatusCh).Should(BeSent(1))
				})

				JustBeforeEach(func() {
					Eventually(gardenContainer.RunCallCount).Should(Equal(1))

					By("starting the readiness check")
					clock.WaitForWatcherAndIncrement(1 * time.Second)
					Eventually(gardenContainer.RunCallCount).Should(Equal(3))
					Eventually(monitorProcess1.WaitCallCount).Should(Equal(1))
					Eventually(monitorProcess2.WaitCallCount).Should(Equal(1))

					By("starting the liveness check")
					clock.WaitForWatcherAndIncrement(1 * time.Second)
					Eventually(gardenContainer.RunCallCount).Should(Equal(5))
					Eventually(monitorProcess1.WaitCallCount).Should(Equal(2))
					Eventually(monitorProcess2.WaitCallCount).Should(Equal(2))
				})

				It("logs healthcheck error with the same source in a readable way", func() {
					Eventually(fakeMetronClient.SendAppErrorLogCallCount).Should(Equal(1))
					_, message, sourceName, _ := fakeMetronClient.SendAppErrorLogArgsForCall(0)
					Expect(sourceName).To(Equal("test"))
					Expect(message).To(ContainSubstring("healthcheck failed; healthcheck failed"))
				})

				It("logs the container lifecycle", func() {
					Eventually(fakeMetronClient.SendAppLogCallCount).Should(Equal(3))
					_, message, _, _ := fakeMetronClient.SendAppLogArgsForCall(0)
					Expect(message).To(Equal("Starting health monitoring of container"))
					_, message, _, _ = fakeMetronClient.SendAppLogArgsForCall(1)
					Expect(message).To(Equal("Container became healthy"))
					_, message, _, _ = fakeMetronClient.SendAppLogArgsForCall(2)
					Expect(message).To(Equal("Container became unhealthy"))
				})
			})
		})
	})
})
