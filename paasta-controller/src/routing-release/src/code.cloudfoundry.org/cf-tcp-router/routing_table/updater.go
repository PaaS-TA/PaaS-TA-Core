package routing_table

import (
	"errors"
	"sync"

	"code.cloudfoundry.org/cf-tcp-router/configurer"
	"code.cloudfoundry.org/cf-tcp-router/models"
	"code.cloudfoundry.org/clock"
	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/routing-api"
	apimodels "code.cloudfoundry.org/routing-api/models"
	uaaclient "code.cloudfoundry.org/uaa-go-client"
)

//go:generate counterfeiter -o fakes/fake_updater.go . Updater
type Updater interface {
	HandleEvent(event routing_api.TcpEvent) error
	Sync()
	Syncing() bool
	PruneStaleRoutes()
}

type updater struct {
	logger           lager.Logger
	routingTable     *models.RoutingTable
	configurer       configurer.RouterConfigurer
	syncing          bool
	routingAPIClient routing_api.Client
	uaaClient        uaaclient.Client
	cachedEvents     []routing_api.TcpEvent
	lock             *sync.Mutex
	klock            clock.Clock
	defaultTTL       int
}

func NewUpdater(logger lager.Logger, routingTable *models.RoutingTable, configurer configurer.RouterConfigurer,
	routingAPIClient routing_api.Client, uaaClient uaaclient.Client, klock clock.Clock, defaultTTL int) Updater {
	return &updater{
		logger:           logger,
		routingTable:     routingTable,
		configurer:       configurer,
		lock:             new(sync.Mutex),
		syncing:          false,
		routingAPIClient: routingAPIClient,
		uaaClient:        uaaClient,
		cachedEvents:     nil,
		klock:            klock,
		defaultTTL:       defaultTTL,
	}
}

func (u *updater) PruneStaleRoutes() {
	logger := u.logger.Session("prune-stale-routes")
	logger.Debug("starting")

	defer func() {
		u.lock.Unlock()
		logger.Debug("completed")
	}()

	u.lock.Lock()
	u.routingTable.PruneEntries(u.defaultTTL)
}

func (u *updater) Sync() {
	logger := u.logger.Session("bulk-sync")
	logger.Debug("starting")

	defer func() {
		u.lock.Lock()
		u.applyCachedEvents(logger)
		u.configurer.Configure(*u.routingTable)
		logger.Debug("applied-fetched-routes-to-routing-table", lager.Data{"size": u.routingTable.Size()})
		u.syncing = false
		u.cachedEvents = nil
		u.lock.Unlock()
		logger.Debug("completed")
	}()

	u.lock.Lock()
	u.syncing = true
	u.cachedEvents = []routing_api.TcpEvent{}
	u.lock.Unlock()

	useCachedToken := true
	var err error
	var tcpRouteMappings []apimodels.TcpRouteMapping
	for count := 0; count < 2; count++ {
		token, tokenErr := u.uaaClient.FetchToken(!useCachedToken)
		if tokenErr != nil {
			logger.Error("error-fetching-token", tokenErr)
			return
		}
		u.routingAPIClient.SetToken(token.AccessToken)
		tcpRouteMappings, err = u.routingAPIClient.TcpRouteMappings()
		if err != nil {
			logger.Error("error-fetching-routes", err)
			if err.Error() == "unauthorized" {
				useCachedToken = false
				logger.Info("retrying-sync")
			} else {
				return
			}
		} else {
			break
		}
	}
	logger.Debug("fetched-tcp-routes", lager.Data{"num-routes": len(tcpRouteMappings)})
	if err == nil {
		// Create a new map and populate using tcp route mappings we got from routing api
		u.routingTable.Entries = make(map[models.RoutingKey]models.RoutingTableEntry)
		for _, routeMapping := range tcpRouteMappings {
			routingKey, backendServerInfo := u.toRoutingTableEntry(logger, routeMapping)
			logger.Debug("creating-routing-table-entry", lager.Data{"key": routingKey, "value": backendServerInfo})
			u.routingTable.UpsertBackendServerKey(routingKey, backendServerInfo)
		}
	}
}

func (u *updater) applyCachedEvents(logger lager.Logger) {
	logger.Debug("applying-cached-events", lager.Data{"cache_size": len(u.cachedEvents)})
	defer logger.Debug("applied-cached-events")
	for _, e := range u.cachedEvents {
		u.handleEvent(logger, e)
	}
}

func (u *updater) Syncing() bool {
	u.lock.Lock()
	defer u.lock.Unlock()
	return u.syncing
}

func (u *updater) HandleEvent(event routing_api.TcpEvent) error {
	u.lock.Lock()
	defer u.lock.Unlock()

	if u.syncing {
		u.logger.Debug("caching-events")
		u.cachedEvents = append(u.cachedEvents, event)
	} else {
		return u.handleEvent(u.logger, event)
	}
	return nil
}

func (u *updater) handleEvent(l lager.Logger, event routing_api.TcpEvent) error {
	logger := l.Session("handle-event", lager.Data{"event": event})
	logger.Debug("starting")
	defer logger.Debug("finished")
	action := event.Action
	switch action {
	case "Upsert":
		return u.handleUpsert(logger, event.TcpRouteMapping)
	case "Delete":
		return u.handleDelete(logger, event.TcpRouteMapping)
	default:
		logger.Info("unknown-event-action")
		return errors.New("unknown-event-action:" + action)
	}
}

func (u *updater) toRoutingTableEntry(logger lager.Logger, routeMapping apimodels.TcpRouteMapping) (models.RoutingKey, models.BackendServerInfo) {
	logger.Debug("converting-tcp-route-mapping", lager.Data{"tcp-route": routeMapping})
	routingKey := models.RoutingKey{Port: routeMapping.ExternalPort}

	var ttl int
	if routeMapping.TTL != nil {
		ttl = *routeMapping.TTL
	}

	backendServerInfo := models.BackendServerInfo{
		Address:         routeMapping.HostIP,
		Port:            routeMapping.HostPort,
		ModificationTag: routeMapping.ModificationTag,
		TTL:             ttl,
	}
	return routingKey, backendServerInfo
}

func (u *updater) handleUpsert(logger lager.Logger, routeMapping apimodels.TcpRouteMapping) error {
	routingKey, backendServerInfo := u.toRoutingTableEntry(logger, routeMapping)

	if u.routingTable.UpsertBackendServerKey(routingKey, backendServerInfo) && !u.syncing {
		logger.Debug("calling-configurer")
		return u.configurer.Configure(*u.routingTable)
	}

	return nil
}

func (u *updater) handleDelete(logger lager.Logger, routeMapping apimodels.TcpRouteMapping) error {
	routingKey, backendServerInfo := u.toRoutingTableEntry(logger, routeMapping)

	if u.routingTable.DeleteBackendServerKey(routingKey, backendServerInfo) && !u.syncing {
		logger.Debug("calling-configurer")
		return u.configurer.Configure(*u.routingTable)
	}

	return nil
}
