package consuldownchecker

import (
	"os"
	"strings"
	"time"

	"code.cloudfoundry.org/clock"
	"code.cloudfoundry.org/consuladapter"
	"code.cloudfoundry.org/lager"
)

type ConsulDownChecker struct {
	logger        lager.Logger
	clock         clock.Clock
	consulClient  consuladapter.Client
	retryInterval time.Duration
}

func NewConsulDownChecker(
	logger lager.Logger,
	clock clock.Clock,
	consulClient consuladapter.Client,
	retryInterval time.Duration,
) *ConsulDownChecker {
	return &ConsulDownChecker{
		logger: logger, clock: clock, consulClient: consulClient, retryInterval: retryInterval,
	}
}

func (c *ConsulDownChecker) Run(signals <-chan os.Signal, ready chan<- struct{}) error {
	logger := c.logger.Session("consul-down-checker")
	logger.Info("starting")
	defer logger.Info("finished")
	retryTimer := c.clock.NewTimer(0)

	hasLeaderCounter := 0
	leaderCheckCounter := 0

	for {
		select {
		case <-signals:
			logger.Info("received-signal")
			return nil
		case <-retryTimer.C():
			leaderCheckCounter++
			hasLeader, err := c.checkForLeader(logger)
			if err != nil {
				return err
			}
			if hasLeader {
				hasLeaderCounter++
				logger.Info("consul-has-leader", lager.Data{"attempts": hasLeaderCounter})
			} else {
				logger.Info("still-down", lager.Data{"attempts": leaderCheckCounter})
				hasLeaderCounter = 0
			}
			if hasLeaderCounter > 2 {
				return nil
			}
			if leaderCheckCounter == 3 {
				close(ready)
			}

			retryTimer.Reset(c.retryInterval)
		}
	}
}

func (c *ConsulDownChecker) checkForLeader(logger lager.Logger) (bool, error) {
	leader, err := c.consulClient.Status().Leader()
	if err != nil && !strings.Contains(err.Error(), "Unexpected response code: 500") {
		logger.Error("failed-getting-leader", err)
		return false, err
	}

	if leader != "" {
		return true, nil
	}

	return false, nil
}
