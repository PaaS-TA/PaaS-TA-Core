package event

import (
	"errors"

	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/routing-api/models"
	"code.cloudfoundry.org/tcp-emitter/routing_table/schema/endpoint"
)

type RoutingEventType string

const (
	RouteRegistrationEvent   RoutingEventType = "RouteRegistrationEvent"
	RouteUnregistrationEvent RoutingEventType = "RouteUnregistrationEvent"
)

type RoutingEvent struct {
	EventType RoutingEventType
	Key       endpoint.RoutingKey
	Entry     endpoint.RoutableEndpoints
}

type RoutingEvents []RoutingEvent

func (r RoutingEvent) Valid() bool {
	if len(r.Entry.Endpoints) == 0 {
		return false
	}
	if len(r.Entry.ExternalEndpoints) == 0 {
		return false
	}
	for _, externalEndpoint := range r.Entry.ExternalEndpoints {
		if externalEndpoint.Port == 0 {
			return false
		}
	}
	return true
}

func (routingEvents RoutingEvents) ToMappingRequests(logger lager.Logger, ttl int) ([]models.TcpRouteMapping, []models.TcpRouteMapping) {
	registrationEvents := RoutingEvents{}
	unregistrationEvents := RoutingEvents{}
	for _, routingEvent := range routingEvents {
		if !routingEvent.Valid() {
			logger.Error("invalid-routing-event", errors.New("Invalid routing event"), lager.Data{"routing-event-key": routingEvent.Key})
			continue
		}

		if routingEvent.EventType == RouteRegistrationEvent {
			registrationEvents = append(registrationEvents, routingEvent)
		} else if routingEvent.EventType == RouteUnregistrationEvent {
			unregistrationEvents = append(unregistrationEvents, routingEvent)
		}
	}

	registrationMappingRequests := buildMappingRequests(registrationEvents, ttl)

	unregistrationMappingRequests := buildMappingRequests(unregistrationEvents, ttl)

	return registrationMappingRequests, unregistrationMappingRequests
}

func buildMappingRequests(routingEvents RoutingEvents, ttl int) []models.TcpRouteMapping {
	mappingRequests := make([]models.TcpRouteMapping, 0)
	for _, routingEvent := range routingEvents {
		mappingRequest := mapRoutingEvent(routingEvent, ttl)
		if mappingRequest != nil {
			mappingRequests = append(mappingRequests, (*mappingRequest)...)
		}
	}
	return mappingRequests
}

func mapRoutingEvent(routingEvent RoutingEvent, ttl int) *[]models.TcpRouteMapping {
	mappingRequests := make([]models.TcpRouteMapping, 0)
	for _, externalEndpoint := range routingEvent.Entry.ExternalEndpoints {
		for _, endpoint := range routingEvent.Entry.Endpoints {
			mappingRequests = append(mappingRequests, models.NewTcpRouteMapping(externalEndpoint.RouterGroupGUID, uint16(externalEndpoint.Port),
				endpoint.Host, uint16(endpoint.Port), ttl))
		}
	}
	return &mappingRequests
}
