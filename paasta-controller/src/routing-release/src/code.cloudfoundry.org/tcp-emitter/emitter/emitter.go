package emitter

import (
	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/routing-api"
	"code.cloudfoundry.org/routing-api/models"
	"code.cloudfoundry.org/tcp-emitter/routing_table/schema/endpoint"
	"code.cloudfoundry.org/tcp-emitter/routing_table/schema/event"
	uaaclient "code.cloudfoundry.org/uaa-go-client"
)

//go:generate counterfeiter -o fakes/fake_emitter.go . Emitter
type Emitter interface {
	Emit(routingEvents event.RoutingEvents) error
}

type tcpEmitter struct {
	logger           lager.Logger
	routingAPIClient routing_api.Client
	uaaClient        uaaclient.Client
	ttl              int
}

func NewEmitter(logger lager.Logger, routingAPIClient routing_api.Client, uaaClient uaaclient.Client, routeTTL int) Emitter {
	return &tcpEmitter{
		logger:           logger,
		routingAPIClient: routingAPIClient,
		uaaClient:        uaaClient,
		ttl:              routeTTL,
	}
}

func (emitter *tcpEmitter) Emit(routingEvents event.RoutingEvents) error {
	emitter.logRoutingEvents(routingEvents)
	defer emitter.logger.Debug("complete-emit")

	registrationMappingRequests, unregistrationMappingRequests := routingEvents.ToMappingRequests(emitter.logger, emitter.ttl)
	useCachedToken := true
	for count := 0; count < 2; count++ {
		token, err := emitter.uaaClient.FetchToken(!useCachedToken)
		if err != nil {
			emitter.logger.Error("unable-to-get-token", err)
			return err
		}
		emitter.routingAPIClient.SetToken(token.AccessToken)
		err = emitter.emit(registrationMappingRequests, unregistrationMappingRequests)
		if err != nil && err.Error() == "unauthorized" {
			useCachedToken = false
			emitter.logger.Info("retrying-emit")
		} else {
			break
		}
	}

	return nil
}

func (emitter *tcpEmitter) emit(registrationMappingRequests, unregistrationMappingRequests []models.TcpRouteMapping) error {
	emitted := true
	if len(registrationMappingRequests) > 0 {
		if err := emitter.routingAPIClient.UpsertTcpRouteMappings(registrationMappingRequests); err != nil {
			emitted = false
			emitter.logger.Error("unable-to-upsert", err)
			return err
		}
		emitter.logger.Debug("successfully-emitted-registration-events",
			lager.Data{"number-of-registration-events": len(registrationMappingRequests)})

	}

	if len(unregistrationMappingRequests) > 0 {
		if err := emitter.routingAPIClient.DeleteTcpRouteMappings(unregistrationMappingRequests); err != nil {
			emitted = false
			emitter.logger.Error("unable-to-delete", err)
			return err
		}
		emitter.logger.Debug("successfully-emitted-unregistration-events",
			lager.Data{"number-of-unregistration-events": len(unregistrationMappingRequests)})

	}

	if emitted {
		emitter.logger.Debug("successfully-emitted-events")
	}
	return nil
}

func (emitter *tcpEmitter) logRoutingEvents(routingEvents event.RoutingEvents) {
	for _, event := range routingEvents {
		endpoints := make([]endpoint.Endpoint, 0)
		for _, endpoint := range event.Entry.Endpoints {
			endpoints = append(endpoints, endpoint)
		}

		ports := make([]uint32, 0)
		for _, extEndpoint := range event.Entry.ExternalEndpoints {
			ports = append(ports, extEndpoint.Port)
		}
		emitter.logger.Info("mapped-routes", lager.Data{
			"external_ports": ports,
			"backends":       endpoints})
	}
}
