package event_test

import (
	"code.cloudfoundry.org/bbs/models"
	"code.cloudfoundry.org/tcp-emitter/routing_table/schema/endpoint"
	"code.cloudfoundry.org/tcp-emitter/routing_table/schema/event"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("RoutingEvent", func() {
	var (
		routingEvent      event.RoutingEvent
		routingKey        endpoint.RoutingKey
		modificationTag   *models.ModificationTag
		endpoints         map[endpoint.EndpointKey]endpoint.Endpoint
		routableEndpoints endpoint.RoutableEndpoints
		logGuid           string
	)
	BeforeEach(func() {
		routingKey = endpoint.NewRoutingKey("process-guid-1", 5222)
		logGuid = "log-guid-1"
		modificationTag = &models.ModificationTag{Epoch: "abc", Index: 1}
		endpoints = map[endpoint.EndpointKey]endpoint.Endpoint{
			endpoint.NewEndpointKey("instance-guid-1", false): endpoint.NewEndpoint(
				"instance-guid-1", false, "some-ip-1", 62004, 5222, modificationTag),
			endpoint.NewEndpointKey("instance-guid-2", false): endpoint.NewEndpoint(
				"instance-guid-2", false, "some-ip-2", 62004, 5222, modificationTag),
		}
	})

	Context("Valid", func() {
		Context("valid routing event is passed", func() {
			BeforeEach(func() {
				externalEndpoints := endpoint.ExternalEndpointInfos{
					endpoint.NewExternalEndpointInfo("some-guid", 61000),
				}
				routableEndpoints = endpoint.NewRoutableEndpoints(externalEndpoints, endpoints, logGuid, modificationTag)
				routingEvent = event.RoutingEvent{EventType: event.RouteRegistrationEvent, Key: routingKey, Entry: routableEndpoints}
			})
			It("returns true", func() {
				Expect(routingEvent.Valid()).To(BeTrue())
			})
		})

		Context("routing event with empty endpoints is passed", func() {
			BeforeEach(func() {
				externalEndpoints := endpoint.ExternalEndpointInfos{}
				routableEndpoints = endpoint.NewRoutableEndpoints(externalEndpoints, endpoints, logGuid, modificationTag)
				routingEvent = event.RoutingEvent{EventType: event.RouteRegistrationEvent, Key: routingKey, Entry: routableEndpoints}
			})
			It("returns false", func() {
				Expect(routingEvent.Valid()).To(BeFalse())
			})
		})

		Context("routing event with one external port is zero", func() {
			BeforeEach(func() {
				externalEndpoints := endpoint.ExternalEndpointInfos{
					endpoint.NewExternalEndpointInfo("router-guid", 0),
				}
				routableEndpoints = endpoint.NewRoutableEndpoints(externalEndpoints, endpoints, logGuid, modificationTag)
				routingEvent = event.RoutingEvent{EventType: event.RouteRegistrationEvent, Key: routingKey, Entry: routableEndpoints}
			})
			It("returns false", func() {
				Expect(routingEvent.Valid()).To(BeFalse())
			})
		})

		Context("routing event with no endpoints", func() {
			BeforeEach(func() {
				externalEndpoints := endpoint.ExternalEndpointInfos{
					endpoint.NewExternalEndpointInfo("r-g", 61000),
				}
				routableEndpoints = endpoint.NewRoutableEndpoints(externalEndpoints, map[endpoint.EndpointKey]endpoint.Endpoint{},
					logGuid, modificationTag)
				routingEvent = event.RoutingEvent{EventType: event.RouteRegistrationEvent, Key: routingKey, Entry: routableEndpoints}
			})
			It("returns false", func() {
				Expect(routingEvent.Valid()).To(BeFalse())
			})
		})
	})
})
