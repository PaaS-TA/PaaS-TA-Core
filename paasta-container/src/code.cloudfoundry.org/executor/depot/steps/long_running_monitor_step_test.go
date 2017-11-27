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

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
)

var _ = Describe("LongRunningMonitorStep", func() {
	var (
		readinessCheck, livenessCheck *fakes.FakeStep
		hasBecomeHealthy              <-chan struct{}
		clock                         *fakeclock.FakeClock
		fakeStreamer                  *fake_log_streamer.FakeLogStreamer
		fakeHealthCheckStreamer       *fake_log_streamer.FakeLogStreamer

		// monitorErr string

		startTimeout      time.Duration
		healthyInterval   time.Duration
		unhealthyInterval time.Duration

		step   steps.Step
		logger *lagertest.TestLogger
	)

	BeforeEach(func() {
		startTimeout = 1 * time.Second
		healthyInterval = 1 * time.Second
		unhealthyInterval = 500 * time.Millisecond

		readinessCheck = new(fakes.FakeStep)
		livenessCheck = new(fakes.FakeStep)

		clock = fakeclock.NewFakeClock(time.Now())

		fakeHealthCheckStreamer = newFakeStreamer()
		fakeStreamer = newFakeStreamer()

		logger = lagertest.NewTestLogger("test")
	})

	JustBeforeEach(func() {
		hasBecomeHealthyChannel := make(chan struct{}, 1000)
		hasBecomeHealthy = hasBecomeHealthyChannel

		fakeStreamer.WithSourceReturns(fakeStreamer)

		step = steps.NewLongRunningMonitor(
			readinessCheck,
			livenessCheck,
			hasBecomeHealthyChannel,
			logger,
			clock,
			fakeStreamer,
			fakeHealthCheckStreamer,
			startTimeout,
		)
	})

	Describe("Perform", func() {
		var (
			readinessResults chan error
			livenessResults  chan error

			performErr     chan error
			donePerforming *sync.WaitGroup
		)

		BeforeEach(func() {
			readinessResults = make(chan error, 10)
			livenessResults = make(chan error, 10)

			readinessCheck.PerformStub = func() error {
				return <-readinessResults
			}
			livenessCheck.PerformStub = func() error {
				return <-livenessResults
			}
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
			readinessResults <- errors.New("readiness-exited")
			livenessResults <- errors.New("liveness-exited")
			donePerforming.Wait()
		})

		It("emits a message to the applications log stream", func() {
			Eventually(fakeStreamer.Stdout().(*gbytes.Buffer)).Should(
				gbytes.Say("Starting health monitoring of container\n"),
			)
		})

		Context("when the readiness check fails", func() {
			BeforeEach(func() {
				readinessResults <- errors.New("booom!")
			})

			It("completes with failure", func() {
				var expectedError interface{}
				Eventually(performErr).Should(Receive(&expectedError))
				err, ok := expectedError.(*steps.EmittableError)
				Expect(ok).To(BeTrue())
				Expect(err.WrappedError()).To(MatchError(ContainSubstring("booom!")))
			})

			It("logs the step", func() {
				Eventually(logger.TestSink.LogMessages).Should(ConsistOf([]string{
					"test.monitor-step.timed-out-before-healthy",
				}))
			})

			It("emits the last healthcheck process response to the log stream", func() {
				Eventually(fakeHealthCheckStreamer.Stderr().(*gbytes.Buffer)).Should(
					gbytes.Say("booom!\n"),
				)
			})

			It("emits a log message explaining the timeout", func() {
				Eventually(fakeStreamer.Stderr().(*gbytes.Buffer)).Should(gbytes.Say(
					fmt.Sprintf("Timed out after %s: health check never passed.\n", startTimeout),
				))
			})
		})

		Context("when there is no start timeout", func() {
			BeforeEach(func() {
				hasBecomeHealthyChannel := make(chan struct{}, 1000)
				hasBecomeHealthy = hasBecomeHealthyChannel

				startTimeout = 0
			})

			Context("when the readiness check passes", func() {
				JustBeforeEach(func() {
					clock.Increment(time.Second)
					readinessResults <- nil
				})

				It("emits a healthy event", func() {
					Eventually(hasBecomeHealthy).Should(Receive())
				})
			})
		})

		Context("when the readiness check passes", func() {
			BeforeEach(func() {
				readinessResults <- nil
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

			Context("and the liveness check fails", func() {
				disaster := errors.New("oh no!")

				BeforeEach(func() {
					livenessResults <- disaster
				})

				JustBeforeEach(func() {
					Eventually(hasBecomeHealthy).Should(Receive())
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

				It("emits a log message for the failure", func() {
					Eventually(fakeStreamer.Stdout().(*gbytes.Buffer)).Should(
						gbytes.Say("Container became unhealthy\n"),
					)
				})

				It("emits the healthcheck process response for the failure", func() {
					Eventually(fakeHealthCheckStreamer.Stderr().(*gbytes.Buffer)).Should(
						gbytes.Say(fmt.Sprintf("oh no!\n")),
					)
				})

				It("completes with failure", func() {
					var expectedError interface{}
					Eventually(performErr).Should(Receive(&expectedError))
					err, ok := expectedError.(*steps.EmittableError)
					Expect(ok).To(BeTrue())
					Expect(err.WrappedError()).To(Equal(disaster))
				})
			})
		})
	})

	Describe("Cancel", func() {
		Context("while doing readiness check", func() {
			var performing chan struct{}

			BeforeEach(func() {
				performing = make(chan struct{})
				cancelled := make(chan struct{})

				readinessCheck.PerformStub = func() error {
					close(performing)

					select {
					case <-cancelled:
						return steps.ErrCancelled
					}
				}

				readinessCheck.CancelStub = func() {
					close(cancelled)
				}
			})

			It("cancels the in-flight check", func() {
				performResult := make(chan error)

				go func() { performResult <- step.Perform() }()

				Eventually(performing).Should(BeClosed())

				step.Cancel()

				Eventually(performResult).Should(Receive(Equal(steps.ErrCancelled)))
			})
		})

		Context("when readiness check passes", func() {
			BeforeEach(func() {
				readinessCheck.PerformReturns(nil)
			})

			Context("and while doing liveness check", func() {
				var performing chan struct{}

				BeforeEach(func() {
					performing = make(chan struct{})
					cancelled := make(chan struct{})

					livenessCheck.PerformStub = func() error {
						close(performing)

						select {
						case <-cancelled:
							return steps.ErrCancelled
						}
					}

					livenessCheck.CancelStub = func() {
						close(cancelled)
					}
				})

				It("cancels the in-flight check", func() {
					performResult := make(chan error)

					go func() { performResult <- step.Perform() }()

					Eventually(performing).Should(BeClosed())

					step.Cancel()

					Eventually(performResult).Should(Receive(Equal(steps.ErrCancelled)))
				})
			})
		})
	})
})
