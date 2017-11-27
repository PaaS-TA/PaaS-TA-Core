package schema_test

import (
	"encoding/json"
	"time"

	"code.cloudfoundry.org/bbs/models"
	"code.cloudfoundry.org/routing-info/tcp_routes"
	"code.cloudfoundry.org/tcp-emitter/routing_table/schema"
	"code.cloudfoundry.org/tcp-emitter/routing_table/schema/endpoint"
	"code.cloudfoundry.org/tcp-emitter/routing_table/schema/event"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
)

const (
	DEFAULT_TIMEOUT          = 5 * time.Second
	DEFAULT_POLLING_INTERVAL = 5 * time.Millisecond
)

type testRoutingTable struct {
	entries map[endpoint.RoutingKey]endpoint.RoutableEndpoints
}

func (t *testRoutingTable) RouteCount() int {
	return 0
}

func (t *testRoutingTable) AddRoutes(desiredLRP *models.DesiredLRP) event.RoutingEvents {
	return event.RoutingEvents{}
}

func (t *testRoutingTable) UpdateRoutes(before, after *models.DesiredLRP) event.RoutingEvents {
	return event.RoutingEvents{}
}

func (t *testRoutingTable) RemoveRoutes(desiredLRP *models.DesiredLRP) event.RoutingEvents {
	return event.RoutingEvents{}
}

func (t *testRoutingTable) AddEndpoint(actualLRP *models.ActualLRPGroup) event.RoutingEvents {
	return event.RoutingEvents{}
}

func (t *testRoutingTable) RemoveEndpoint(actualLRP *models.ActualLRPGroup) event.RoutingEvents {
	return event.RoutingEvents{}
}

func (t *testRoutingTable) Swap(table schema.RoutingTable) event.RoutingEvents {
	return event.RoutingEvents{}
}

func (t *testRoutingTable) GetRoutingEvents() event.RoutingEvents {
	return event.RoutingEvents{}
}

var _ = Describe("RoutingTable", func() {

	var (
		routingTable    schema.RoutingTable
		modificationTag *models.ModificationTag
		tcpRoutes       tcp_routes.TCPRoutes
	)

	getDesiredLRP := func(processGuid, logGuid string,
		tcpRoutes tcp_routes.TCPRoutes, modificationTag *models.ModificationTag) *models.DesiredLRP {
		var desiredLRP models.DesiredLRP
		portMap := map[uint32]struct{}{}
		for _, tcpRoute := range tcpRoutes {
			portMap[tcpRoute.ContainerPort] = struct{}{}
		}

		ports := []uint32{}
		for k, _ := range portMap {
			ports = append(ports, k)
		}

		desiredLRP.ProcessGuid = processGuid
		desiredLRP.Ports = ports
		desiredLRP.LogGuid = logGuid
		desiredLRP.ModificationTag = modificationTag
		desiredLRP.Routes = tcpRoutes.RoutingInfo()

		// add 'diego-ssh' data for testing sanitize
		routingInfo := json.RawMessage([]byte(`{ "private_key": "fake-key" }`))
		(*desiredLRP.Routes)["diego-ssh"] = &routingInfo

		return &desiredLRP
	}

	getActualLRP := func(processGuid, instanceGuid, hostAddress string,
		hostPort, containerPort uint32, evacuating bool,
		modificationTag *models.ModificationTag) *models.ActualLRPGroup {
		if evacuating {
			return &models.ActualLRPGroup{
				Instance: nil,
				Evacuating: &models.ActualLRP{
					ActualLRPKey:         models.NewActualLRPKey(processGuid, 0, "domain"),
					ActualLRPInstanceKey: models.NewActualLRPInstanceKey(instanceGuid, "cell-id-1"),
					ActualLRPNetInfo: models.NewActualLRPNetInfo(
						hostAddress,
						"1.2.3.4",
						models.NewPortMapping(hostPort, containerPort),
					),
					State:           models.ActualLRPStateRunning,
					ModificationTag: *modificationTag,
				},
			}
		} else {
			return &models.ActualLRPGroup{
				Instance: &models.ActualLRP{
					ActualLRPKey:         models.NewActualLRPKey(processGuid, 0, "domain"),
					ActualLRPInstanceKey: models.NewActualLRPInstanceKey(instanceGuid, "cell-id-1"),
					ActualLRPNetInfo: models.NewActualLRPNetInfo(
						hostAddress,
						"1.2.3.4",
						models.NewPortMapping(hostPort, containerPort),
					),
					State:           models.ActualLRPStateRunning,
					ModificationTag: *modificationTag,
				},
				Evacuating: nil,
			}
		}
	}

	BeforeEach(func() {
		tcpRoutes = tcp_routes.TCPRoutes{
			tcp_routes.TCPRoute{
				ExternalPort:  61000,
				ContainerPort: 5222,
			},
		}
	})

	Context("when no entry exist for route", func() {
		BeforeEach(func() {
			routingTable = schema.NewTable(logger, nil)
			modificationTag = &models.ModificationTag{Epoch: "abc", Index: 0}
		})

		Describe("AddRoutes", func() {
			It("emits nothing", func() {
				desiredLRP := getDesiredLRP("process-guid-1", "log-guid-1", tcpRoutes, modificationTag)
				routingEvents := routingTable.AddRoutes(desiredLRP)
				Expect(routingEvents).To(HaveLen(0))
			})

			It("does not emit any sensitive information", func() {
				desiredLRP := getDesiredLRP("process-guid-1", "log-guid-1", tcpRoutes, modificationTag)
				routingEvents := routingTable.AddRoutes(desiredLRP)
				Consistently(logger).ShouldNot(gbytes.Say("private_key"))
				Expect(routingEvents).To(HaveLen(0))
			})

			It("logs required routing info", func() {
				desiredLRP := getDesiredLRP("process-guid-1", "log-guid-1", tcpRoutes, modificationTag)
				routingEvents := routingTable.AddRoutes(desiredLRP)
				for i := 0; i < 3; i++ {
					Eventually(logger, DEFAULT_TIMEOUT).Should(gbytes.Say("process-guid.*process-guid-1"))
					Eventually(logger, DEFAULT_TIMEOUT).Should(gbytes.Say("routes.*tcp-router.*61000.*5222"))
				}

				Expect(routingEvents).To(HaveLen(0))
			})
		})

		Describe("UpdateRoutes", func() {
			It("emits nothing", func() {
				beforeLRP := getDesiredLRP("process-guid-1", "log-guid-1", tcpRoutes, modificationTag)
				newModificationTag := &models.ModificationTag{Epoch: "abc", Index: 1}
				afterLRP := getDesiredLRP("process-guid-1", "log-guid-1", tcpRoutes, newModificationTag)
				routingEvents := routingTable.UpdateRoutes(beforeLRP, afterLRP)
				Expect(routingEvents).To(HaveLen(0))
			})

			It("does not log sensitive info", func() {
				beforeLRP := getDesiredLRP("process-guid-1", "log-guid-1", tcpRoutes, modificationTag)
				newModificationTag := &models.ModificationTag{Epoch: "abc", Index: 1}
				afterLRP := getDesiredLRP("process-guid-1", "log-guid-1", tcpRoutes, newModificationTag)
				routingEvents := routingTable.UpdateRoutes(beforeLRP, afterLRP)
				Consistently(logger).ShouldNot(gbytes.Say("private_key"))
				Expect(routingEvents).To(HaveLen(0))
			})

			It("logs required routing info", func() {
				beforeLRP := getDesiredLRP("process-guid-1", "log-guid-1", tcpRoutes, modificationTag)
				newModificationTag := &models.ModificationTag{Epoch: "abc", Index: 1}
				afterLRP := getDesiredLRP("process-guid-1", "log-guid-1", tcpRoutes, newModificationTag)
				routingEvents := routingTable.UpdateRoutes(beforeLRP, afterLRP)
				for i := 0; i < 3; i++ {
					Eventually(logger, DEFAULT_TIMEOUT).Should(gbytes.Say("after_lrp.*process-guid.*process-guid-1.*routes.*tcp-router.*external_port.*61000.*container_port.*5222"))
				}
				Expect(routingEvents).To(HaveLen(0))
			})
		})

		Describe("RemoveRoutes", func() {
			It("emits nothing", func() {
				desiredLRP := getDesiredLRP("process-guid-10", "log-guid-10", tcpRoutes, modificationTag)
				routingEvents := routingTable.RemoveRoutes(desiredLRP)
				Expect(routingEvents).To(HaveLen(0))
			})

			It("does not log sensitive info", func() {
				desiredLRP := getDesiredLRP("process-guid-10", "log-guid-10", tcpRoutes, modificationTag)
				routingEvents := routingTable.RemoveRoutes(desiredLRP)
				Consistently(logger).ShouldNot(gbytes.Say("private_key"))
				Expect(routingEvents).To(HaveLen(0))
			})

			It("logs required routing info", func() {
				desiredLRP := getDesiredLRP("process-guid-10", "log-guid-10", tcpRoutes, modificationTag)
				routingEvents := routingTable.RemoveRoutes(desiredLRP)
				Eventually(logger, DEFAULT_TIMEOUT, DEFAULT_POLLING_INTERVAL).Should(gbytes.Say("starting.*process-guid-10.*external_port.*61000.*container_port.*5222"))
				Eventually(logger, DEFAULT_TIMEOUT, DEFAULT_POLLING_INTERVAL).Should(gbytes.Say("completed.*process-guid-10.*external_port.*61000.*container_port.*5222"))
				Expect(routingEvents).To(HaveLen(0))
			})
		})

		Describe("AddEndpoint", func() {
			It("emits nothing", func() {
				actualLRP := getActualLRP("process-guid-1", "instance-guid-1", "some-ip-1", 61104, 5222, false, modificationTag)
				routingEvents := routingTable.AddEndpoint(actualLRP)
				Expect(routingEvents).To(HaveLen(0))
			})

			It("does not log sensitive info", func() {
				actualLRP := getActualLRP("process-guid-1", "instance-guid-1", "some-ip-1", 61104, 5222, false, modificationTag)
				routingEvents := routingTable.AddEndpoint(actualLRP)
				Expect(routingEvents).To(HaveLen(0))
				Consistently(logger).ShouldNot(gbytes.Say("private_key"))
			})

			It("logs required routing info", func() {
				actualLRP := getActualLRP("process-guid-1", "instance-guid-1", "some-ip-1", 61104, 5222, false, modificationTag)
				routingEvents := routingTable.AddEndpoint(actualLRP)
				Expect(routingEvents).To(HaveLen(0))
				Eventually(logger, DEFAULT_TIMEOUT, DEFAULT_POLLING_INTERVAL).Should(gbytes.Say("process_guid.*process-guid-1"))
				Eventually(logger, DEFAULT_TIMEOUT, DEFAULT_POLLING_INTERVAL).Should(gbytes.Say("ports.*5222.*61104"))
			})
		})

		Describe("RemoveEndpoint", func() {
			It("emits nothing", func() {
				actualLRP := getActualLRP("process-guid-1", "instance-guid-1", "some-ip-1", 61104, 5222, false, modificationTag)
				routingEvents := routingTable.RemoveEndpoint(actualLRP)
				Expect(routingEvents).To(HaveLen(0))
			})

			It("does not log sensitive info", func() {
				actualLRP := getActualLRP("process-guid-1", "instance-guid-1", "some-ip-1", 61104, 5222, false, modificationTag)
				routingEvents := routingTable.RemoveEndpoint(actualLRP)
				Expect(routingEvents).To(HaveLen(0))
				Consistently(logger).ShouldNot(gbytes.Say("private_key"))
			})

			It("logs required routing info", func() {
				actualLRP := getActualLRP("process-guid-1", "instance-guid-1", "some-ip-1", 61104, 5222, false, modificationTag)
				routingEvents := routingTable.RemoveEndpoint(actualLRP)
				Expect(routingEvents).To(HaveLen(0))
				Eventually(logger, DEFAULT_TIMEOUT, DEFAULT_POLLING_INTERVAL).Should(gbytes.Say("starting.*process-guid-1.*ports.*5222.*61104"))
				Eventually(logger, DEFAULT_TIMEOUT, DEFAULT_POLLING_INTERVAL).Should(gbytes.Say("completed.*process-guid-1.*ports.*5222.*61104"))
			})
		})

		Describe("Swap", func() {
			var (
				tempRoutingTable schema.RoutingTable
				key              endpoint.RoutingKey
				endpoints        map[endpoint.EndpointKey]endpoint.Endpoint
				modificationTag  *models.ModificationTag
				logGuid          string
			)

			BeforeEach(func() {
				logGuid = "log-guid-1"
				externalEndpoints := endpoint.ExternalEndpointInfos{
					endpoint.NewExternalEndpointInfo("router-group-guid", 61000),
				}
				key = endpoint.NewRoutingKey("process-guid-1", 5222)
				modificationTag = &models.ModificationTag{Epoch: "abc", Index: 1}
				endpoints = map[endpoint.EndpointKey]endpoint.Endpoint{
					endpoint.NewEndpointKey("instance-guid-1", false): endpoint.NewEndpoint(
						"instance-guid-1", false, "some-ip-1", 62004, 5222, modificationTag),
					endpoint.NewEndpointKey("instance-guid-2", false): endpoint.NewEndpoint(
						"instance-guid-2", false, "some-ip-2", 62004, 5222, modificationTag),
				}

				tempRoutingTable = schema.NewTable(logger, map[endpoint.RoutingKey]endpoint.RoutableEndpoints{
					key: endpoint.NewRoutableEndpoints(externalEndpoints, endpoints, logGuid, modificationTag),
				})
			})

			It("emits routing events for new routes", func() {
				Expect(routingTable.RouteCount()).Should(Equal(0))
				routingEvents := routingTable.Swap(tempRoutingTable)
				Expect(routingTable.RouteCount()).Should(Equal(1))
				Expect(routingEvents).To(HaveLen(1))
				routingEvent := routingEvents[0]
				Expect(routingEvent.Key).Should(Equal(key))
				Expect(routingEvent.EventType).Should(Equal(event.RouteRegistrationEvent))
				externalInfo := endpoint.ExternalEndpointInfos{
					endpoint.NewExternalEndpointInfo("router-group-guid", 61000),
				}
				expectedEntry := endpoint.NewRoutableEndpoints(
					externalInfo, endpoints, logGuid, modificationTag)
				Expect(routingEvent.Entry).Should(Equal(expectedEntry))
			})
		})

		Describe("GetRoutingEvents", func() {
			It("returns empty routing events", func() {
				routingEvents := routingTable.GetRoutingEvents()
				Expect(routingEvents).To(HaveLen(0))
			})
		})
	})

	Context("when there exists an entry for route", func() {
		var (
			endpoints         map[endpoint.EndpointKey]endpoint.Endpoint
			key               endpoint.RoutingKey
			logGuid           string
			externalEndpoints endpoint.ExternalEndpointInfos
		)

		BeforeEach(func() {
			logGuid = "log-guid-1"
			externalEndpoints = endpoint.ExternalEndpointInfos{
				endpoint.NewExternalEndpointInfo("router-group-guid", 61000),
			}
			key = endpoint.NewRoutingKey("process-guid-1", 5222)
			modificationTag = &models.ModificationTag{Epoch: "abc", Index: 1}
			endpoints = map[endpoint.EndpointKey]endpoint.Endpoint{
				endpoint.NewEndpointKey("instance-guid-1", false): endpoint.NewEndpoint(
					"instance-guid-1", false, "some-ip-1", 62004, 5222, modificationTag),
				endpoint.NewEndpointKey("instance-guid-2", false): endpoint.NewEndpoint(
					"instance-guid-2", false, "some-ip-2", 62004, 5222, modificationTag),
			}
		})

		Describe("AddRoutes", func() {
			BeforeEach(func() {
				routingTable = schema.NewTable(logger, map[endpoint.RoutingKey]endpoint.RoutableEndpoints{
					key: endpoint.NewRoutableEndpoints(externalEndpoints, endpoints, logGuid, modificationTag),
				})
				Expect(routingTable.RouteCount()).Should(Equal(1))
			})

			Context("existing external port changes", func() {
				var (
					newTcpRoutes tcp_routes.TCPRoutes
				)
				BeforeEach(func() {
					newTcpRoutes = tcp_routes.TCPRoutes{
						tcp_routes.TCPRoute{
							RouterGroupGuid: "router-group-guid",
							ExternalPort:    61001,
							ContainerPort:   5222,
						},
					}
				})

				It("emits routing event with modified external port", func() {
					newModificationTag := &models.ModificationTag{Epoch: "abc", Index: 2}
					desiredLRP := getDesiredLRP("process-guid-1", "log-guid-1", newTcpRoutes, newModificationTag)
					routingEvents := routingTable.AddRoutes(desiredLRP)
					Expect(routingEvents).To(HaveLen(2))

					routingEvent := routingEvents[1]
					Expect(routingEvent.Key).Should(Equal(key))
					Expect(routingEvent.EventType).Should(Equal(event.RouteUnregistrationEvent))
					unregistrationExpectedEntry := endpoint.NewRoutableEndpoints(
						externalEndpoints, endpoints, logGuid, modificationTag)
					Expect(routingEvent.Entry).Should(Equal(unregistrationExpectedEntry))

					externalInfo := []endpoint.ExternalEndpointInfo{
						endpoint.NewExternalEndpointInfo("router-group-guid", 61001),
					}
					routingEvent = routingEvents[0]
					Expect(routingEvent.Key).Should(Equal(key))
					Expect(routingEvent.EventType).Should(Equal(event.RouteRegistrationEvent))
					registrationExpectedEntry := endpoint.NewRoutableEndpoints(
						externalInfo, endpoints, logGuid, newModificationTag)
					Expect(routingEvent.Entry).Should(Equal(registrationExpectedEntry))

					Expect(routingTable.RouteCount()).Should(Equal(1))
				})

				Context("older modification tag", func() {
					It("emits nothing", func() {
						desiredLRP := getDesiredLRP("process-guid-1", "log-guid-1", tcpRoutes, modificationTag)
						routingEvents := routingTable.AddRoutes(desiredLRP)
						Expect(routingEvents).To(HaveLen(0))
						Expect(routingTable.RouteCount()).Should(Equal(1))
					})
				})
			})

			Context("new external port is added", func() {
				BeforeEach(func() {
					tcpRoutes = tcp_routes.TCPRoutes{
						tcp_routes.TCPRoute{
							RouterGroupGuid: "router-group-guid",
							ExternalPort:    61000,
							ContainerPort:   5222,
						},
						tcp_routes.TCPRoute{
							RouterGroupGuid: "router-group-guid",
							ExternalPort:    61001,
							ContainerPort:   5222,
						},
						tcp_routes.TCPRoute{
							RouterGroupGuid: "router-group-guid",
							ExternalPort:    61002,
							ContainerPort:   5222,
						},
					}
				})

				It("emits routing event with both external ports", func() {
					modificationTag = &models.ModificationTag{Epoch: "abc", Index: 2}
					desiredLRP := getDesiredLRP("process-guid-1", "log-guid-1", tcpRoutes, modificationTag)
					routingEvents := routingTable.AddRoutes(desiredLRP)
					Expect(routingEvents).To(HaveLen(1))
					routingEvent := routingEvents[0]
					Expect(routingEvent.Key).Should(Equal(key))
					Expect(routingEvent.EventType).Should(Equal(event.RouteRegistrationEvent))
					externalInfo := []endpoint.ExternalEndpointInfo{
						endpoint.NewExternalEndpointInfo("router-group-guid", 61000),
						endpoint.NewExternalEndpointInfo("router-group-guid", 61001),
						endpoint.NewExternalEndpointInfo("router-group-guid", 61002),
					}
					expectedEntry := endpoint.NewRoutableEndpoints(
						externalInfo, endpoints, logGuid, modificationTag)
					Expect(routingEvent.Entry).Should(Equal(expectedEntry))
					Expect(routingTable.RouteCount()).Should(Equal(1))
				})

				Context("older modification tag", func() {
					It("emits nothing", func() {
						desiredLRP := getDesiredLRP("process-guid-1", "log-guid-1", tcpRoutes, modificationTag)
						routingEvents := routingTable.AddRoutes(desiredLRP)
						Expect(routingEvents).To(HaveLen(0))
						Expect(routingTable.RouteCount()).Should(Equal(1))
					})
				})
			})

			Context("multiple external port added and multiple existing external ports deleted", func() {
				var (
					newTcpRoutes tcp_routes.TCPRoutes
				)
				BeforeEach(func() {
					externalEndpoints = endpoint.ExternalEndpointInfos{
						endpoint.NewExternalEndpointInfo("router-group-guid", 61000),
						endpoint.NewExternalEndpointInfo("router-group-guid", 61001),
					}
					routingTable = schema.NewTable(logger, map[endpoint.RoutingKey]endpoint.RoutableEndpoints{
						key: endpoint.NewRoutableEndpoints(externalEndpoints, endpoints, logGuid, modificationTag),
					})
					Expect(routingTable.RouteCount()).Should(Equal(1))

					newTcpRoutes = tcp_routes.TCPRoutes{
						tcp_routes.TCPRoute{
							RouterGroupGuid: "router-group-guid",
							ExternalPort:    61002,
							ContainerPort:   5222,
						},
						tcp_routes.TCPRoute{
							RouterGroupGuid: "router-group-guid",
							ExternalPort:    61003,
							ContainerPort:   5222,
						},
					}
				})

				It("emits routing event with both external ports", func() {
					newModificationTag := &models.ModificationTag{Epoch: "abc", Index: 2}
					desiredLRP := getDesiredLRP("process-guid-1", "log-guid-1", newTcpRoutes, newModificationTag)
					routingEvents := routingTable.AddRoutes(desiredLRP)
					Expect(routingEvents).To(HaveLen(2))

					routingEvent := routingEvents[1]
					Expect(routingEvent.Key).Should(Equal(key))
					Expect(routingEvent.EventType).Should(Equal(event.RouteUnregistrationEvent))
					unregistrationExpectedEntry := endpoint.NewRoutableEndpoints(
						externalEndpoints, endpoints, logGuid, modificationTag)
					Expect(routingEvent.Entry).Should(Equal(unregistrationExpectedEntry))

					externalInfo := []endpoint.ExternalEndpointInfo{
						endpoint.NewExternalEndpointInfo("router-group-guid", 61002),
						endpoint.NewExternalEndpointInfo("router-group-guid", 61003),
					}
					routingEvent = routingEvents[0]
					Expect(routingEvent.Key).Should(Equal(key))
					Expect(routingEvent.EventType).Should(Equal(event.RouteRegistrationEvent))
					registrationExpectedEntry := endpoint.NewRoutableEndpoints(
						externalInfo, endpoints, logGuid, newModificationTag)
					Expect(routingEvent.Entry).Should(Equal(registrationExpectedEntry))

					Expect(routingTable.RouteCount()).Should(Equal(1))
				})

				Context("older modification tag", func() {
					It("emits nothing", func() {
						desiredLRP := getDesiredLRP("process-guid-1", "log-guid-1", tcpRoutes, modificationTag)
						routingEvents := routingTable.AddRoutes(desiredLRP)
						Expect(routingEvents).To(HaveLen(0))
						Expect(routingTable.RouteCount()).Should(Equal(1))
					})
				})
			})

			Context("no changes to external port", func() {
				It("emits nothing", func() {
					tag := &models.ModificationTag{Epoch: "abc", Index: 2}
					desiredLRP := getDesiredLRP("process-guid-1", "log-guid-1", tcpRoutes, tag)
					routingEvents := routingTable.AddRoutes(desiredLRP)
					Expect(routingEvents).To(HaveLen(0))
					Expect(routingTable.RouteCount()).Should(Equal(1))
				})
			})

			Context("when two disjoint (external port, container port) pairs are given", func() {
				var endpoints2 map[endpoint.EndpointKey]endpoint.Endpoint
				var key2 endpoint.RoutingKey

				createdExpectedEvents := func(newModificationTag *models.ModificationTag) []event.RoutingEvent {
					externalInfo1 := []endpoint.ExternalEndpointInfo{
						endpoint.NewExternalEndpointInfo("router-group-guid", 61001),
					}
					expectedEntry1 := endpoint.NewRoutableEndpoints(
						externalInfo1, endpoints, logGuid, newModificationTag)

					externalInfo2 := []endpoint.ExternalEndpointInfo{
						endpoint.NewExternalEndpointInfo("router-group-guid", 61002),
					}
					expectedEntry2 := endpoint.NewRoutableEndpoints(
						externalInfo2, endpoints2, logGuid, newModificationTag)

					externalInfo3 := []endpoint.ExternalEndpointInfo{
						endpoint.NewExternalEndpointInfo("router-group-guid", 61000),
					}
					expectedEntry3 := endpoint.NewRoutableEndpoints(
						externalInfo3, endpoints, logGuid, modificationTag)

					return []event.RoutingEvent{
						event.RoutingEvent{
							EventType: event.RouteRegistrationEvent,
							Key:       key2,
							Entry:     expectedEntry2,
						}, event.RoutingEvent{
							EventType: event.RouteRegistrationEvent,
							Key:       key,
							Entry:     expectedEntry1,
						}, event.RoutingEvent{
							EventType: event.RouteUnregistrationEvent,
							Key:       key,
							Entry:     expectedEntry3,
						},
					}
				}

				BeforeEach(func() {
					key2 = endpoint.NewRoutingKey("process-guid-1", 5223)
					endpoints2 = map[endpoint.EndpointKey]endpoint.Endpoint{
						endpoint.NewEndpointKey("instance-guid-1", false): endpoint.NewEndpoint(
							"instance-guid-1", false, "some-ip-1", 63004, 5223, modificationTag),
					}
					routingTable = schema.NewTable(logger, map[endpoint.RoutingKey]endpoint.RoutableEndpoints{
						key:  endpoint.NewRoutableEndpoints(externalEndpoints, endpoints, logGuid, modificationTag),
						key2: endpoint.NewRoutableEndpoints(nil, endpoints2, logGuid, modificationTag),
					})
					Expect(routingTable.RouteCount()).Should(Equal(2))

					tcpRoutes = tcp_routes.TCPRoutes{
						tcp_routes.TCPRoute{
							RouterGroupGuid: "router-group-guid",
							ExternalPort:    61001,
							ContainerPort:   5222,
						},
						tcp_routes.TCPRoute{
							RouterGroupGuid: "router-group-guid",
							ExternalPort:    61002,
							ContainerPort:   5223,
						},
					}
				})

				It("emits two separate registration events with no overlap", func() {
					newModificationTag := &models.ModificationTag{Epoch: "abc", Index: 2}
					desiredLRP := getDesiredLRP("process-guid-1", "log-guid-1", tcpRoutes, newModificationTag)
					routingEvents := routingTable.AddRoutes(desiredLRP)

					expectedEvents := createdExpectedEvents(newModificationTag)

					// Two registration and one unregistration events
					Expect(routingEvents).To(HaveLen(3))
					Expect(routingEvents).To(ConsistOf(expectedEvents))
					Expect(routingTable.RouteCount()).Should(Equal(2))
				})
			})

			Context("when container ports don't match", func() {
				BeforeEach(func() {
					tcpRoutes = tcp_routes.TCPRoutes{
						tcp_routes.TCPRoute{
							ExternalPort:  61000,
							ContainerPort: 5223,
						},
					}
				})

				It("emits nothing", func() {
					newTag := &models.ModificationTag{Epoch: "abc", Index: 2}
					desiredLRP := getDesiredLRP("process-guid-1", "log-guid-1", tcpRoutes, newTag)
					routingEvents := routingTable.AddRoutes(desiredLRP)
					Expect(routingEvents).To(HaveLen(0))
					Expect(routingTable.RouteCount()).Should(Equal(2))
				})
			})
		})

		Describe("UpdateRoutes", func() {
			var (
				oldTcpRoutes       tcp_routes.TCPRoutes
				newTcpRoutes       tcp_routes.TCPRoutes
				newModificationTag *models.ModificationTag
			)

			BeforeEach(func() {
				newModificationTag = &models.ModificationTag{Epoch: "abc", Index: 2}
				routingTable = schema.NewTable(logger, map[endpoint.RoutingKey]endpoint.RoutableEndpoints{
					key: endpoint.NewRoutableEndpoints(externalEndpoints, endpoints, logGuid, modificationTag),
				})
				Expect(routingTable.RouteCount()).Should(Equal(1))
				oldTcpRoutes = tcp_routes.TCPRoutes{
					tcp_routes.TCPRoute{
						RouterGroupGuid: "router-group-guid",
						ExternalPort:    61000,
						ContainerPort:   5222,
					},
				}
			})

			Context("when there is no change in container ports", func() {
				BeforeEach(func() {
					newTcpRoutes = tcp_routes.TCPRoutes{
						tcp_routes.TCPRoute{
							RouterGroupGuid: "router-group-guid",
							ExternalPort:    61001,
							ContainerPort:   5222,
						},
					}
				})

				Context("when there is change in external port", func() {
					It("emits registration and unregistration events", func() {
						beforeLRP := getDesiredLRP("process-guid-1", "log-guid-1", oldTcpRoutes, modificationTag)
						afterLRP := getDesiredLRP("process-guid-1", "log-guid-1", newTcpRoutes, newModificationTag)
						routingEvents := routingTable.UpdateRoutes(beforeLRP, afterLRP)
						Expect(routingEvents).To(HaveLen(2))

						externalInfo := []endpoint.ExternalEndpointInfo{
							endpoint.NewExternalEndpointInfo("router-group-guid", 61001),
						}
						routingEvent := routingEvents[0]
						Expect(routingEvent.Key).Should(Equal(key))
						Expect(routingEvent.EventType).Should(Equal(event.RouteRegistrationEvent))
						registrationExpectedEntry := endpoint.NewRoutableEndpoints(
							externalInfo, endpoints, logGuid, newModificationTag)
						Expect(routingEvent.Entry).Should(Equal(registrationExpectedEntry))

						routingEvent = routingEvents[1]
						Expect(routingEvent.Key).Should(Equal(key))
						Expect(routingEvent.EventType).Should(Equal(event.RouteUnregistrationEvent))
						unregistrationExpectedEntry := endpoint.NewRoutableEndpoints(
							externalEndpoints, endpoints, logGuid, modificationTag)
						Expect(routingEvent.Entry).Should(Equal(unregistrationExpectedEntry))

						Expect(routingTable.RouteCount()).Should(Equal(1))
					})

					Context("with older modification tag", func() {
						It("emits nothing", func() {
							beforeLRP := getDesiredLRP("process-guid-1", "log-guid-1", oldTcpRoutes, modificationTag)
							afterLRP := getDesiredLRP("process-guid-1", "log-guid-1", newTcpRoutes, modificationTag)
							routingEvents := routingTable.UpdateRoutes(beforeLRP, afterLRP)
							Expect(routingEvents).To(HaveLen(0))
							Expect(routingTable.RouteCount()).Should(Equal(1))
						})
					})
				})

				Context("when there is no change in external port", func() {
					It("emits nothing", func() {
						beforeLRP := getDesiredLRP("process-guid-1", "log-guid-1", oldTcpRoutes, modificationTag)
						afterLRP := getDesiredLRP("process-guid-1", "log-guid-1", oldTcpRoutes, newModificationTag)
						routingEvents := routingTable.UpdateRoutes(beforeLRP, afterLRP)
						Expect(routingEvents).To(HaveLen(0))
						Expect(routingTable.RouteCount()).Should(Equal(1))
					})
				})
			})

			Context("when new container port is added", func() {
				Context("when mapped to new external port", func() {
					BeforeEach(func() {
						newTcpRoutes = tcp_routes.TCPRoutes{
							tcp_routes.TCPRoute{
								RouterGroupGuid: "router-group-guid",
								ExternalPort:    61000,
								ContainerPort:   5222,
							},
							tcp_routes.TCPRoute{
								RouterGroupGuid: "router-group-guid",
								ExternalPort:    61001,
								ContainerPort:   5223,
							},
						}
					})

					Context("no backends for new container port", func() {
						It("emits no routing events and adds to routing table entry", func() {
							beforeLRP := getDesiredLRP("process-guid-1", "log-guid-1", oldTcpRoutes, modificationTag)
							afterLRP := getDesiredLRP("process-guid-1", "log-guid-1", newTcpRoutes, newModificationTag)
							routingEvents := routingTable.UpdateRoutes(beforeLRP, afterLRP)
							Expect(routingEvents).To(HaveLen(0))
							Expect(routingTable.RouteCount()).Should(Equal(2))
						})

						Context("with older modification tag", func() {
							It("emits nothing but add the routing table entry", func() {
								beforeLRP := getDesiredLRP("process-guid-1", "log-guid-1", oldTcpRoutes, modificationTag)
								afterLRP := getDesiredLRP("process-guid-1", "log-guid-1", newTcpRoutes, modificationTag)
								currentRoutesCount := routingTable.RouteCount()
								routingEvents := routingTable.UpdateRoutes(beforeLRP, afterLRP)
								Expect(routingEvents).To(HaveLen(0))
								Expect(routingTable.RouteCount()).Should(Equal(currentRoutesCount + 1))
							})
						})
					})

					Context("existing backends for new container port", func() {
						var (
							newKey       endpoint.RoutingKey
							newEndpoints map[endpoint.EndpointKey]endpoint.Endpoint
						)
						BeforeEach(func() {
							newKey = endpoint.NewRoutingKey("process-guid-1", 5223)
							newEndpoints = map[endpoint.EndpointKey]endpoint.Endpoint{
								endpoint.NewEndpointKey("instance-guid-1", false): endpoint.NewEndpoint(
									"instance-guid-1", false, "some-ip-1", 62006, 5223, modificationTag),
							}
							routingTable = schema.NewTable(logger, map[endpoint.RoutingKey]endpoint.RoutableEndpoints{
								key:    endpoint.NewRoutableEndpoints(externalEndpoints, endpoints, logGuid, modificationTag),
								newKey: endpoint.NewRoutableEndpoints(endpoint.ExternalEndpointInfos{}, newEndpoints, logGuid, modificationTag),
							})
						})

						It("emits registration events for new container port", func() {
							beforeLRP := getDesiredLRP("process-guid-1", "log-guid-1", oldTcpRoutes, modificationTag)
							afterLRP := getDesiredLRP("process-guid-1", "log-guid-1", newTcpRoutes, newModificationTag)
							currentRoutesCount := routingTable.RouteCount()
							routingEvents := routingTable.UpdateRoutes(beforeLRP, afterLRP)
							Expect(routingEvents).To(HaveLen(1))

							externalInfo := []endpoint.ExternalEndpointInfo{
								endpoint.NewExternalEndpointInfo("router-group-guid", 61001),
							}
							routingEvent := routingEvents[0]
							Expect(routingEvent.Key).Should(Equal(newKey))
							Expect(routingEvent.EventType).Should(Equal(event.RouteRegistrationEvent))
							registrationExpectedEntry := endpoint.NewRoutableEndpoints(
								externalInfo, newEndpoints, logGuid, newModificationTag)
							Expect(routingEvent.Entry).Should(Equal(registrationExpectedEntry))

							Expect(routingTable.RouteCount()).Should(Equal(currentRoutesCount))
						})

						Context("with older modification tag", func() {
							It("emits nothing", func() {
								beforeLRP := getDesiredLRP("process-guid-1", "log-guid-1", oldTcpRoutes, modificationTag)
								afterLRP := getDesiredLRP("process-guid-1", "log-guid-1", newTcpRoutes, modificationTag)
								currentRoutesCount := routingTable.RouteCount()
								routingEvents := routingTable.UpdateRoutes(beforeLRP, afterLRP)
								Expect(routingEvents).To(HaveLen(0))
								Expect(routingTable.RouteCount()).Should(Equal(currentRoutesCount))
							})
						})
					})

				})

				Context("when mapped to existing external port", func() {
					BeforeEach(func() {
						newTcpRoutes = tcp_routes.TCPRoutes{
							tcp_routes.TCPRoute{
								RouterGroupGuid: "router-group-guid",
								ExternalPort:    61000,
								ContainerPort:   5222,
							},
							tcp_routes.TCPRoute{
								RouterGroupGuid: "router-group-guid",
								ExternalPort:    61000,
								ContainerPort:   5223,
							},
						}
					})

					Context("no backends for new container port", func() {
						It("emits no routing events and adds to routing table entry", func() {
							beforeLRP := getDesiredLRP("process-guid-1", "log-guid-1", oldTcpRoutes, modificationTag)
							afterLRP := getDesiredLRP("process-guid-1", "log-guid-1", newTcpRoutes, newModificationTag)
							currentRoutesCount := routingTable.RouteCount()
							routingEvents := routingTable.UpdateRoutes(beforeLRP, afterLRP)
							Expect(routingEvents).To(HaveLen(0))
							Expect(routingTable.RouteCount()).Should(Equal(currentRoutesCount + 1))
						})

						Context("with older modification tag", func() {
							It("emits nothing but add the routing table entry", func() {
								beforeLRP := getDesiredLRP("process-guid-1", "log-guid-1", oldTcpRoutes, modificationTag)
								afterLRP := getDesiredLRP("process-guid-1", "log-guid-1", newTcpRoutes, modificationTag)
								currentRoutesCount := routingTable.RouteCount()
								routingEvents := routingTable.UpdateRoutes(beforeLRP, afterLRP)
								Expect(routingEvents).To(HaveLen(0))
								Expect(routingTable.RouteCount()).Should(Equal(currentRoutesCount + 1))
							})
						})
					})

					Context("existing backends for new container port", func() {
						var (
							newKey       endpoint.RoutingKey
							newEndpoints map[endpoint.EndpointKey]endpoint.Endpoint
						)
						BeforeEach(func() {
							newKey = endpoint.NewRoutingKey("process-guid-1", 5223)
							newEndpoints = map[endpoint.EndpointKey]endpoint.Endpoint{
								endpoint.NewEndpointKey("instance-guid-1", false): endpoint.NewEndpoint(
									"instance-guid-1", false, "some-ip-1", 62006, 5223, modificationTag),
							}
							routingTable = schema.NewTable(logger, map[endpoint.RoutingKey]endpoint.RoutableEndpoints{
								key:    endpoint.NewRoutableEndpoints(externalEndpoints, endpoints, logGuid, modificationTag),
								newKey: endpoint.NewRoutableEndpoints(endpoint.ExternalEndpointInfos{}, newEndpoints, logGuid, modificationTag),
							})
						})

						It("emits registration events for new container port", func() {
							beforeLRP := getDesiredLRP("process-guid-1", "log-guid-1", oldTcpRoutes, modificationTag)
							afterLRP := getDesiredLRP("process-guid-1", "log-guid-1", newTcpRoutes, newModificationTag)
							currentRoutesCount := routingTable.RouteCount()
							routingEvents := routingTable.UpdateRoutes(beforeLRP, afterLRP)
							Expect(routingEvents).To(HaveLen(1))

							externalInfo := []endpoint.ExternalEndpointInfo{
								endpoint.NewExternalEndpointInfo("router-group-guid", 61000),
							}
							routingEvent := routingEvents[0]
							Expect(routingEvent.Key).Should(Equal(newKey))
							Expect(routingEvent.EventType).Should(Equal(event.RouteRegistrationEvent))
							registrationExpectedEntry := endpoint.NewRoutableEndpoints(
								externalInfo, newEndpoints, logGuid, newModificationTag)
							Expect(routingEvent.Entry).Should(Equal(registrationExpectedEntry))

							Expect(routingTable.RouteCount()).Should(Equal(currentRoutesCount))
						})

						Context("with older modification tag", func() {
							It("emits nothing", func() {
								beforeLRP := getDesiredLRP("process-guid-1", "log-guid-1", oldTcpRoutes, modificationTag)
								afterLRP := getDesiredLRP("process-guid-1", "log-guid-1", newTcpRoutes, modificationTag)
								currentRoutesCount := routingTable.RouteCount()
								routingEvents := routingTable.UpdateRoutes(beforeLRP, afterLRP)
								Expect(routingEvents).To(HaveLen(0))
								Expect(routingTable.RouteCount()).Should(Equal(currentRoutesCount))
							})
						})
					})
				})
			})

			Context("when existing container port is removed", func() {

				Context("when there are no routes left", func() {
					BeforeEach(func() {
						newTcpRoutes = tcp_routes.TCPRoutes{}
					})
					It("emits only unregistration events", func() {
						beforeLRP := getDesiredLRP("process-guid-1", "log-guid-1", oldTcpRoutes, modificationTag)
						afterLRP := getDesiredLRP("process-guid-1", "log-guid-1", newTcpRoutes, newModificationTag)
						currentRoutesCount := routingTable.RouteCount()
						routingEvents := routingTable.UpdateRoutes(beforeLRP, afterLRP)
						Expect(routingEvents).To(HaveLen(1))

						routingEvent := routingEvents[0]
						Expect(routingEvent.Key).Should(Equal(key))
						Expect(routingEvent.EventType).Should(Equal(event.RouteUnregistrationEvent))
						unregistrationExpectedEntry := endpoint.NewRoutableEndpoints(
							externalEndpoints, endpoints, logGuid, modificationTag)
						Expect(routingEvent.Entry).Should(Equal(unregistrationExpectedEntry))
						Expect(routingTable.RouteCount()).Should(Equal(currentRoutesCount - 1))
					})

					Context("with older modification tag", func() {
						It("emits nothing", func() {
							beforeLRP := getDesiredLRP("process-guid-1", "log-guid-1", oldTcpRoutes, modificationTag)
							afterLRP := getDesiredLRP("process-guid-1", "log-guid-1", newTcpRoutes, modificationTag)
							currentRoutesCount := routingTable.RouteCount()
							routingEvents := routingTable.UpdateRoutes(beforeLRP, afterLRP)
							Expect(routingEvents).To(HaveLen(0))
							Expect(routingTable.RouteCount()).Should(Equal(currentRoutesCount))
						})
					})
				})

				Context("when container port is switched", func() {
					BeforeEach(func() {
						newTcpRoutes = tcp_routes.TCPRoutes{
							tcp_routes.TCPRoute{
								RouterGroupGuid: "router-group-guid",
								ExternalPort:    61000,
								ContainerPort:   5223,
							},
						}
					})

					Context("no backends for new container port", func() {
						It("emits no routing events and adds to routing table entry", func() {
							beforeLRP := getDesiredLRP("process-guid-1", "log-guid-1", oldTcpRoutes, modificationTag)
							afterLRP := getDesiredLRP("process-guid-1", "log-guid-1", newTcpRoutes, newModificationTag)
							currentRoutesCount := routingTable.RouteCount()
							routingEvents := routingTable.UpdateRoutes(beforeLRP, afterLRP)
							Expect(routingEvents).To(HaveLen(1))

							routingEvent := routingEvents[0]
							Expect(routingEvent.Key).Should(Equal(key))
							Expect(routingEvent.EventType).Should(Equal(event.RouteUnregistrationEvent))
							unregistrationExpectedEntry := endpoint.NewRoutableEndpoints(
								externalEndpoints, endpoints, logGuid, modificationTag)
							Expect(routingEvent.Entry).Should(Equal(unregistrationExpectedEntry))

							Expect(routingTable.RouteCount()).Should(Equal(currentRoutesCount))
						})

						Context("with older modification tag", func() {
							It("emits nothing but add the routing table entry", func() {
								beforeLRP := getDesiredLRP("process-guid-1", "log-guid-1", oldTcpRoutes, modificationTag)
								afterLRP := getDesiredLRP("process-guid-1", "log-guid-1", newTcpRoutes, modificationTag)
								currentRoutesCount := routingTable.RouteCount()
								routingEvents := routingTable.UpdateRoutes(beforeLRP, afterLRP)
								Expect(routingEvents).To(HaveLen(0))
								Expect(routingTable.RouteCount()).Should(Equal(currentRoutesCount + 1))
							})
						})
					})

					Context("existing backends for new container port", func() {
						var (
							newKey       endpoint.RoutingKey
							newEndpoints map[endpoint.EndpointKey]endpoint.Endpoint
						)
						BeforeEach(func() {
							newKey = endpoint.NewRoutingKey("process-guid-1", 5223)
							newEndpoints = map[endpoint.EndpointKey]endpoint.Endpoint{
								endpoint.NewEndpointKey("instance-guid-1", false): endpoint.NewEndpoint(
									"instance-guid-1", false, "some-ip-1", 62006, 5223, modificationTag),
							}
							routingTable = schema.NewTable(logger, map[endpoint.RoutingKey]endpoint.RoutableEndpoints{
								key:    endpoint.NewRoutableEndpoints(externalEndpoints, endpoints, logGuid, modificationTag),
								newKey: endpoint.NewRoutableEndpoints(endpoint.ExternalEndpointInfos{}, newEndpoints, logGuid, modificationTag),
							})
						})

						It("emits registration events for new container port", func() {
							beforeLRP := getDesiredLRP("process-guid-1", "log-guid-1", oldTcpRoutes, modificationTag)
							afterLRP := getDesiredLRP("process-guid-1", "log-guid-1", newTcpRoutes, newModificationTag)
							currentRoutesCount := routingTable.RouteCount()
							routingEvents := routingTable.UpdateRoutes(beforeLRP, afterLRP)
							Expect(routingEvents).To(HaveLen(2))

							externalInfo := []endpoint.ExternalEndpointInfo{
								endpoint.NewExternalEndpointInfo("router-group-guid", 61000),
							}
							routingEvent := routingEvents[0]
							Expect(routingEvent.Key).Should(Equal(newKey))
							Expect(routingEvent.EventType).Should(Equal(event.RouteRegistrationEvent))
							registrationExpectedEntry := endpoint.NewRoutableEndpoints(
								externalInfo, newEndpoints, logGuid, newModificationTag)
							Expect(routingEvent.Entry).Should(Equal(registrationExpectedEntry))

							routingEvent = routingEvents[1]
							Expect(routingEvent.Key).Should(Equal(key))
							Expect(routingEvent.EventType).Should(Equal(event.RouteUnregistrationEvent))
							unregistrationExpectedEntry := endpoint.NewRoutableEndpoints(
								externalInfo, endpoints, logGuid, modificationTag)
							Expect(routingEvent.Entry).Should(Equal(unregistrationExpectedEntry))

							Expect(routingTable.RouteCount()).Should(Equal(currentRoutesCount - 1))
						})

						Context("with older modification tag", func() {
							It("emits nothing", func() {
								beforeLRP := getDesiredLRP("process-guid-1", "log-guid-1", oldTcpRoutes, modificationTag)
								afterLRP := getDesiredLRP("process-guid-1", "log-guid-1", newTcpRoutes, modificationTag)
								currentRoutesCount := routingTable.RouteCount()
								routingEvents := routingTable.UpdateRoutes(beforeLRP, afterLRP)
								Expect(routingEvents).To(HaveLen(0))
								Expect(routingTable.RouteCount()).Should(Equal(currentRoutesCount))
							})
						})
					})
				})
			})
		})

		Describe("RemoveRoutes", func() {
			Context("when entry does not have endpoints", func() {
				BeforeEach(func() {
					emptyEndpoints := make(map[endpoint.EndpointKey]endpoint.Endpoint)
					routingTable = schema.NewTable(logger, map[endpoint.RoutingKey]endpoint.RoutableEndpoints{
						key: endpoint.NewRoutableEndpoints(externalEndpoints, emptyEndpoints, logGuid, modificationTag),
					})
					Expect(routingTable.RouteCount()).Should(Equal(1))
				})

				It("emits nothing", func() {
					modificationTag = &models.ModificationTag{Epoch: "abc", Index: 2}
					desiredLRP := getDesiredLRP("process-guid-1", "log-guid-1", tcpRoutes, modificationTag)
					routingEvents := routingTable.RemoveRoutes(desiredLRP)
					Expect(routingEvents).To(HaveLen(0))
				})
			})

			Context("when entry does have endpoints", func() {
				BeforeEach(func() {
					routingTable = schema.NewTable(logger, map[endpoint.RoutingKey]endpoint.RoutableEndpoints{
						key: endpoint.NewRoutableEndpoints(externalEndpoints, endpoints, logGuid, modificationTag),
					})
					Expect(routingTable.RouteCount()).Should(Equal(1))
				})

				It("emits unregistration routing events", func() {
					newModificationTag := &models.ModificationTag{Epoch: "abc", Index: 2}
					desiredLRP := getDesiredLRP("process-guid-1", "log-guid-1", tcpRoutes, newModificationTag)
					routingEvents := routingTable.RemoveRoutes(desiredLRP)
					Expect(routingEvents).To(HaveLen(1))
					routingEvent := routingEvents[0]
					Expect(routingEvent.Key).Should(Equal(key))
					Expect(routingEvent.EventType).Should(Equal(event.RouteUnregistrationEvent))
					expectedEntry := endpoint.NewRoutableEndpoints(
						externalEndpoints, endpoints, logGuid, modificationTag)
					Expect(routingEvent.Entry).Should(Equal(expectedEntry))
					Expect(routingTable.RouteCount()).Should(Equal(0))
				})

				Context("when there are no external endpoints", func() {
					BeforeEach(func() {
						routingTable = schema.NewTable(logger, map[endpoint.RoutingKey]endpoint.RoutableEndpoints{
							key: endpoint.NewRoutableEndpoints(endpoint.ExternalEndpointInfos{}, endpoints, logGuid, modificationTag),
						})
						Expect(routingTable.RouteCount()).Should(Equal(1))
					})

					It("does not emit any routing events", func() {
						newModificationTag := &models.ModificationTag{Epoch: "abc", Index: 2}
						desiredLRP := getDesiredLRP("process-guid-1", "log-guid-1", tcp_routes.TCPRoutes{}, newModificationTag)
						desiredLRP.Ports = []uint32{5222}
						routingEvents := routingTable.RemoveRoutes(desiredLRP)
						Expect(routingEvents).To(HaveLen(0))
					})
				})
			})
		})

		Describe("AddEndpoint", func() {
			Context("with no existing endpoints", func() {
				BeforeEach(func() {
					routingTable = schema.NewTable(logger, map[endpoint.RoutingKey]endpoint.RoutableEndpoints{
						key: endpoint.NewRoutableEndpoints(externalEndpoints, nil, logGuid, modificationTag),
					})
					Expect(routingTable.RouteCount()).Should(Equal(1))
				})

				It("emits routing events", func() {
					newTag := &models.ModificationTag{Epoch: "abc", Index: 1}
					actualLRP := getActualLRP("process-guid-1", "instance-guid-1", "some-ip-1", 61104, 5222, false, newTag)
					routingEvents := routingTable.AddEndpoint(actualLRP)
					Expect(routingEvents).To(HaveLen(1))
					routingEvent := routingEvents[0]
					Expect(routingEvent.Key).Should(Equal(key))
					Expect(routingEvent.EventType).Should(Equal(event.RouteRegistrationEvent))

					expectedEndpoints := map[endpoint.EndpointKey]endpoint.Endpoint{
						endpoint.NewEndpointKey("instance-guid-1", false): endpoint.NewEndpoint(
							"instance-guid-1", false, "some-ip-1", 61104, 5222, newTag),
					}

					expectedEntry := endpoint.NewRoutableEndpoints(
						externalEndpoints, expectedEndpoints, logGuid, modificationTag)
					Expect(routingEvent.Entry).Should(Equal(expectedEntry))
					Expect(routingTable.RouteCount()).Should(Equal(1))
				})
			})

			Context("with existing endpoints", func() {
				BeforeEach(func() {
					routingTable = schema.NewTable(logger, map[endpoint.RoutingKey]endpoint.RoutableEndpoints{
						key: endpoint.NewRoutableEndpoints(externalEndpoints, endpoints, logGuid, modificationTag),
					})
					Expect(routingTable.RouteCount()).Should(Equal(1))
				})

				Context("with different instance guid", func() {
					It("emits routing events", func() {
						newTag := &models.ModificationTag{Epoch: "abc", Index: 2}
						actualLRP := getActualLRP("process-guid-1", "instance-guid-3", "some-ip-3", 61104, 5222, false, newTag)
						routingEvents := routingTable.AddEndpoint(actualLRP)
						Expect(routingEvents).To(HaveLen(1))
						routingEvent := routingEvents[0]
						Expect(routingEvent.Key).Should(Equal(key))
						Expect(routingEvent.EventType).Should(Equal(event.RouteRegistrationEvent))

						expectedEndpoints := map[endpoint.EndpointKey]endpoint.Endpoint{}
						for k, v := range endpoints {
							expectedEndpoints[k] = v
						}
						expectedEndpoints[endpoint.NewEndpointKey("instance-guid-3", false)] =
							endpoint.NewEndpoint(
								"instance-guid-3", false, "some-ip-3", 61104, 5222, newTag)
						expectedEntry := endpoint.NewRoutableEndpoints(
							externalEndpoints, expectedEndpoints, logGuid, modificationTag)
						Expect(routingEvent.Entry.Endpoints).Should(HaveLen(3))
						Expect(routingEvent.Entry).Should(Equal(expectedEntry))
						Expect(routingTable.RouteCount()).Should(Equal(1))
					})
				})

				Context("with same instance guid", func() {
					Context("newer modification tag", func() {
						It("emits routing events", func() {
							newTag := &models.ModificationTag{Epoch: "abc", Index: 2}
							actualLRP := getActualLRP("process-guid-1", "instance-guid-1", "some-ip-1", 61105, 5222, false, newTag)
							routingEvents := routingTable.AddEndpoint(actualLRP)
							Expect(routingEvents).To(HaveLen(1))
							routingEvent := routingEvents[0]
							Expect(routingEvent.Key).Should(Equal(key))
							Expect(routingEvent.EventType).Should(Equal(event.RouteRegistrationEvent))

							expectedEndpoints := map[endpoint.EndpointKey]endpoint.Endpoint{
								endpoint.NewEndpointKey("instance-guid-1", false): endpoint.NewEndpoint(
									"instance-guid-1", false, "some-ip-1", 61105, 5222, newTag),
								endpoint.NewEndpointKey("instance-guid-2", false): endpoint.NewEndpoint(
									"instance-guid-2", false, "some-ip-2", 62004, 5222, modificationTag),
							}
							expectedEntry := endpoint.NewRoutableEndpoints(
								externalEndpoints, expectedEndpoints, logGuid, modificationTag)
							Expect(routingEvent.Entry.Endpoints).Should(HaveLen(2))
							Expect(routingEvent.Entry).Should(Equal(expectedEntry))
							Expect(routingTable.RouteCount()).Should(Equal(1))
						})
					})

					Context("older modification tag", func() {
						It("emits nothing", func() {
							olderTag := &models.ModificationTag{Epoch: "abc", Index: 0}
							actualLRP := getActualLRP("process-guid-1", "instance-guid-1", "some-ip-1", 61105, 5222, false, olderTag)
							routingEvents := routingTable.AddEndpoint(actualLRP)
							Expect(routingEvents).To(HaveLen(0))
						})
					})
				})
			})
		})

		Describe("RemoveEndpoint", func() {
			Context("with no existing endpoints", func() {
				BeforeEach(func() {
					routingTable = schema.NewTable(logger, map[endpoint.RoutingKey]endpoint.RoutableEndpoints{
						key: endpoint.NewRoutableEndpoints(externalEndpoints, nil, logGuid, modificationTag),
					})
					Expect(routingTable.RouteCount()).Should(Equal(1))
				})

				It("emits nothing", func() {
					newTag := &models.ModificationTag{Epoch: "abc", Index: 1}
					actualLRP := getActualLRP("process-guid-1", "instance-guid-1", "some-ip-1", 61104, 5222, false, newTag)
					routingEvents := routingTable.RemoveEndpoint(actualLRP)
					Expect(routingEvents).To(HaveLen(0))
				})
			})

			Context("with existing endpoints", func() {
				BeforeEach(func() {
					routingTable = schema.NewTable(logger, map[endpoint.RoutingKey]endpoint.RoutableEndpoints{
						key: endpoint.NewRoutableEndpoints(externalEndpoints, endpoints, logGuid, modificationTag),
					})
					Expect(routingTable.RouteCount()).Should(Equal(1))
				})

				Context("with instance guid not present in existing endpoints", func() {
					It("emits nothing", func() {
						newTag := &models.ModificationTag{Epoch: "abc", Index: 2}
						actualLRP := getActualLRP("process-guid-1", "instance-guid-3", "some-ip-3", 62004, 5222, false, newTag)
						routingEvents := routingTable.RemoveEndpoint(actualLRP)
						Expect(routingEvents).To(HaveLen(0))
					})
				})

				Context("with same instance guid", func() {
					Context("newer modification tag", func() {
						It("emits routing events", func() {
							newTag := &models.ModificationTag{Epoch: "abc", Index: 2}
							actualLRP := getActualLRP("process-guid-1", "instance-guid-1", "some-ip-1", 62004, 5222, false, newTag)
							routingEvents := routingTable.RemoveEndpoint(actualLRP)
							Expect(routingEvents).To(HaveLen(1))
							routingEvent := routingEvents[0]
							Expect(routingEvent.Key).Should(Equal(key))
							Expect(routingEvent.EventType).Should(Equal(event.RouteUnregistrationEvent))

							expectedEndpoints := map[endpoint.EndpointKey]endpoint.Endpoint{
								endpoint.NewEndpointKey("instance-guid-1", false): endpoint.NewEndpoint(
									"instance-guid-1", false, "some-ip-1", 62004, 5222, modificationTag),
							}
							expectedEntry := endpoint.NewRoutableEndpoints(
								externalEndpoints, expectedEndpoints, logGuid, modificationTag)
							Expect(routingEvent.Entry.Endpoints).Should(HaveLen(1))
							Expect(routingEvent.Entry).Should(Equal(expectedEntry))
							Expect(routingTable.RouteCount()).Should(Equal(1))
						})
					})

					Context("same modification tag", func() {
						It("emits routing events", func() {
							actualLRP := getActualLRP("process-guid-1", "instance-guid-1", "some-ip-1", 62004, 5222, false, modificationTag)
							routingEvents := routingTable.RemoveEndpoint(actualLRP)
							Expect(routingEvents).To(HaveLen(1))
							routingEvent := routingEvents[0]
							Expect(routingEvent.Key).Should(Equal(key))
							Expect(routingEvent.EventType).Should(Equal(event.RouteUnregistrationEvent))

							expectedEndpoints := map[endpoint.EndpointKey]endpoint.Endpoint{
								endpoint.NewEndpointKey("instance-guid-1", false): endpoint.NewEndpoint(
									"instance-guid-1", false, "some-ip-1", 62004, 5222, modificationTag),
							}
							expectedEntry := endpoint.NewRoutableEndpoints(
								externalEndpoints, expectedEndpoints, logGuid, modificationTag)
							Expect(routingEvent.Entry.Endpoints).Should(HaveLen(1))
							Expect(routingEvent.Entry).Should(Equal(expectedEntry))
							Expect(routingTable.RouteCount()).Should(Equal(1))
						})
					})

					Context("older modification tag", func() {
						It("emits nothing", func() {
							olderTag := &models.ModificationTag{Epoch: "abc", Index: 0}
							actualLRP := getActualLRP("process-guid-1", "instance-guid-1", "some-ip-1", 62004, 5222, false, olderTag)
							routingEvents := routingTable.RemoveEndpoint(actualLRP)
							Expect(routingEvents).To(HaveLen(0))
						})
					})
				})

			})
		})

		Describe("GetRoutingEvents", func() {
			BeforeEach(func() {
				routingTable = schema.NewTable(logger, map[endpoint.RoutingKey]endpoint.RoutableEndpoints{
					key: endpoint.NewRoutableEndpoints(externalEndpoints, endpoints, logGuid, modificationTag),
				})
				Expect(routingTable.RouteCount()).Should(Equal(1))
			})

			It("returns routing events for entries in routing table", func() {
				routingEvents := routingTable.GetRoutingEvents()
				Expect(routingEvents).To(HaveLen(1))
				routingEvent := routingEvents[0]
				Expect(routingEvent.Key).Should(Equal(key))
				Expect(routingEvent.EventType).Should(Equal(event.RouteRegistrationEvent))
				externalInfo := []endpoint.ExternalEndpointInfo{
					endpoint.NewExternalEndpointInfo("router-group-guid", 61000),
				}
				expectedEntry := endpoint.NewRoutableEndpoints(
					externalInfo, endpoints, logGuid, modificationTag)
				Expect(routingEvent.Entry).Should(Equal(expectedEntry))
				Expect(routingTable.RouteCount()).Should(Equal(1))
			})
		})

		Describe("Swap", func() {
			var (
				tempRoutingTable   schema.RoutingTable
				key                endpoint.RoutingKey
				existingKey        endpoint.RoutingKey
				endpoints          map[endpoint.EndpointKey]endpoint.Endpoint
				existingEndpoints  map[endpoint.EndpointKey]endpoint.Endpoint
				modificationTag    *models.ModificationTag
				newModificationTag *models.ModificationTag
				logGuid            string
				existingLogGuid    string
			)

			BeforeEach(func() {
				existingLogGuid = "log-guid-1"
				existingExternalEndpoint := endpoint.ExternalEndpointInfos{
					endpoint.NewExternalEndpointInfo("router-group-guid", 61000),
				}
				existingKey = endpoint.NewRoutingKey("process-guid-1", 5222)
				modificationTag = &models.ModificationTag{Epoch: "abc", Index: 1}
				existingEndpoints = map[endpoint.EndpointKey]endpoint.Endpoint{
					endpoint.NewEndpointKey("instance-guid-1", false): endpoint.NewEndpoint(
						"instance-guid-1", false, "some-ip-1", 62004, 5222, modificationTag),
					endpoint.NewEndpointKey("instance-guid-2", false): endpoint.NewEndpoint(
						"instance-guid-2", false, "some-ip-2", 62004, 5222, modificationTag),
				}
				routingTable = schema.NewTable(logger, map[endpoint.RoutingKey]endpoint.RoutableEndpoints{
					existingKey: endpoint.NewRoutableEndpoints(existingExternalEndpoint, existingEndpoints, existingLogGuid, modificationTag),
				})
				Expect(routingTable.RouteCount()).Should(Equal(1))
			})

			Context("when adding a new routing key (process-guid, container-port)", func() {

				BeforeEach(func() {
					logGuid = "log-guid-2"
					externalEndpoints := endpoint.ExternalEndpointInfos{
						endpoint.NewExternalEndpointInfo("router-group-guid", 62000),
					}
					key = endpoint.NewRoutingKey("process-guid-2", 6379)
					newModificationTag = &models.ModificationTag{Epoch: "abc", Index: 2}
					endpoints = map[endpoint.EndpointKey]endpoint.Endpoint{
						endpoint.NewEndpointKey("instance-guid-1", false): endpoint.NewEndpoint(
							"instance-guid-1", false, "some-ip-3", 63004, 6379, newModificationTag),
						endpoint.NewEndpointKey("instance-guid-2", false): endpoint.NewEndpoint(
							"instance-guid-2", false, "some-ip-4", 63004, 6379, newModificationTag),
					}
					tempRoutingTable = schema.NewTable(logger, map[endpoint.RoutingKey]endpoint.RoutableEndpoints{
						key: endpoint.NewRoutableEndpoints(externalEndpoints, endpoints, logGuid, newModificationTag),
					})
					Expect(tempRoutingTable.RouteCount()).Should(Equal(1))
				})

				It("overwrites the existing entries and emits registration and unregistration routing events", func() {
					routingEvents := routingTable.Swap(tempRoutingTable)
					Expect(routingEvents).To(HaveLen(2))

					routingEvent := routingEvents[0]
					Expect(routingEvent.Key).Should(Equal(key))
					Expect(routingEvent.EventType).Should(Equal(event.RouteRegistrationEvent))
					externalInfo := endpoint.ExternalEndpointInfos{
						endpoint.NewExternalEndpointInfo("router-group-guid", 62000),
					}
					expectedEntry := endpoint.NewRoutableEndpoints(
						externalInfo, endpoints, logGuid, newModificationTag)
					Expect(routingEvent.Entry).Should(Equal(expectedEntry))

					routingEvent = routingEvents[1]
					Expect(routingEvent.Key).Should(Equal(existingKey))
					Expect(routingEvent.EventType).Should(Equal(event.RouteUnregistrationEvent))
					externalInfo = endpoint.ExternalEndpointInfos{
						endpoint.NewExternalEndpointInfo("router-group-guid", 61000),
					}
					expectedEntry = endpoint.NewRoutableEndpoints(
						externalInfo, existingEndpoints, existingLogGuid, modificationTag)
					Expect(routingEvent.Entry).Should(Equal(expectedEntry))
					Expect(routingTable.RouteCount()).Should(Equal(1))
				})
			})

			Context("when updating an existing routing key (process-guid, container-port)", func() {

				BeforeEach(func() {
					logGuid = "log-guid-2"
					externalEndpoints := endpoint.ExternalEndpointInfos{
						endpoint.NewExternalEndpointInfo("router-group-guid", 62000),
					}
					newModificationTag = &models.ModificationTag{Epoch: "abc", Index: 2}
					endpoints = map[endpoint.EndpointKey]endpoint.Endpoint{
						endpoint.NewEndpointKey("instance-guid-3", false): endpoint.NewEndpoint(
							"instance-guid-1", false, "some-ip-3", 63004, 5222, newModificationTag),
						endpoint.NewEndpointKey("instance-guid-4", false): endpoint.NewEndpoint(
							"instance-guid-2", false, "some-ip-4", 63004, 5222, newModificationTag),
					}
					tempRoutingTable = schema.NewTable(logger, map[endpoint.RoutingKey]endpoint.RoutableEndpoints{
						existingKey: endpoint.NewRoutableEndpoints(externalEndpoints, endpoints, logGuid, newModificationTag),
					})
					Expect(tempRoutingTable.RouteCount()).Should(Equal(1))
				})

				It("overwrites the existing entries and emits registration and unregistration routing events", func() {
					routingEvents := routingTable.Swap(tempRoutingTable)
					Expect(routingEvents).To(HaveLen(2))

					routingEvent := routingEvents[0]
					Expect(routingEvent.Key).Should(Equal(existingKey))
					Expect(routingEvent.EventType).Should(Equal(event.RouteRegistrationEvent))
					externalInfo := endpoint.ExternalEndpointInfos{
						endpoint.NewExternalEndpointInfo("router-group-guid", 62000),
					}
					expectedEntry := endpoint.NewRoutableEndpoints(
						externalInfo, endpoints, logGuid, newModificationTag)
					Expect(routingEvent.Entry).Should(Equal(expectedEntry))

					routingEvent = routingEvents[1]
					Expect(routingEvent.Key).Should(Equal(existingKey))
					Expect(routingEvent.EventType).Should(Equal(event.RouteUnregistrationEvent))
					externalInfo = endpoint.ExternalEndpointInfos{
						endpoint.NewExternalEndpointInfo("router-group-guid", 61000),
					}
					expectedEntry = endpoint.NewRoutableEndpoints(
						externalInfo, existingEndpoints, existingLogGuid, modificationTag)
					Expect(routingEvent.Entry).Should(Equal(expectedEntry))
					Expect(routingTable.RouteCount()).Should(Equal(1))
				})
			})
		})
	})

	Describe("Swap", func() {

		var (
			tempRoutingTable testRoutingTable
		)

		BeforeEach(func() {
			routingTable = schema.NewTable(logger, nil)
			tempRoutingTable = testRoutingTable{}

			logGuid := "log-guid-1"
			externalEndpoints := endpoint.ExternalEndpointInfos{
				endpoint.NewExternalEndpointInfo("router-group-guid", 61000),
			}
			key := endpoint.NewRoutingKey("process-guid-1", 5222)
			modificationTag := &models.ModificationTag{Epoch: "abc", Index: 1}
			endpoints := map[endpoint.EndpointKey]endpoint.Endpoint{
				endpoint.NewEndpointKey("instance-guid-1", false): endpoint.NewEndpoint(
					"instance-guid-1", false, "some-ip-1", 62004, 5222, modificationTag),
				endpoint.NewEndpointKey("instance-guid-2", false): endpoint.NewEndpoint(
					"instance-guid-2", false, "some-ip-2", 62004, 5222, modificationTag),
			}

			tempRoutingTable.entries = map[endpoint.RoutingKey]endpoint.RoutableEndpoints{
				key: endpoint.NewRoutableEndpoints(externalEndpoints, endpoints, logGuid, modificationTag),
			}

		})
		Context("when the routing tables are of different type", func() {

			It("should not swap the tables", func() {
				routingEvents := routingTable.Swap(&tempRoutingTable)
				Expect(routingEvents).To(HaveLen(0))
				Expect(routingTable.RouteCount()).Should(Equal(0))
			})
		})
	})

})
