package event_test

import (
	"code.cloudfoundry.org/bbs/models"
	apimodels "code.cloudfoundry.org/routing-api/models"
	"code.cloudfoundry.org/tcp-emitter/routing_table/schema/endpoint"
	"code.cloudfoundry.org/tcp-emitter/routing_table/schema/event"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("MappingRequestBuilder", func() {

	var (
		routingEvents                  event.RoutingEvents
		expectedRegistrationRequests   []apimodels.TcpRouteMapping
		expectedUnregistrationRequests []apimodels.TcpRouteMapping
		endpoints1                     map[endpoint.EndpointKey]endpoint.Endpoint
		endpoints2                     map[endpoint.EndpointKey]endpoint.Endpoint
		routingKey1                    endpoint.RoutingKey
		routingKey2                    endpoint.RoutingKey
		routableEndpoints1             endpoint.RoutableEndpoints
		routableEndpoints2             endpoint.RoutableEndpoints
		logGuid                        string
		modificationTag                models.ModificationTag
		ttl                            int
	)

	BeforeEach(func() {
		logGuid = "log-guid-1"
		ttl = 60
		modificationTag = models.ModificationTag{Epoch: "abc", Index: 0}

		endpoints1 = map[endpoint.EndpointKey]endpoint.Endpoint{
			endpoint.NewEndpointKey("instance-guid-1", false): endpoint.NewEndpoint(
				"instance-guid-1", false, "some-ip-1", 62003, 5222, &modificationTag),
			endpoint.NewEndpointKey("instance-guid-2", false): endpoint.NewEndpoint(
				"instance-guid-2", false, "some-ip-2", 62004, 5222, &modificationTag),
		}
		endpoints2 = map[endpoint.EndpointKey]endpoint.Endpoint{
			endpoint.NewEndpointKey("instance-guid-1", false): endpoint.NewEndpoint(
				"instance-guid-3", false, "some-ip-3", 62005, 5222, &modificationTag),
			endpoint.NewEndpointKey("instance-guid-2", false): endpoint.NewEndpoint(
				"instance-guid-4", false, "some-ip-4", 62006, 5222, &modificationTag),
		}

		routingKey1 = endpoint.NewRoutingKey("process-guid-1", 5222)
		routingKey2 = endpoint.NewRoutingKey("process-guid-2", 5222)

		extenralEndpointInfo1 := endpoint.NewExternalEndpointInfo("123", 61000)
		extenralEndpointInfo2 := endpoint.NewExternalEndpointInfo("456", 61001)
		extenralEndpointInfo3 := endpoint.NewExternalEndpointInfo("789", 61002)
		endpointInfo1 := endpoint.ExternalEndpointInfos{extenralEndpointInfo1}
		endpointInfo2 := endpoint.ExternalEndpointInfos{
			extenralEndpointInfo2,
			extenralEndpointInfo3,
		}

		routableEndpoints1 = endpoint.NewRoutableEndpoints(endpointInfo1, endpoints1, logGuid, &modificationTag)
		routableEndpoints2 = endpoint.NewRoutableEndpoints(endpointInfo2, endpoints2, logGuid, &modificationTag)
	})

	Context("with valid routing events", func() {
		BeforeEach(func() {
			routingEvents = event.RoutingEvents{
				event.RoutingEvent{
					EventType: event.RouteRegistrationEvent,
					Key:       routingKey1,
					Entry:     routableEndpoints1,
				},
				event.RoutingEvent{
					EventType: event.RouteUnregistrationEvent,
					Key:       routingKey2,
					Entry:     routableEndpoints2,
				},
			}

			expectedRegistrationRequests = []apimodels.TcpRouteMapping{
				apimodels.NewTcpRouteMapping("123", 61000, "some-ip-1", 62003, 60),
				apimodels.NewTcpRouteMapping("123", 61000, "some-ip-2", 62004, 60),
			}

			expectedUnregistrationRequests = []apimodels.TcpRouteMapping{
				apimodels.NewTcpRouteMapping("456", 61001, "some-ip-3", 62005, 60),
				apimodels.NewTcpRouteMapping("456", 61001, "some-ip-4", 62006, 60),
				apimodels.NewTcpRouteMapping("789", 61002, "some-ip-3", 62005, 60),
				apimodels.NewTcpRouteMapping("789", 61002, "some-ip-4", 62006, 60),
			}
		})

		It("returns valid registration and unregistration mapping requests ", func() {
			registrationRequests, unregistrationRequests := routingEvents.ToMappingRequests(logger, ttl)
			Expect(registrationRequests).Should(HaveLen(len(expectedRegistrationRequests)))
			Expect(registrationRequests).Should(ConsistOf(expectedRegistrationRequests))
			Expect(unregistrationRequests).Should(HaveLen(len(expectedUnregistrationRequests)))
			Expect(unregistrationRequests).Should(ConsistOf(expectedUnregistrationRequests))
		})
	})

	Context("with no unregistration events", func() {
		BeforeEach(func() {
			routingEvents = event.RoutingEvents{
				event.RoutingEvent{
					EventType: event.RouteRegistrationEvent,
					Key:       routingKey1,
					Entry:     routableEndpoints1,
				},
				event.RoutingEvent{
					EventType: event.RouteRegistrationEvent,
					Key:       routingKey2,
					Entry:     routableEndpoints2,
				},
			}

			expectedRegistrationRequests = []apimodels.TcpRouteMapping{
				apimodels.NewTcpRouteMapping("123", 61000, "some-ip-1", 62003, ttl),
				apimodels.NewTcpRouteMapping("123", 61000, "some-ip-2", 62004, ttl),
				apimodels.NewTcpRouteMapping("456", 61001, "some-ip-3", 62005, ttl),
				apimodels.NewTcpRouteMapping("456", 61001, "some-ip-4", 62006, ttl),
				apimodels.NewTcpRouteMapping("789", 61002, "some-ip-3", 62005, ttl),
				apimodels.NewTcpRouteMapping("789", 61002, "some-ip-4", 62006, ttl),
			}

			expectedUnregistrationRequests = []apimodels.TcpRouteMapping{}
		})

		It("returns only registration mapping requests ", func() {
			registrationRequests, unregistrationRequests := routingEvents.ToMappingRequests(logger, ttl)
			Expect(registrationRequests).Should(HaveLen(len(expectedRegistrationRequests)))
			Expect(registrationRequests).Should(ConsistOf(expectedRegistrationRequests))
			Expect(unregistrationRequests).Should(HaveLen(0))
		})
	})

	Context("with no registration events", func() {
		BeforeEach(func() {
			routingEvents = event.RoutingEvents{
				event.RoutingEvent{
					EventType: event.RouteUnregistrationEvent,
					Key:       routingKey1,
					Entry:     routableEndpoints1,
				},
				event.RoutingEvent{
					EventType: event.RouteUnregistrationEvent,
					Key:       routingKey2,
					Entry:     routableEndpoints2,
				},
			}

			expectedUnregistrationRequests = []apimodels.TcpRouteMapping{
				apimodels.NewTcpRouteMapping("123", 61000, "some-ip-1", 62003, ttl),
				apimodels.NewTcpRouteMapping("123", 61000, "some-ip-2", 62004, ttl),
				apimodels.NewTcpRouteMapping("456", 61001, "some-ip-3", 62005, ttl),
				apimodels.NewTcpRouteMapping("456", 61001, "some-ip-4", 62006, ttl),
				apimodels.NewTcpRouteMapping("789", 61002, "some-ip-3", 62005, ttl),
				apimodels.NewTcpRouteMapping("789", 61002, "some-ip-4", 62006, ttl),
			}

			expectedRegistrationRequests = []apimodels.TcpRouteMapping{}
		})

		It("returns only unregistration mapping requests ", func() {
			registrationRequests, unregistrationRequests := routingEvents.ToMappingRequests(logger, ttl)
			Expect(unregistrationRequests).Should(HaveLen(len(expectedUnregistrationRequests)))
			Expect(unregistrationRequests).Should(ConsistOf(expectedUnregistrationRequests))
			Expect(registrationRequests).Should(HaveLen(0))
		})
	})

	Context("with an invalid external port in route registration event", func() {

		It("returns an empty registration request", func() {
			extenralEndpointInfo1 := endpoint.ExternalEndpointInfos{
				endpoint.NewExternalEndpointInfo("123", 0),
			}

			routableEndpoints1 := endpoint.NewRoutableEndpoints(
				extenralEndpointInfo1, endpoints1, logGuid, &modificationTag)

			routingEvents = event.RoutingEvents{
				event.RoutingEvent{
					EventType: event.RouteRegistrationEvent,
					Key:       routingKey1,
					Entry:     routableEndpoints1,
				},
			}
			registrationRequests, unregistrationRequests := routingEvents.ToMappingRequests(logger, ttl)
			Expect(unregistrationRequests).Should(HaveLen(0))
			Expect(registrationRequests).Should(HaveLen(0))
		})

		Context("and multiple external ports", func() {
			It("disregards the entire routing event", func() {
				extenralEndpointInfo1 := endpoint.NewExternalEndpointInfo("123", 0)
				extenralEndpointInfo2 := endpoint.NewExternalEndpointInfo("123", 61000)
				externalInfo := []endpoint.ExternalEndpointInfo{
					extenralEndpointInfo1,
					extenralEndpointInfo2,
				}

				routableEndpoints1 := endpoint.NewRoutableEndpoints(
					externalInfo, endpoints1, logGuid, &modificationTag)

				routingEvents = event.RoutingEvents{
					event.RoutingEvent{
						EventType: event.RouteRegistrationEvent,
						Key:       routingKey1,
						Entry:     routableEndpoints1,
					},
				}

				registrationRequests, unregistrationRequests := routingEvents.ToMappingRequests(logger, ttl)
				Expect(unregistrationRequests).Should(HaveLen(0))
				Expect(registrationRequests).Should(HaveLen(0))
			})
		})
	})

	Context("with empty endpoints in routing event", func() {
		It("returns an empty mapping request", func() {
			extenralEndpointInfo1 := endpoint.ExternalEndpointInfos{
				endpoint.NewExternalEndpointInfo("123", 0),
			}

			routableEndpoints1 := endpoint.NewRoutableEndpoints(
				extenralEndpointInfo1, nil, logGuid, &modificationTag)

			routingEvents = event.RoutingEvents{
				event.RoutingEvent{
					EventType: event.RouteRegistrationEvent,
					Key:       routingKey1,
					Entry:     routableEndpoints1,
				},
			}

			registrationRequests, unregistrationRequests := routingEvents.ToMappingRequests(logger, ttl)
			Expect(unregistrationRequests).Should(HaveLen(0))
			Expect(registrationRequests).Should(HaveLen(0))
		})
	})
})
