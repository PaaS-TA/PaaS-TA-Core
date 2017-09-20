package utils

import (
	"fmt"
	"time"
)

type Sleeper interface {
	Sleep(time.Duration)
}

type Retrier struct {
	Sleeper    Sleeper
	RetryDelay time.Duration
}

func NewRetrier(sleeper Sleeper, retryDelay time.Duration) Retrier {
	return Retrier{
		Sleeper:    sleeper,
		RetryDelay: retryDelay,
	}
}

func (r Retrier) TryUntil(timeout Timeout, f func() error) error {
	var lastError error
	for {
		select {
		case <-timeout.Done():
			return fmt.Errorf("timeout exceeded: %q", lastError)
		default:
			err := f()
			if err != nil {
				lastError = err
				r.Sleeper.Sleep(r.RetryDelay)
				continue
			}
			return nil
		}
	}
}
