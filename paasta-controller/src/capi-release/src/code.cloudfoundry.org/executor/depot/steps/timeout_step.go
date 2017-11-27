package steps

import (
	"time"

	"code.cloudfoundry.org/lager"
)

type timeoutStep struct {
	substep    Step
	timeout    time.Duration
	cancelChan chan struct{}
	logger     lager.Logger
}

func NewTimeout(substep Step, timeout time.Duration, logger lager.Logger) *timeoutStep {
	return &timeoutStep{
		substep:    substep,
		timeout:    timeout,
		cancelChan: make(chan struct{}),
		logger:     logger.Session("timeout-step"),
	}
}

func (step *timeoutStep) Perform() error {
	resultChan := make(chan error, 1)
	timer := time.NewTimer(step.timeout)
	defer timer.Stop()

	go func() {
		resultChan <- step.substep.Perform()
	}()

	for {
		select {
		case err := <-resultChan:
			return err

		case <-timer.C:
			step.logger.Error("timed-out", nil)

			step.substep.Cancel()

			err := <-resultChan
			return NewEmittableError(err, emittableMessage(step.timeout, err))
		}
	}
}

func (step *timeoutStep) Cancel() {
	step.substep.Cancel()
}

func emittableMessage(timeout time.Duration, substepErr error) string {
	message := "exceeded " + timeout.String() + " timeout"

	if emittable, ok := substepErr.(*EmittableError); ok {
		message += "; " + emittable.Error()
	}

	return message
}
