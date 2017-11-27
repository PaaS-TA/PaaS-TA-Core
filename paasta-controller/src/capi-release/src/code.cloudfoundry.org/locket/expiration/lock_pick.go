package expiration

import (
	"sync"
	"time"

	"code.cloudfoundry.org/clock"
	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/locket/db"
	"code.cloudfoundry.org/runtimeschema/metric"
)

const (
	locksExpired = metric.Counter("LocksExpired")
)

//go:generate counterfeiter . LockPick
type LockPick interface {
	RegisterTTL(logger lager.Logger, lock *db.Lock)
}

type lockPick struct {
	lockDB    db.LockDB
	clock     clock.Clock
	lockTTLs  map[checkKey]chanAndIndex
	lockMutex *sync.Mutex
}

type chanAndIndex struct {
	channel chan struct{}
	index   int64
}

type checkKey struct {
	key string
	id  string
}

func NewLockPick(lockDB db.LockDB, clock clock.Clock) lockPick {
	return lockPick{
		lockDB:    lockDB,
		clock:     clock,
		lockTTLs:  make(map[checkKey]chanAndIndex),
		lockMutex: &sync.Mutex{},
	}
}

func (l lockPick) RegisterTTL(logger lager.Logger, lock *db.Lock) {
	logger = logger.Session("register-ttl", lager.Data{"key": lock.Key, "modified-index": lock.ModifiedIndex})
	logger.Debug("starting")
	logger.Debug("completed")

	newChanIndex := chanAndIndex{
		channel: make(chan struct{}),
		index:   lock.ModifiedIndex,
	}
	l.lockMutex.Lock()
	defer l.lockMutex.Unlock()

	channelIndex, ok := l.lockTTLs[checkKeyFromLock(lock)]
	if ok && channelIndex.index >= newChanIndex.index {
		logger.Debug("found-expiration-goroutine-for-index", lager.Data{"index": channelIndex.index})
		return
	}

	if ok && channelIndex.index < newChanIndex.index {
		close(channelIndex.channel)
	}

	l.lockTTLs[checkKeyFromLock(lock)] = newChanIndex
	go l.checkExpiration(logger, lock, newChanIndex.channel)
}

func (l lockPick) checkExpiration(logger lager.Logger, lock *db.Lock, closeChan chan struct{}) {
	lockTimer := l.clock.NewTimer(time.Duration(lock.TtlInSeconds) * time.Second)

	for {
		select {
		case <-closeChan:
			logger.Debug("cancelling-old-check-goroutine")
			return
		case <-lockTimer.C():
			defer func() {
				l.lockMutex.Lock()
				chanIndex := l.lockTTLs[checkKeyFromLock(lock)]
				if chanIndex.index == lock.ModifiedIndex {
					delete(l.lockTTLs, checkKeyFromLock(lock))
				}
				l.lockMutex.Unlock()
			}()

			fetchedLock, err := l.lockDB.Fetch(logger, lock.Key)
			if err != nil {
				return
			}

			if fetchedLock.ModifiedIndex == lock.ModifiedIndex && fetchedLock.ModifiedId == lock.ModifiedId {
				logger.Info("lock-expired")
				locksExpired.Increment()

				err = l.lockDB.Release(logger, lock.Resource)
				if err != nil {
					logger.Error("failed-to-release-lock", err)
					return
				}
			}
			return
		}
	}
}

func checkKeyFromLock(lock *db.Lock) checkKey {
	return checkKey{
		key: lock.Key,
		id:  lock.ModifiedId,
	}
}
