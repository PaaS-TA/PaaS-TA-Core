package utils

import "time"

type Timeout interface {
	Done() <-chan struct{}
}

type timeout struct {
	done chan struct{}
}

func (t timeout) Done() <-chan struct{} { return t.done }

func NewTimeout(timer <-chan time.Time) Timeout {
	done := make(chan struct{})

	go func(t <-chan time.Time, d chan<- struct{}) {
		<-t
		close(d)
	}(timer, done)

	return timeout{
		done: done,
	}
}
