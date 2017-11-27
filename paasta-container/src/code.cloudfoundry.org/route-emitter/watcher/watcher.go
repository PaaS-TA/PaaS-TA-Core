package watcher

import (
	"fmt"
	"os"
	"sync"
	"sync/atomic"
	"time"

	"code.cloudfoundry.org/bbs"
	"code.cloudfoundry.org/bbs/events"
	"code.cloudfoundry.org/bbs/models"
	"code.cloudfoundry.org/clock"
	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/route-emitter/routingtable"
	"code.cloudfoundry.org/route-emitter/syncer"
	"code.cloudfoundry.org/runtimeschema/metric"
)

var (
	routeSyncDuration = metric.Duration("RouteEmitterSyncDuration")
)

//go:generate counterfeiter -o fakes/fake_routehandler.go . RouteHandler
type RouteHandler interface {
	HandleEvent(logger lager.Logger, event models.Event)
	Sync(
		logger lager.Logger,
		desired []*models.DesiredLRPSchedulingInfo,
		runningActual []*routingtable.ActualLRPRoutingInfo,
		domains models.DomainSet,
		cachedEvents map[string]models.Event,
	)
	Emit(logger lager.Logger)
	ShouldRefreshDesired(*routingtable.ActualLRPRoutingInfo) bool
	RefreshDesired(lager.Logger, []*models.DesiredLRPSchedulingInfo)
}

type Watcher struct {
	cellID       string
	bbsClient    bbs.Client
	clock        clock.Clock
	routeHandler RouteHandler
	syncEvents   syncer.Events
	logger       lager.Logger
}

func NewWatcher(
	cellID string,
	bbsClient bbs.Client,
	clock clock.Clock,
	routeHandler RouteHandler,
	syncEvents syncer.Events,
	logger lager.Logger,
) *Watcher {
	return &Watcher{
		cellID:       cellID,
		bbsClient:    bbsClient,
		clock:        clock,
		routeHandler: routeHandler,
		syncEvents:   syncEvents,
		logger:       logger.Session("watcher"),
	}
}

type syncEventResult struct {
	startTime     time.Time
	desired       []*models.DesiredLRPSchedulingInfo
	runningActual []*routingtable.ActualLRPRoutingInfo
	domains       models.DomainSet
	err           error
}

func (watcher *Watcher) Run(signals <-chan os.Signal, ready chan<- struct{}) error {
	watcher.logger.Debug("starting", lager.Data{"cell-id": watcher.cellID})
	defer watcher.logger.Debug("finished")

	eventChan := make(chan models.Event)
	resubscribeChannel := make(chan error)

	eventSource := &atomic.Value{}
	var stopEventSource int32

	go watcher.checkForEvents(resubscribeChannel, eventChan, eventSource, watcher.logger)
	watcher.logger.Debug("listening-on-channels")
	close(ready)
	watcher.logger.Debug("started")

	cachedEvents := make(map[string]models.Event)
	syncEnd := make(chan *syncEventResult)
	syncing := false

	for {
		select {
		case event := <-eventChan:
			if syncing {
				if watcher.eventCellIDMatches(watcher.logger, event) {
					watcher.logger.Info("caching-event", lager.Data{
						"type": event.EventType(),
					})
					cachedEvents[event.Key()] = event
				} else {
					logSkippedEvent(watcher.logger, event)
				}
				continue
			}
			logger := watcher.logger.Session("handling-event")
			watcher.handleEvent(logger, event)
		case <-watcher.syncEvents.Emit:
			logger := watcher.logger.Session("emit")
			watcher.routeHandler.Emit(logger)
		case syncEvent := <-syncEnd:
			syncing = false
			logger := watcher.logger.Session("sync")
			var cachedDesired []*models.DesiredLRPSchedulingInfo
			for _, e := range cachedEvents {
				desired := watcher.retrieveDesired(logger, e)
				if len(desired) > 0 {
					cachedDesired = append(cachedDesired, desired...)
				}
			}

			if syncEvent.err != nil {
				logger.Error("failed-to-sync-events", syncEvent.err)
				continue
			}

			if len(cachedDesired) > 0 {
				syncEvent.desired = append(syncEvent.desired, cachedDesired...)
			}

			logger.Debug("calling-handler-sync")
			watcher.routeHandler.Sync(logger,
				syncEvent.desired,
				syncEvent.runningActual,
				syncEvent.domains,
				cachedEvents,
			)

			after := watcher.clock.Now()
			if err := routeSyncDuration.Send(after.Sub(syncEvent.startTime)); err != nil {
				watcher.logger.Error("failed-to-send-route-sync-duration-metric", err)
			}

			cachedEvents = make(map[string]models.Event)
			logger.Info("complete")
		case <-watcher.syncEvents.Sync:
			if syncing {
				watcher.logger.Debug("sync-already-in-progress")
				continue
			}
			logger := watcher.logger.Session("sync")
			logger.Info("starting")
			go watcher.sync(logger, syncEnd)
			syncing = true
		case err := <-resubscribeChannel:
			watcher.logger.Error("event-source-error", err)
			if es := eventSource.Load(); es != nil {
				err := es.(events.EventSource).Close()
				if err != nil {
					watcher.logger.Error("failed-closing-event-source", err)
				}
			}
			go watcher.checkForEvents(resubscribeChannel, eventChan, eventSource, watcher.logger)

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

func (w *Watcher) cacheIncomingEvents(
	eventChan chan models.Event,
	cachedEventsChan chan map[string]models.Event,
	done chan struct{},
) {
	cachedEvents := make(map[string]models.Event)
	for {
		select {
		case event := <-eventChan:
			w.logger.Info("caching-event", lager.Data{
				"type": event.EventType(),
			})
			cachedEvents[event.Key()] = event
		case <-done:
			cachedEventsChan <- cachedEvents
			return
		}
	}
}

func (w *Watcher) retrieveDesired(logger lager.Logger, event models.Event) []*models.DesiredLRPSchedulingInfo {
	var routingInfo *routingtable.ActualLRPRoutingInfo
	switch event := event.(type) {
	case *models.ActualLRPCreatedEvent:
		routingInfo = routingtable.NewActualLRPRoutingInfo(event.ActualLrpGroup)
	case *models.ActualLRPChangedEvent:
		routingInfo = routingtable.NewActualLRPRoutingInfo(event.After)
	default:
	}
	var desiredLRPs []*models.DesiredLRPSchedulingInfo
	var err error
	if routingInfo != nil && routingInfo.ActualLRP.State == models.ActualLRPStateRunning {
		if w.routeHandler.ShouldRefreshDesired(routingInfo) {
			logger.Info("refreshing-desired-lrp-info", lager.Data{"process-guid": routingInfo.ActualLRP.ProcessGuid})
			desiredLRPs, err = w.bbsClient.DesiredLRPSchedulingInfos(logger, models.DesiredLRPFilter{
				ProcessGuids: []string{routingInfo.ActualLRP.ProcessGuid},
			})
			if err != nil {
				logger.Error("failed-getting-desired-lrps-for-missing-actual-lrp", err)
			}
		}
	}

	return desiredLRPs
}

func (w *Watcher) handleEvent(logger lager.Logger, event models.Event) {
	if !w.eventCellIDMatches(logger, event) {
		logSkippedEvent(logger, event)
		return
	}

	desiredLRPs := w.retrieveDesired(logger, event)
	if len(desiredLRPs) > 0 {
		w.routeHandler.RefreshDesired(logger, desiredLRPs)
	}
	w.routeHandler.HandleEvent(logger, event)
}

func logSkippedEvent(logger lager.Logger, event models.Event) {
	data := lager.Data{"event-type": event.EventType()}
	switch e := event.(type) {
	case *models.ActualLRPCreatedEvent:
		data["lrp"] = routingtable.ActualLRPData(routingtable.NewActualLRPRoutingInfo(e.ActualLrpGroup))
	case *models.ActualLRPRemovedEvent:
		data["lrp"] = routingtable.ActualLRPData(routingtable.NewActualLRPRoutingInfo(e.ActualLrpGroup))
	case *models.ActualLRPChangedEvent:
		data["before"] = routingtable.ActualLRPData(routingtable.NewActualLRPRoutingInfo(e.Before))
		data["after"] = routingtable.ActualLRPData(routingtable.NewActualLRPRoutingInfo(e.After))
	}
	logger.Debug("skipping-event", data)
}

// returns true if the event is relevant to the local cell, e.g. an actual lrp
// started or stopped on the local cell
func (watcher *Watcher) eventCellIDMatches(logger lager.Logger, event models.Event) bool {
	if watcher.cellID == "" {
		return true
	}

	switch event := event.(type) {
	case *models.DesiredLRPCreatedEvent:
		return true
	case *models.DesiredLRPChangedEvent:
		return true
	case *models.DesiredLRPRemovedEvent:
		return true
	case *models.ActualLRPCreatedEvent:
		lrp, _ := event.ActualLrpGroup.Resolve()
		return lrp.ActualLRPInstanceKey.CellId == watcher.cellID
	case *models.ActualLRPChangedEvent:
		beforeLRP, _ := event.Before.Resolve()
		afterLRP, _ := event.After.Resolve()
		if beforeLRP.State == models.ActualLRPStateRunning {
			return beforeLRP.ActualLRPInstanceKey.CellId == watcher.cellID
		} else if afterLRP.State == models.ActualLRPStateRunning {
			return afterLRP.ActualLRPInstanceKey.CellId == watcher.cellID
		}
		// this shouldn't matter if we pass it through or not, since the event is
		// a no-op from the route-emitter point of view
		return false
	case *models.ActualLRPRemovedEvent:
		lrp, _ := event.ActualLrpGroup.Resolve()
		return lrp.ActualLRPInstanceKey.CellId == watcher.cellID
	default:
		return false
	}
}

func (w *Watcher) sync(logger lager.Logger, ch chan<- *syncEventResult) {
	var desiredSchedulingInfo []*models.DesiredLRPSchedulingInfo
	var runningActualLRPs []*routingtable.ActualLRPRoutingInfo
	var domains models.DomainSet

	var actualErr, desiredErr, domainsErr error
	before := w.clock.Now()

	wg := sync.WaitGroup{}

	wg.Add(1)
	go func() {
		defer wg.Done()
		logger.Debug("getting-actual-lrps")
		var actualLRPGroups []*models.ActualLRPGroup
		actualLRPGroups, actualErr = w.bbsClient.ActualLRPGroups(logger, models.ActualLRPFilter{CellID: w.cellID})
		if actualErr != nil {
			logger.Error("failed-getting-actual-lrps", actualErr)
			return
		}
		logger.Debug("succeeded-getting-actual-lrps", lager.Data{"num-actual-responses": len(actualLRPGroups)})

		runningActualLRPs = make([]*routingtable.ActualLRPRoutingInfo, 0, len(actualLRPGroups))
		for _, actualLRPGroup := range actualLRPGroups {
			actualLRP, evacuating := actualLRPGroup.Resolve()
			if actualLRP.State == models.ActualLRPStateRunning {
				runningActualLRPs = append(runningActualLRPs, &routingtable.ActualLRPRoutingInfo{
					ActualLRP:  actualLRP,
					Evacuating: evacuating,
				})
			}
		}

		if w.cellID != "" {
			guids := make([]string, 0, len(runningActualLRPs))
			// filter the desired lrp scheduling info by process guids
			for _, lrpInfo := range runningActualLRPs {
				guids = append(guids, lrpInfo.ActualLRP.ProcessGuid)
			}
			if len(guids) > 0 {
				desiredSchedulingInfo, desiredErr = getSchedulingInfos(logger, w.bbsClient, guids)
			}
		}
	}()

	if w.cellID == "" {
		wg.Add(1)
		go func() {
			defer wg.Done()
			desiredSchedulingInfo, desiredErr = getSchedulingInfos(logger, w.bbsClient, nil)
		}()
	}

	wg.Add(1)
	go func() {
		defer wg.Done()
		var domainArray []string
		logger.Debug("getting-domains")
		domainArray, domainsErr = w.bbsClient.Domains(logger)
		if domainsErr != nil {
			logger.Error("failed-getting-domains", domainsErr)
			return
		}

		domains = models.NewDomainSet(domainArray)
		logger.Debug("succeeded-getting-domains", lager.Data{"num-domains": len(domains)})
	}()

	wg.Wait()

	var err error
	if actualErr != nil || desiredErr != nil || domainsErr != nil {
		err = fmt.Errorf("failed to sync: %s, %s, %s", actualErr, desiredErr, domainsErr)
	}

	ch <- &syncEventResult{
		startTime:     before,
		desired:       desiredSchedulingInfo,
		runningActual: runningActualLRPs,
		domains:       domains,
		err:           err,
	}
}

func (w *Watcher) checkForEvents(resubscribeChannel chan error, eventChan chan models.Event, eventSource *atomic.Value, logger lager.Logger) {
	var err error
	var es events.EventSource

	logger.Info("subscribing-to-bbs-events")
	es, err = w.bbsClient.SubscribeToEventsByCellID(logger, w.cellID)
	if err != nil {
		resubscribeChannel <- err
		return
	}
	logger.Info("subscribed-to-bbs-events")

	eventSource.Store(es)

	var event models.Event
	for {
		event, err = es.Next()
		if err != nil {
			switch err {
			case events.ErrUnrecognizedEventType:
				logger.Error("failed-getting-next-event", err)
			default:
				resubscribeChannel <- err
				return
			}
		}

		if event != nil {
			eventChan <- event
		}
	}
}

func getSchedulingInfos(logger lager.Logger, bbsClient bbs.Client, guids []string) ([]*models.DesiredLRPSchedulingInfo, error) {
	logger.Debug("getting-scheduling-infos", lager.Data{"guids-length": len(guids)})
	schedulingInfos, err := bbsClient.DesiredLRPSchedulingInfos(logger, models.DesiredLRPFilter{
		ProcessGuids: guids,
	})
	if err != nil {
		logger.Error("failed-getting-scheduling-infos", err)
		return nil, err
	}

	logger.Debug("succeeded-getting-scheduling-infos", lager.Data{"num-desired-responses": len(schedulingInfos)})
	return schedulingInfos, nil
}
