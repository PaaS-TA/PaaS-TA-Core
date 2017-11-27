package steps_test

import (
	"errors"

	"code.cloudfoundry.org/lager/lagertest"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"

	"code.cloudfoundry.org/executor/depot/steps"
	"code.cloudfoundry.org/executor/depot/steps/fakes"
)

var _ = Describe("TryStep", func() {
	var step steps.Step
	var subStep steps.Step
	var thingHappened bool
	var cancelled bool
	var logger *lagertest.TestLogger

	BeforeEach(func() {
		thingHappened, cancelled = false, false

		subStep = &fakes.FakeStep{
			PerformStub: func() error {
				thingHappened = true
				return nil
			},
			CancelStub: func() {
				cancelled = true
			},
		}

		logger = lagertest.NewTestLogger("test")
	})

	JustBeforeEach(func() {
		step = steps.NewTry(subStep, logger)
	})

	It("performs its substep", func() {
		err := step.Perform()
		Expect(err).NotTo(HaveOccurred())

		Expect(thingHappened).To(BeTrue())
	})

	Context("when the substep fails", func() {
		disaster := errors.New("oh no!")

		BeforeEach(func() {
			subStep = &fakes.FakeStep{
				PerformStub: func() error {
					return disaster
				},
			}
		})

		It("succeeds anyway", func() {
			err := step.Perform()
			Expect(err).NotTo(HaveOccurred())
		})

		It("logs the failure", func() {
			err := step.Perform()
			Expect(err).NotTo(HaveOccurred())

			Expect(logger).To(gbytes.Say("failed"))
			Expect(logger).To(gbytes.Say("oh no!"))
		})
	})

	Context("when told to cancel", func() {
		It("passes the message along", func() {
			Expect(cancelled).To(BeFalse())
			step.Cancel()
			Expect(cancelled).To(BeTrue())
		})
	})
})
