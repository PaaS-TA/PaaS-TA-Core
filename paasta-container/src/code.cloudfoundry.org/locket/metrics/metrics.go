package metrics

import (
	"os"
	"time"

	"code.cloudfoundry.org/clock"
	loggregator_v2 "code.cloudfoundry.org/go-loggregator/compatibility"
	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/locket/db"
	"code.cloudfoundry.org/locket/models"
	"github.com/tedsuo/ifrit"
)

const (
	activeLocks     = "ActiveLocks"
	activePresences = "ActivePresences"
)

type metricsNotifier struct {
	logger          lager.Logger
	ticker          clock.Clock
	metricsInterval time.Duration
	lockDB          db.LockDB
	metronClient    loggregator_v2.IngressClient
}

func NewMetricsNotifier(logger lager.Logger, ticker clock.Clock, metronClient loggregator_v2.IngressClient, metricsInterval time.Duration, lockDB db.LockDB) ifrit.Runner {
	return &metricsNotifier{
		logger:          logger,
		ticker:          ticker,
		metricsInterval: metricsInterval,
		lockDB:          lockDB,
		metronClient:    metronClient,
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

			err = notifier.metronClient.SendMetric(activeLocks, locks)
			if err != nil {
				logger.Error("failed-sending-lock-count", err)
			}

			err = notifier.metronClient.SendMetric(activePresences, presences)
			if err != nil {
				logger.Error("failed-sending-presences-count", err)
			}
		}
	}
	return nil
}
