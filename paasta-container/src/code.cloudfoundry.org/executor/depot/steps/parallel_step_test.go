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

var _ = Describe("ParallelStep", func() {
	var step steps.Step
	var subStep1 steps.Step
	var subStep2 steps.Step

	var thingHappened chan bool
	var cancelled chan bool

	BeforeEach(func() {
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

	JustBeforeEach(func() {
		step = steps.NewParallel([]steps.Step{subStep1, subStep2})
	})

	It("performs its substeps in parallel", func(done Done) {
		defer close(done)

		err := step.Perform()
		Expect(err).NotTo(HaveOccurred())

		Eventually(thingHappened).Should(Receive())
		Eventually(thingHappened).Should(Receive())
	}, 2)

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

		Context("when step is cancelled", func() {
			BeforeEach(func() {
				subStep1 = &fakes.FakeStep{
					PerformStub: func() error {
						return steps.ErrCancelled
					},
				}
			})

			It("does not add cancelled error to message", func() {
				err := step.Perform()
				Expect(err).To(HaveOccurred())
				errMsg := err.Error()
				Expect(errMsg).NotTo(HavePrefix(";"))
				Expect(errMsg).To(ContainSubstring("oh my"))
				Expect(errMsg).NotTo(ContainSubstring(steps.ErrCancelled.Error()))
			})
		})
	})

	Context("when one of the substeps fails", func() {
		disaster := errors.New("oh no!")
		var triggerStep2 chan struct{}
		var step2Completed chan struct{}

		BeforeEach(func() {
			triggerStep2 = make(chan struct{})
			step2Completed = make(chan struct{})

			subStep1 = &fakes.FakeStep{
				PerformStub: func() error {
					return disaster
				},
			}

			subStep2 = &fakes.FakeStep{
				PerformStub: func() error {
					<-triggerStep2
					close(step2Completed)
					return nil
				},
			}
		})

		It("waits for the rest to finish", func() {
			errs := make(chan error)

			go func() {
				errs <- step.Perform()
			}()

			Consistently(errs).ShouldNot(Receive())

			close(triggerStep2)

			Eventually(step2Completed).Should(BeClosed())

			var err error
			Eventually(errs).Should(Receive(&err))
			Expect(err.(*multierror.Error).WrappedErrors()).To(ConsistOf(disaster))
		})
	})

	Context("when told to cancel", func() {
		It("passes the message along", func() {
			step.Cancel()

			Eventually(cancelled).Should(Receive())
			Eventually(cancelled).Should(Receive())
		})
	})
})
