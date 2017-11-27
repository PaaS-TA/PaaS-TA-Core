package steps_test

import (
	"bytes"
	"errors"

	"code.cloudfoundry.org/lager/lagertest"

	"code.cloudfoundry.org/executor/depot/log_streamer/fake_log_streamer"

	"code.cloudfoundry.org/executor/depot/steps"
	"code.cloudfoundry.org/executor/depot/steps/fakes"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("EmitProgressStep", func() {
	var step steps.Step
	var subStep steps.Step
	var cancelled bool
	var errorToReturn error
	var fakeStreamer *fake_log_streamer.FakeLogStreamer
	var startMessage, successMessage, failureMessage string
	var logger *lagertest.TestLogger
	var stderrBuffer *bytes.Buffer
	var stdoutBuffer *bytes.Buffer

	BeforeEach(func() {
		stderrBuffer = new(bytes.Buffer)
		stdoutBuffer = new(bytes.Buffer)
		errorToReturn = nil
		startMessage, successMessage, failureMessage = "", "", ""
		cancelled = false
		fakeStreamer = new(fake_log_streamer.FakeLogStreamer)

		fakeStreamer.StderrReturns(stderrBuffer)
		fakeStreamer.StdoutReturns(stdoutBuffer)

		subStep = &fakes.FakeStep{
			PerformStub: func() error {
				fakeStreamer.Stdout().Write([]byte("RUNNING\n"))
				return errorToReturn
			},
			CancelStub: func() {
				cancelled = true
			},
		}

		logger = lagertest.NewTestLogger("test")
	})

	JustBeforeEach(func() {
		step = steps.NewEmitProgress(subStep, startMessage, successMessage, failureMessage, fakeStreamer, logger)
	})

	Context("running", func() {
		Context("when there is a start message", func() {
			BeforeEach(func() {
				startMessage = "STARTING"
			})

			It("should emit the start message before performing", func() {
				err := step.Perform()
				Expect(err).NotTo(HaveOccurred())
				Expect(stdoutBuffer.String()).To(Equal("STARTING\nRUNNING\n"))
			})
		})

		Context("when there is no start or success message", func() {
			It("should not emit the start message (i.e. a newline) before performing", func() {
				err := step.Perform()
				Expect(err).NotTo(HaveOccurred())
				Expect(stdoutBuffer.String()).To(Equal("RUNNING\n"))
			})
		})

		Context("when the substep succeeds and there is a success message", func() {
			BeforeEach(func() {
				successMessage = "SUCCESS"
			})

			It("should emit the sucess message", func() {
				err := step.Perform()
				Expect(err).NotTo(HaveOccurred())
				Expect(stdoutBuffer.String()).To(Equal("RUNNING\nSUCCESS\n"))
			})
		})

		Context("when the substep fails", func() {
			BeforeEach(func() {
				errorToReturn = errors.New("bam!")
			})

			It("should pass the error along", func() {
				err := step.Perform()
				Expect(err).To(MatchError(errorToReturn))
			})

			Context("and there is a failure message", func() {
				BeforeEach(func() {
					failureMessage = "FAIL"
				})

				It("should emit the failure message", func() {
					step.Perform()

					Expect(stdoutBuffer.String()).To(Equal("RUNNING\n"))
					Expect(stderrBuffer.String()).To(Equal("FAIL\n"))
				})

				Context("with an emittable error", func() {
					BeforeEach(func() {
						errorToReturn = steps.NewEmittableError(errors.New("bam!"), "Failed to reticulate")
					})

					It("should print out the emittable error", func() {
						step.Perform()

						Expect(stdoutBuffer.String()).To(Equal("RUNNING\n"))
						Expect(stderrBuffer.String()).To(Equal("FAIL: Failed to reticulate\n"))
					})

					It("logs the error", func() {
						step.Perform()

						logs := logger.TestSink.Logs()
						Expect(logs).To(HaveLen(1))

						Expect(logs[0].Message).To(ContainSubstring("errored"))
						Expect(logs[0].Data["wrapped-error"]).To(Equal("bam!"))
						Expect(logs[0].Data["message-emitted"]).To(Equal("Failed to reticulate"))
					})

					Context("without a wrapped error", func() {
						BeforeEach(func() {
							errorToReturn = steps.NewEmittableError(nil, "Failed to reticulate")
						})

						It("should print out the emittable error", func() {
							step.Perform()

							Expect(stdoutBuffer.String()).To(Equal("RUNNING\n"))
							Expect(stderrBuffer.String()).To(Equal("FAIL: Failed to reticulate\n"))
						})

						It("logs the error", func() {
							step.Perform()

							logs := logger.TestSink.Logs()
							Expect(logs).To(HaveLen(1))

							Expect(logs[0].Message).To(ContainSubstring("errored"))
							Expect(logs[0].Data["wrapped-error"]).To(BeEmpty())
							Expect(logs[0].Data["message-emitted"]).To(Equal("Failed to reticulate"))
						})
					})
				})
			})

			Context("and there is no failure message", func() {
				BeforeEach(func() {
					errorToReturn = steps.NewEmittableError(errors.New("bam!"), "Failed to reticulate")
				})

				It("should not emit the failure message or error, even with an emittable error", func() {
					step.Perform()

					Expect(stdoutBuffer.String()).To(Equal("RUNNING\n"))
					Expect(stderrBuffer.String()).To(BeEmpty())
				})
			})
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
