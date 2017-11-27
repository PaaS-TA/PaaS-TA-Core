package emitter

import (
	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/route-emitter/routingtable"
	"code.cloudfoundry.org/routing-api"
	"code.cloudfoundry.org/routing-api/models"
	uaaclient "code.cloudfoundry.org/uaa-go-client"
)

//go:generate counterfeiter -o fakes/fake_routing_api_emitter.go . RoutingAPIEmitter
type RoutingAPIEmitter interface {
	Emit(routingEvents routingtable.TCPRouteMappings) error
}

type routingAPIEmitter struct {
	logger           lager.Logger
	routingAPIClient routing_api.Client
	ttl              int
	uaaClient        uaaclient.Client
}

func NewRoutingAPIEmitter(logger lager.Logger, routingAPIClient routing_api.Client, uaaClient uaaclient.Client, routeTTL int) RoutingAPIEmitter {
	return &routingAPIEmitter{
		logger:           logger,
		routingAPIClient: routingAPIClient,
		ttl:              routeTTL,
		uaaClient:        uaaClient,
	}
}

func (t *routingAPIEmitter) Emit(tcpEvents routingtable.TCPRouteMappings) error {
	defer t.logger.Debug("complete-emit")

	if len(tcpEvents.Registrations) <= 0 && len(tcpEvents.Unregistrations) <= 0 {
		return nil
	}

	err := t.emit(tcpEvents.Registrations, tcpEvents.Unregistrations)
	if err != nil {
		return err
	}

	return nil
}

func (t *routingAPIEmitter) emit(registrationMappingRequests, unregistrationMappingRequests []models.TcpRouteMapping) error {
	var forceUpdate bool

	for count := 0; count < 2; count++ {
		forceUpdate = count > 0
		token, err := t.uaaClient.FetchToken(forceUpdate)
		if err != nil {
			return err
		}

		t.routingAPIClient.SetToken(token.AccessToken)

		err = t.emitRoutingAPI(registrationMappingRequests, unregistrationMappingRequests)
		if err != nil && count > 0 {
			return err
		} else if err == nil {
			break
		}
	}

	t.logger.Debug("successfully-emitted-events")
	return nil
}

func (t *routingAPIEmitter) emitRoutingAPI(regMsgs, unregMsgs []models.TcpRouteMapping) error {
	for i := range regMsgs {
		regMsgs[i].TTL = &t.ttl
	}
	for i := range unregMsgs {
		unregMsgs[i].TTL = &t.ttl
	}

	if len(regMsgs) > 0 {
		if err := t.routingAPIClient.UpsertTcpRouteMappings(regMsgs); err != nil {
			t.logger.Error("unable-to-upsert", err)
			return err
		}
		t.logger.Debug("successfully-emitted-registration-events",
			lager.Data{"number-of-registration-events": len(regMsgs)})
	}

	if len(unregMsgs) > 0 {
		if err := t.routingAPIClient.DeleteTcpRouteMappings(unregMsgs); err != nil {
			t.logger.Error("unable-to-delete", err)
			return err
		}
		t.logger.Debug("successfully-emitted-unregistration-events",
			lager.Data{"number-of-unregistration-events": len(unregMsgs)})
	}
	return nil
}
