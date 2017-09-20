package steps

import (
	"errors"

	"github.com/hashicorp/go-multierror"
)

var CodependentStepExitedError = errors.New("Codependent step exited")

type codependentStep struct {
	substeps    []Step
	errorOnExit bool
}

func NewCodependent(substeps []Step, errorOnExit bool) *codependentStep {
	return &codependentStep{
		substeps:    substeps,
		errorOnExit: errorOnExit,
	}
}

func (step *codependentStep) Perform() error {
	errs := make(chan error, len(step.substeps))

	for _, step := range step.substeps {
		go func(step Step) {
			errs <- step.Perform()
		}(step)
	}

	var aggregate *multierror.Error
	var cancelled bool

	for _ = range step.substeps {
		err := <-errs
		if step.errorOnExit && err == nil {
			err = CodependentStepExitedError
		}

		if err != nil {
			aggregate = multierror.Append(aggregate, err)

			if !cancelled {
				cancelled = true
				step.Cancel()
			}
		}
	}

	return aggregate.ErrorOrNil()
}

func (step *codependentStep) Cancel() {
	for _, substep := range step.substeps {
		substep.Cancel()
	}
}
