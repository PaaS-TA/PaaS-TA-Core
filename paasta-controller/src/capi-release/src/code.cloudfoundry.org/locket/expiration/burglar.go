package expiration

import (
	"os"
	"time"

	"code.cloudfoundry.org/clock"
	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/locket/db"
)

type burglar struct {
	logger        lager.Logger
	lockDB        db.LockDB
	lockPick      LockPick
	clock         clock.Clock
	checkInterval time.Duration
}

func NewBurglar(logger lager.Logger, lockDB db.LockDB, lockPick LockPick, clock clock.Clock, checkInterval time.Duration) burglar {
	return burglar{
		logger:        logger,
		lockDB:        lockDB,
		lockPick:      lockPick,
		clock:         clock,
		checkInterval: checkInterval,
	}
}

func (b burglar) Run(signals <-chan os.Signal, ready chan<- struct{}) error {
	logger := b.logger.Session("burglar")

	logger.Info("started")
	defer logger.Info("complete")

	locks, err := b.lockDB.FetchAll(logger, "")
	if err != nil {
		logger.Error("failed-fetching-locks", err)
	}

	for _, lock := range locks {
		b.lockPick.RegisterTTL(logger, lock)
	}

	check := b.clock.NewTicker(b.checkInterval)

	close(ready)

	for {
		select {
		case sig := <-signals:
			logger.Info("signalled", lager.Data{"signal": sig})
			return nil
		case <-check.C():
			locks, err := b.lockDB.FetchAll(logger, "")
			if err != nil {
				logger.Error("failed-fetching-locks", err)
				continue
			}

			for _, lock := range locks {
				b.lockPick.RegisterTTL(logger, lock)
			}
		}
	}

	return nil
}
