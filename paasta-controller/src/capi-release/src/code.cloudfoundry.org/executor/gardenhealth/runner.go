package gardenhealth

import (
	"os"
	"time"

	"code.cloudfoundry.org/clock"
	"code.cloudfoundry.org/executor"
	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/loggregator_v2"
)

const UnhealthyCell = "UnhealthyCell"

type HealthcheckTimeoutError struct{}

func (HealthcheckTimeoutError) Error() string {
	return "garden healthcheck timed out"
}

// Runner coordinates health checks against an executor client.  When checks fail or
// time out, its executor will be marked as unhealthy until a successful check occurs.
//
// See NewRunner and Runner.Run for more details.
type Runner struct {
	failures         int
	healthy          bool
	checkInterval    time.Duration
	emissionInterval time.Duration
	timeoutInterval  time.Duration
	logger           lager.Logger
	checker          Checker
	executorClient   executor.Client
	metronClient     loggregator_v2.Client
	clock            clock.Clock
}

// NewRunner constructs a healthcheck runner.
//
// The checkInterval parameter controls how often the healthcheck should run, and
// the timeoutInterval sets the time to wait for the healthcheck to complete before
// marking the executor as unhealthy.
func NewRunner(
	checkInterval time.Duration,
	emissionInterval time.Duration,
	timeoutInterval time.Duration,
	logger lager.Logger,
	checker Checker,
	executorClient executor.Client,
	metronClient loggregator_v2.Client,
	clock clock.Clock,
) *Runner {
	return &Runner{
		checkInterval:    checkInterval,
		emissionInterval: emissionInterval,
		timeoutInterval:  timeoutInterval,
		logger:           logger.Session("garden-healthcheck"),
		checker:          checker,
		executorClient:   executorClient,
		metronClient:     metronClient,
		clock:            clock,
		healthy:          false,
		failures:         0,
	}
}

// Run coordinates the execution of the healthcheck. It responds to incoming signals,
// monitors the elapsed time to determine timeouts, and ensures the healthcheck runs periodically.
//
// Note: If the healthcheck has not returned before the timeout expires, we
// intentionally do not kill the healthcheck process, and we don't spawn a new healthcheck
// until the existing healthcheck exits. It may be necessary for an operator to
// inspect the long running container to debug the problem.
func (r *Runner) Run(signals <-chan os.Signal, ready chan<- struct{}) error {
	logger := r.logger.Session("garden-health")
	healthcheckTimeout := r.clock.NewTimer(r.timeoutInterval)
	healthcheckComplete := make(chan error, 1)

	logger.Info("starting")

	go r.healthcheckCycle(logger, healthcheckComplete)

	select {
	case <-signals:
		return nil

	case <-healthcheckTimeout.C():
		logger.Error("failed-initial-healthcheck-timeout", nil)
		r.setUnhealthy(logger)
		return HealthcheckTimeoutError{}

	case err := <-healthcheckComplete:
		if err != nil {
			logger.Error("failed-initial-healthcheck", err)
			r.setUnhealthy(logger)
			return err
		}
		healthcheckTimeout.Stop()
	}

	logger.Info("passed-initial-healthcheck")
	r.setHealthy(logger)

	close(ready)
	logger.Info("started")

	startHealthcheck := r.clock.NewTimer(r.checkInterval)
	emitInterval := r.clock.NewTicker(r.emissionInterval)
	defer emitInterval.Stop()

	for {
		select {
		case <-signals:
			logger.Info("complete")
			return nil

		case <-startHealthcheck.C():
			logger.Info("check-starting")
			healthcheckTimeout.Reset(r.timeoutInterval)
			go r.healthcheckCycle(logger, healthcheckComplete)

		case <-healthcheckTimeout.C():
			logger.Error("failed-healthcheck-timeout", nil)
			r.setUnhealthy(logger)

		case <-emitInterval.C():
			r.emitUnhealthyCellMetric(logger)

		case err := <-healthcheckComplete:

			timeoutOk := healthcheckTimeout.Stop()
			switch err.(type) {
			case nil:
				logger.Info("passed-health-check")
				if timeoutOk {
					r.setHealthy(logger)
				}

			default:
				logger.Error("failed-health-check", err)
				r.setUnhealthy(logger)
			}

			startHealthcheck.Reset(r.checkInterval)
			logger.Info("check-complete")
		}
	}
}

func (r *Runner) setHealthy(logger lager.Logger) {
	r.logger.Info("set-state-healthy")
	r.executorClient.SetHealthy(logger, true)
	r.emitUnhealthyCellMetric(logger)
}

func (r *Runner) setUnhealthy(logger lager.Logger) {
	r.logger.Error("set-state-unhealthy", nil)
	r.executorClient.SetHealthy(logger, false)
	r.emitUnhealthyCellMetric(logger)
}

func (r *Runner) emitUnhealthyCellMetric(logger lager.Logger) {
	var err error
	if r.executorClient.Healthy(logger) {
		err = r.metronClient.SendMetric(UnhealthyCell, 0)
	} else {
		err = r.metronClient.SendMetric(UnhealthyCell, 1)
	}

	if err != nil {
		logger.Error("failed-to-send-unhealthy-cell-metric", err)
	}
}

func (r *Runner) healthcheckCycle(logger lager.Logger, healthcheckComplete chan<- error) {
	healthcheckComplete <- r.checker.Healthcheck(logger)
}
