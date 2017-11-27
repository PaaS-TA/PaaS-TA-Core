package emitter

import (
	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/route-emitter/routingtable"
)

type CachingNATSEmitter struct {
	NATSEmitter
	cache routingtable.MessagesToEmit
}

func NewCachingNATSEmitter(natsEmitter NATSEmitter) *CachingNATSEmitter {
	return &CachingNATSEmitter{
		NATSEmitter: natsEmitter,
		cache:       routingtable.MessagesToEmit{},
	}
}

func (c *CachingNATSEmitter) Emit(logger lager.Logger, msgs routingtable.MessagesToEmit) error {
	logger.Debug("caching-nats-events", lager.Data{"messages": msgs})
	c.cache = c.cache.Merge(msgs)
	return nil
}

func (c *CachingNATSEmitter) Cache() routingtable.MessagesToEmit {
	return c.cache
}

func (c *CachingNATSEmitter) EmitCached() error {
	err := c.NATSEmitter.Emit(c.cache)
	c.cache = routingtable.MessagesToEmit{}
	return err
}
