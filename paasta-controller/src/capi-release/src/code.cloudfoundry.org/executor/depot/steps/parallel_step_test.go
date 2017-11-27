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
