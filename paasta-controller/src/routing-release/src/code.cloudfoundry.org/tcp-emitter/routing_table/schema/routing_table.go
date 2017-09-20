package schema

import (
	"encoding/json"
	"sync"

	"code.cloudfoundry.org/bbs/models"
	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/routing-info/tcp_routes"
	"code.cloudfoundry.org/tcp-emitter/routing_table/schema/endpoint"
	"code.cloudfoundry.org/tcp-emitter/routing_table/schema/event"
	"code.cloudfoundry.org/tcp-emitter/routing_table/util"
)

type routeInfo struct {
	ProcessGuid string
	Routes      map[string]*json.RawMessage
}

//go:generate counterfeiter -o fakes/fake_routing_table.go . RoutingTable
type RoutingTable interface {
	RouteCount() int

	AddRoutes(desiredLRP *models.DesiredLRP) event.RoutingEvents
	UpdateRoutes(beforeLRP, afterLRP *models.DesiredLRP) event.RoutingEvents
	RemoveRoutes(desiredLRP *models.DesiredLRP) event.RoutingEvents

	AddEndpoint(actualLRP *models.ActualLRPGroup) event.RoutingEvents
	RemoveEndpoint(actualLRP *models.ActualLRPGroup) event.RoutingEvents

	Swap(t RoutingTable) event.RoutingEvents

	GetRoutingEvents() event.RoutingEvents
}

type routingTable struct {
	entries map[endpoint.RoutingKey]endpoint.RoutableEndpoints
	sync.Locker
	logger lager.Logger
}

func NewTable(logger lager.Logger, entries map[endpoint.RoutingKey]endpoint.RoutableEndpoints) RoutingTable {
	if entries == nil {
		entries = make(map[endpoint.RoutingKey]endpoint.RoutableEndpoints)
	}
	return &routingTable{
		entries: entries,
		Locker:  &sync.Mutex{},
		logger:  logger,
	}
}

func (table *routingTable) GetRoutingEvents() event.RoutingEvents {
	routingEvents := event.RoutingEvents{}

	table.Lock()
	defer table.Unlock()
	table.logger.Debug("get-routing-events", lager.Data{"count": len(table.entries)})

	for key, entry := range table.entries {
		//always register everything on sync
		routingEvents = append(routingEvents, table.createRoutingEvent(table.logger, key, entry, event.RouteRegistrationEvent)...)
	}

	return routingEvents
}

func (table *routingTable) Swap(t RoutingTable) event.RoutingEvents {

	routingEvents := event.RoutingEvents{}

	newTable, ok := t.(*routingTable)
	if !ok {
		return routingEvents
	}

	table.Lock()
	defer table.Unlock()

	newEntries := newTable.entries
	for key, newEntry := range newEntries {
		//always register everything on sync
		routingEvents = append(routingEvents, table.createRoutingEvent(table.logger, key, newEntry, event.RouteRegistrationEvent)...)

		newExternalEndpoints := newEntry.ExternalEndpoints
		existingEntry := table.entries[key]

		unregistrationEntry := existingEntry.RemoveExternalEndpoints(newExternalEndpoints)
		routingEvents = append(routingEvents, table.createRoutingEvent(table.logger, key, unregistrationEntry, event.RouteUnregistrationEvent)...)
	}

	for key, existingEntry := range table.entries {
		if _, ok := newEntries[key]; !ok {
			routingEvents = append(routingEvents, table.createRoutingEvent(table.logger, key, existingEntry, event.RouteUnregistrationEvent)...)
		}
	}

	table.entries = newEntries

	return routingEvents
}

func (table *routingTable) RouteCount() int {
	table.Lock()
	defer table.Unlock()
	return len(table.entries)
}

func (table *routingTable) AddRoutes(desiredLRP *models.DesiredLRP) event.RoutingEvents {
	logger := table.logger.Session("AddRoutes", lager.Data{"desired_lrp": util.DesiredLRPData(desiredLRP)})
	logger.Debug("starting")
	defer logger.Debug("completed")

	table.Lock()
	defer table.Unlock()

	return table.addRoutes(logger, desiredLRP)
}

func (table *routingTable) addRoutes(logger lager.Logger, desiredLRP *models.DesiredLRP) event.RoutingEvents {
	routingKeys := endpoint.NewRoutingKeysFromDesired(desiredLRP)
	routes, _ := tcp_routes.TCPRoutesFromRoutingInfo(desiredLRP.Routes)

	routingEvents := event.RoutingEvents{}
	for _, key := range routingKeys {
		existingEntry := table.entries[key]
		existingModificationTag := existingEntry.ModificationTag
		if !existingModificationTag.SucceededBy(desiredLRP.ModificationTag) {
			continue
		}
		routingEvents = append(routingEvents, table.mergeRoutes(logger, existingEntry,
			routes, key, desiredLRP.LogGuid, desiredLRP.ModificationTag)...)
	}
	return routingEvents
}

func (table *routingTable) UpdateRoutes(beforeLRP, afterLRP *models.DesiredLRP) event.RoutingEvents {
	logger := table.logger.Session("UpdateRoutes", lager.Data{"before_lrp": util.DesiredLRPData(beforeLRP), "after_lrp": util.DesiredLRPData(afterLRP)})
	logger.Debug("starting")
	defer logger.Debug("completed")

	beforeRoutingKeys := endpoint.NewRoutingKeysFromDesired(beforeLRP)
	afterRoutingKeys := endpoint.NewRoutingKeysFromDesired(afterLRP)

	deletedRoutingKeys := beforeRoutingKeys.Remove(afterRoutingKeys)
	logger.Debug("keys-to-be-deleted", lager.Data{"count": len(deletedRoutingKeys)})

	table.Lock()
	defer table.Unlock()

	routingEvents := table.addRoutes(logger, afterLRP)
	routingEvents = append(routingEvents,
		table.removeRoutingKeys(logger, deletedRoutingKeys, afterLRP.ModificationTag)...)
	return routingEvents
}

func (table *routingTable) RemoveRoutes(desiredLRP *models.DesiredLRP) event.RoutingEvents {
	logger := table.logger.Session("RemoveRoutes", lager.Data{"desired_lrp": util.DesiredLRPData(desiredLRP)})
	logger.Debug("starting")
	defer logger.Debug("completed")

	routingKeys := endpoint.NewRoutingKeysFromDesired(desiredLRP)

	table.Lock()
	defer table.Unlock()

	routingEvents := table.removeRoutingKeys(logger, routingKeys, desiredLRP.ModificationTag)
	return routingEvents
}

func (table *routingTable) removeRoutingKeys(
	logger lager.Logger,
	routingKeys endpoint.RoutingKeys,
	modificationTag *models.ModificationTag,
) event.RoutingEvents {
	routingEvents := event.RoutingEvents{}
	for _, key := range routingKeys {
		if existingEntry, ok := table.entries[key]; ok {
			existingModificationTag := existingEntry.ModificationTag
			if !existingModificationTag.SucceededBy(modificationTag) {
				continue
			}
			if len(existingEntry.Endpoints) > 0 && len(existingEntry.ExternalEndpoints) > 0 {
				routingEvents = append(routingEvents, event.RoutingEvent{
					EventType: event.RouteUnregistrationEvent,
					Key:       key,
					Entry:     existingEntry,
				})
			}

			delete(table.entries, key)
			logger.Debug("route-deleted", lager.Data{"routing-key": key})
		}
	}
	return routingEvents
}

func (table *routingTable) mergeRoutes(
	logger lager.Logger,
	existingEntry endpoint.RoutableEndpoints,
	routes tcp_routes.TCPRoutes,
	key endpoint.RoutingKey,
	logGUID string,
	modificationTag *models.ModificationTag) event.RoutingEvents {
	var registrationNeeded bool

	var newExternalEndpoints endpoint.ExternalEndpointInfos

	for _, route := range routes {
		if key.ContainerPort == route.ContainerPort {
			if !existingEntry.ExternalEndpoints.ContainsExternalPort(route.ExternalPort) {
				newExternalEndpoints = append(newExternalEndpoints,
					endpoint.NewExternalEndpointInfo(route.RouterGroupGuid, route.ExternalPort))
				registrationNeeded = true
			} else {
				newExternalEndpoints = append(newExternalEndpoints,
					endpoint.NewExternalEndpointInfo(route.RouterGroupGuid, route.ExternalPort))
			}
		}
	}

	routingEvents := event.RoutingEvents{}

	if registrationNeeded {
		updatedEntry := existingEntry.Copy()
		updatedEntry.ExternalEndpoints = newExternalEndpoints
		updatedEntry.LogGUID = logGUID
		updatedEntry.ModificationTag = modificationTag
		table.entries[key] = updatedEntry
		routingEvents = append(routingEvents, table.createRoutingEvent(logger, key, updatedEntry, event.RouteRegistrationEvent)...)
		logger.Debug("routing-table-entry-updated", lager.Data{"key": key})
	}

	unregistrationEntry := existingEntry.RemoveExternalEndpoints(newExternalEndpoints)
	routingEvents = append(routingEvents, table.createRoutingEvent(logger, key, unregistrationEntry, event.RouteUnregistrationEvent)...)

	return routingEvents
}

func (table *routingTable) AddEndpoint(actualLRP *models.ActualLRPGroup) event.RoutingEvents {
	logger := table.logger.Session("AddEndpoint", lager.Data{"actual_lrp": actualLRP})
	logger.Debug("starting")
	defer logger.Debug("completed")

	endpoints := endpoint.NewEndpointsFromActual(actualLRP)

	routingEvents := event.RoutingEvents{}

	for _, key := range endpoint.NewRoutingKeysFromActual(actualLRP) {
		for _, endpoint := range endpoints {
			if key.ContainerPort == endpoint.ContainerPort {
				routingEvents = append(routingEvents, table.addEndpoint(logger, key, endpoint)...)
			}
		}
	}
	return routingEvents
}

func (table *routingTable) addEndpoint(logger lager.Logger, key endpoint.RoutingKey, endpoint endpoint.Endpoint) event.RoutingEvents {
	table.Lock()
	defer table.Unlock()

	currentEntry := table.entries[key]

	if existingEndpoint, ok := currentEntry.Endpoints[endpoint.Key()]; ok {
		if !existingEndpoint.ModificationTag.SucceededBy(endpoint.ModificationTag) {
			return event.RoutingEvents{}
		}
	}

	newEntry := currentEntry.Copy()
	newEntry.Endpoints[endpoint.Key()] = endpoint
	table.entries[key] = newEntry

	return table.getRegistrationEvents(logger, key, currentEntry, newEntry)
}

func (table *routingTable) RemoveEndpoint(actualLRP *models.ActualLRPGroup) event.RoutingEvents {
	logger := table.logger.Session("RemoveEndpoint", lager.Data{"actual_lrp": actualLRP})
	logger.Debug("starting")
	defer logger.Debug("completed")

	endpoints := endpoint.NewEndpointsFromActual(actualLRP)

	routingEvents := event.RoutingEvents{}

	for _, key := range endpoint.NewRoutingKeysFromActual(actualLRP) {
		for _, endpoint := range endpoints {
			if key.ContainerPort == endpoint.ContainerPort {
				routingEvents = append(routingEvents, table.removeEndpoint(logger, key, endpoint)...)
			}
		}
	}
	return routingEvents
}

func (table *routingTable) removeEndpoint(logger lager.Logger, key endpoint.RoutingKey, endpoint endpoint.Endpoint) event.RoutingEvents {
	table.Lock()
	defer table.Unlock()

	currentEntry := table.entries[key]
	endpointKey := endpoint.Key()
	currentEndpoint, ok := currentEntry.Endpoints[endpointKey]

	if !ok || !(currentEndpoint.ModificationTag.Equal(endpoint.ModificationTag) ||
		currentEndpoint.ModificationTag.SucceededBy(endpoint.ModificationTag)) {
		return event.RoutingEvents{}
	}

	newEntry := currentEntry.Copy()
	delete(newEntry.Endpoints, endpointKey)
	table.entries[key] = newEntry

	if !currentEntry.HaveExternalEndpointsChanged(newEntry) && !currentEntry.HaveEndpointsChanged(newEntry) {
		logger.Debug("no-change-to-endpoints")
		return event.RoutingEvents{}
	}

	deletedEntry := table.getDeletedEntry(currentEntry, newEntry)

	return table.createRoutingEvent(logger, key, deletedEntry, event.RouteUnregistrationEvent)
}

func (table *routingTable) getRegistrationEvents(
	logger lager.Logger,
	key endpoint.RoutingKey,
	existingEntry, newEntry endpoint.RoutableEndpoints) event.RoutingEvents {
	logger.Debug("get-registration-events")
	if newEntry.ExternalEndpoints.HasNoExternalPorts(logger) {
		return event.RoutingEvents{}
	}

	if !existingEntry.HaveExternalEndpointsChanged(newEntry) &&
		!existingEntry.HaveEndpointsChanged(newEntry) {
		logger.Debug("no-change-to-endpoints")
		return event.RoutingEvents{}
	}

	routingEvents := event.RoutingEvents{}

	// We are replacing the whole mapping so just check if there exists any endpoints
	if len(newEntry.Endpoints) > 0 {
		routingEvents = append(routingEvents, event.RoutingEvent{
			EventType: event.RouteRegistrationEvent,
			Key:       key,
			Entry:     newEntry,
		})
	}
	return routingEvents
}

func (table *routingTable) createRoutingEvent(logger lager.Logger, key endpoint.RoutingKey, entry endpoint.RoutableEndpoints, eventType event.RoutingEventType) event.RoutingEvents {
	logger.Debug("create-routing-events")
	// in which case does a entry end up with no external endpoints ?
	if entry.ExternalEndpoints.HasNoExternalPorts(logger) {
		return event.RoutingEvents{}
	}

	if len(entry.Endpoints) > 0 {
		logger.Debug("endpoints", lager.Data{"count": len(entry.Endpoints)})
		return event.RoutingEvents{
			event.RoutingEvent{
				EventType: eventType,
				Key:       key,
				Entry:     entry,
			},
		}
	}
	return event.RoutingEvents{}
}

func (table *routingTable) getDeletedEntry(existingEntry, newEntry endpoint.RoutableEndpoints) endpoint.RoutableEndpoints {
	// Assuming ExternalEndpoints for both existingEntry, newEntry are the same.
	gapEntry := existingEntry.Copy()
	for endpointKey := range existingEntry.Endpoints {
		if _, ok := newEntry.Endpoints[endpointKey]; ok {
			delete(gapEntry.Endpoints, endpointKey)
		}
	}
	return gapEntry
}
