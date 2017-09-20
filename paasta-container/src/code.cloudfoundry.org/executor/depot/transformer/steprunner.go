package transformer

import (
	"os"

	"code.cloudfoundry.org/executor/depot/steps"
)

type StepRunner struct {
	action            steps.Step
	healthCheckPassed <-chan struct{}
}

func newStepRunner(action steps.Step, healthCheckPassed <-chan struct{}) *StepRunner {
	return &StepRunner{action: action, healthCheckPassed: healthCheckPassed}
}

func (p *StepRunner) Run(signals <-chan os.Signal, ready chan<- struct{}) error {
	resultCh := make(chan error)
	go func() {
		resultCh <- p.action.Perform()
	}()

	for {
		select {
		case <-p.healthCheckPassed:
			p.healthCheckPassed = nil
			close(ready)

		case <-signals:
			signals = nil
			p.action.Cancel()

		case err := <-resultCh:
			if p.healthCheckPassed != nil {
				close(ready)
			}
			return err
		}
	}
}
