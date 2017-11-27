package steps

import "sync"

type canceller struct {
	cancelled chan struct{}
	once      sync.Once
}

func newCanceller() *canceller {
	return &canceller{
		cancelled: make(chan struct{}),
	}
}

func (c *canceller) Cancelled() <-chan struct{} {
	return c.cancelled
}

func (c *canceller) Cancel() {
	c.once.Do(func() {
		close(c.cancelled)
	})
}
