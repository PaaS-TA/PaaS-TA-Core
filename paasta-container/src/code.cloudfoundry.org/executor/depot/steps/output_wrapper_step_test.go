package steps_test

import (
	"bytes"
	"errors"

	"code.cloudfoundry.org/executor/depot/steps"
	"code.cloudfoundry.org/executor/depot/steps/fakes"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("OutputWrapperStep", func() {
	var (
		subStep *fakes.FakeStep
		step    steps.Step
		buffer  *bytes.Buffer
	)

	BeforeEach(func() {
		subStep = &fakes.FakeStep{}
		buffer = bytes.NewBuffer(nil)
		step = steps.NewOutputWrapper(subStep, buffer)
	})

	Context("Perform", func() {
		It("calls perform on the substep", func() {
			step.Perform()
			Expect(subStep.PerformCallCount()).To(Equal(1))
		})

		Context("when the substep fails", func() {
			var (
				errStr string
			)

			BeforeEach(func() {
				subStep.PerformReturns(errors.New("BOOOM!"))
				errStr = "error reason"
			})

			JustBeforeEach(func() {
				buffer.WriteString(errStr)
			})

			It("wraps the buffer content in an emittable error", func() {
				err := step.Perform()
				Expect(err).To(MatchError("error reason"))
			})

			Context("when the output has whitespaces", func() {
				BeforeEach(func() {
					errStr = "\r\nerror reason\r\n"
				})

				It("trims the extra whitespace", func() {
					err := step.Perform()
					Expect(err).To(MatchError("error reason"))
				})
			})
		})

		Context("when the substep is cancelled", func() {
			BeforeEach(func() {
				subStep.PerformReturns(steps.ErrCancelled)
			})

			It("returns the ErrCancelled error", func() {
				err := step.Perform()
				Expect(err).To(Equal(steps.ErrCancelled))
			})

			Context("and the buffer has data", func() {
				BeforeEach(func() {
					buffer.WriteString("error reason")
				})

				It("wraps the buffer content in an emittable error", func() {
					err := step.Perform()
					Expect(err).To(MatchError("error reason"))
				})
			})
		})
	})

	Context("Cancel", func() {
		It("calls cancel on the substep", func() {
			step.Cancel()
			Expect(subStep.CancelCallCount()).To(Equal(1))
		})
	})
})
