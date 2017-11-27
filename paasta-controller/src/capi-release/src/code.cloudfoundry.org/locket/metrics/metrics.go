package metrics

import (
	"os"
	"time"

	"code.cloudfoundry.org/clock"
	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/locket/db"
	"code.cloudfoundry.org/locket/models"
	"code.cloudfoundry.org/runtimeschema/metric"
	"github.com/tedsuo/ifrit"
)

const (
	activeLocks     = metric.Metric("ActiveLocks")
	activePresences = metric.Metric("ActivePresences")
)

type metricsNotifier struct {
	logger          lager.Logger
	ticker          clock.Clock
	metricsInterval time.Duration
	lockDB          db.LockDB
}

func NewMetricsNotifier(logger lager.Logger, ticker clock.Clock, metricsInterval time.Duration, lockDB db.LockDB) ifrit.Runner {
	return &metricsNotifier{
		logger:          logger,
		ticker:          ticker,
		metricsInterval: metricsInterval,
		lockDB:          lockDB,
	}
}

func (notifier *metricsNotifier) Run(signals <-chan os.Signal, ready chan<- struct{}) error {
	logger := notifier.logger.Session("metrics-notifier")
	logger.Info("starting")
	defer logger.Info("compeleted")
	close(ready)

	tick := notifier.ticker.NewTicker(notifier.metricsInterval)
	for {
		select {
		case <-signals:
			return nil
		case <-tick.C():
			locks, err := notifier.lockDB.Count(logger, models.LockType)
			if err != nil {
				logger.Error("failed-to-retrieve-lock-count", err)
				continue
			}
			presences, err := notifier.lockDB.Count(logger, models.PresenceType)
			if err != nil {
				logger.Error("failed-to-retrieve-presence-count", err)
				continue
			}

			activeLocks.Send(locks)
			activePresences.Send(presences)
		}
	}
	return nil
}
