package steps

type serialStep struct {
	steps  []Step
	cancel chan struct{}
}

func NewSerial(steps []Step) *serialStep {
	return &serialStep{
		steps: steps,

		cancel: make(chan struct{}),
	}
}

func (runner *serialStep) Perform() error {
	for _, action := range runner.steps {
		err := action.Perform()
		if err != nil {
			return err
		}
	}

	return nil
}

func (runner *serialStep) Cancel() {
	for _, step := range runner.steps {
		step.Cancel()
	}
}
