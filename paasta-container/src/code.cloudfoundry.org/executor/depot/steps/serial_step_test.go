package steps_test

import (
	"errors"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"code.cloudfoundry.org/executor/depot/steps"
	"code.cloudfoundry.org/executor/depot/steps/fakes"
)

var _ = Describe("SerialStep", func() {
	Describe("Perform", func() {
		It("performs them all in order and returns nil", func() {
			seq := make(chan int, 3)

			sequence := steps.NewSerial([]steps.Step{
				&fakes.FakeStep{
					PerformStub: func() error {
						seq <- 1
						return nil
					},
				},
				&fakes.FakeStep{
					PerformStub: func() error {
						seq <- 2
						return nil
					},
				},
				&fakes.FakeStep{
					PerformStub: func() error {
						seq <- 3
						return nil
					},
				},
			})

			result := make(chan error)
			go func() { result <- sequence.Perform() }()

			Eventually(seq).Should(Receive(Equal(1)))
			Eventually(seq).Should(Receive(Equal(2)))
			Eventually(seq).Should(Receive(Equal(3)))

			Eventually(result).Should(Receive(BeNil()))
		})

		Context("when an step fails in the middle", func() {
			It("returns the error and does not continue performing", func() {
				disaster := errors.New("oh no!")

				seq := make(chan int, 3)

				sequence := steps.NewSerial([]steps.Step{
					&fakes.FakeStep{
						PerformStub: func() error {
							seq <- 1
							return nil
						},
					},
					&fakes.FakeStep{
						PerformStub: func() error {
							return disaster
						},
					},
					&fakes.FakeStep{
						PerformStub: func() error {
							seq <- 3
							return nil
						},
					},
				})

				result := make(chan error)
				go func() { result <- sequence.Perform() }()

				Eventually(seq).Should(Receive(Equal(1)))

				Eventually(result).Should(Receive(Equal(disaster)))

				Consistently(seq).ShouldNot(Receive())
			})
		})
	})

	Describe("Cancel", func() {
		It("cancels all sub-steps", func() {
			step1 := &fakes.FakeStep{}
			step2 := &fakes.FakeStep{}
			step3 := &fakes.FakeStep{}

			sequence := steps.NewSerial([]steps.Step{step1, step2, step3})

			sequence.Cancel()

			Expect(step1.CancelCallCount()).To(Equal(1))
			Expect(step2.CancelCallCount()).To(Equal(1))
			Expect(step3.CancelCallCount()).To(Equal(1))
		})
	})
})
