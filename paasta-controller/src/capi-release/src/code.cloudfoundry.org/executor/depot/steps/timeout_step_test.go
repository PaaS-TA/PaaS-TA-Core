package steps_test

import (
	"errors"
	"time"

	"code.cloudfoundry.org/executor/depot/steps"
	"code.cloudfoundry.org/executor/depot/steps/fakes"
	"code.cloudfoundry.org/lager/lagertest"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("TimeoutStep", func() {
	var (
		substepReadyChan    chan struct{}
		substepPerformTime  time.Duration
		substepFinishedChan chan struct{}
		substepPerformError error
		substep             *fakes.FakeStep

		timeout time.Duration
		logger  *lagertest.TestLogger
	)

	BeforeEach(func() {
		substepReadyChan = make(chan struct{})
		substepFinishedChan = make(chan struct{})

		substep = &fakes.FakeStep{
			PerformStub: func() error {
				close(substepReadyChan)
				time.Sleep(substepPerformTime)
				close(substepFinishedChan)
				return substepPerformError
			},
		}

		logger = lagertest.NewTestLogger("test")
	})

	Describe("Perform", func() {
		var err error

		JustBeforeEach(func() {
			err = steps.NewTimeout(substep, timeout, logger).Perform()
		})

		Context("When the substep finishes before the timeout expires", func() {
			BeforeEach(func() {
				substepPerformTime = 10 * time.Millisecond
				timeout = 100 * time.Millisecond
			})

			Context("when the substep returns an error", func() {
				BeforeEach(func() {
					substepPerformError = errors.New("some error")
				})

				It("performs the substep", func() {
					Expect(substepFinishedChan).To(BeClosed())
				})

				It("returns this error", func() {
					Expect(err).To(HaveOccurred())
					Expect(err).To(Equal(substepPerformError))
				})
			})

			Context("when the substep does not error", func() {
				BeforeEach(func() {
					substepPerformError = nil
				})

				It("performs the substep", func() {
					Expect(substepFinishedChan).To(BeClosed())
				})

				It("does not error", func() {
					Expect(err).NotTo(HaveOccurred())
				})
			})
		})

		Context("When the timeout expires before the substep finishes", func() {
			BeforeEach(func() {
				substepPerformTime = 100 * time.Millisecond
				timeout = 10 * time.Millisecond
			})

			It("cancels the substep", func() {
				Expect(substep.CancelCallCount()).To(Equal(1))
			})

			It("waits until the substep completes performing", func() {
				Expect(substepFinishedChan).To(BeClosed())
			})

			It("logs the timeout", func() {
				Eventually(logger.TestSink.LogMessages).Should(ConsistOf([]string{
					"test.timeout-step.timed-out",
				}))
			})

			Context("when the substep does not error", func() {
				BeforeEach(func() {
					substepPerformError = nil
				})

				It("returns an emittable error", func() {
					Expect(err).To(HaveOccurred())
					Expect(err).To(BeAssignableToTypeOf(&steps.EmittableError{}))
				})
			})

			Context("when the substep returns an error", func() {
				Context("when the error is not emittable", func() {
					BeforeEach(func() {
						substepPerformError = errors.New("some error")
					})

					It("returns a timeout error which does not include the error returned by the substep", func() {
						Expect(err).To(HaveOccurred())
						Expect(err).To(BeAssignableToTypeOf(&steps.EmittableError{}))
						Expect(err.Error()).NotTo(ContainSubstring("some error"))
						Expect(err.(*steps.EmittableError).WrappedError()).To(Equal(substepPerformError))
					})
				})

				Context("when the error is emittable", func() {
					BeforeEach(func() {
						substepPerformError = steps.NewEmittableError(nil, "some error")
					})

					It("returns a timeout error which includes the error returned by the substep", func() {
						Expect(err).To(HaveOccurred())
						Expect(err).To(BeAssignableToTypeOf(&steps.EmittableError{}))
						Expect(err.Error()).To(ContainSubstring("some error"))
						Expect(err.(*steps.EmittableError).WrappedError()).To(Equal(substepPerformError))
					})
				})
			})
		})
	})

	Describe("Cancel", func() {
		It("cancels the nested step", func() {
			step := steps.NewTimeout(substep, timeout, logger)
			step.Cancel()

			Expect(substep.CancelCallCount()).To(Equal(1))
		})
	})
})
