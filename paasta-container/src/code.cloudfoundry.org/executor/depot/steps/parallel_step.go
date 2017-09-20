package steps

import "github.com/hashicorp/go-multierror"

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

	var aggregate *multierror.Error

	for _ = range step.substeps {
		err := <-errs
		if err != nil {
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
