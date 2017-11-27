package lock

import (
	"context"
	"os"
	"time"

	"code.cloudfoundry.org/clock"
	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/locket/models"
)

type lockRunner struct {
	logger lager.Logger

	locker         models.LocketClient
	lock           *models.Resource
	ttlInSeconds   int64
	clock          clock.Clock
	retryInterval  time.Duration
	exitOnLostLock bool
}

func NewLockRunner(
	logger lager.Logger,
	locker models.LocketClient,
	lock *models.Resource,
	ttlInSeconds int64,
	clock clock.Clock,
	retryInterval time.Duration,
) *lockRunner {
	return &lockRunner{
		logger:         logger,
		locker:         locker,
		lock:           lock,
		ttlInSeconds:   ttlInSeconds,
		clock:          clock,
		retryInterval:  retryInterval,
		exitOnLostLock: true,
	}
}

func NewPresenceRunner(
	logger lager.Logger,
	locker models.LocketClient,
	lock *models.Resource,
	ttlInSeconds int64,
	clock clock.Clock,
	retryInterval time.Duration,
) *lockRunner {
	return &lockRunner{
		logger:         logger,
		locker:         locker,
		lock:           lock,
		ttlInSeconds:   ttlInSeconds,
		clock:          clock,
		retryInterval:  retryInterval,
		exitOnLostLock: false,
	}
}

func (l *lockRunner) Run(signals <-chan os.Signal, ready chan<- struct{}) error {
	logger := l.logger.Session("sql-lock", lager.Data{"lock": l.lock, "ttl_in_seconds": l.ttlInSeconds})

	logger.Info("started")
	defer logger.Info("completed")

	var acquired, isReady bool
	_, err := l.locker.Lock(context.Background(), &models.LockRequest{Resource: l.lock, TtlInSeconds: l.ttlInSeconds})
	if err != nil {
		logger.Error("failed-to-acquire-lock", err)
	} else {
		logger.Info("acquired-lock")
		close(ready)
		acquired = true
		isReady = true
	}

	retry := l.clock.NewTimer(l.retryInterval)

	for {
		select {
		case sig := <-signals:
			logger.Info("signalled", lager.Data{"signal": sig})

			_, err := l.locker.Release(context.Background(), &models.ReleaseRequest{Resource: l.lock})
			if err != nil {
				logger.Error("failed-to-release-lock", err)
			} else {
				logger.Info("released-lock")
			}

			return nil

		case <-retry.C():
			_, err := l.locker.Lock(context.Background(), &models.LockRequest{Resource: l.lock, TtlInSeconds: l.ttlInSeconds})
			if err != nil {
				if acquired {
					logger.Error("lost-lock", err)
					if l.exitOnLostLock {
						return err
					}

					acquired = false
				}
			} else if !acquired {
				logger.Info("acquired-lock")
				if !isReady {
					close(ready)
					isReady = true
				}
				acquired = true
			}

			retry.Reset(l.retryInterval)
		}
	}

	return nil
}
