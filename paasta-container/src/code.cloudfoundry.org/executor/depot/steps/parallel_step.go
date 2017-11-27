package steps

import (
	"fmt"

	"github.com/hashicorp/go-multierror"
)

type parallelStep struct {
	substeps []Step
}

func NewParallel(substeps []Step) *parallelStep {
	return &parallelStep{
		substeps: substeps,
	}
}

func (step *parallelStep) Perform() error {
	errs := make(chan error, len(step.substeps))

	for _, step := range step.substeps {
		go func(step Step) {
			errs <- step.Perform()
		}(step)
	}

	aggregate := &multierror.Error{}
	aggregate.ErrorFormat = step.errorFormat

	for _ = range step.substeps {
		err := <-errs
		if err != nil && err != ErrCancelled {
			aggregate = multierror.Append(aggregate, err)
		}
	}

	return aggregate.ErrorOrNil()
}

func (step *parallelStep) Cancel() {
	for _, step := range step.substeps {
		step.Cancel()
	}
}

func (step *parallelStep) errorFormat(errs []error) string {
	var err string
	for _, e := range errs {
		if err == "" {
			err = e.Error()
		} else {
			err = fmt.Sprintf("%s; %s", err, e)
		}
	}
	return err
}
