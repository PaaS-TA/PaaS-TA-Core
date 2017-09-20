package routing_table

import (
	"sync"

	"code.cloudfoundry.org/bbs/models"
	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/runtimeschema/metric"
)

var addressCollisions = metric.Counter("AddressCollisions")

//go:generate counterfeiter -o fake_routing_table/fake_routing_table.go . RoutingTable

type RoutingTable interface {
	RouteCount() int

	Swap(newTable RoutingTable, domains models.DomainSet) MessagesToEmit

	SetRoutes(key RoutingKey, routes []Route, modTag *models.ModificationTag) MessagesToEmit
	RemoveRoutes(key RoutingKey, modTag *models.ModificationTag) MessagesToEmit
	AddEndpoint(key RoutingKey, endpoint Endpoint) MessagesToEmit
	RemoveEndpoint(key RoutingKey, endpoint Endpoint) MessagesToEmit
	EndpointsForIndex(key RoutingKey, index int32) []Endpoint

	MessagesToEmit() MessagesToEmit
}

type noopLocker struct{}

func (noopLocker) Lock()   {}
func (noopLocker) Unlock() {}

type routingTable struct {
	entries        map[RoutingKey]RoutableEndpoints
	addressEntries map[Address]EndpointKey // for collision detection
	sync.Locker
	messageBuilder MessageBuilder
	logger         lager.Logger
}

func NewTempTable(routesMap RoutesByRoutingKey, endpointsByKey EndpointsByRoutingKey) RoutingTable {
	entries := make(map[RoutingKey]RoutableEndpoints)
	addressEntries := make(map[Address]EndpointKey)

	for key, routes := range routesMap {
		entries[key] = RoutableEndpoints{
			Routes: routes,
		}
	}

	for key, endpoints := range endpointsByKey {
		entry, ok := entries[key]
		if !ok {
			entry = RoutableEndpoints{}
		}
		entry.Endpoints = EndpointsAsMap(endpoints)
		entries[key] = entry
		for _, endpoint := range endpoints {
			addressEntries[endpoint.address()] = endpoint.key()
		}
	}

	return &routingTable{
		entries:        entries,
		addressEntries: addressEntries,
		Locker:         noopLocker{},
		messageBuilder: NoopMessageBuilder{},
	}
}

func NewTable(logger lager.Logger) RoutingTable {
	return &routingTable{
		entries:        make(map[RoutingKey]RoutableEndpoints),
		addressEntries: make(map[Address]EndpointKey),
		Locker:         &sync.Mutex{},
		messageBuilder: MessagesToEmitBuilder{},
		logger:         logger,
	}
}

func (table *routingTable) EndpointsForIndex(key RoutingKey, index int32) []Endpoint {
	table.Lock()
	defer table.Unlock()

	endpointsForIndex := make([]Endpoint, 0, 2)
	endpointsForKey := table.entries[key].Endpoints

	for i := range endpointsForKey {
		endpoint := endpointsForKey[i]
		if endpoint.Index == index {
			endpointsForIndex = append(endpointsForIndex, endpoint)
		}
	}

	return endpointsForIndex
}

func (table *routingTable) RouteCount() int {
	table.Lock()

	count := 0
	for _, entry := range table.entries {
		count += len(entry.Routes)
	}

	table.Unlock()
	return count
}

func (table *routingTable) Swap(t RoutingTable, domains models.DomainSet) MessagesToEmit {
	messagesToEmit := MessagesToEmit{}

	newTable, ok := t.(*routingTable)
	if !ok {
		return messagesToEmit
	}
	newEntries := newTable.entries
	updatedEntries := make(map[RoutingKey]RoutableEndpoints)
	updatedAddressEntries := make(map[Address]EndpointKey)

	table.Lock()
	for key, newEntry := range newEntries {
		// See if we have a match
		existingEntry, _ := table.entries[key]

		//always register everything on sync  NOTE if a merge does occur we may return an altered newEntry
		messagesToEmit = messagesToEmit.merge(table.messageBuilder.MergedRegistrations(&existingEntry, &newEntry, domains))
		updatedEntries[key] = newEntry
		for _, endpoint := range newEntry.Endpoints {
			updatedAddressEntries[endpoint.address()] = endpoint.key()
		}
	}

	for key, existingEntry := range table.entries {
		newEntry, ok := newEntries[key]
		messagesToEmit = messagesToEmit.merge(table.messageBuilder.UnregistrationsFor(&existingEntry, &newEntry, domains))

		// maybe reemit old ones no longer found in the new table
		if !ok {
			unfreshRegistrations := table.messageBuilder.UnfreshRegistrations(&existingEntry, domains)
			if len(unfreshRegistrations.RegistrationMessages) > 0 {
				updatedEntries[key] = existingEntry
				for _, endpoint := range existingEntry.Endpoints {
					updatedAddressEntries[endpoint.address()] = endpoint.key()
				}
				messagesToEmit = messagesToEmit.merge(unfreshRegistrations)
			}
		}
	}

	table.entries = updatedEntries
	table.addressEntries = updatedAddressEntries
	table.Unlock()

	return messagesToEmit
}

func (table *routingTable) MessagesToEmit() MessagesToEmit {
	table.Lock()

	messagesToEmit := MessagesToEmit{}
	for _, entry := range table.entries {
		messagesToEmit = messagesToEmit.merge(table.messageBuilder.RegistrationsFor(nil, &entry))
	}

	table.Unlock()
	return messagesToEmit
}

func (table *routingTable) SetRoutes(key RoutingKey, routes []Route, modTag *models.ModificationTag) MessagesToEmit {
	table.Lock()
	defer table.Unlock()

	currentEntry := table.entries[key]
	if !currentEntry.ModificationTag.SucceededBy(modTag) {
		return MessagesToEmit{}
	}

	newEntry := currentEntry.copy()
	newEntry.Routes = routes
	newEntry.ModificationTag = modTag
	table.entries[key] = newEntry

	return table.emit(key, currentEntry, newEntry)
}

func (table *routingTable) RemoveRoutes(key RoutingKey, modTag *models.ModificationTag) MessagesToEmit {
	table.Lock()
	defer table.Unlock()

	currentEntry := table.entries[key]
	if !(currentEntry.ModificationTag.Equal(modTag) || currentEntry.ModificationTag.SucceededBy(modTag)) {
		return MessagesToEmit{}
	}

	newEntry := NewRoutableEndpoints()
	newEntry.Endpoints = currentEntry.Endpoints

	table.entries[key] = newEntry

	return table.emit(key, currentEntry, newEntry)
}

func (table *routingTable) AddEndpoint(key RoutingKey, endpoint Endpoint) MessagesToEmit {
	table.Lock()
	defer table.Unlock()

	currentEntry := table.entries[key]
	newEntry := currentEntry.copy()
	newEntry.Endpoints[endpoint.key()] = endpoint
	table.entries[key] = newEntry

	address := endpoint.address()

	if existingEndpointKey, ok := table.addressEntries[address]; ok {
		if existingEndpointKey.InstanceGuid != endpoint.InstanceGuid {
			addressCollisions.Add(1)
			existingInstanceGuid := existingEndpointKey.InstanceGuid
			table.logger.Info("collision-detected-with-endpoint", lager.Data{
				"instance_guid_a": existingInstanceGuid,
				"instance_guid_b": endpoint.InstanceGuid,
				"Address":         endpoint.address(),
			})
		}
	}

	table.addressEntries[address] = endpoint.key()

	return table.emit(key, currentEntry, newEntry)
}

func (table *routingTable) RemoveEndpoint(key RoutingKey, endpoint Endpoint) MessagesToEmit {
	table.Lock()
	defer table.Unlock()

	currentEntry := table.entries[key]
	endpointKey := endpoint.key()
	currentEndpoint, ok := currentEntry.Endpoints[endpointKey]
	if !ok || !(currentEndpoint.ModificationTag.Equal(endpoint.ModificationTag) || currentEndpoint.ModificationTag.SucceededBy(endpoint.ModificationTag)) {
		return MessagesToEmit{}
	}

	newEntry := currentEntry.copy()
	delete(newEntry.Endpoints, endpointKey)
	table.entries[key] = newEntry

	delete(table.addressEntries, endpoint.address())

	return table.emit(key, currentEntry, newEntry)
}

func (table *routingTable) emit(key RoutingKey, oldEntry, newEntry RoutableEndpoints) MessagesToEmit {
	messagesToEmit := table.messageBuilder.RegistrationsFor(&oldEntry, &newEntry)
	messagesToEmit = messagesToEmit.merge(table.messageBuilder.UnregistrationsFor(&oldEntry, &newEntry, nil))

	return messagesToEmit
}
