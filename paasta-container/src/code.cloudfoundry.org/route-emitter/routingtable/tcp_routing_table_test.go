package routingtable_test

import (
	"encoding/json"
	"time"

	"code.cloudfoundry.org/bbs/models"
	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/lager/lagertest"
	"code.cloudfoundry.org/route-emitter/routingtable"
	"code.cloudfoundry.org/route-emitter/routingtable/fakeroutingtable"
	tcpmodels "code.cloudfoundry.org/routing-api/models"
	"code.cloudfoundry.org/routing-info/tcp_routes"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
)

const (
	DEFAULT_TIMEOUT          = 5 * time.Second
	DEFAULT_POLLING_INTERVAL = 5 * time.Millisecond
)

var _ = Describe("TCPRoutingTable", func() {

	var (
		routingTable    routingtable.RoutingTable
		modificationTag *models.ModificationTag
		tcpRoutes       tcp_routes.TCPRoutes
		logger          lager.Logger
	)

	getDesiredLRP := func(
		processGuid, logGuid string,
		tcpRoutes tcp_routes.TCPRoutes,
		modificationTag *models.ModificationTag,
	) *models.DesiredLRPSchedulingInfo {
		var desiredLRP models.DesiredLRP
		portMap := map[uint32]struct{}{}
		for _, tcpRoute := range tcpRoutes {
			portMap[tcpRoute.ContainerPort] = struct{}{}
		}

		ports := []uint32{}
		for k := range portMap {
			ports = append(ports, k)
		}

		desiredLRP.ProcessGuid = processGuid
		desiredLRP.Ports = ports
		desiredLRP.LogGuid = logGuid
		desiredLRP.Instances = 3
		desiredLRP.ModificationTag = modificationTag
		desiredLRP.Routes = tcpRoutes.RoutingInfo()
		desiredLRP.Domain = "domain"

		// add 'diego-ssh' data for testing sanitize
		routingInfo := json.RawMessage([]byte(`{ "private_key": "fake-key" }`))
		(*desiredLRP.Routes)["diego-ssh"] = &routingInfo

		info := desiredLRP.DesiredLRPSchedulingInfo()
		return &info
	}

	getActualLRP := func(
		processGuid, instanceGuid, hostAddress, instanceAddress string,
		hostPort, containerPort uint32,
		evacuating bool,
		modificationTag *models.ModificationTag,
	) *routingtable.ActualLRPRoutingInfo {
		return &routingtable.ActualLRPRoutingInfo{
			ActualLRP: &models.ActualLRP{
				ActualLRPKey:         models.NewActualLRPKey(processGuid, 0, "domain"),
				ActualLRPInstanceKey: models.NewActualLRPInstanceKey(instanceGuid, "cell-id-1"),
				ActualLRPNetInfo: models.NewActualLRPNetInfo(
					hostAddress,
					instanceAddress,
					models.NewPortMapping(hostPort, containerPort),
				),
				State:           models.ActualLRPStateRunning,
				ModificationTag: *modificationTag,
			},
			Evacuating: evacuating,
		}
	}

	BeforeEach(func() {
		logger = lagertest.NewTestLogger("test")
		tcpRoutes = tcp_routes.TCPRoutes{
			tcp_routes.TCPRoute{
				RouterGroupGuid: "router-group-guid",
				ExternalPort:    61000,
				ContainerPort:   5222,
			},
		}
	})

	Context("when no entry exist for route", func() {
		BeforeEach(func() {
			routingTable = routingtable.NewRoutingTable(logger, false)
			modificationTag = &models.ModificationTag{Epoch: "abc", Index: 0}
		})

		Describe("AddRoutes", func() {
			It("emits nothing", func() {
				desiredLRP := getDesiredLRP("process-guid-1", "log-guid-1", tcpRoutes, modificationTag)
				routingEvents, _ := routingTable.SetRoutes(nil, desiredLRP)
				Expect(routingEvents.Registrations).To(HaveLen(0))
				Expect(routingEvents.Unregistrations).To(HaveLen(0))
			})

			It("does not emit any sensitive information", func() {
				desiredLRP := getDesiredLRP("process-guid-1", "log-guid-1", tcpRoutes, modificationTag)
				routingEvents, _ := routingTable.SetRoutes(nil, desiredLRP)
				Consistently(logger).ShouldNot(gbytes.Say("private_key"))
				Expect(routingEvents.Registrations).To(HaveLen(0))
				Expect(routingEvents.Unregistrations).To(HaveLen(0))
			})

			It("logs required routing info", func() {
				desiredLRP := getDesiredLRP("process-guid-1", "log-guid-1", tcpRoutes, modificationTag)
				routingEvents, _ := routingTable.SetRoutes(nil, desiredLRP)
				// for i := 0; i < 3; i++ {
				Eventually(logger, DEFAULT_TIMEOUT, DEFAULT_POLLING_INTERVAL).Should(gbytes.Say("process-guid.*process-guid-1"))
				Eventually(logger, DEFAULT_TIMEOUT, DEFAULT_POLLING_INTERVAL).Should(gbytes.Say("routes.*tcp-router.*61000.*5222"))
				// }

				Expect(routingEvents.Registrations).To(HaveLen(0))
				Expect(routingEvents.Unregistrations).To(HaveLen(0))
			})
		})

		Describe("RemoveRoutes", func() {
			It("emits nothing", func() {
				desiredLRP := getDesiredLRP("process-guid-10", "log-guid-10", tcpRoutes, modificationTag)
				routingEvents, _ := routingTable.RemoveRoutes(desiredLRP)
				Expect(routingEvents.Registrations).To(HaveLen(0))
				Expect(routingEvents.Unregistrations).To(HaveLen(0))
			})

			It("does not log sensitive info", func() {
				desiredLRP := getDesiredLRP("process-guid-10", "log-guid-10", tcpRoutes, modificationTag)
				routingEvents, _ := routingTable.RemoveRoutes(desiredLRP)
				Consistently(logger).ShouldNot(gbytes.Say("private_key"))
				Expect(routingEvents.Registrations).To(HaveLen(0))
				Expect(routingEvents.Unregistrations).To(HaveLen(0))
			})

			It("logs required routing info", func() {
				desiredLRP := getDesiredLRP("process-guid-10", "log-guid-10", tcpRoutes, modificationTag)
				routingEvents, _ := routingTable.RemoveRoutes(desiredLRP)
				Eventually(logger, DEFAULT_TIMEOUT, DEFAULT_POLLING_INTERVAL).Should(gbytes.Say("starting.*process-guid-10.*external_port.*61000.*container_port.*5222"))
				Eventually(logger, DEFAULT_TIMEOUT, DEFAULT_POLLING_INTERVAL).Should(gbytes.Say("completed.*process-guid-10.*external_port.*61000.*container_port.*5222"))
				Expect(routingEvents.Registrations).To(HaveLen(0))
				Expect(routingEvents.Unregistrations).To(HaveLen(0))
			})
		})

		Describe("AddEndpoint", func() {
			It("emits nothing", func() {
				actualLRP := getActualLRP("process-guid-1", "instance-guid-1", "some-ip-1", "container-ip-1", 61104, 5222, false, modificationTag)
				routingEvents, _ := routingTable.AddEndpoint(actualLRP)
				Expect(routingEvents.Registrations).To(HaveLen(0))
				Expect(routingEvents.Unregistrations).To(HaveLen(0))
			})

			It("does not log sensitive info", func() {
				actualLRP := getActualLRP("process-guid-1", "instance-guid-1", "some-ip-1", "container-ip-1", 61104, 5222, false, modificationTag)
				routingEvents, _ := routingTable.AddEndpoint(actualLRP)
				Expect(routingEvents.Registrations).To(HaveLen(0))
				Expect(routingEvents.Unregistrations).To(HaveLen(0))
				Consistently(logger).ShouldNot(gbytes.Say("private_key"))
			})

			It("logs required routing info", func() {
				actualLRP := getActualLRP("process-guid-1", "instance-guid-1", "some-ip-1", "container-ip-1", 61104, 5222, false, modificationTag)
				routingEvents, _ := routingTable.AddEndpoint(actualLRP)
				Expect(routingEvents.Registrations).To(HaveLen(0))
				Expect(routingEvents.Unregistrations).To(HaveLen(0))
				Eventually(logger, DEFAULT_TIMEOUT, DEFAULT_POLLING_INTERVAL).Should(gbytes.Say("process_guid.*process-guid-1"))
				Eventually(logger, DEFAULT_TIMEOUT, DEFAULT_POLLING_INTERVAL).Should(gbytes.Say("ports.*5222.*61104"))
			})
		})

		Describe("RemoveEndpoint", func() {
			It("emits nothing", func() {
				actualLRP := getActualLRP("process-guid-1", "instance-guid-1", "some-ip-1", "container-ip-1", 61104, 5222, false, modificationTag)
				routingEvents, _ := routingTable.RemoveEndpoint(actualLRP)
				Expect(routingEvents.Registrations).To(HaveLen(0))
				Expect(routingEvents.Unregistrations).To(HaveLen(0))
			})

			It("does not log sensitive info", func() {
				actualLRP := getActualLRP("process-guid-1", "instance-guid-1", "some-ip-1", "container-ip-1", 61104, 5222, false, modificationTag)
				routingEvents, _ := routingTable.RemoveEndpoint(actualLRP)
				Expect(routingEvents.Registrations).To(HaveLen(0))
				Expect(routingEvents.Unregistrations).To(HaveLen(0))
				Consistently(logger).ShouldNot(gbytes.Say("private_key"))
			})

			It("logs required routing info", func() {
				actualLRP := getActualLRP("process-guid-1", "instance-guid-1", "some-ip-1", "container-ip-1", 61104, 5222, false, modificationTag)
				routingEvents, _ := routingTable.RemoveEndpoint(actualLRP)
				Expect(routingEvents.Registrations).To(HaveLen(0))
				Expect(routingEvents.Unregistrations).To(HaveLen(0))
				Eventually(logger, DEFAULT_TIMEOUT, DEFAULT_POLLING_INTERVAL).Should(gbytes.Say("starting.*process-guid-1.*ports.*5222.*61104"))
				Eventually(logger, DEFAULT_TIMEOUT, DEFAULT_POLLING_INTERVAL).Should(gbytes.Say("completed.*process-guid-1.*ports.*5222.*61104"))
			})
		})

		Describe("Swap", func() {
			var (
				tempRoutingTable routingtable.RoutingTable
				logGuid          string
			)

			BeforeEach(func() {
				logGuid = "log-guid-1"
				tempRoutingTable = routingtable.NewRoutingTable(logger, false)
				beforeLRPSchedulingInfo := getDesiredLRP("process-guid-1", logGuid, tcpRoutes, modificationTag)
				tempRoutingTable.SetRoutes(nil, beforeLRPSchedulingInfo)
				tempRoutingTable.AddEndpoint(getActualLRP("process-guid-1", "instance-guid-1", "some-ip-1", "container-ip-1", 62004, 5222, false, modificationTag))
				tempRoutingTable.AddEndpoint(getActualLRP("process-guid-1", "instance-guid-2", "some-ip-2", "container-ip-2", 62004, 5222, false, modificationTag))
				Expect(tempRoutingTable.TCPAssociationsCount()).Should(Equal(2))
			})

			It("emits routing events for new routes", func() {
				Expect(routingTable.TCPAssociationsCount()).Should(Equal(0))
				routingEvents, _ := routingTable.Swap(tempRoutingTable, models.DomainSet{})
				Expect(routingTable.TCPAssociationsCount()).Should(Equal(2))

				ttl := 0
				Expect(routingEvents.Registrations).To(ConsistOf(tcpmodels.TcpRouteMapping{
					TcpMappingEntity: tcpmodels.TcpMappingEntity{
						RouterGroupGuid: "router-group-guid",
						HostPort:        62004,
						HostIP:          "some-ip-1",
						ExternalPort:    61000,
						TTL:             &ttl,
					},
				}, tcpmodels.TcpRouteMapping{
					TcpMappingEntity: tcpmodels.TcpMappingEntity{
						RouterGroupGuid: "router-group-guid",
						HostPort:        62004,
						HostIP:          "some-ip-2",
						ExternalPort:    61000,
						TTL:             &ttl,
					},
				}))
			})

			Context("when the table is configured to emit direct instance route", func() {
				BeforeEach(func() {
					routingTable = routingtable.NewRoutingTable(logger, true)
				})

				It("emits routing events for new routes", func() {
					Expect(routingTable.TCPAssociationsCount()).Should(Equal(0))
					routingEvents, _ := routingTable.Swap(tempRoutingTable, models.DomainSet{})
					Expect(routingTable.TCPAssociationsCount()).Should(Equal(2))

					ttl := 0
					Expect(routingEvents.Registrations).To(ConsistOf(tcpmodels.TcpRouteMapping{
						TcpMappingEntity: tcpmodels.TcpMappingEntity{
							RouterGroupGuid: "router-group-guid",
							HostPort:        5222,
							HostIP:          "container-ip-1",
							ExternalPort:    61000,
							TTL:             &ttl,
						},
					}, tcpmodels.TcpRouteMapping{
						TcpMappingEntity: tcpmodels.TcpMappingEntity{
							RouterGroupGuid: "router-group-guid",
							HostPort:        5222,
							HostIP:          "container-ip-2",
							ExternalPort:    61000,
							TTL:             &ttl,
						},
					}))
				})
			})
		})

		Describe("GetRoutingEvents", func() {
			It("returns empty routing events", func() {
				routingEvents, _ := routingTable.GetRoutingEvents()
				Expect(routingEvents.Registrations).To(HaveLen(0))
				Expect(routingEvents.Unregistrations).To(HaveLen(0))
			})
		})

		Context("when the routing tables are of different type", func() {
			It("should not swap the tables", func() {
				routingTable = routingtable.NewRoutingTable(logger, false)
				fakeTable := &fakeroutingtable.FakeRoutingTable{}
				routingEvents, _ := routingTable.Swap(fakeTable, models.DomainSet{})
				Expect(routingEvents.Registrations).To(HaveLen(0))
				Expect(routingEvents.Unregistrations).To(HaveLen(0))
				Expect(routingTable.TCPAssociationsCount()).Should(Equal(0))
			})
		})
	})

	Context("when there exists an entry for route", func() {
		var (
			logGuid string
		)

		BeforeEach(func() {
			logGuid = "log-guid-1"
			modificationTag = &models.ModificationTag{Epoch: "abc", Index: 1}
		})

		Describe("HasExternalRoutes", func() {
			It("returns the associated desired state", func() {
				routingTable = routingtable.NewRoutingTable(logger, true)
				beforeLRPSchedulingInfo := getDesiredLRP("process-guid-1", logGuid, tcpRoutes, modificationTag)
				routingTable.SetRoutes(nil, beforeLRPSchedulingInfo)
				routingInfo := getActualLRP("process-guid-1", "instance-guid-2", "some-ip-2", "container-ip-2", 62004, 5222, false, modificationTag)
				routingTable.AddEndpoint(routingInfo)
				Expect(routingTable.HasExternalRoutes(routingInfo)).To(BeTrue())
			})
		})

		Describe("AddRoutes", func() {
			BeforeEach(func() {
				routingTable = routingtable.NewRoutingTable(logger, false)
				beforeLRPSchedulingInfo := getDesiredLRP("process-guid-1", "log-guid-1", tcpRoutes, modificationTag)
				routingTable.SetRoutes(nil, beforeLRPSchedulingInfo)
				routingTable.AddEndpoint(getActualLRP("process-guid-1", "instance-guid-1", "some-ip-1", "container-ip-1", 62004, 5222, false, modificationTag))
				routingTable.AddEndpoint(getActualLRP("process-guid-1", "instance-guid-2", "some-ip-2", "container-ip-2", 62004, 5222, false, modificationTag))
				Expect(routingTable.TCPAssociationsCount()).Should(Equal(2))
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
					routingEvents, _ := routingTable.SetRoutes(nil, desiredLRP)

					ttl := 0
					Expect(routingEvents.Unregistrations).Should(ConsistOf(tcpmodels.TcpRouteMapping{
						TcpMappingEntity: tcpmodels.TcpMappingEntity{
							RouterGroupGuid: "router-group-guid",
							HostPort:        62004,
							HostIP:          "some-ip-1",
							ExternalPort:    61000,
							TTL:             &ttl,
						},
					}, tcpmodels.TcpRouteMapping{
						TcpMappingEntity: tcpmodels.TcpMappingEntity{
							RouterGroupGuid: "router-group-guid",
							HostPort:        62004,
							HostIP:          "some-ip-2",
							ExternalPort:    61000,
							TTL:             &ttl,
						},
					}))

					Expect(routingEvents.Registrations).Should(ConsistOf(tcpmodels.TcpRouteMapping{
						TcpMappingEntity: tcpmodels.TcpMappingEntity{
							RouterGroupGuid: "router-group-guid",
							HostPort:        62004,
							HostIP:          "some-ip-1",
							ExternalPort:    61001,
							TTL:             &ttl,
						},
					}, tcpmodels.TcpRouteMapping{
						TcpMappingEntity: tcpmodels.TcpMappingEntity{
							RouterGroupGuid: "router-group-guid",
							HostPort:        62004,
							HostIP:          "some-ip-2",
							ExternalPort:    61001,
							TTL:             &ttl,
						},
					}))

					Expect(routingTable.TCPAssociationsCount()).Should(Equal(2))
				})

				Context("older modification tag", func() {
					It("emits nothing", func() {
						desiredLRP := getDesiredLRP("process-guid-1", "log-guid-1", tcpRoutes, modificationTag)
						routingEvents, _ := routingTable.SetRoutes(nil, desiredLRP)
						Expect(routingEvents.Registrations).To(HaveLen(0))
						Expect(routingEvents.Unregistrations).To(HaveLen(0))
					})

					It("does not change the routing table", func() {
						Expect(routingTable.TCPAssociationsCount()).Should(Equal(2))
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
					routingEvents, _ := routingTable.SetRoutes(nil, desiredLRP)

					ttl := 0
					Expect(routingEvents.Registrations).Should(ConsistOf(tcpmodels.TcpRouteMapping{
						TcpMappingEntity: tcpmodels.TcpMappingEntity{
							RouterGroupGuid: "router-group-guid",
							HostPort:        62004,
							HostIP:          "some-ip-1",
							ExternalPort:    61001,
							TTL:             &ttl,
						},
					}, tcpmodels.TcpRouteMapping{
						TcpMappingEntity: tcpmodels.TcpMappingEntity{
							RouterGroupGuid: "router-group-guid",
							HostPort:        62004,
							HostIP:          "some-ip-2",
							ExternalPort:    61001,
							TTL:             &ttl,
						},
					}, tcpmodels.TcpRouteMapping{
						TcpMappingEntity: tcpmodels.TcpMappingEntity{
							RouterGroupGuid: "router-group-guid",
							HostPort:        62004,
							HostIP:          "some-ip-1",
							ExternalPort:    61002,
							TTL:             &ttl,
						},
					}, tcpmodels.TcpRouteMapping{
						TcpMappingEntity: tcpmodels.TcpMappingEntity{
							RouterGroupGuid: "router-group-guid",
							HostPort:        62004,
							HostIP:          "some-ip-2",
							ExternalPort:    61002,
							TTL:             &ttl,
						},
					}))
					Expect(routingEvents.Unregistrations).Should(HaveLen(0))

					Expect(routingTable.TCPAssociationsCount()).Should(Equal(6))
				})

				Context("older modification tag", func() {
					It("emits nothing", func() {
						desiredLRP := getDesiredLRP("process-guid-1", "log-guid-1", tcpRoutes, modificationTag)
						routingEvents, _ := routingTable.SetRoutes(nil, desiredLRP)
						Expect(routingEvents.Registrations).To(HaveLen(0))
						Expect(routingEvents.Unregistrations).To(HaveLen(0))
					})

					It("does not change the routing table", func() {
						Expect(routingTable.TCPAssociationsCount()).Should(Equal(2))
					})
				})
			})

			Context("multiple external port added and multiple existing external ports deleted", func() {
				var (
					newTcpRoutes tcp_routes.TCPRoutes
				)
				BeforeEach(func() {
					currentTcpRoutes := tcp_routes.TCPRoutes{
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
					}

					desiredLRP := getDesiredLRP("process-guid-1", "log-guid-1", currentTcpRoutes, modificationTag)
					routingTable = routingtable.NewRoutingTable(logger, false)
					routingTable.SetRoutes(nil, desiredLRP)
					routingTable.AddEndpoint(getActualLRP("process-guid-1", "instance-guid-1", "some-ip-1", "container-ip-1", 62004, 5222, false, modificationTag))
					routingTable.AddEndpoint(getActualLRP("process-guid-1", "instance-guid-2", "some-ip-2", "container-ip-2", 62004, 5222, false, modificationTag))
					Expect(routingTable.TCPAssociationsCount()).Should(Equal(4))

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
					routingEvents, _ := routingTable.SetRoutes(nil, desiredLRP)

					ttl := 0
					Expect(routingEvents.Registrations).Should(ConsistOf(tcpmodels.TcpRouteMapping{
						TcpMappingEntity: tcpmodels.TcpMappingEntity{
							RouterGroupGuid: "router-group-guid",
							HostPort:        62004,
							HostIP:          "some-ip-1",
							ExternalPort:    61002,
							TTL:             &ttl,
						},
					}, tcpmodels.TcpRouteMapping{
						TcpMappingEntity: tcpmodels.TcpMappingEntity{
							RouterGroupGuid: "router-group-guid",
							HostPort:        62004,
							HostIP:          "some-ip-2",
							ExternalPort:    61002,
							TTL:             &ttl,
						},
					}, tcpmodels.TcpRouteMapping{
						TcpMappingEntity: tcpmodels.TcpMappingEntity{
							RouterGroupGuid: "router-group-guid",
							HostPort:        62004,
							HostIP:          "some-ip-1",
							ExternalPort:    61003,
							TTL:             &ttl,
						},
					}, tcpmodels.TcpRouteMapping{
						TcpMappingEntity: tcpmodels.TcpMappingEntity{
							RouterGroupGuid: "router-group-guid",
							HostPort:        62004,
							HostIP:          "some-ip-2",
							ExternalPort:    61003,
							TTL:             &ttl,
						},
					}))

					Expect(routingEvents.Unregistrations).Should(ConsistOf(tcpmodels.TcpRouteMapping{
						TcpMappingEntity: tcpmodels.TcpMappingEntity{
							RouterGroupGuid: "router-group-guid",
							HostPort:        62004,
							HostIP:          "some-ip-1",
							ExternalPort:    61000,
							TTL:             &ttl,
						},
					}, tcpmodels.TcpRouteMapping{
						TcpMappingEntity: tcpmodels.TcpMappingEntity{
							RouterGroupGuid: "router-group-guid",
							HostPort:        62004,
							HostIP:          "some-ip-2",
							ExternalPort:    61000,
							TTL:             &ttl,
						},
					}, tcpmodels.TcpRouteMapping{
						TcpMappingEntity: tcpmodels.TcpMappingEntity{
							RouterGroupGuid: "router-group-guid",
							HostPort:        62004,
							HostIP:          "some-ip-1",
							ExternalPort:    61001,
							TTL:             &ttl,
						},
					}, tcpmodels.TcpRouteMapping{
						TcpMappingEntity: tcpmodels.TcpMappingEntity{
							RouterGroupGuid: "router-group-guid",
							HostPort:        62004,
							HostIP:          "some-ip-2",
							ExternalPort:    61001,
							TTL:             &ttl,
						},
					}))
					Expect(routingTable.TCPAssociationsCount()).Should(Equal(4))
				})
			})

			Context("older modification tag", func() {
				It("emits nothing", func() {
					desiredLRP := getDesiredLRP("process-guid-1", "log-guid-1", tcpRoutes, modificationTag)
					routingEvents, _ := routingTable.SetRoutes(nil, desiredLRP)
					Expect(routingEvents.Registrations).To(HaveLen(0))
					Expect(routingEvents.Unregistrations).To(HaveLen(0))
					Expect(routingTable.TCPAssociationsCount()).Should(Equal(2))
				})
			})

			Context("no changes to external port", func() {
				It("emits nothing", func() {
					tag := &models.ModificationTag{Epoch: "abc", Index: 2}
					desiredLRP := getDesiredLRP("process-guid-1", "log-guid-1", tcpRoutes, tag)
					routingEvents, _ := routingTable.SetRoutes(nil, desiredLRP)
					Expect(routingEvents.Registrations).To(HaveLen(0))
					Expect(routingEvents.Unregistrations).To(HaveLen(0))
					Expect(routingTable.TCPAssociationsCount()).Should(Equal(2))
				})
			})

			Context("when two disjoint (external port, container port) pairs are given", func() {
				BeforeEach(func() {
					beforeLRPSchedulingInfo := getDesiredLRP("process-guid-1", "log-guid-1", tcpRoutes, modificationTag)
					routingTable = routingtable.NewRoutingTable(logger, false)
					routingTable.SetRoutes(nil, beforeLRPSchedulingInfo)
					routingTable.AddEndpoint(getActualLRP("process-guid-1", "instance-guid-1", "some-ip-1", "container-ip-1", 62004, 5222, false, modificationTag))
					routingTable.AddEndpoint(getActualLRP("process-guid-1", "instance-guid-1", "some-ip-1", "container-ip-1", 63004, 5223, false, modificationTag))
					Expect(routingTable.TCPAssociationsCount()).Should(Equal(1))
				})

				It("emits two separate registration events with no overlap", func() {
					newTcpRoutes := tcp_routes.TCPRoutes{
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
					newModificationTag := &models.ModificationTag{Epoch: "abc", Index: 2}
					routingEvents, _ := routingTable.SetRoutes(nil, getDesiredLRP("process-guid-1", logGuid, newTcpRoutes, newModificationTag))

					// Two registration and one unregistration events
					ttl := 0
					Expect(routingEvents.Registrations).To(ConsistOf(tcpmodels.TcpRouteMapping{
						TcpMappingEntity: tcpmodels.TcpMappingEntity{
							RouterGroupGuid: "router-group-guid",
							HostPort:        62004,
							HostIP:          "some-ip-1",
							ExternalPort:    61001,
							TTL:             &ttl,
						},
					}, tcpmodels.TcpRouteMapping{
						TcpMappingEntity: tcpmodels.TcpMappingEntity{
							RouterGroupGuid: "router-group-guid",
							HostPort:        63004,
							HostIP:          "some-ip-1",
							ExternalPort:    61002,
							TTL:             &ttl,
						},
					}))
					Expect(routingEvents.Unregistrations).Should(ConsistOf(tcpmodels.TcpRouteMapping{
						TcpMappingEntity: tcpmodels.TcpMappingEntity{
							RouterGroupGuid: "router-group-guid",
							HostPort:        62004,
							HostIP:          "some-ip-1",
							ExternalPort:    61000,
							TTL:             &ttl,
						},
					}))
					Expect(routingTable.TCPAssociationsCount()).Should(Equal(2))
				})
			})

			Context("when container ports don't match", func() {
				BeforeEach(func() {
					tcpRoutes = tcp_routes.TCPRoutes{
						tcp_routes.TCPRoute{
							RouterGroupGuid: "router-group-guid",
							ExternalPort:    61000,
							ContainerPort:   5222,
						},
						tcp_routes.TCPRoute{
							ExternalPort:  61000,
							ContainerPort: 5223,
						},
					}
				})

				It("emits nothing", func() {
					newTag := &models.ModificationTag{Epoch: "abc", Index: 2}
					desiredLRP := getDesiredLRP("process-guid-1", "log-guid-1", tcpRoutes, newTag)
					routingEvents, _ := routingTable.SetRoutes(nil, desiredLRP)
					Expect(routingEvents.Registrations).To(HaveLen(0))
					Expect(routingEvents.Unregistrations).To(HaveLen(0))
					Expect(routingTable.TCPAssociationsCount()).Should(Equal(2))
				})
			})
		})

		Describe("Updating routes", func() {
			var (
				newTcpRoutes            tcp_routes.TCPRoutes
				newModificationTag      *models.ModificationTag
				beforeLRPSchedulingInfo *models.DesiredLRPSchedulingInfo
			)

			BeforeEach(func() {
				newModificationTag = &models.ModificationTag{Epoch: "abc", Index: 2}
				routingTable = routingtable.NewRoutingTable(logger, false)
				beforeLRPSchedulingInfo = getDesiredLRP("process-guid-1", "log-guid-1", tcpRoutes, modificationTag)
				routingTable.SetRoutes(nil, beforeLRPSchedulingInfo)
				routingTable.AddEndpoint(getActualLRP("process-guid-1", "instance-guid-1", "some-ip-1", "container-ip-1", 62004, 5222, false, modificationTag))
				Expect(routingTable.TCPAssociationsCount()).Should(Equal(1))
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

				Context("existing router group guid changes", func() {
					BeforeEach(func() {
						newTcpRoutes = tcp_routes.TCPRoutes{
							tcp_routes.TCPRoute{
								RouterGroupGuid: "new-router-group-guid",
								ExternalPort:    61000,
								ContainerPort:   5222,
							},
						}
					})

					It("emits unregistration and registration mappings", func() {
						newModificationTag := &models.ModificationTag{Epoch: "abc", Index: 2}
						beforeLRP := getDesiredLRP("process-guid-1", "log-guid-1", tcpRoutes, modificationTag)
						afterLRP := getDesiredLRP("process-guid-1", "log-guid-1", newTcpRoutes, newModificationTag)
						routingEvents, _ := routingTable.SetRoutes(beforeLRP, afterLRP)

						ttl := 0
						Expect(routingEvents.Registrations).To(ConsistOf(tcpmodels.TcpRouteMapping{
							TcpMappingEntity: tcpmodels.TcpMappingEntity{
								RouterGroupGuid: "new-router-group-guid",
								HostPort:        62004,
								HostIP:          "some-ip-1",
								ExternalPort:    61000,
								TTL:             &ttl,
							},
						}))
						Expect(routingEvents.Unregistrations).To(ConsistOf(tcpmodels.TcpRouteMapping{
							TcpMappingEntity: tcpmodels.TcpMappingEntity{
								RouterGroupGuid: "router-group-guid",
								HostPort:        62004,
								HostIP:          "some-ip-1",
								ExternalPort:    61000,
								TTL:             &ttl,
							},
						}))
					})
				})

				Context("when there is change in external port", func() {
					It("emits registration and unregistration events", func() {
						afterLRP := getDesiredLRP("process-guid-1", "log-guid-1", newTcpRoutes, newModificationTag)
						routingEvents, _ := routingTable.SetRoutes(nil, afterLRP)

						ttl := 0
						Expect(routingEvents.Unregistrations).To(ConsistOf(tcpmodels.TcpRouteMapping{
							TcpMappingEntity: tcpmodels.TcpMappingEntity{
								RouterGroupGuid: "router-group-guid",
								HostPort:        62004,
								HostIP:          "some-ip-1",
								ExternalPort:    61000,
								TTL:             &ttl,
							},
						}))
						Expect(routingEvents.Registrations).To(ConsistOf(tcpmodels.TcpRouteMapping{
							TcpMappingEntity: tcpmodels.TcpMappingEntity{
								RouterGroupGuid: "router-group-guid",
								HostPort:        62004,
								HostIP:          "some-ip-1",
								ExternalPort:    61001,
								TTL:             &ttl,
							},
						}))

						Expect(routingTable.TCPAssociationsCount()).Should(Equal(1))
					})

					Context("with older modification tag", func() {
						It("emits nothing", func() {
							afterLRP := getDesiredLRP("process-guid-1", "log-guid-1", newTcpRoutes, modificationTag)
							routingEvents, _ := routingTable.SetRoutes(nil, afterLRP)
							Expect(routingEvents.Registrations).To(HaveLen(0))
							Expect(routingEvents.Unregistrations).To(HaveLen(0))
							Expect(routingTable.TCPAssociationsCount()).Should(Equal(1))
						})
					})
				})

				Context("when there is no change in external port", func() {
					It("emits nothing", func() {
						afterLRP := getDesiredLRP("process-guid-1", "log-guid-1", tcpRoutes, newModificationTag)
						routingEvents, _ := routingTable.SetRoutes(beforeLRPSchedulingInfo, afterLRP)
						Expect(routingEvents.Registrations).To(HaveLen(0))
						Expect(routingEvents.Unregistrations).To(HaveLen(0))
						Expect(routingTable.TCPAssociationsCount()).Should(Equal(1))
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
							afterLRP := getDesiredLRP("process-guid-1", "log-guid-1", newTcpRoutes, newModificationTag)
							routingEvents, _ := routingTable.SetRoutes(beforeLRPSchedulingInfo, afterLRP)
							Expect(routingEvents.Registrations).To(HaveLen(0))
							Expect(routingEvents.Unregistrations).To(HaveLen(0))
							Expect(routingTable.TCPAssociationsCount()).Should(Equal(1))
						})

						Context("with older modification tag", func() {
							It("emits nothing but add the routing table entry", func() {
								afterLRP := getDesiredLRP("process-guid-1", "log-guid-1", newTcpRoutes, modificationTag)
								currentRoutesCount := routingTable.TCPAssociationsCount()
								routingEvents, _ := routingTable.SetRoutes(beforeLRPSchedulingInfo, afterLRP)
								Expect(routingEvents.Registrations).To(HaveLen(0))
								Expect(routingEvents.Unregistrations).To(HaveLen(0))
								Expect(routingTable.TCPAssociationsCount()).Should(Equal(currentRoutesCount))
							})
						})
					})

					Context("existing backends for new container port", func() {
						BeforeEach(func() {
							routingTable.AddEndpoint(getActualLRP("process-guid-1", "instance-guid-2", "some-ip-1", "container-ip-1", 62006, 5223, false, modificationTag))
						})

						It("emits registration events for new container port", func() {
							afterLRP := getDesiredLRP("process-guid-1", "log-guid-1", newTcpRoutes, newModificationTag)
							currentRoutesCount := routingTable.TCPAssociationsCount()
							routingEvents, _ := routingTable.SetRoutes(beforeLRPSchedulingInfo, afterLRP)
							ttl := 0
							Expect(routingEvents.Registrations).To(ConsistOf(tcpmodels.TcpRouteMapping{
								TcpMappingEntity: tcpmodels.TcpMappingEntity{
									RouterGroupGuid: "router-group-guid",
									HostPort:        62006,
									HostIP:          "some-ip-1",
									ExternalPort:    61001,
									TTL:             &ttl,
								},
							}))
							Expect(routingEvents.Unregistrations).To(HaveLen(0))

							Expect(routingTable.TCPAssociationsCount()).Should(Equal(currentRoutesCount + 1))
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
								ExternalPort:    61001,
								ContainerPort:   5223,
							},
						}
					})

					Context("no backends for new container port", func() {
						var (
							routingEvents routingtable.TCPRouteMappings
						)

						BeforeEach(func() {
							afterLRP := getDesiredLRP("process-guid-1", "log-guid-1", newTcpRoutes, newModificationTag)
							routingEvents, _ = routingTable.SetRoutes(beforeLRPSchedulingInfo, afterLRP)
						})

						It("emits no routing events and adds to routing table entry", func() {
							Expect(routingEvents.Registrations).To(HaveLen(0))
							Expect(routingEvents.Unregistrations).To(HaveLen(0))
						})

						Context("when the actual lrp is updated", func() {
							BeforeEach(func() {
								oldActualLRP := getActualLRP("process-guid-1", "instance-guid-1", "some-ip-1", "container-ip-1", 62004, 5222, false, modificationTag)
								routingTable.RemoveEndpoint(oldActualLRP)
								newActualLRP := oldActualLRP
								newActualLRP.ActualLRP.ActualLRPNetInfo = models.NewActualLRPNetInfo(
									"some-ip-1",
									"container-ip-1",
									models.NewPortMapping(62004, 5222),
									models.NewPortMapping(62005, 5223),
								)
								routingEvents, _ = routingTable.AddEndpoint(newActualLRP)
							})

							It("emits two registration events", func() {
								ttl := 0
								Expect(routingEvents.Registrations).To(ConsistOf(tcpmodels.TcpRouteMapping{
									TcpMappingEntity: tcpmodels.TcpMappingEntity{
										RouterGroupGuid: "router-group-guid",
										HostPort:        62004,
										HostIP:          "some-ip-1",
										ExternalPort:    61000,
										TTL:             &ttl,
									},
								}, tcpmodels.TcpRouteMapping{
									TcpMappingEntity: tcpmodels.TcpMappingEntity{
										RouterGroupGuid: "router-group-guid",
										HostPort:        62005,
										HostIP:          "some-ip-1",
										ExternalPort:    61001,
										TTL:             &ttl,
									},
								}))
							})
						})

						Context("with older modification tag", func() {
							It("emits nothing but add the routing table entry", func() {
								afterLRP := getDesiredLRP("process-guid-1", "log-guid-1", newTcpRoutes, modificationTag)
								currentRoutesCount := routingTable.TCPAssociationsCount()
								routingEvents, _ := routingTable.SetRoutes(beforeLRPSchedulingInfo, afterLRP)
								Expect(routingEvents.Registrations).To(HaveLen(0))
								Expect(routingEvents.Unregistrations).To(HaveLen(0))
								Expect(routingTable.TCPAssociationsCount()).Should(Equal(currentRoutesCount))
							})
						})
					})

					Context("existing backends for new container port", func() {
						BeforeEach(func() {
							routingTable.AddEndpoint(getActualLRP("process-guid-1", "instance-guid-2", "some-ip-1", "container-ip-1", 62006, 5223, false, modificationTag))
						})

						XIt("emits registration events for new container port", func() {
							afterLRP := getDesiredLRP("process-guid-1", "log-guid-1", newTcpRoutes, newModificationTag)
							currentRoutesCount := routingTable.TCPAssociationsCount()
							routingEvents, _ := routingTable.SetRoutes(beforeLRPSchedulingInfo, afterLRP)

							ttl := 0
							Expect(routingEvents.Registrations).To(ConsistOf(tcpmodels.TcpRouteMapping{
								TcpMappingEntity: tcpmodels.TcpMappingEntity{
									RouterGroupGuid: "router-group-guid",
									HostPort:        62006,
									HostIP:          "some-ip-1",
									ExternalPort:    61000,
									TTL:             &ttl,
								},
							}))
							Expect(routingTable.TCPAssociationsCount()).Should(Equal(currentRoutesCount))
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
						afterLRP := getDesiredLRP("process-guid-1", "log-guid-1", newTcpRoutes, newModificationTag)
						routingEvents, _ := routingTable.SetRoutes(beforeLRPSchedulingInfo, afterLRP)
						ttl := 0
						Expect(routingEvents.Unregistrations).To(ConsistOf(tcpmodels.TcpRouteMapping{
							TcpMappingEntity: tcpmodels.TcpMappingEntity{
								RouterGroupGuid: "router-group-guid",
								HostPort:        62004,
								HostIP:          "some-ip-1",
								ExternalPort:    61000,
								TTL:             &ttl,
							},
						}))
					})

					Context("with older modification tag", func() {
						It("emits nothing", func() {
							afterLRP := getDesiredLRP("process-guid-1", "log-guid-1", newTcpRoutes, modificationTag)
							routingEvents, _ := routingTable.SetRoutes(beforeLRPSchedulingInfo, afterLRP)
							Expect(routingEvents.Registrations).To(HaveLen(0))
							Expect(routingEvents.Unregistrations).To(HaveLen(0))
							Expect(routingTable.TCPAssociationsCount()).Should(Equal(1))
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
						It("only emits unregistration events and adds to routing table entry", func() {
							afterLRP := getDesiredLRP("process-guid-1", "log-guid-1", newTcpRoutes, newModificationTag)
							routingEvents, _ := routingTable.SetRoutes(beforeLRPSchedulingInfo, afterLRP)
							ttl := 0
							Expect(routingEvents.Unregistrations).To(ConsistOf(tcpmodels.TcpRouteMapping{
								TcpMappingEntity: tcpmodels.TcpMappingEntity{
									RouterGroupGuid: "router-group-guid",
									HostPort:        62004,
									HostIP:          "some-ip-1",
									ExternalPort:    61000,
									TTL:             &ttl,
								},
							}))
						})

						Context("with older modification tag", func() {
							It("emits nothing but add the routing table entry", func() {
								afterLRP := getDesiredLRP("process-guid-1", "log-guid-1", newTcpRoutes, modificationTag)
								Expect(routingTable.TCPAssociationsCount()).Should(Equal(1))
								routingEvents, _ := routingTable.SetRoutes(beforeLRPSchedulingInfo, afterLRP)
								Expect(routingEvents.Registrations).To(HaveLen(0))
								Expect(routingEvents.Unregistrations).To(HaveLen(0))
								Expect(routingTable.TCPAssociationsCount()).Should(Equal(1))
							})
						})
					})

					Context("existing backends for new container port", func() {
						BeforeEach(func() {
							routingTable.AddEndpoint(getActualLRP("process-guid-1", "instance-guid-1", "some-ip-1", "container-ip-1", 62006, 5223, false, modificationTag))
							Expect(routingTable.TCPAssociationsCount()).Should(Equal(1))
						})

						It("emits registration events for new container port", func() {
							afterLRP := getDesiredLRP("process-guid-1", "log-guid-1", newTcpRoutes, newModificationTag)
							// currentRoutesCount := routingTable.RouteCount()
							routingEvents, _ := routingTable.SetRoutes(beforeLRPSchedulingInfo, afterLRP)
							Expect(routingEvents.Registrations).To(HaveLen(1))
							Expect(routingEvents.Unregistrations).To(HaveLen(1))

							ttl := 0
							Expect(routingEvents.Registrations).To(ConsistOf(tcpmodels.TcpRouteMapping{
								TcpMappingEntity: tcpmodels.TcpMappingEntity{
									RouterGroupGuid: "router-group-guid",
									HostPort:        62006,
									HostIP:          "some-ip-1",
									ExternalPort:    61000,
									TTL:             &ttl,
								},
							}))

							Expect(routingEvents.Unregistrations).To(ConsistOf(tcpmodels.TcpRouteMapping{
								TcpMappingEntity: tcpmodels.TcpMappingEntity{
									RouterGroupGuid: "router-group-guid",
									HostPort:        62004,
									HostIP:          "some-ip-1",
									ExternalPort:    61000,
									TTL:             &ttl,
								},
							}))
						})
					})
				})
			})

			Context("when an existing route is removed", func() {
				BeforeEach(func() {
					newTcpRoutes = tcp_routes.TCPRoutes{
						tcp_routes.TCPRoute{
							RouterGroupGuid: "router-group-guid",
							ExternalPort:    61000,
							ContainerPort:   5222,
						},
						tcp_routes.TCPRoute{
							RouterGroupGuid: "router-group-guid",
							ExternalPort:    62000,
							ContainerPort:   5222,
						},
					}
					routingTable = routingtable.NewRoutingTable(logger, false)
					beforeLRPSchedulingInfo = getDesiredLRP("process-guid-1", "log-guid-1", newTcpRoutes, modificationTag)
					routingTable.SetRoutes(nil, beforeLRPSchedulingInfo)
					routingTable.AddEndpoint(getActualLRP("process-guid-1", "instance-guid-1", "some-ip-1", "container-ip-1", 62004, 5222, false, modificationTag))
				})

				It("emits an unregistration event and keeps the other route", func() {
					afterLRP := getDesiredLRP("process-guid-1", "log-guid-1", tcpRoutes, newModificationTag)
					// currentRoutesCount := routingTable.RouteCount()
					routingEvents, _ := routingTable.SetRoutes(beforeLRPSchedulingInfo, afterLRP)
					Expect(routingEvents.Unregistrations).To(HaveLen(1))

					ttl := 0
					Expect(routingEvents.Unregistrations).To(ConsistOf(
						tcpmodels.TcpRouteMapping{
							TcpMappingEntity: tcpmodels.TcpMappingEntity{
								RouterGroupGuid: "router-group-guid",
								HostPort:        62004,
								HostIP:          "some-ip-1",
								ExternalPort:    62000,
								TTL:             &ttl,
							},
						},
					))
					Expect(routingTable.TCPAssociationsCount()).Should(Equal(1))
				})
			})
		})

		Describe("RemoveRoutes", func() {
			Context("when entry does not have endpoints", func() {
				var (
					desiredLRP *models.DesiredLRPSchedulingInfo
				)

				BeforeEach(func() {
					routingTable = routingtable.NewRoutingTable(logger, false)
					desiredLRP = getDesiredLRP("process-guid-1", "log-guid-1", tcpRoutes, modificationTag)
					routingTable.SetRoutes(nil, desiredLRP)
					Expect(routingTable.TCPAssociationsCount()).Should(Equal(0))
				})

				It("emits nothing", func() {
					modificationTag = &models.ModificationTag{Epoch: "abc", Index: 2}
					desiredLRP := getDesiredLRP("process-guid-1", "log-guid-1", tcpRoutes, modificationTag)
					routingEvents, _ := routingTable.RemoveRoutes(desiredLRP)
					Expect(routingEvents.Registrations).To(HaveLen(0))
					Expect(routingEvents.Unregistrations).To(HaveLen(0))
				})
			})

			Context("when entry does have endpoints", func() {
				BeforeEach(func() {
					tcpRoutes = tcp_routes.TCPRoutes{
						tcp_routes.TCPRoute{
							RouterGroupGuid: "router-group-guid",
							ExternalPort:    61000,
							ContainerPort:   5222,
						},
					}
					routingTable = routingtable.NewRoutingTable(logger, false)
					modificationTag := &models.ModificationTag{Epoch: "abc", Index: 1}
					desiredLRP := getDesiredLRP("process-guid-1", "log-guid-1", tcpRoutes, modificationTag)
					routingTable.SetRoutes(nil, desiredLRP)
					routingTable.AddEndpoint(getActualLRP("process-guid-1", "instance-guid-1", "some-ip-1", "container-ip-1", 61000, 5222, false, modificationTag))
					Expect(routingTable.TCPAssociationsCount()).Should(Equal(1))
				})

				It("emits unregistration routing events", func() {
					newModificationTag := &models.ModificationTag{Epoch: "abc", Index: 2}
					desiredLRP := getDesiredLRP("process-guid-1", "log-guid-1", tcpRoutes, newModificationTag)
					routingEvents, _ := routingTable.RemoveRoutes(desiredLRP)
					Expect(routingEvents.Unregistrations).To(HaveLen(1))
					ttl := 0
					Expect(routingEvents.Unregistrations).To(ConsistOf(tcpmodels.TcpRouteMapping{
						TcpMappingEntity: tcpmodels.TcpMappingEntity{
							RouterGroupGuid: "router-group-guid",
							HostPort:        61000,
							HostIP:          "some-ip-1",
							ExternalPort:    61000,
							TTL:             &ttl,
						},
					}))
					Expect(routingTable.TCPAssociationsCount()).Should(Equal(0))
				})

				Context("when there are no external endpoints", func() {
					BeforeEach(func() {
						routingTable = routingtable.NewRoutingTable(logger, false)
						modificationTag := &models.ModificationTag{Epoch: "abc", Index: 1}
						desiredLRP := getDesiredLRP("process-guid-1", "log-guid-1", tcpRoutes, modificationTag)
						routingTable.SetRoutes(nil, desiredLRP)
						routingTable.AddEndpoint(getActualLRP("process-guid-1", "instance-guid-1", "some-ip-1", "container-ip-1", 61000, 5222, false, modificationTag))
						Expect(routingTable.TCPAssociationsCount()).Should(Equal(1))
					})

					It("does not emit any routing events", func() {
						newModificationTag := &models.ModificationTag{Epoch: "abc", Index: 2}
						desiredLRP := getDesiredLRP("process-guid-1", "log-guid-1", tcp_routes.TCPRoutes{}, newModificationTag)
						routingEvents, _ := routingTable.RemoveRoutes(desiredLRP)
						Expect(routingEvents.Registrations).To(HaveLen(0))
						Expect(routingEvents.Unregistrations).To(HaveLen(0))
					})
				})
			})
		})

		Describe("AddEndpoint", func() {
			Context("with no existing endpoints", func() {
				BeforeEach(func() {
					routingTable = routingtable.NewRoutingTable(logger, false)
					beforeLRPSchedulingInfo := getDesiredLRP("process-guid-1", "log-guid-1", tcpRoutes, modificationTag)
					routingTable.SetRoutes(nil, beforeLRPSchedulingInfo)
					Expect(routingTable.TCPAssociationsCount()).Should(Equal(0))
				})

				It("emits routing events", func() {
					newTag := &models.ModificationTag{Epoch: "abc", Index: 1}
					actualLRP := getActualLRP("process-guid-1", "instance-guid-1", "some-ip-1", "container-ip-1", 61104, 5222, false, newTag)
					routingEvents, _ := routingTable.AddEndpoint(actualLRP)

					ttl := 0
					Expect(routingEvents.Registrations).To(ConsistOf(tcpmodels.TcpRouteMapping{
						TcpMappingEntity: tcpmodels.TcpMappingEntity{
							RouterGroupGuid: "router-group-guid",
							HostPort:        61104,
							HostIP:          "some-ip-1",
							ExternalPort:    61000,
							TTL:             &ttl,
						},
					}))
					Expect(routingTable.TCPAssociationsCount()).Should(Equal(1))
				})
			})

			Context("with existing endpoints", func() {
				BeforeEach(func() {
					routingTable = routingtable.NewRoutingTable(logger, false)
					beforeLRPSchedulingInfo := getDesiredLRP("process-guid-1", "log-guid-1", tcpRoutes, modificationTag)
					routingTable.SetRoutes(nil, beforeLRPSchedulingInfo)
					routingTable.AddEndpoint(getActualLRP("process-guid-1", "instance-guid-1", "some-ip-1", "container-ip-1", 62004, 5222, false, modificationTag))
					routingTable.AddEndpoint(getActualLRP("process-guid-1", "instance-guid-2", "some-ip-2", "container-ip-2", 62004, 5222, false, modificationTag))
					Expect(routingTable.TCPAssociationsCount()).Should(Equal(2))
				})

				Context("with different instance guid", func() {
					It("emits routing events", func() {
						newTag := &models.ModificationTag{Epoch: "abc", Index: 2}
						actualLRP := getActualLRP("process-guid-1", "instance-guid-3", "some-ip-3", "container-ip-3", 61104, 5222, false, newTag)
						routingEvents, _ := routingTable.AddEndpoint(actualLRP)
						ttl := 0
						Expect(routingEvents.Registrations).To(ConsistOf(tcpmodels.TcpRouteMapping{
							TcpMappingEntity: tcpmodels.TcpMappingEntity{
								RouterGroupGuid: "router-group-guid",
								HostPort:        61104,
								HostIP:          "some-ip-3",
								ExternalPort:    61000,
								TTL:             &ttl,
							},
						}))
						Expect(routingTable.TCPAssociationsCount()).Should(Equal(3))
					})
				})

				Context("with same instance guid", func() {
					Context("newer modification tag", func() {
						It("emits routing events", func() {
							newTag := &models.ModificationTag{Epoch: "abc", Index: 2}
							actualLRP := getActualLRP("process-guid-1", "instance-guid-1", "some-ip-1", "container-ip-1", 61105, 5222, false, newTag)
							routingEvents, _ := routingTable.AddEndpoint(actualLRP)
							ttl := 0
							Expect(routingEvents.Registrations).To(ConsistOf(tcpmodels.TcpRouteMapping{
								TcpMappingEntity: tcpmodels.TcpMappingEntity{
									RouterGroupGuid: "router-group-guid",
									HostPort:        61105,
									HostIP:          "some-ip-1",
									ExternalPort:    61000,
									TTL:             &ttl,
								},
							}))
							Expect(routingTable.TCPAssociationsCount()).Should(Equal(2))
						})
					})

					Context("older modification tag", func() {
						It("emits nothing", func() {
							olderTag := &models.ModificationTag{Epoch: "abc", Index: 0}
							actualLRP := getActualLRP("process-guid-1", "instance-guid-1", "some-ip-1", "container-ip-1", 61105, 5222, false, olderTag)
							routingEvents, _ := routingTable.AddEndpoint(actualLRP)
							Expect(routingEvents.Registrations).To(HaveLen(0))
							Expect(routingEvents.Unregistrations).To(HaveLen(0))
						})
					})
				})
			})
		})

		Describe("RemoveEndpoint", func() {
			Context("with no existing endpoints", func() {
				BeforeEach(func() {
					routingTable = routingtable.NewRoutingTable(logger, false)
					beforeLRPSchedulingInfo := getDesiredLRP("process-guid-1", "log-guid-1", tcpRoutes, modificationTag)
					routingTable.SetRoutes(nil, beforeLRPSchedulingInfo)
					Expect(routingTable.TCPAssociationsCount()).Should(Equal(0))
				})

				It("emits nothing", func() {
					newTag := &models.ModificationTag{Epoch: "abc", Index: 1}
					actualLRP := getActualLRP("process-guid-1", "instance-guid-1", "some-ip-1", "container-ip-1", 61104, 5222, false, newTag)
					routingEvents, _ := routingTable.RemoveEndpoint(actualLRP)
					Expect(routingEvents.Registrations).To(HaveLen(0))
					Expect(routingEvents.Unregistrations).To(HaveLen(0))
				})
			})

			Context("with existing endpoints", func() {
				BeforeEach(func() {
					routingTable = routingtable.NewRoutingTable(logger, false)
					beforeLRPSchedulingInfo := getDesiredLRP("process-guid-1", "log-guid-1", tcpRoutes, modificationTag)
					routingTable.SetRoutes(nil, beforeLRPSchedulingInfo)
					routingTable.AddEndpoint(getActualLRP("process-guid-1", "instance-guid-1", "some-ip-1", "container-ip-1", 62004, 5222, false, modificationTag))
					Expect(routingTable.TCPAssociationsCount()).Should(Equal(1))
				})

				Context("with instance guid not present in existing endpoints", func() {
					It("emits nothing", func() {
						newTag := &models.ModificationTag{Epoch: "abc", Index: 2}
						actualLRP := getActualLRP("process-guid-1", "instance-guid-3", "some-ip-3", "container-ip-3", 62004, 5222, false, newTag)
						routingEvents, _ := routingTable.RemoveEndpoint(actualLRP)
						Expect(routingEvents.Registrations).To(HaveLen(0))
						Expect(routingEvents.Unregistrations).To(HaveLen(0))
					})
				})

				Context("with same instance guid", func() {
					Context("newer modification tag", func() {
						It("emits routing events", func() {
							newTag := &models.ModificationTag{Epoch: "abc", Index: 2}
							actualLRP := getActualLRP("process-guid-1", "instance-guid-1", "some-ip-1", "container-ip-1", 62004, 5222, false, newTag)
							routingEvents, _ := routingTable.RemoveEndpoint(actualLRP)
							ttl := 0
							Expect(routingEvents.Unregistrations).To(ConsistOf(tcpmodels.TcpRouteMapping{
								TcpMappingEntity: tcpmodels.TcpMappingEntity{
									RouterGroupGuid: "router-group-guid",
									HostPort:        62004,
									HostIP:          "some-ip-1",
									ExternalPort:    61000,
									TTL:             &ttl,
								},
							}))

							Expect(routingTable.TCPAssociationsCount()).Should(Equal(0))
						})
					})

					Context("same modification tag", func() {
						It("emits routing events", func() {
							actualLRP := getActualLRP("process-guid-1", "instance-guid-1", "some-ip-1", "container-ip-1", 62004, 5222, false, modificationTag)
							routingEvents, _ := routingTable.RemoveEndpoint(actualLRP)
							ttl := 0
							Expect(routingEvents.Unregistrations).To(ConsistOf(tcpmodels.TcpRouteMapping{
								TcpMappingEntity: tcpmodels.TcpMappingEntity{
									RouterGroupGuid: "router-group-guid",
									HostPort:        62004,
									HostIP:          "some-ip-1",
									ExternalPort:    61000,
									TTL:             &ttl,
								},
							}))
							Expect(routingTable.TCPAssociationsCount()).Should(Equal(0))
						})
					})

					Context("older modification tag", func() {
						It("emits nothing", func() {
							olderTag := &models.ModificationTag{Epoch: "abc", Index: 0}
							actualLRP := getActualLRP("process-guid-1", "instance-guid-1", "some-ip-1", "container-ip-1", 62004, 5222, false, olderTag)
							routingEvents, _ := routingTable.RemoveEndpoint(actualLRP)
							Expect(routingEvents.Registrations).To(HaveLen(0))
							Expect(routingEvents.Unregistrations).To(HaveLen(0))
						})
					})
				})
			})
		})

		Describe("GetRoutingEvents", func() {
			BeforeEach(func() {
				routingTable = routingtable.NewRoutingTable(logger, false)
				beforeLRPSchedulingInfo := getDesiredLRP("process-guid-1", "log-guid-1", tcpRoutes, modificationTag)
				routingTable.SetRoutes(nil, beforeLRPSchedulingInfo)
				routingTable.AddEndpoint(getActualLRP("process-guid-1", "instance-guid-1", "some-ip-1", "container-ip-1", 62004, 5222, false, modificationTag))
				Expect(routingTable.TCPAssociationsCount()).Should(Equal(1))
			})

			It("returns routing events for entries in routing table", func() {
				routingEvents, _ := routingTable.GetRoutingEvents()
				ttl := 0
				Expect(routingEvents.Registrations).To(ConsistOf(tcpmodels.TcpRouteMapping{
					TcpMappingEntity: tcpmodels.TcpMappingEntity{
						RouterGroupGuid: "router-group-guid",
						HostPort:        62004,
						HostIP:          "some-ip-1",
						ExternalPort:    61000,
						TTL:             &ttl,
					},
				}))
				Expect(routingTable.TCPAssociationsCount()).Should(Equal(1))
			})
		})

		Describe("Swap", func() {
			var (
				tempRoutingTable   routingtable.RoutingTable
				newModificationTag *models.ModificationTag
				logGuid            string
				existingLogGuid    string
			)

			BeforeEach(func() {
				existingLogGuid = "log-guid-1"
				newModificationTag = &models.ModificationTag{Epoch: "abc", Index: 2}
				routingTable = routingtable.NewRoutingTable(logger, false)
				beforeLRPSchedulingInfo := getDesiredLRP("process-guid-1", existingLogGuid, tcpRoutes, modificationTag)
				routingTable.SetRoutes(nil, beforeLRPSchedulingInfo)
				routingTable.AddEndpoint(getActualLRP("process-guid-1", "instance-guid-1", "some-ip-1", "container-ip-1", 62004, 5222, false, modificationTag))
				routingTable.AddEndpoint(getActualLRP("process-guid-1", "instance-guid-2", "some-ip-2", "container-ip-2", 62004, 5222, false, modificationTag))
				Expect(routingTable.TCPAssociationsCount()).Should(Equal(2))
			})

			Context("when adding a new routing key (process-guid, container-port)", func() {

				BeforeEach(func() {
					logGuid = "log-guid-2"
					tempRoutingTable = routingtable.NewRoutingTable(logger, false)
					beforeLRPSchedulingInfo := getDesiredLRP("process-guid-2", logGuid, tcpRoutes, newModificationTag)
					tempRoutingTable.SetRoutes(nil, beforeLRPSchedulingInfo)
					tempRoutingTable.AddEndpoint(getActualLRP("process-guid-2", "instance-guid-1", "some-ip-3", "container-ip-3", 63004, 5222, false, newModificationTag))
					tempRoutingTable.AddEndpoint(getActualLRP("process-guid-2", "instance-guid-2", "some-ip-4", "container-ip-4", 63004, 5222, false, newModificationTag))
					Expect(tempRoutingTable.TCPAssociationsCount()).Should(Equal(2))
				})

				It("overwrites the existing entries and emits registration and unregistration routing events", func() {
					routingEvents, _ := routingTable.Swap(tempRoutingTable, models.DomainSet{})
					ttl := 0
					Expect(routingEvents.Unregistrations).To(ConsistOf(tcpmodels.TcpRouteMapping{
						TcpMappingEntity: tcpmodels.TcpMappingEntity{
							RouterGroupGuid: "router-group-guid",
							HostPort:        62004,
							HostIP:          "some-ip-1",
							ExternalPort:    61000,
							TTL:             &ttl,
						},
					}, tcpmodels.TcpRouteMapping{
						TcpMappingEntity: tcpmodels.TcpMappingEntity{
							RouterGroupGuid: "router-group-guid",
							HostPort:        62004,
							HostIP:          "some-ip-2",
							ExternalPort:    61000,
							TTL:             &ttl,
						},
					}))

					Expect(routingEvents.Registrations).To(ConsistOf(tcpmodels.TcpRouteMapping{
						TcpMappingEntity: tcpmodels.TcpMappingEntity{
							RouterGroupGuid: "router-group-guid",
							HostPort:        63004,
							HostIP:          "some-ip-3",
							ExternalPort:    61000,
							TTL:             &ttl,
						},
					}, tcpmodels.TcpRouteMapping{
						TcpMappingEntity: tcpmodels.TcpMappingEntity{
							RouterGroupGuid: "router-group-guid",
							HostPort:        63004,
							HostIP:          "some-ip-4",
							ExternalPort:    61000,
							TTL:             &ttl,
						},
					}))
				})
			})

			Context("when updating an existing routing key (process-guid, container-port)", func() {
				BeforeEach(func() {
					logGuid = "log-guid-2"
					tempRoutingTable = routingtable.NewRoutingTable(logger, false)
					beforeLRPSchedulingInfo := getDesiredLRP("process-guid-1", logGuid, tcpRoutes, newModificationTag)
					tempRoutingTable.SetRoutes(nil, beforeLRPSchedulingInfo)
					tempRoutingTable.AddEndpoint(getActualLRP("process-guid-1", "instance-guid-1", "some-ip-3", "container-ip-3", 63004, 5222, false, newModificationTag))
					tempRoutingTable.AddEndpoint(getActualLRP("process-guid-1", "instance-guid-2", "some-ip-4", "container-ip-4", 63004, 5222, false, newModificationTag))
					Expect(tempRoutingTable.TCPAssociationsCount()).Should(Equal(2))

				})

				It("overwrites the existing entries and emits registration and unregistration routing events", func() {
					routingEvents, _ := routingTable.Swap(tempRoutingTable, models.DomainSet{})
					ttl := 0
					Expect(routingEvents.Unregistrations).To(ConsistOf(tcpmodels.TcpRouteMapping{
						TcpMappingEntity: tcpmodels.TcpMappingEntity{
							RouterGroupGuid: "router-group-guid",
							HostPort:        62004,
							HostIP:          "some-ip-1",
							ExternalPort:    61000,
							TTL:             &ttl,
						},
					}, tcpmodels.TcpRouteMapping{
						TcpMappingEntity: tcpmodels.TcpMappingEntity{
							RouterGroupGuid: "router-group-guid",
							HostPort:        62004,
							HostIP:          "some-ip-2",
							ExternalPort:    61000,
							TTL:             &ttl,
						},
					}))

					Expect(routingEvents.Registrations).To(ConsistOf(tcpmodels.TcpRouteMapping{
						TcpMappingEntity: tcpmodels.TcpMappingEntity{
							RouterGroupGuid: "router-group-guid",
							HostPort:        63004,
							HostIP:          "some-ip-3",
							ExternalPort:    61000,
							TTL:             &ttl,
						},
					}, tcpmodels.TcpRouteMapping{
						TcpMappingEntity: tcpmodels.TcpMappingEntity{
							RouterGroupGuid: "router-group-guid",
							HostPort:        63004,
							HostIP:          "some-ip-4",
							ExternalPort:    61000,
							TTL:             &ttl,
						},
					}))
					Expect(routingTable.TCPAssociationsCount()).Should(Equal(2))
				})
			})

			Context("when updating the router group guid", func() {
				BeforeEach(func() {
					newModificationTag = &models.ModificationTag{Epoch: "abc", Index: 2}
					newTcpRoutes := tcp_routes.TCPRoutes{
						tcp_routes.TCPRoute{
							RouterGroupGuid: "new-router-group-guid",
							ExternalPort:    61000,
							ContainerPort:   5222,
						},
					}
					beforeLRPSchedulingInfo := getDesiredLRP("process-guid-1", existingLogGuid, newTcpRoutes, newModificationTag)
					tempRoutingTable = routingtable.NewRoutingTable(logger, false)
					tempRoutingTable.SetRoutes(nil, beforeLRPSchedulingInfo)
					tempRoutingTable.AddEndpoint(getActualLRP("process-guid-1", "instance-guid-1", "some-ip-1", "container-ip-1", 62004, 5222, false, modificationTag))
					tempRoutingTable.AddEndpoint(getActualLRP("process-guid-1", "instance-guid-2", "some-ip-2", "container-ip-2", 62004, 5222, false, modificationTag))

					Expect(tempRoutingTable.TCPAssociationsCount()).Should(Equal(2))
				})

				It("emits registration and unregistration events", func() {
					domains := models.DomainSet{}
					domains.Add("domain")
					routingEvents, _ := routingTable.Swap(tempRoutingTable, domains)

					ttl := 0
					Expect(routingEvents.Registrations).To(ConsistOf(tcpmodels.TcpRouteMapping{
						TcpMappingEntity: tcpmodels.TcpMappingEntity{
							RouterGroupGuid: "new-router-group-guid",
							HostPort:        62004,
							HostIP:          "some-ip-1",
							ExternalPort:    61000,
							TTL:             &ttl,
						},
					}, tcpmodels.TcpRouteMapping{
						TcpMappingEntity: tcpmodels.TcpMappingEntity{
							RouterGroupGuid: "new-router-group-guid",
							HostPort:        62004,
							HostIP:          "some-ip-2",
							ExternalPort:    61000,
							TTL:             &ttl,
						},
					}))
					Expect(routingEvents.Unregistrations).To(ConsistOf(tcpmodels.TcpRouteMapping{
						TcpMappingEntity: tcpmodels.TcpMappingEntity{
							RouterGroupGuid: "router-group-guid",
							HostPort:        62004,
							HostIP:          "some-ip-1",
							ExternalPort:    61000,
							TTL:             &ttl,
						},
					}, tcpmodels.TcpRouteMapping{
						TcpMappingEntity: tcpmodels.TcpMappingEntity{
							RouterGroupGuid: "router-group-guid",
							HostPort:        62004,
							HostIP:          "some-ip-2",
							ExternalPort:    61000,
							TTL:             &ttl,
						},
					}))
				})
			})
		})
	})
})
