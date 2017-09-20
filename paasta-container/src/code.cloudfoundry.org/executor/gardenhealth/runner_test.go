package gardenhealth_test

import (
	"errors"
	"os"
	"time"

	"code.cloudfoundry.org/executor/gardenhealth"
	"github.com/cloudfoundry/dropsonde/metric_sender/fake"
	"github.com/cloudfoundry/dropsonde/metrics"
	"github.com/tedsuo/ifrit"
	"github.com/tedsuo/ifrit/ginkgomon"

	"code.cloudfoundry.org/clock/fakeclock"
	fakeexecutor "code.cloudfoundry.org/executor/fakes"
	"code.cloudfoundry.org/executor/gardenhealth/fakegardenhealth"
	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/lager/lagertest"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
)

var _ = Describe("Runner", func() {
	var (
		runner                          *gardenhealth.Runner
		process                         ifrit.Process
		logger                          *lagertest.TestLogger
		checker                         *fakegardenhealth.FakeChecker
		executorClient                  *fakeexecutor.FakeClient
		sender                          *fake.FakeMetricSender
		fakeClock                       *fakeclock.FakeClock
		checkInterval, emissionInterval time.Duration
		timeoutDuration                 time.Duration
	)

	BeforeEach(func() {
		logger = lagertest.NewTestLogger("test")
		checker = &fakegardenhealth.FakeChecker{}
		executorClient = &fakeexecutor.FakeClient{}
		fakeClock = fakeclock.NewFakeClock(time.Now())
		checkInterval = 2 * time.Minute
		timeoutDuration = 1 * time.Minute
		emissionInterval = 30 * time.Second

		sender = fake.NewFakeMetricSender()
		metrics.Initialize(sender, nil)
	})

	JustBeforeEach(func() {
		runner = gardenhealth.NewRunner(checkInterval, emissionInterval, timeoutDuration, logger, checker, executorClient, fakeClock)
		process = ifrit.Background(runner)
	})

	AfterEach(func() {
		ginkgomon.Interrupt(process)
	})

	Describe("Run", func() {
		Context("When garden is immediately unhealthy", func() {
			Context("because the health check fails", func() {
				var checkErr = gardenhealth.UnrecoverableError("nope")
				BeforeEach(func() {
					checker.HealthcheckReturns(checkErr)
					executorClient.HealthyReturns(false)
				})

				It("fails without becoming ready", func() {
					Eventually(process.Wait()).Should(Receive(Equal(checkErr)))
					Consistently(process.Ready()).ShouldNot(BeClosed())
				})

				It("emits a metric for unhealthy cell", func() {
					Eventually(process.Wait()).Should(Receive(Equal(checkErr)))
					Eventually(func() float64 {
						return sender.GetValue("UnhealthyCell").Value
					}).Should(Equal(float64(1)))
				})
			})

			Context("because the health check timed out", func() {
				var blockHealthcheck chan struct{}

				BeforeEach(func() {
					blockHealthcheck = make(chan struct{})
					checker.HealthcheckStub = func(lager.Logger) error {
						<-blockHealthcheck
						return nil
					}
				})

				JustBeforeEach(func() {
					fakeClock.WaitForWatcherAndIncrement(timeoutDuration)
				})

				AfterEach(func() {
					// Send to the channel to eliminate the race
					blockHealthcheck <- struct{}{}
					close(blockHealthcheck)
					blockHealthcheck = nil
				})

				It("fails without becoming ready", func() {
					Eventually(process.Wait()).Should(Receive(Equal(gardenhealth.HealthcheckTimeoutError{})))
					Consistently(process.Ready()).ShouldNot(BeClosed())
				})

				It("emits a metric for unhealthy cell", func() {
					Eventually(process.Wait()).Should(Receive(Equal(gardenhealth.HealthcheckTimeoutError{})))
					Eventually(func() float64 {
						return sender.GetValue("UnhealthyCell").Value
					}).Should(Equal(float64(1)))
				})
			})
		})

		Context("When garden is healthy", func() {
			BeforeEach(func() {
				executorClient.HealthyReturns(true)
			})

			It("sets healthy to true only once", func() {
				Eventually(executorClient.SetHealthyCallCount).Should(Equal(1))
				_, healthy := executorClient.SetHealthyArgsForCall(0)
				Expect(healthy).Should(Equal(true))
				Expect(executorClient.SetHealthyCallCount()).To(Equal(1))
			})

			It("continues to check at the correct interval", func() {
				Eventually(checker.HealthcheckCallCount).Should(Equal(1))
				Eventually(process.Ready()).Should(BeClosed())

				fakeClock.WaitForNWatchersAndIncrement(checkInterval, 2)
				Eventually(checker.HealthcheckCallCount).Should(Equal(2))
				Eventually(logger).Should(gbytes.Say("check-complete"))

				fakeClock.WaitForNWatchersAndIncrement(checkInterval, 2)
				Eventually(checker.HealthcheckCallCount).Should(Equal(3))
				Eventually(logger).Should(gbytes.Say("check-complete"))

				fakeClock.WaitForNWatchersAndIncrement(checkInterval, 2)
				Eventually(checker.HealthcheckCallCount).Should(Equal(4))
				Eventually(logger).Should(gbytes.Say("check-complete"))
			})

			It("emits a metric for healthy cell", func() {
				Eventually(func() float64 {
					return sender.GetValue("UnhealthyCell").Value
				}).Should(Equal(float64(0)))
			})
		})

		Context("when garden is intermittently healthy", func() {
			var checkErr = errors.New("nope")

			BeforeEach(func() {
				executorClient.HealthyReturns(true)
			})

			It("Sets healthy to false after it fails, then to true after success and emits respective metrics", func() {
				Eventually(executorClient.SetHealthyCallCount).Should(Equal(1))
				_, healthy := executorClient.SetHealthyArgsForCall(0)
				Expect(healthy).Should(Equal(true))
				Expect(sender.GetValue("UnhealthyCell").Value).To(Equal(float64(0)))

				checker.HealthcheckReturns(checkErr)
				executorClient.HealthyReturns(false)
				fakeClock.WaitForWatcherAndIncrement(checkInterval)

				Eventually(executorClient.SetHealthyCallCount).Should(Equal(2))
				_, healthy = executorClient.SetHealthyArgsForCall(1)
				Expect(healthy).Should(Equal(false))
				Expect(sender.GetValue("UnhealthyCell").Value).To(Equal(float64(1)))

				checker.HealthcheckReturns(nil)
				executorClient.HealthyReturns(true)
				fakeClock.WaitForNWatchersAndIncrement(checkInterval, 2)

				Eventually(executorClient.SetHealthyCallCount).Should(Equal(3))
				_, healthy = executorClient.SetHealthyArgsForCall(2)
				Expect(healthy).Should(Equal(true))
				Expect(sender.GetValue("UnhealthyCell").Value).To(Equal(float64(0)))
			})
		})

		Context("When the healthcheck times out", func() {
			var blockHealthcheck chan struct{}

			BeforeEach(func() {
				blockHealthcheck = make(chan struct{})
				checker.HealthcheckStub = func(lager.Logger) error {
					logger.Info("blocking")
					<-blockHealthcheck
					logger.Info("unblocking")
					return nil
				}
			})

			AfterEach(func() {
				close(blockHealthcheck)
			})

			It("sets the executor to unhealthy and emits the unhealthy metric", func() {
				Eventually(blockHealthcheck).Should(BeSent(struct{}{}))
				Eventually(executorClient.SetHealthyCallCount).Should(Equal(1))

				fakeClock.WaitForNWatchersAndIncrement(checkInterval, 2)
				Eventually(checker.HealthcheckCallCount).Should(Equal(2))

				fakeClock.WaitForNWatchersAndIncrement(timeoutDuration, 2)

				Eventually(executorClient.SetHealthyCallCount).Should(Equal(2))
				_, healthy := executorClient.SetHealthyArgsForCall(1)
				Expect(healthy).Should(Equal(false))
				Eventually(func() float64 {
					return sender.GetValue("UnhealthyCell").Value
				}).Should(Equal(float64(1)))
			})
		})

		Context("When the runner is signaled", func() {
			Context("during the initial health check", func() {
				var blockHealthcheck chan struct{}

				BeforeEach(func() {
					blockHealthcheck = make(chan struct{})
					checker.HealthcheckStub = func(lager.Logger) error {
						<-blockHealthcheck
						return nil
					}
				})

				JustBeforeEach(func() {
					process.Signal(os.Interrupt)
				})

				It("exits with no error", func() {
					Eventually(process.Wait()).Should(Receive(BeNil()))
				})
			})

			Context("After the initial health check", func() {
				It("exits imediately with no error", func() {
					Eventually(executorClient.SetHealthyCallCount).Should(Equal(1))

					process.Signal(os.Interrupt)
					Eventually(process.Wait()).Should(Receive(BeNil()))
				})
			})
		})

		Context("UnhealthyCell metric emission", func() {
			It("emits the UnhealthyCell every emitInterval", func() {
				Eventually(executorClient.HealthyCallCount).Should(Equal(1))
				Eventually(func() float64 {
					return sender.GetValue("UnhealthyCell").Value
				}).Should(Equal(float64(1)))

				executorClient.HealthyReturns(true)
				fakeClock.WaitForWatcherAndIncrement(emissionInterval)

				Eventually(executorClient.HealthyCallCount).Should(Equal(2))
				Eventually(func() float64 {
					return sender.GetValue("UnhealthyCell").Value
				}).Should(Equal(float64(0)))

				executorClient.HealthyReturns(false)
				fakeClock.WaitForWatcherAndIncrement(emissionInterval)

				Eventually(executorClient.HealthyCallCount).Should(Equal(3))
				Eventually(func() float64 {
					return sender.GetValue("UnhealthyCell").Value
				}).Should(Equal(float64(1)))
			})
		})
	})
})
