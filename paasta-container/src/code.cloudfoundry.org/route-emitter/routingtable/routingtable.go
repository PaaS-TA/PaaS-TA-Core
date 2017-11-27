package routingtable

import (
	"sync"

	tcpmodels "code.cloudfoundry.org/routing-api/models"
	"code.cloudfoundry.org/runtimeschema/metric"

	"code.cloudfoundry.org/bbs/models"
	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/routing-info/cfroutes"
	"code.cloudfoundry.org/routing-info/tcp_routes"
)

type TCPRouteMappings struct {
	Registrations   []tcpmodels.TcpRouteMapping
	Unregistrations []tcpmodels.TcpRouteMapping
}

func (mappings TCPRouteMappings) Merge(other TCPRouteMappings) TCPRouteMappings {
	var result TCPRouteMappings
	result.Registrations = append(mappings.Registrations, other.Registrations...)
	result.Unregistrations = append(mappings.Unregistrations, other.Unregistrations...)
	return result
}

var addressCollisions = metric.Counter("AddressCollisions")

//go:generate counterfeiter -o fakeroutingtable/fake_routingtable.go . RoutingTable

type RoutingTable interface {
	// table modification
	SetRoutes(beforeLRP, afterLRP *models.DesiredLRPSchedulingInfo) (TCPRouteMappings, MessagesToEmit)
	RemoveRoutes(desiredLRP *models.DesiredLRPSchedulingInfo) (TCPRouteMappings, MessagesToEmit)
	AddEndpoint(actualLRP *ActualLRPRoutingInfo) (TCPRouteMappings, MessagesToEmit)
	RemoveEndpoint(actualLRP *ActualLRPRoutingInfo) (TCPRouteMappings, MessagesToEmit)
	Swap(t RoutingTable, domains models.DomainSet) (TCPRouteMappings, MessagesToEmit)
	GetRoutingEvents() (TCPRouteMappings, MessagesToEmit)

	// routes

	HasExternalRoutes(actual *ActualLRPRoutingInfo) bool
	HTTPAssociationsCount() int // return number of associations desired-lrp-http-routes * actual-lrps
	TCPAssociationsCount() int  // return number of associations desired-lrp-tcp-routes * actual-lrps
	TableSize() int
}

type routingTable struct {
	entries             map[RoutingKey]RoutableEndpoints
	addressEntries      map[Address]EndpointKey
	addressGenerator    func(endpoint Endpoint) Address
	directInstanceRoute bool
	logger              lager.Logger
	sync.Locker
}

func NewRoutingTable(logger lager.Logger, directInstanceRoute bool) RoutingTable {
	addressGenerator := func(endpoint Endpoint) Address {
		return Address{Host: endpoint.Host, Port: endpoint.Port}
	}

	if directInstanceRoute {
		addressGenerator = func(endpoint Endpoint) Address {
			return Address{Host: endpoint.ContainerIP, Port: endpoint.ContainerPort}
		}
	}

	return &routingTable{
		entries:             make(map[RoutingKey]RoutableEndpoints),
		addressEntries:      make(map[Address]EndpointKey),
		directInstanceRoute: directInstanceRoute,
		addressGenerator:    addressGenerator,
		logger:              logger,
		Locker:              &sync.Mutex{},
	}
}

func (table *routingTable) AddEndpoint(actualLRP *ActualLRPRoutingInfo) (TCPRouteMappings, MessagesToEmit) {
	logger := table.logger.Session("AddEndpoint", lager.Data{"actual_lrp": actualLRP})
	logger.Debug("starting")
	defer logger.Debug("completed")

	table.Lock()
	defer table.Unlock()

	logger.Info("handler-add-and-emit", lager.Data{"net_info": actualLRP.ActualLRP.ActualLRPNetInfo})
	endpoints := NewEndpointsFromActual(actualLRP)

	// collision detection

	for _, endpoint := range endpoints {
		address := table.addressGenerator(endpoint)
		// if the address exists and the instance guid doesn't match then we have a collision
		if existingEndpointKey, ok := table.addressEntries[address]; ok && existingEndpointKey.InstanceGUID != endpoint.InstanceGUID {
			addressCollisions.Add(1)
			existingInstanceGuid := existingEndpointKey.InstanceGUID
			table.logger.Info("collision-detected-with-endpoint", lager.Data{
				"instance_guid_a": existingInstanceGuid,
				"instance_guid_b": endpoint.InstanceGUID,
				"Address":         address,
			})
		}

		table.addressEntries[address] = endpoint.key()
	}

	// add endpoints

	var messagesToEmit MessagesToEmit
	var mappings TCPRouteMappings

	for _, routingEndpoint := range endpoints {
		key := RoutingKey{
			ProcessGUID:   actualLRP.ActualLRP.ProcessGuid,
			ContainerPort: routingEndpoint.ContainerPort,
		}
		currentEntry := table.entries[key]
		if currentEntry.DesiredInstances > 0 && routingEndpoint.Index >= currentEntry.DesiredInstances {
			logger.Debug("skipping-undesired-instance")
			continue
		}
		currentEndpoint, ok := currentEntry.Endpoints[routingEndpoint.key()]
		if ok && !currentEndpoint.ModificationTag.SucceededBy(routingEndpoint.ModificationTag) {
			continue
		}
		newEntry := currentEntry.copy()
		newEntry.Endpoints[routingEndpoint.key()] = routingEndpoint
		table.entries[key] = newEntry
		mapping, message := table.emitDiffMessages(key, currentEntry, newEntry)
		mappings = mappings.Merge(mapping)
		messagesToEmit = messagesToEmit.Merge(message)
	}

	return mappings, messagesToEmit
}

func (table *routingTable) RemoveEndpoint(actualLRP *ActualLRPRoutingInfo) (TCPRouteMappings, MessagesToEmit) {
	logger := table.logger.Session("RemoveEndpoint", lager.Data{"actual_lrp": actualLRP})
	logger.Debug("starting")
	defer logger.Debug("completed")

	table.Lock()
	defer table.Unlock()

	table.logger.Session("removing-endpoint")
	table.logger.Info("starting")
	defer table.logger.Info("complete")
	endpoints := NewEndpointsFromActual(actualLRP)

	// remove address
	for _, endpoint := range endpoints {
		address := table.addressGenerator(endpoint)
		currentEntry, ok := table.addressEntries[address]
		if ok && currentEntry.InstanceGUID != endpoint.InstanceGUID {
			table.logger.Info("collision-detected-with-endpoint", lager.Data{
				"instance_guid_a": currentEntry.InstanceGUID,
				"instance_guid_b": endpoint.InstanceGUID,
				"Address":         address,
			})
			continue
		}
		delete(table.addressEntries, address)
	}

	// remove endpoint
	var messagesToEmit MessagesToEmit
	var mappings TCPRouteMappings
	for _, routingEndpoint := range endpoints {
		key := RoutingKey{
			ProcessGUID:   actualLRP.ActualLRP.ProcessGuid,
			ContainerPort: routingEndpoint.ContainerPort,
		}

		currentEntry := table.entries[key]
		endpointKey := routingEndpoint.key()
		currentEndpoint, ok := currentEntry.Endpoints[endpointKey]

		if !ok ||
			(!currentEndpoint.ModificationTag.Equal(routingEndpoint.ModificationTag) &&
				!currentEndpoint.ModificationTag.SucceededBy(routingEndpoint.ModificationTag)) {
			continue
		}

		newEntry := currentEntry.copy()
		delete(newEntry.Endpoints, endpointKey)

		table.entries[key] = newEntry
		table.deleteEntryIfEmpty(key)

		mapping, message := table.emitDiffMessages(key, currentEntry, newEntry)
		messagesToEmit = messagesToEmit.Merge(message)
		mappings = mappings.Merge(mapping)
	}

	return mappings, messagesToEmit
}

func (t *routingTable) Swap(other RoutingTable, domains models.DomainSet) (TCPRouteMappings, MessagesToEmit) {
	logger := t.logger.Session("swap", lager.Data{"received-domains": domains})
	logger.Info("started")
	defer logger.Info("finished")

	t.Lock()
	defer t.Unlock()

	otherTable, ok := other.(*routingTable)
	if !ok {
		logger.Error("failed-to-convert-to-routing-table", nil)
		return TCPRouteMappings{}, MessagesToEmit{}
	}

	// collision detection swap

	t.addressEntries = otherTable.addressEntries

	var messagesToEmit MessagesToEmit
	var mappings TCPRouteMappings

	mergedRoutingKeys := map[RoutingKey]struct{}{}
	for key, _ := range otherTable.entries {
		mergedRoutingKeys[key] = struct{}{}
	}
	for key, _ := range t.entries {
		mergedRoutingKeys[key] = struct{}{}
	}

	for key := range mergedRoutingKeys {
		existingEntry, ok := t.entries[key]
		newEntry := otherTable.entries[key]
		if !ok {
			// routing key only exist in the new table
			mapping, message := t.emitDiffMessages(key, RoutableEndpoints{}, newEntry)
			messagesToEmit = messagesToEmit.Merge(message)
			mappings = mappings.Merge(mapping)
			continue
		}

		// entry exists in both tables or in old table, merge the two entries to ensure non-fresh domain endpoints aren't removed
		merged := mergeUnfreshRoutes(existingEntry, newEntry, domains)
		otherTable.entries[key] = merged
		otherTable.deleteEntryIfEmpty(key)
		mapping, message := t.emitDiffMessages(key, existingEntry, merged)
		messagesToEmit = messagesToEmit.Merge(message)
		mappings = mappings.Merge(mapping)
	}

	t.entries = otherTable.entries

	return mappings, messagesToEmit
}

// merge the routes from both endpoints, ensuring that non-fresh routes aren't removed
func mergeUnfreshRoutes(before, after RoutableEndpoints, domains models.DomainSet) RoutableEndpoints {
	merged := after.copy()
	merged.Domain = before.Domain

	if !domains.Contains(before.Domain) {
		// non-fresh domain, append routes from older endpoint
		for _, oldRoute := range before.Routes {
			routeExistInNewLRP := func() bool {
				for _, newRoute := range after.Routes {
					if newRoute == oldRoute {
						return true
					}
				}
				return false
			}
			if !routeExistInNewLRP() {
				merged.Routes = append(merged.Routes, oldRoute)
			}
		}
	}

	return merged
}

func (t *routingTable) GetRoutingEvents() (TCPRouteMappings, MessagesToEmit) {
	logger := t.logger.Session("get-routing-events")
	logger.Info("started")
	defer logger.Info("finished")

	t.Lock()
	defer t.Unlock()

	var messagesToEmit MessagesToEmit
	var mappings TCPRouteMappings
	for key, route := range t.entries {
		mapping, message := t.emitDiffMessages(key, RoutableEndpoints{}, route)

		mappings = mappings.Merge(mapping)
		messagesToEmit = messagesToEmit.Merge(message)
	}

	return mappings, messagesToEmit
}

type externalRoute interface {
	MessageFor(endpoint Endpoint, directInstanceAddress bool) (*RegistryMessage, *tcpmodels.TcpRouteMapping)
}

func httpRoutesFromSchedulingInfo(lrp *models.DesiredLRPSchedulingInfo) map[RoutingKey][]externalRoute {
	if lrp == nil {
		return nil
	}

	routes, _ := cfroutes.CFRoutesFromRoutingInfo(lrp.Routes)
	routeEntries := make(map[RoutingKey][]externalRoute)
	for _, route := range routes {
		key := RoutingKey{ProcessGUID: lrp.ProcessGuid, ContainerPort: route.Port}

		routes := []externalRoute{}
		for _, hostname := range route.Hostnames {
			route := Route{
				Hostname:         hostname,
				LogGUID:          lrp.LogGuid,
				RouteServiceUrl:  route.RouteServiceUrl,
				IsolationSegment: route.IsolationSegment,
			}
			routes = append(routes, route)
		}
		routeEntries[key] = append(routeEntries[key], routes...)
	}
	return routeEntries
}

func tcpRoutesFromSchedulingInfo(lrp *models.DesiredLRPSchedulingInfo) map[RoutingKey][]externalRoute {
	if lrp == nil {
		return nil
	}

	routes, _ := tcp_routes.TCPRoutesFromRoutingInfo(&lrp.Routes)

	routeEntries := make(map[RoutingKey][]externalRoute)
	for _, route := range routes {
		key := RoutingKey{ProcessGUID: lrp.ProcessGuid, ContainerPort: route.ContainerPort}

		routeEntries[key] = append(routeEntries[key], ExternalEndpointInfo{
			RouterGroupGUID: route.RouterGroupGuid,
			Port:            route.ExternalPort,
		})
	}
	return routeEntries
}

func (table *routingTable) SetRoutes(before, after *models.DesiredLRPSchedulingInfo) (TCPRouteMappings, MessagesToEmit) {
	logger := table.logger.Session("set-routes", lager.Data{"before_lrp": DesiredLRPData(before), "after_lrp": DesiredLRPData(after)})
	logger.Info("started")
	defer logger.Info("finished")

	table.Lock()
	defer table.Unlock()

	// update routes
	httpRemovedRouteEntries := httpRoutesFromSchedulingInfo(before)
	httpRouteEntries := httpRoutesFromSchedulingInfo(after)
	tcpRemovedRouteEntries := tcpRoutesFromSchedulingInfo(before)
	tcpRouteEntries := tcpRoutesFromSchedulingInfo(after)

	var messagesToEmit MessagesToEmit = MessagesToEmit{}
	var mappings TCPRouteMappings

	// merge the http and tcp routes
	mergeRoutes := func(first, second map[RoutingKey][]externalRoute) map[RoutingKey][]externalRoute {
		result := make(map[RoutingKey][]externalRoute)
		for key, routes := range first {
			result[key] = append(routes, second[key]...)
		}

		for key, routes := range second {
			_, ok := first[key]
			if ok {
				// this has been merged already in the previous loop
				continue
			}
			result[key] = routes
		}

		return result
	}

	routeEntries := mergeRoutes(httpRouteEntries, tcpRouteEntries)
	removedRouteEntries := mergeRoutes(httpRemovedRouteEntries, tcpRemovedRouteEntries)
	addedKeys := make(map[RoutingKey]struct{})

	for key, routes := range routeEntries {
		currentEntry := table.entries[key]
		// if modification tag is old, ignore the new lrp
		if !currentEntry.ModificationTag.SucceededBy(&after.ModificationTag) {
			continue
		}

		addedKeys[key] = struct{}{}
		newEntry := currentEntry.copy()
		newEntry.Domain = after.Domain
		newEntry.Routes = routes
		newEntry.ModificationTag = &after.ModificationTag
		newEntry.DesiredInstances = after.Instances

		// check if scaling down
		if before != nil && after != nil && before.Instances > after.Instances {
			newEndpoints := make(map[EndpointKey]Endpoint)

			for endpointKey, endpoint := range newEntry.Endpoints {
				if endpoint.Index < after.Instances {
					newEndpoints[endpointKey] = endpoint
				}
			}
			newEntry.Endpoints = newEndpoints
		}

		table.entries[key] = newEntry

		mapping, message := table.emitDiffMessages(key, currentEntry, newEntry)
		messagesToEmit = messagesToEmit.Merge(message)
		mappings = mappings.Merge(mapping)
	}

	for key := range removedRouteEntries {
		if _, ok := routeEntries[key]; ok {
			continue
		}

		currentEntry := table.entries[key]
		if after == nil {
			// this is a delete (after == nil), then before lrp modification tag must be >=
			if !currentEntry.ModificationTag.Equal(&before.ModificationTag) && !currentEntry.ModificationTag.SucceededBy(&before.ModificationTag) {
				logger.Debug("skipping-old-update")
				continue
			}
		} else if !currentEntry.ModificationTag.SucceededBy(&after.ModificationTag) {
			// this is an update, then after lrp modification tag must be >
			continue
		}

		newEntry := currentEntry.copy()
		newEntry.Routes = nil
		if after != nil {
			newEntry.Domain = after.Domain
			newEntry.ModificationTag = &after.ModificationTag
			newEntry.DesiredInstances = after.Instances
		}

		table.entries[key] = newEntry

		table.deleteEntryIfEmpty(key)

		mapping, message := table.emitDiffMessages(key, currentEntry, newEntry)
		messagesToEmit = messagesToEmit.Merge(message)
		mappings = mappings.Merge(mapping)
	}

	return mappings, messagesToEmit
}

func (table *routingTable) deleteEntryIfEmpty(key RoutingKey) {
	entry := table.entries[key]
	if len(entry.Endpoints) == 0 && len(entry.Routes) == 0 {
		delete(table.entries, key)
	}
}

func (table *routingTable) emitDiffMessages(key RoutingKey, oldEntry, newEntry RoutableEndpoints) (TCPRouteMappings, MessagesToEmit) {
	routesDiff := diffRoutes(oldEntry.Routes, newEntry.Routes)
	endpointsDiff := diffEndpoints(oldEntry.Endpoints, newEntry.Endpoints)
	return table.messages(routesDiff, endpointsDiff)
}

type routesDiff struct {
	before, after, removed, added []externalRoute
}

type endpointsDiff struct {
	before, after, removed, added map[EndpointKey]Endpoint
}

func diffRoutes(before, after []externalRoute) routesDiff {
	existingRoutes := map[externalRoute]struct{}{}
	newRoutes := map[externalRoute]struct{}{}
	for _, route := range before {
		existingRoutes[route] = struct{}{}
	}
	for _, route := range after {
		newRoutes[route] = struct{}{}
	}

	diff := routesDiff{
		before: before,
		after:  after,
	}
	// generate the diff
	for route := range existingRoutes {
		if _, ok := newRoutes[route]; !ok {
			diff.removed = append(diff.removed, route)
		}
	}

	for route := range newRoutes {
		if _, ok := existingRoutes[route]; !ok {
			diff.added = append(diff.added, route)
		}
	}

	return diff
}

// endpoints are different if any field is different other than the following:
// 1. ModificationTag
// 2. Evacuating
func endpointDifferent(before, after Endpoint) bool {
	modificationTag := before.ModificationTag
	evacuating := before.Evacuating
	before.ModificationTag = after.ModificationTag
	before.Evacuating = after.Evacuating
	defer func() {
		before.ModificationTag = modificationTag
		before.Evacuating = evacuating
	}()
	return before != after
}

func diffEndpoints(before, after map[EndpointKey]Endpoint) endpointsDiff {
	diff := endpointsDiff{
		before:  before,
		after:   after,
		removed: make(map[EndpointKey]Endpoint),
		added:   make(map[EndpointKey]Endpoint),
	}
	// generate the diff
	for key, endpoint := range before {
		newEndpoint, ok := after[key]
		if !ok {
			key.Evacuating = !key.Evacuating
			newEndpoint, ok = after[key]
		}
		if !ok || endpointDifferent(newEndpoint, endpoint) {
			diff.removed[key] = endpoint
		}
	}
	for key, endpoint := range after {
		oldEndpoint, ok := before[key]
		if !ok {
			key.Evacuating = !key.Evacuating
			oldEndpoint, ok = before[key]
		}
		if !ok || endpointDifferent(endpoint, oldEndpoint) {
			diff.added[key] = endpoint
		}
	}

	return diff
}

func (table *routingTable) messages(routesDiff routesDiff, endpointDiff endpointsDiff) (TCPRouteMappings, MessagesToEmit) {
	// maps used to remove duplicates
	unregistrations := map[externalRoute]map[Endpoint]bool{}
	registrations := map[externalRoute]map[Endpoint]bool{}

	// for removed routes remove endpoints previously registered
	for _, route := range routesDiff.removed {
		for _, container := range endpointDiff.before {
			if unregistrations[route] != nil && unregistrations[route][container] {
				continue
			}
			if unregistrations[route] == nil {
				unregistrations[route] = map[Endpoint]bool{}
			}
			unregistrations[route][container] = true
		}
	}

	// for added routes add all currently known endpoints
	for _, route := range routesDiff.added {
		for _, container := range endpointDiff.after {
			if registrations[route] != nil && registrations[route][container] {
				continue
			}
			if registrations[route] == nil {
				registrations[route] = map[Endpoint]bool{}
			}
			registrations[route][container] = true
		}
	}

	// for removed endpoints remove routes previously registered
	for _, container := range endpointDiff.removed {
		for _, route := range routesDiff.before {
			if unregistrations[route] != nil && unregistrations[route][container] {
				continue
			}
			if unregistrations[route] == nil {
				unregistrations[route] = map[Endpoint]bool{}
			}
			unregistrations[route][container] = true
		}
	}

	// for added endpoints register all current routes
	for _, container := range endpointDiff.added {
		for _, route := range routesDiff.after {
			if registrations[route] != nil && registrations[route][container] {
				continue
			}
			if registrations[route] == nil {
				registrations[route] = map[Endpoint]bool{}
			}
			registrations[route][container] = true
		}
	}

	messages := MessagesToEmit{}
	mappings := TCPRouteMappings{}

	for r, es := range registrations {
		for e := range es {
			msg, mapping := r.MessageFor(e, table.directInstanceRoute)
			if msg != nil {
				messages.RegistrationMessages = append(messages.RegistrationMessages, *msg)
			}
			if mapping != nil {
				mappings.Registrations = append(mappings.Registrations, *mapping)
			}
		}
	}
	for r, es := range unregistrations {
		for e := range es {
			msg, mapping := r.MessageFor(e, table.directInstanceRoute)
			if msg != nil {
				messages.UnregistrationMessages = append(messages.UnregistrationMessages, *msg)
			}
			if mapping != nil {
				mappings.Unregistrations = append(mappings.Unregistrations, *mapping)
			}
		}
	}
	return mappings, messages
}

func (table *routingTable) RemoveRoutes(desiredLRP *models.DesiredLRPSchedulingInfo) (TCPRouteMappings, MessagesToEmit) {
	logger := table.logger.Session("RemoveRoutes", lager.Data{"desired_lrp": DesiredLRPData(desiredLRP)})
	logger.Debug("starting")
	defer logger.Debug("completed")

	return table.SetRoutes(desiredLRP, nil)
}

func (t *routingTable) HTTPAssociationsCount() int {
	t.Lock()
	defer t.Unlock()

	count := 0
	for _, entry := range t.entries {
		count += numberOfHTTPRoutes(entry) * len(entry.Endpoints)
	}

	return count
}

func (t *routingTable) TCPAssociationsCount() int {
	t.Lock()
	defer t.Unlock()

	count := 0
	for _, entry := range t.entries {
		count += numberOfTCPRoutes(entry) * len(entry.Endpoints)
	}

	return count
}

func (t *routingTable) TableSize() int {
	t.Lock()
	defer t.Unlock()

	return len(t.entries)
}

func numberOfTCPRoutes(routableEndpoints RoutableEndpoints) int {
	count := 0
	for _, route := range routableEndpoints.Routes {
		if _, ok := route.(ExternalEndpointInfo); ok {
			count++
		}
	}
	return count
}

func numberOfHTTPRoutes(routableEndpoints RoutableEndpoints) int {
	return len(routableEndpoints.Routes) - numberOfTCPRoutes(routableEndpoints)
}

func (t *routingTable) HasExternalRoutes(actual *ActualLRPRoutingInfo) bool {
	for _, key := range NewRoutingKeysFromActual(actual) {
		if len(t.entries[key].Routes) > 0 {
			return true
		}
	}

	return false
}
