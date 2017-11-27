package steps

import (
	"fmt"
	"time"

	"code.cloudfoundry.org/clock"
	"code.cloudfoundry.org/executor/depot/log_streamer"
	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/workpool"
)

func invalidInterval(field string, interval time.Duration) error {
	return fmt.Errorf("The %s interval, %s, is not positive.", field, interval.String())
}

const timeoutMessage = "Timed out after %s: health check never passed.\n"
const timeoutCrashReason = "Instance never healthy after %s: %s"
const healthcheckNowUnhealthy = "Instance became unhealthy: %s"

type monitorStep struct {
	checkFunc         func() Step
	hasStartedRunning chan<- struct{}

	logger      lager.Logger
	clock       clock.Clock
	logStreamer log_streamer.LogStreamer

	startTimeout      time.Duration
	healthyInterval   time.Duration
	unhealthyInterval time.Duration
	workPool          *workpool.WorkPool

	*canceller
}

func NewMonitor(
	checkFunc func() Step,
	hasStartedRunning chan<- struct{},
	logger lager.Logger,
	clock clock.Clock,
	logStreamer log_streamer.LogStreamer,
	startTimeout time.Duration,
	healthyInterval time.Duration,
	unhealthyInterval time.Duration,
	workPool *workpool.WorkPool,
) Step {
	logger = logger.Session("monitor-step")

	return &monitorStep{
		checkFunc:         checkFunc,
		hasStartedRunning: hasStartedRunning,
		logger:            logger,
		clock:             clock,
		logStreamer:       logStreamer,
		startTimeout:      startTimeout,
		healthyInterval:   healthyInterval,
		unhealthyInterval: unhealthyInterval,

		canceller: newCanceller(),
		workPool:  workPool,
	}
}

func (step *monitorStep) Perform() error {
	if step.healthyInterval <= 0 {
		return invalidInterval("healthy", step.healthyInterval)
	}

	if step.unhealthyInterval <= 0 {
		return invalidInterval("unhealthy", step.unhealthyInterval)
	}

	healthy := false
	interval := step.unhealthyInterval

	var startBy *time.Time
	if step.startTimeout > 0 {
		t := step.clock.Now().Add(step.startTimeout)
		startBy = &t
	}

	timer := step.clock.NewTimer(interval)
	defer timer.Stop()

	fmt.Fprint(step.logStreamer.Stdout(), "Starting health monitoring of container\n")

	for {
		select {
		case now := <-timer.C():
			stepResult := make(chan error)

			check := step.checkFunc()

			step.workPool.Submit(func() {
				stepResult <- check.Perform()
			})

			select {
			case stepErr := <-stepResult:
				nowHealthy := stepErr == nil

				if healthy && !nowHealthy {
					step.logger.Info("transitioned-to-unhealthy")

					fmt.Fprintf(step.logStreamer.Stderr(), "%s\n", stepErr.Error())
					fmt.Fprint(step.logStreamer.Stdout(), "Container became unhealthy\n")

					return NewEmittableError(stepErr, healthcheckNowUnhealthy, stepErr.Error())
				} else if !healthy && nowHealthy {
					step.logger.Info("transitioned-to-healthy")
					healthy = true
					step.hasStartedRunning <- struct{}{}

					fmt.Fprint(step.logStreamer.Stdout(), "Container became healthy\n")

					interval = step.healthyInterval
					startBy = nil
				}

				if startBy != nil && now.After(*startBy) {
					if !healthy {
						fmt.Fprintf(step.logStreamer.Stderr(), "%s\n", stepErr.Error())
						fmt.Fprintf(step.logStreamer.Stderr(), timeoutMessage, step.startTimeout)

						step.logger.Info("timed-out-before-healthy", lager.Data{
							"step-error": stepErr.Error(),
						})

						return NewEmittableError(stepErr, timeoutCrashReason, step.startTimeout, stepErr.Error())
					}

					startBy = nil
				}

			case <-step.Cancelled():
				check.Cancel()
				return <-stepResult
			}

		case <-step.Cancelled():
			return ErrCancelled
		}

		timer.Reset(interval)
	}

	panic("unreachable")
}
