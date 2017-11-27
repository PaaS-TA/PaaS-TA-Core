package steps

import (
	"code.cloudfoundry.org/executor/depot/log_streamer"

	"code.cloudfoundry.org/lager"
)

type emitProgressStep struct {
	substep        Step
	logger         lager.Logger
	startMessage   string
	successMessage string
	failureMessage string
	streamer       log_streamer.LogStreamer
}

func NewEmitProgress(
	substep Step,
	startMessage,
	successMessage,
	failureMessage string,
	streamer log_streamer.LogStreamer,
	logger lager.Logger,
) *emitProgressStep {
	logger = logger.Session("emit-progress-step")
	return &emitProgressStep{
		substep:        substep,
		logger:         logger,
		startMessage:   startMessage,
		successMessage: successMessage,
		failureMessage: failureMessage,
		streamer:       streamer,
	}
}

func (step *emitProgressStep) Perform() error {
	if step.startMessage != "" {
		step.streamer.Stdout().Write([]byte(step.startMessage + "\n"))
	}

	err := step.substep.Perform()
	if err != nil {
		if step.failureMessage != "" {
			step.streamer.Stderr().Write([]byte(step.failureMessage))

			if emittableError, ok := err.(*EmittableError); ok {
				step.streamer.Stderr().Write([]byte(": "))
				step.streamer.Stderr().Write([]byte(emittableError.Error()))

				wrappedMessage := ""
				if emittableError.WrappedError() != nil {
					wrappedMessage = emittableError.WrappedError().Error()
				}

				step.logger.Info("errored", lager.Data{
					"wrapped-error":   wrappedMessage,
					"message-emitted": emittableError.Error(),
				})
			}

			step.streamer.Stderr().Write([]byte("\n"))
		}
	} else {
		if step.successMessage != "" {
			step.streamer.Stdout().Write([]byte(step.successMessage + "\n"))
		}
	}

	return err
}

func (step *emitProgressStep) Cancel() {
	step.substep.Cancel()
}
