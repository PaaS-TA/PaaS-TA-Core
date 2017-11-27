package steps_test

import (
	"errors"
	"fmt"
	"sync"
	"time"

	"code.cloudfoundry.org/clock/fakeclock"
	"code.cloudfoundry.org/executor/depot/log_streamer/fake_log_streamer"
	"code.cloudfoundry.org/executor/depot/steps"
	"code.cloudfoundry.org/executor/depot/steps/fakes"
	"code.cloudfoundry.org/lager/lagertest"
	"code.cloudfoundry.org/workpool"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
)

var _ = Describe("MonitorStep", func() {
	var (
		fakeStep1 *fakes.FakeStep
		fakeStep2 *fakes.FakeStep

		checkSteps chan *fakes.FakeStep

		checkFunc        func() steps.Step
		hasBecomeHealthy <-chan struct{}
		clock            *fakeclock.FakeClock
		fakeStreamer     *fake_log_streamer.FakeLogStreamer

		startTimeout      time.Duration
		healthyInterval   time.Duration
		unhealthyInterval time.Duration

		step   steps.Step
		logger *lagertest.TestLogger
	)

	const numOfConcurrentMonitorSteps = 3

	BeforeEach(func() {
		startTimeout = 0
		healthyInterval = 1 * time.Second
		unhealthyInterval = 500 * time.Millisecond

		fakeStep1 = new(fakes.FakeStep)
		fakeStep2 = new(fakes.FakeStep)

		checkSteps = make(chan *fakes.FakeStep, 2)
		checkSteps <- fakeStep1
		checkSteps <- fakeStep2

		clock = fakeclock.NewFakeClock(time.Now())

		fakeStreamer = newFakeStreamer()

		checkFunc = func() steps.Step {
			return <-checkSteps
		}

		logger = lagertest.NewTestLogger("test")
	})

	JustBeforeEach(func() {
		hasBecomeHealthyChannel := make(chan struct{}, 1000)
		hasBecomeHealthy = hasBecomeHealthyChannel

		workPool, err := workpool.NewWorkPool(numOfConcurrentMonitorSteps)
		Expect(err).NotTo(HaveOccurred())

		step = steps.NewMonitor(
			checkFunc,
			hasBecomeHealthyChannel,
			logger,
			clock,
			fakeStreamer,
			startTimeout,
			healthyInterval,
			unhealthyInterval,
			workPool,
		)
	})

	expectCheckAfterInterval := func(fakeStep *fakes.FakeStep, d time.Duration) {
		previousCheckCount := fakeStep.PerformCallCount()

		clock.Increment(d - 1*time.Microsecond)
		Consistently(fakeStep.PerformCallCount, 0.05).Should(Equal(previousCheckCount))

		clock.WaitForWatcherAndIncrement(d)
		Eventually(fakeStep.PerformCallCount).Should(Equal(previousCheckCount + 1))
	}

	Describe("Throttling", func() {
		var (
			throttleChan chan struct{}
			doneChan     chan struct{}
			fakeStep     *fakes.FakeStep
		)

		BeforeEach(func() {
			throttleChan = make(chan struct{}, numOfConcurrentMonitorSteps)
			doneChan = make(chan struct{}, 1)
			fakeStep = new(fakes.FakeStep)
			fakeStep.PerformStub = func() error {
				throttleChan <- struct{}{}
				<-doneChan
				return nil
			}
			checkFunc = func() steps.Step {
				return fakeStep
			}

		})

		AfterEach(func() {
			step.Cancel()
		})

		It("throttles concurrent health check", func() {
			for i := 0; i < 5; i++ {
				go step.Perform()
			}

			Consistently(fakeStep.PerformCallCount).Should(Equal(0))
			clock.Increment(501 * time.Millisecond)

			Eventually(func() int {
				return len(throttleChan)
			}).Should(Equal(numOfConcurrentMonitorSteps))
			Consistently(func() int {
				return len(throttleChan)
			}).Should(Equal(numOfConcurrentMonitorSteps))

			Eventually(fakeStep.PerformCallCount).Should(Equal(numOfConcurrentMonitorSteps))

			doneChan <- struct{}{}

			Eventually(fakeStep.PerformCallCount).Should(Equal(numOfConcurrentMonitorSteps + 1))

			close(doneChan)

			Eventually(fakeStep.PerformCallCount).Should(Equal(5))
		})
	})

	Describe("Perform", func() {
		var (
			checkResults chan<- error

			performErr     chan error
			donePerforming *sync.WaitGroup
		)

		BeforeEach(func() {
			results := make(chan error, 10)
			checkResults = results

			var currentResult error

			fakedResult := func() error {
				select {
				case currentResult = <-results:
				default:
				}

				return currentResult
			}

			fakeStep1.PerformStub = fakedResult
			fakeStep2.PerformStub = fakedResult
		})

		JustBeforeEach(func() {
			performErr = make(chan error, 1)
			donePerforming = new(sync.WaitGroup)

			donePerforming.Add(1)
			go func() {
				defer donePerforming.Done()
				performErr <- step.Perform()
			}()
		})

		AfterEach(func() {
			step.Cancel()
			donePerforming.Wait()
		})

		It("emits a message to the applications log stream", func() {
			Eventually(fakeStreamer.Stdout().(*gbytes.Buffer)).Should(
				gbytes.Say("Starting health monitoring of container\n"),
			)
		})

		Context("when the check succeeds", func() {
			BeforeEach(func() {
				checkResults <- nil
			})

			Context("and the unhealthy interval passes", func() {
				JustBeforeEach(func() {
					expectCheckAfterInterval(fakeStep1, unhealthyInterval)
				})

				It("emits a healthy event", func() {
					Eventually(hasBecomeHealthy).Should(Receive())
				})

				It("emits a log message for the success", func() {
					Eventually(fakeStreamer.Stdout().(*gbytes.Buffer)).Should(
						gbytes.Say("Container became healthy\n"),
					)
				})

				It("logs the step", func() {
					Eventually(logger.TestSink.LogMessages).Should(ConsistOf([]string{
						"test.monitor-step.transitioned-to-healthy",
					}))
				})

				Context("and the healthy interval passes", func() {
					JustBeforeEach(func() {
						Eventually(hasBecomeHealthy).Should(Receive())
						expectCheckAfterInterval(fakeStep2, healthyInterval)
					})

					It("does not emit another healthy event", func() {
						Consistently(hasBecomeHealthy).ShouldNot(Receive())
					})
				})

				Context("and the check begins to fail", func() {
					disaster := errors.New("oh no!")

					BeforeEach(func() {
						checkResults <- disaster
					})

					Context("and the healthy interval passes", func() {
						JustBeforeEach(func() {
							Eventually(hasBecomeHealthy).Should(Receive())
							expectCheckAfterInterval(fakeStep2, healthyInterval)
						})

						It("emits nothing", func() {
							Consistently(hasBecomeHealthy).ShouldNot(Receive())
						})

						It("logs the step", func() {
							Eventually(func() []string { return logger.TestSink.LogMessages() }).Should(ConsistOf([]string{
								"test.monitor-step.transitioned-to-healthy",
								"test.monitor-step.transitioned-to-unhealthy",
							}))
						})

						It("emits a log message for the success", func() {
							Eventually(fakeStreamer.Stdout().(*gbytes.Buffer)).Should(
								gbytes.Say("Container became unhealthy\n"),
							)
						})

						It("completes with failure", func() {
							Eventually(performErr).Should(Receive(Equal(disaster)))
						})
					})
				})
			})
		})

		Context("when the check is failing immediately", func() {
			BeforeEach(func() {
				checkResults <- errors.New("not up yet!")
			})

			Context("and the start timeout is exceeded", func() {
				BeforeEach(func() {
					startTimeout = 60 * time.Millisecond
					unhealthyInterval = 30 * time.Millisecond
				})

				It("completes with failure", func() {
					expectCheckAfterInterval(fakeStep1, unhealthyInterval)
					Consistently(performErr).ShouldNot(Receive())
					expectCheckAfterInterval(fakeStep2, unhealthyInterval)
					Eventually(performErr).Should(Receive(MatchError("not up yet!")))
				})

				It("logs the step", func() {
					expectCheckAfterInterval(fakeStep1, unhealthyInterval)
					expectCheckAfterInterval(fakeStep2, unhealthyInterval)
					Eventually(logger.TestSink.LogMessages).Should(ConsistOf([]string{
						"test.monitor-step.timed-out-before-healthy",
					}))
				})

				It("emits a log message explaining the timeout", func() {
					expectCheckAfterInterval(fakeStep1, unhealthyInterval)
					expectCheckAfterInterval(fakeStep2, unhealthyInterval)
					Eventually(fakeStreamer.Stderr().(*gbytes.Buffer)).Should(gbytes.Say(
						fmt.Sprintf("Timed out after %s: health check never passed.\n", startTimeout),
					))
				})
			})

			Context("and the unhealthy interval passes", func() {
				JustBeforeEach(func() {
					expectCheckAfterInterval(fakeStep1, unhealthyInterval)
				})

				It("does not emit an unhealthy event", func() {
					Consistently(hasBecomeHealthy).ShouldNot(Receive())
				})

				It("does not exit", func() {
					Consistently(performErr).ShouldNot(Receive())
				})

				Context("and the unhealthy interval passes again", func() {
					JustBeforeEach(func() {
						expectCheckAfterInterval(fakeStep2, unhealthyInterval)
					})

					It("does not emit an unhealthy event", func() {
						Consistently(hasBecomeHealthy).ShouldNot(Receive())
					})

					It("does not exit", func() {
						Consistently(performErr).ShouldNot(Receive())
					})
				})
			})
		})
	})

	Describe("Cancel", func() {
		It("interrupts the monitoring", func() {
			performResult := make(chan error)
			go func() { performResult <- step.Perform() }()
			step.Cancel()
			Eventually(performResult).Should(Receive(Equal(steps.ErrCancelled)))
		})

		Context("while checking", func() {
			var performing chan struct{}

			BeforeEach(func() {
				performing = make(chan struct{})
				cancelled := make(chan struct{})

				fakeStep1.PerformStub = func() error {
					close(performing)

					select {
					case <-cancelled:
						return steps.ErrCancelled
					}
				}

				fakeStep1.CancelStub = func() {
					close(cancelled)
				}
			})

			It("cancels the in-flight check", func() {
				performResult := make(chan error)

				go func() { performResult <- step.Perform() }()

				expectCheckAfterInterval(fakeStep1, unhealthyInterval)

				Eventually(performing).Should(BeClosed())

				step.Cancel()

				Eventually(performResult).Should(Receive(Equal(steps.ErrCancelled)))
			})
		})
	})
})
