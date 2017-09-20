package routing_table

import "code.cloudfoundry.org/bbs/models"

type EndpointKey struct {
	InstanceGuid string
	Evacuating   bool
}

type Address struct {
	Host string
	Port uint32
}

type Endpoint struct {
	InstanceGuid    string
	Index           int32
	Host            string
	Domain          string
	Port            uint32
	ContainerPort   uint32
	Evacuating      bool
	ModificationTag *models.ModificationTag
}

func (e Endpoint) key() EndpointKey {
	return EndpointKey{InstanceGuid: e.InstanceGuid, Evacuating: e.Evacuating}
}

func (e Endpoint) address() Address {
	return Address{Host: e.Host, Port: e.Port}
}

type Route struct {
	Hostname        string
	LogGuid         string
	RouteServiceUrl string
}

type RoutableEndpoints struct {
	Endpoints       map[EndpointKey]Endpoint
	Routes          []Route
	ModificationTag *models.ModificationTag
}

type RoutingKey struct {
	ProcessGuid   string
	ContainerPort uint32
}

func NewRoutableEndpoints() RoutableEndpoints {
	return RoutableEndpoints{
		Endpoints: map[EndpointKey]Endpoint{},
	}
}

func (entry RoutableEndpoints) hasEndpoint(endpoint Endpoint) bool {
	key := endpoint.key()
	_, found := entry.Endpoints[key]
	if !found {
		key.Evacuating = !key.Evacuating
		_, found = entry.Endpoints[key]
	}
	return found
}

func (entry RoutableEndpoints) hasHostname(hostname string) bool {
	for _, route := range entry.Routes {
		if route.Hostname == hostname {
			return true
		}
	}
	return false
}

func (entry RoutableEndpoints) hasRouteServiceUrl(routeServiceUrl string) bool {
	for _, route := range entry.Routes {
		if route.RouteServiceUrl == routeServiceUrl {
			return true
		}
	}
	return false
}

func (entry RoutableEndpoints) copy() RoutableEndpoints {
	clone := RoutableEndpoints{
		Endpoints:       map[EndpointKey]Endpoint{},
		Routes:          make([]Route, len(entry.Routes)),
		ModificationTag: entry.ModificationTag,
	}

	copy(clone.Routes, entry.Routes)

	for k, v := range entry.Endpoints {
		clone.Endpoints[k] = v
	}

	return clone
}

func routesAsMap(routes []string) map[string]struct{} {
	routesMap := map[string]struct{}{}
	for _, route := range routes {
		routesMap[route] = struct{}{}
	}
	return routesMap
}

func EndpointsAsMap(endpoints []Endpoint) map[EndpointKey]Endpoint {
	endpointsMap := map[EndpointKey]Endpoint{}
	for _, endpoint := range endpoints {
		endpointsMap[endpoint.key()] = endpoint
	}
	return endpointsMap
}
