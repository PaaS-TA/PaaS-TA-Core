package watcher

import (
	"os"
	"sync/atomic"

	"code.cloudfoundry.org/bbs"
	"code.cloudfoundry.org/bbs/events"
	"code.cloudfoundry.org/bbs/models"
	"code.cloudfoundry.org/clock"
	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/tcp-emitter/routing_table"
	"code.cloudfoundry.org/tcp-emitter/routing_table/schema"
)

type Watcher struct {
	bbsClient           bbs.Client
	clock               clock.Clock
	routingTableHandler routing_table.RoutingTableHandler
	syncChannel         chan struct{}
	logger              lager.Logger
}

type syncEndEvent struct {
	table  schema.RoutingTable
	logger lager.Logger
}

func NewWatcher(
	bbsClient bbs.Client,
	clock clock.Clock,
	routingTableHandler routing_table.RoutingTableHandler,
	syncChannel chan struct{},
	logger lager.Logger,
) *Watcher {
	return &Watcher{
		bbsClient:           bbsClient,
		clock:               clock,
		routingTableHandler: routingTableHandler,
		syncChannel:         syncChannel,
		logger:              logger.Session("watcher"),
	}
}

func (watcher *Watcher) Run(signals <-chan os.Signal, ready chan<- struct{}) error {
	watcher.logger.Debug("starting")
	defer watcher.logger.Debug("finished")

	eventChan := make(chan models.Event)

	var eventSource atomic.Value
	var stopEventSource int32

	go func() {
		var err error
		var es events.EventSource

		for {
			if atomic.LoadInt32(&stopEventSource) == 1 {
				return
			}

			watcher.logger.Info("subscribing-to-bbs-events")
			es, err = watcher.bbsClient.SubscribeToEvents(watcher.logger)
			if err != nil {
				watcher.logger.Error("failed-subscribing-to-bbs-events", err)
				continue
			}
			watcher.logger.Info("subscribed-to-bbs-events")

			eventSource.Store(es)

			var event models.Event
			for {
				event, err = es.Next()
				if err != nil {
					watcher.logger.Error("failed-getting-next-event", err)
					break
				}

				if event != nil {
					eventChan <- event
				}
			}
		}
	}()
	watcher.logger.Debug("listening-on-channels")
	close(ready)
	watcher.logger.Debug("started")

	for {
		select {
		case event := <-eventChan:
			watcher.routingTableHandler.HandleEvent(event)

		case <-watcher.syncChannel:
			watcher.routingTableHandler.Sync()

		case <-signals:
			watcher.logger.Info("stopping")
			atomic.StoreInt32(&stopEventSource, 1)
			if es := eventSource.Load(); es != nil {
				err := es.(events.EventSource).Close()
				if err != nil {
					watcher.logger.Error("failed-closing-event-source", err)
				}
			}
			return nil
		}
	}
}
