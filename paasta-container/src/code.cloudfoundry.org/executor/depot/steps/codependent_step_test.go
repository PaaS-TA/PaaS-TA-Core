package steps_test

import (
	"errors"
	"sync"

	"github.com/hashicorp/go-multierror"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"code.cloudfoundry.org/executor/depot/steps"
	"code.cloudfoundry.org/executor/depot/steps/fakes"
)

var _ = Describe("CodependentStep", func() {
	var step steps.Step
	var subStep1 *fakes.FakeStep
	var subStep2 *fakes.FakeStep

	var thingHappened chan bool
	var cancelled chan bool

	var errorOnExit bool

	BeforeEach(func() {
		errorOnExit = false

		thingHappened = make(chan bool, 2)
		cancelled = make(chan bool, 2)

		running := new(sync.WaitGroup)
		running.Add(2)

		subStep1 = &fakes.FakeStep{
			PerformStub: func() error {
				running.Done()
				running.Wait()
				thingHappened <- true
				return nil
			},
			CancelStub: func() {
				cancelled <- true
			},
		}

		subStep2 = &fakes.FakeStep{
			PerformStub: func() error {
				running.Done()
				running.Wait()
				thingHappened <- true
				return nil
			},
			CancelStub: func() {
				cancelled <- true
			},
		}
	})

	Describe("Perform", func() {
		JustBeforeEach(func() {
			step = steps.NewCodependent([]steps.Step{subStep1, subStep2}, errorOnExit)
		})

		It("performs its substeps in parallel", func() {
			err := step.Perform()
			Expect(err).NotTo(HaveOccurred())

			Eventually(thingHappened).Should(Receive())
			Eventually(thingHappened).Should(Receive())
		})

		Context("when one of the substeps fails", func() {
			disaster := errors.New("oh no!")

			BeforeEach(func() {
				subStep1 = &fakes.FakeStep{
					PerformStub: func() error {
						return disaster
					},
				}

				subStep2 = &fakes.FakeStep{
					PerformStub: func() error {
						return nil
					},
				}
			})

			It("returns an aggregate of the failures", func() {
				err := step.Perform()
				Expect(err.(*multierror.Error).WrappedErrors()).To(ConsistOf(disaster))
			})

			It("cancels all the steps", func() {
				step.Perform()

				Expect(subStep1.CancelCallCount()).To(Equal(1))
				Expect(subStep2.CancelCallCount()).To(Equal(1))
			})

			Context("when step is cancelled", func() {
				BeforeEach(func() {
					subStep2 = &fakes.FakeStep{
						PerformStub: func() error {
							return steps.ErrCancelled
						},
					}
				})

				It("does not add cancelled error to message", func() {
					err := step.Perform()
					Expect(err).To(HaveOccurred())
					errMsg := err.Error()
					Expect(errMsg).To(Equal("oh no!"))
				})
			})
		})

		Context("when one of the substeps exits without failure", func() {
			var (
				cancelledError, err error
				errCh               chan error
			)

			BeforeEach(func() {
				cancelledError = errors.New("I was cancelled yo.")
				cancelled2 := make(chan bool, 1)

				subStep1.PerformStub = func() error {
					return nil
				}

				subStep2.PerformStub = func() error {
					<-cancelled2
					return cancelledError
				}

				subStep2.CancelStub = func() {
					cancelled2 <- true
				}
			})

			JustBeforeEach(func() {
				errCh = make(chan error)

				go func() {
					errCh <- step.Perform()
				}()
			})

			It("continues to perform the other step", func() {
				Consistently(errCh).ShouldNot(Receive())

				By("cancelling, it should return")
				step.Cancel()
				Eventually(errCh).Should(Receive())
			})

			Context("when errorOnExit is set to true", func() {
				BeforeEach(func() {
					errorOnExit = true
				})

				It("returns an aggregate of the failures", func() {
					Eventually(errCh).Should(Receive(&err))
					Expect(err.(*multierror.Error).WrappedErrors()).To(ConsistOf(cancelledError, steps.CodependentStepExitedError))
				})

				It("cancels all the steps", func() {
					Eventually(errCh).Should(Receive())

					Expect(subStep1.CancelCallCount()).To(Equal(1))
					Expect(subStep2.CancelCallCount()).To(Equal(1))
				})
			})
		})

		Context("when multiple substeps fail", func() {
			disaster1 := errors.New("oh no")
			disaster2 := errors.New("oh my")

			BeforeEach(func() {
				subStep1 = &fakes.FakeStep{
					PerformStub: func() error {
						return disaster1
					},
				}

				subStep2 = &fakes.FakeStep{
					PerformStub: func() error {
						return disaster2
					},
				}
			})

			It("joins the error messages with a semicolon", func() {
				err := step.Perform()
				Expect(err).To(HaveOccurred())
				errMsg := err.Error()
				Expect(errMsg).NotTo(HavePrefix(";"))
				Expect(errMsg).To(ContainSubstring("oh no"))
				Expect(errMsg).To(ContainSubstring("oh my"))
				Expect(errMsg).To(MatchRegexp(`\w+; \w+`))
			})
		})
	})

	Describe("Cancel", func() {
		It("cancels all sub-steps", func() {
			step1 := &fakes.FakeStep{}
			step2 := &fakes.FakeStep{}
			step3 := &fakes.FakeStep{}

			sequence := steps.NewCodependent([]steps.Step{step1, step2, step3}, errorOnExit)

			sequence.Cancel()

			Expect(step1.CancelCallCount()).To(Equal(1))
			Expect(step2.CancelCallCount()).To(Equal(1))
			Expect(step3.CancelCallCount()).To(Equal(1))
		})
	})
})
