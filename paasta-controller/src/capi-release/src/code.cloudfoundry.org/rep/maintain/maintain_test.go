package maintain_test

import (
	"errors"
	"os"
	"time"

	"code.cloudfoundry.org/bbs/models"
	"code.cloudfoundry.org/clock/fakeclock"
	"code.cloudfoundry.org/executor"
	fake_client "code.cloudfoundry.org/executor/fakes"
	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/lager/lagertest"
	"code.cloudfoundry.org/rep/maintain"
	"code.cloudfoundry.org/rep/maintain/maintainfakes"
	"github.com/tedsuo/ifrit"
	"github.com/tedsuo/ifrit/ginkgomon"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
)

var _ = Describe("Maintain Presence", func() {
	var (
		config          maintain.Config
		fakeHeartbeater *maintainfakes.FakeRunner
		fakeClient      *fake_client.FakeClient
		serviceClient   *maintainfakes.FakeCellPresenceClient
		logger          *lagertest.TestLogger

		maintainer        ifrit.Runner
		maintainProcess   ifrit.Process
		heartbeaterErrors chan error
		observedSignals   chan os.Signal
		clock             *fakeclock.FakeClock
		pingErrors        chan error
	)

	BeforeEach(func() {
		pingErrors = make(chan error, 1)
		fakeClient = &fake_client.FakeClient{
			PingStub: func(lager.Logger) error {
				return <-pingErrors
			},
		}
		resources := executor.ExecutorResources{MemoryMB: 128, DiskMB: 1024, Containers: 6}
		fakeClient.TotalResourcesReturns(resources, nil)

		logger = lagertest.NewTestLogger("test")
		clock = fakeclock.NewFakeClock(time.Now())

		heartbeaterErrors = make(chan error)
		observedSignals = make(chan os.Signal, 2)
		fakeHeartbeater = &maintainfakes.FakeRunner{
			RunStub: func(sigChan <-chan os.Signal, ready chan<- struct{}) error {
				defer GinkgoRecover()
				logger.Info("fake-heartbeat-started")
				close(ready)
				for {
					select {
					case sig := <-sigChan:
						logger.Info("fake-heartbeat-received-signal")
						Eventually(observedSignals, time.Millisecond).Should(BeSent(sig))
						return nil
					case err := <-heartbeaterErrors:
						logger.Info("fake-heartbeat-received-error")
						return err
					}
				}
			},
		}

		serviceClient = &maintainfakes.FakeCellPresenceClient{}
		serviceClient.NewCellPresenceRunnerReturns(fakeHeartbeater)

		config = maintain.Config{
			CellID:                "cell-id",
			RepAddress:            "1.2.3.4",
			RepUrl:                "https://cell-id.service.cf.internal",
			Zone:                  "az1",
			RetryInterval:         1 * time.Second,
			RootFSProviders:       []string{"provider-1", "provider-2"},
			PlacementTags:         []string{"test-tag-1", "test-tag-2"},
			OptionalPlacementTags: []string{"optional-test-tag-1", "optional-test-tag-2"},
		}
		maintainer = maintain.New(logger, config, fakeClient, serviceClient, 10*time.Second, clock)
	})

	AfterEach(func() {
		logger.Info("test-complete-signaling-maintainer-to-stop")
		close(pingErrors)
		ginkgomon.Interrupt(maintainProcess)
	})

	It("pings the executor", func() {
		pingErrors <- nil
		maintainProcess = ginkgomon.Invoke(maintainer)
		Expect(fakeClient.PingCallCount()).To(Equal(1))
	})

	Context("when pinging the executor fails", func() {
		It("keeps pinging until it succeeds, then starts heartbeating the executor's presence", func() {
			maintainProcess = ifrit.Background(maintainer)
			ready := maintainProcess.Ready()

			for i := 1; i <= 4; i++ {
				pingErrors <- errors.New("ping failed")
				Eventually(fakeClient.PingCallCount).Should(Equal(i))
				clock.WaitForWatcherAndIncrement(1 * time.Second)
				Expect(ready).NotTo(BeClosed())
			}

			pingErrors <- nil
			clock.WaitForWatcherAndIncrement(1 * time.Second)
			Eventually(fakeClient.PingCallCount).Should(BeNumerically(">=", 5))

			Eventually(ready).Should(BeClosed())
			Expect(fakeHeartbeater.RunCallCount()).To(Equal(1))
		})
	})

	Context("when pinging the executor succeeds", func() {
		Context("when the heartbeater is not ready", func() {
			BeforeEach(func() {
				fakeHeartbeater = &maintainfakes.FakeRunner{
					RunStub: func(sigChan <-chan os.Signal, ready chan<- struct{}) error {
						defer GinkgoRecover()
						for {
							select {
							case sig := <-sigChan:
								logger.Info("never-ready-fake-heartbeat-received-signal")
								Eventually(observedSignals, time.Millisecond).Should(BeSent(sig))
								return nil
							case err := <-heartbeaterErrors:
								logger.Info("never-ready-fake-heartbeat-received-error")
								return err
							}
						}
					},
				}

				serviceClient.NewCellPresenceRunnerReturns(fakeHeartbeater)

				pingErrors <- nil
				maintainProcess = ifrit.Background(maintainer)
			})

			It("exits when signaled", func() {
				maintainProcess.Signal(os.Interrupt)
				var err error
				Eventually(maintainProcess.Wait()).Should(Receive(&err))
				Expect(err).NotTo(HaveOccurred())
				Expect(maintainProcess.Ready()).NotTo(BeClosed())
			})

			Context("when the heartbeat errors", func() {
				BeforeEach(func() {
					heartbeaterErrors <- errors.New("oh no")
					pingErrors <- nil
				})

				It("does not shut down", func() {
					Consistently(maintainProcess.Wait()).ShouldNot(Receive(), "should not shut down")
				})

				It("retries to heartbeat", func() {
					Eventually(serviceClient.NewCellPresenceRunnerCallCount).Should(Equal(2))
					Eventually(fakeHeartbeater.RunCallCount).Should(Equal(2))
				})
			})
		})

		Context("when the heartbeater is ready", func() {
			BeforeEach(func() {
				pingErrors <- nil
				maintainProcess = ginkgomon.Invoke(maintainer)
				Eventually(fakeClient.PingCallCount).Should(Equal(1))
				Expect(maintainProcess.Ready()).To(BeClosed())
			})

			It("starts maintaining presence", func() {
				Expect(serviceClient.NewCellPresenceRunnerCallCount()).To(Equal(1))

				expectedPresence := models.NewCellPresence(
					"cell-id",
					"1.2.3.4",
					"https://cell-id.service.cf.internal",
					"az1",
					models.NewCellCapacity(128, 1024, 6),
					[]string{"provider-1", "provider-2"},
					[]string{},
					[]string{"test-tag-1", "test-tag-2"},
					[]string{"optional-test-tag-1", "optional-test-tag-2"},
				)

				_, presence, retryInterval, lockTTL := serviceClient.NewCellPresenceRunnerArgsForCall(0)
				Expect(*presence).To(Equal(expectedPresence))
				Expect(retryInterval).To(Equal(1 * time.Second))
				Expect(lockTTL).To(Equal(10 * time.Second))

				Eventually(fakeHeartbeater.RunCallCount).Should(Equal(1))
			})

			It("continues pings the executor on an interval", func() {
				for i := 2; i < 6; i++ {
					pingErrors <- nil
					clock.Increment(1 * time.Second)
					Eventually(fakeClient.PingCallCount).Should(Equal(i))
				}
			})

			Context("when the executor ping fails", func() {
				BeforeEach(func() {
					pingErrors <- errors.New("failed to ping")
					clock.Increment(1 * time.Second)
					Eventually(fakeClient.PingCallCount).Should(Equal(3))
				})

				It("stops heartbeating the executor's presence", func() {
					Eventually(observedSignals).Should(Receive(Equal(os.Kill)))
				})

				It("continues pinging the executor", func() {
					clock.Increment(1 * time.Second)
					for i := 4; i < 8; i++ {
						pingErrors <- errors.New("failed again")
						Eventually(fakeClient.PingCallCount).Should(Equal(i))
						clock.Increment(1 * time.Second)
					}
				})

				Context("when the executor ping succeeds again", func() {
					BeforeEach(func() {
						pingErrors <- nil
						Eventually(fakeHeartbeater.RunCallCount).Should(Equal(2))
						pingErrors <- nil
						clock.Increment(1 * time.Second)
						Eventually(fakeClient.PingCallCount).Should(Equal(4))
					})

					It("begins heartbeating the executor's presence again", func() {
						Eventually(fakeHeartbeater.RunCallCount, 10*config.RetryInterval).Should(Equal(2))
					})

					It("continues to ping the executor", func() {
						for i := 5; i < 7; i++ {
							pingErrors <- nil
							clock.Increment(1 * time.Second)
							Eventually(fakeClient.PingCallCount).Should(Equal(i))
						}
					})
				})
			})

			Context("when heartbeating fails", func() {
				BeforeEach(func() {
					heartbeaterErrors <- errors.New("heartbeating failed")
				})

				It("does not shut down", func() {
					Consistently(maintainProcess.Wait()).ShouldNot(Receive(), "should not shut down")
				})

				It("continues pinging the executor", func() {
					for i := 2; i < 6; i++ {
						pingErrors <- nil
						Eventually(fakeClient.PingCallCount).Should(Equal(i))
						clock.WaitForWatcherAndIncrement(1 * time.Second)
					}
				})

				It("logs an error message", func() {
					Eventually(logger.TestSink.Buffer).Should(gbytes.Say("lost-lock"))
				})

				It("tries to restart heartbeating each time the ping succeeds", func() {
					Expect(fakeHeartbeater.RunCallCount()).To(Equal(1))

					Eventually(logger.TestSink.Buffer).Should(gbytes.Say("lost-lock"))

					pingErrors <- nil
					clock.Increment(1 * time.Second)

					Eventually(fakeHeartbeater.RunCallCount).Should(Equal(2))
				})
			})
		})
	})
})
