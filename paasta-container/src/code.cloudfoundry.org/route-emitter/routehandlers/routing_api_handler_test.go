package routehandlers_test

import (
	"code.cloudfoundry.org/bbs/models"
	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/lager/lagertest"
	emitterfakes "code.cloudfoundry.org/route-emitter/emitter/fakes"
	"code.cloudfoundry.org/route-emitter/routehandlers"
	"code.cloudfoundry.org/route-emitter/routingtable"
	"code.cloudfoundry.org/route-emitter/routingtable/fakeroutingtable"
	tcpmodels "code.cloudfoundry.org/routing-api/models"
	"code.cloudfoundry.org/routing-info/tcp_routes"
	fake_metrics_sender "github.com/cloudfoundry/dropsonde/metric_sender/fake"
	"github.com/cloudfoundry/dropsonde/metrics"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("RoutingAPIHandler", func() {
	var (
		logger                lager.Logger
		fakeRoutingTable      *fakeroutingtable.FakeRoutingTable
		fakeRoutingAPIEmitter *emitterfakes.FakeRoutingAPIEmitter
		routeHandler          *routehandlers.Handler
		fakeMetricSender      *fake_metrics_sender.FakeMetricSender
		emptyNatsMessages     routingtable.MessagesToEmit
	)

	BeforeEach(func() {
		logger = lagertest.NewTestLogger("test")
		emptyNatsMessages = routingtable.MessagesToEmit{}
		fakeRoutingTable = new(fakeroutingtable.FakeRoutingTable)
		fakeRoutingAPIEmitter = new(emitterfakes.FakeRoutingAPIEmitter)
		routeHandler = routehandlers.NewHandler(fakeRoutingTable, nil, fakeRoutingAPIEmitter, false)

		fakeMetricSender = fake_metrics_sender.NewFakeMetricSender()
		metrics.Initialize(fakeMetricSender, nil)
	})

	Describe("DesiredLRP Event", func() {
		var (
			desiredLRP    *models.DesiredLRP
			routingEvents routingtable.TCPRouteMappings
		)

		BeforeEach(func() {
			externalPort := uint32(61000)
			containerPort := uint32(5222)
			tcpRoutes := tcp_routes.TCPRoutes{
				tcp_routes.TCPRoute{
					ExternalPort:  externalPort,
					ContainerPort: containerPort,
				},
			}
			desiredLRP = &models.DesiredLRP{
				ProcessGuid: "process-guid-1",
				Ports:       []uint32{containerPort},
				LogGuid:     "log-guid",
				Routes:      tcpRoutes.RoutingInfo(),
			}
			routingEvents = routingtable.TCPRouteMappings{
				Registrations: []tcpmodels.TcpRouteMapping{},
			}
		})

		Describe("HandleDesiredCreate", func() {
			JustBeforeEach(func() {
				routeHandler.HandleEvent(logger, models.NewDesiredLRPCreatedEvent(desiredLRP))
			})

			It("invokes AddRoutes on RoutingTable", func() {
				Expect(fakeRoutingTable.SetRoutesCallCount()).Should(Equal(1))
				before, after := fakeRoutingTable.SetRoutesArgsForCall(0)
				Expect(before).To(BeNil())
				Expect(*after).Should(Equal(desiredLRP.DesiredLRPSchedulingInfo()))
			})

			Context("when there are routing events", func() {
				BeforeEach(func() {
					fakeRoutingTable.SetRoutesReturns(routingEvents, emptyNatsMessages)
				})

				It("invokes Emit on Emitter", func() {
					Expect(fakeRoutingAPIEmitter.EmitCallCount()).Should(Equal(1))
					events := fakeRoutingAPIEmitter.EmitArgsForCall(0)
					Expect(events).Should(Equal(routingEvents))
				})
			})
		})

		Describe("HandleDesiredUpdate", func() {
			var after *models.DesiredLRP

			BeforeEach(func() {
				externalPort := uint32(62000)
				containerPort := uint32(5222)
				tcpRoutes := tcp_routes.TCPRoutes{
					tcp_routes.TCPRoute{
						ExternalPort:  externalPort,
						ContainerPort: containerPort,
					},
				}
				after = &models.DesiredLRP{
					ProcessGuid: "process-guid-1",
					Ports:       []uint32{containerPort},
					LogGuid:     "log-guid",
					Routes:      tcpRoutes.RoutingInfo(),
				}
			})

			JustBeforeEach(func() {
				routeHandler.HandleEvent(logger, models.NewDesiredLRPChangedEvent(desiredLRP, after))
			})

			It("invokes UpdateRoutes on RoutingTable", func() {
				Expect(fakeRoutingTable.SetRoutesCallCount()).Should(Equal(1))
				beforeLrp, afterLrp := fakeRoutingTable.SetRoutesArgsForCall(0)
				Expect(*beforeLrp).Should(Equal(desiredLRP.DesiredLRPSchedulingInfo()))
				Expect(*afterLrp).Should(Equal(after.DesiredLRPSchedulingInfo()))
			})

			Context("when there are routing events", func() {
				BeforeEach(func() {
					fakeRoutingTable.SetRoutesReturns(routingEvents, emptyNatsMessages)
				})

				It("invokes Emit on Emitter", func() {
					Expect(fakeRoutingAPIEmitter.EmitCallCount()).Should(Equal(1))
					events := fakeRoutingAPIEmitter.EmitArgsForCall(0)
					Expect(events).Should(Equal(routingEvents))
				})
			})
		})

		Describe("HandleDesiredDelete", func() {
			BeforeEach(func() {
				unregistrationEvent := routingtable.TCPRouteMappings{
					Unregistrations: []tcpmodels.TcpRouteMapping{},
				}
				fakeRoutingTable.RemoveRoutesReturns(unregistrationEvent, emptyNatsMessages)
			})
			JustBeforeEach(func() {
				routeHandler.HandleEvent(logger, models.NewDesiredLRPRemovedEvent(desiredLRP))
			})

			It("does not invoke AddRoutes on RoutingTable", func() {
				Expect(fakeRoutingTable.RemoveRoutesCallCount()).Should(Equal(1))
				Expect(fakeRoutingAPIEmitter.EmitCallCount()).Should(Equal(1))
				lrp := fakeRoutingTable.RemoveRoutesArgsForCall(0)
				Expect(*lrp).Should(Equal(desiredLRP.DesiredLRPSchedulingInfo()))
			})
		})
	})

	Describe("ActualLRP Event", func() {
		var (
			actualLRP     *models.ActualLRPGroup
			routingEvents routingtable.TCPRouteMappings
		)

		BeforeEach(func() {
			routingEvents = routingtable.TCPRouteMappings{
				Registrations: []tcpmodels.TcpRouteMapping{},
			}
		})

		Describe("HandleActualCreate", func() {
			JustBeforeEach(func() {
				routeHandler.HandleEvent(logger, models.NewActualLRPCreatedEvent(actualLRP))
			})

			Context("when state is Running", func() {
				BeforeEach(func() {
					actualLRP = &models.ActualLRPGroup{
						Instance: &models.ActualLRP{
							ActualLRPKey:         models.NewActualLRPKey("process-guid", 0, "domain"),
							ActualLRPInstanceKey: models.NewActualLRPInstanceKey("instance-guid", "cell-id"),
							ActualLRPNetInfo: models.NewActualLRPNetInfo(
								"some-ip",
								"container-ip",
								models.NewPortMapping(611006, 5222),
							),
							State: models.ActualLRPStateRunning,
						},
						Evacuating: nil,
					}
				})

				It("invokes AddEndpoint on RoutingTable", func() {
					Expect(fakeRoutingTable.AddEndpointCallCount()).Should(Equal(1))
					lrp := fakeRoutingTable.AddEndpointArgsForCall(0)
					Expect(lrp).Should(Equal(routingtable.NewActualLRPRoutingInfo(actualLRP)))
				})

				Context("when there are routing events", func() {
					BeforeEach(func() {
						fakeRoutingTable.AddEndpointReturns(routingEvents, emptyNatsMessages)
					})

					It("invokes Emit on Emitter", func() {
						Expect(fakeRoutingAPIEmitter.EmitCallCount()).Should(Equal(1))
						events := fakeRoutingAPIEmitter.EmitArgsForCall(0)
						Expect(events).Should(Equal(routingEvents))
					})
				})
			})

			Context("when state is not in Running", func() {
				BeforeEach(func() {
					actualLRP = &models.ActualLRPGroup{
						Instance: &models.ActualLRP{
							ActualLRPKey:         models.NewActualLRPKey("process-guid", 0, "domain"),
							ActualLRPInstanceKey: models.NewActualLRPInstanceKey("instance-guid", "cell-id"),
							ActualLRPNetInfo: models.NewActualLRPNetInfo(
								"some-ip",
								"container-ip",
								models.NewPortMapping(611006, 5222),
							),
							State: models.ActualLRPStateClaimed,
						},
						Evacuating: nil,
					}
				})

				It("does not invoke AddEndpoint on RoutingTable", func() {
					Expect(fakeRoutingTable.AddEndpointCallCount()).Should(Equal(0))
				})

				It("does not invoke Emit on Emitter", func() {
					Expect(fakeRoutingAPIEmitter.EmitCallCount()).Should(Equal(0))
				})
			})
		})

		Describe("HandleActualUpdate", func() {
			var (
				afterLRP *models.ActualLRPGroup
			)

			JustBeforeEach(func() {
				routeHandler.HandleEvent(logger, models.NewActualLRPChangedEvent(actualLRP, afterLRP))
			})

			Context("when after state is Running", func() {
				BeforeEach(func() {
					actualLRP = &models.ActualLRPGroup{
						Instance: &models.ActualLRP{
							ActualLRPKey:         models.NewActualLRPKey("process-guid", 0, "domain"),
							ActualLRPInstanceKey: models.NewActualLRPInstanceKey("instance-guid", "cell-id"),
							ActualLRPNetInfo: models.NewActualLRPNetInfo(
								"",
								"",
							),
							State: models.ActualLRPStateClaimed,
						},
						Evacuating: nil,
					}

					afterLRP = &models.ActualLRPGroup{
						Instance: &models.ActualLRP{
							ActualLRPKey:         models.NewActualLRPKey("process-guid", 0, "domain"),
							ActualLRPInstanceKey: models.NewActualLRPInstanceKey("instance-guid", "cell-id"),
							ActualLRPNetInfo: models.NewActualLRPNetInfo(
								"some-ip",
								"container-ip",
								models.NewPortMapping(611006, 5222),
							),
							State: models.ActualLRPStateRunning,
						},
						Evacuating: nil,
					}
				})

				It("invokes AddEndpoint on RoutingTable", func() {
					Expect(fakeRoutingTable.AddEndpointCallCount()).Should(Equal(1))
					lrp := fakeRoutingTable.AddEndpointArgsForCall(0)
					Expect(lrp.ActualLRP).Should(Equal(afterLRP.Instance))
				})

				Context("when there are routing events", func() {
					BeforeEach(func() {
						fakeRoutingTable.AddEndpointReturns(routingEvents, emptyNatsMessages)
					})

					It("invokes Emit on Emitter", func() {
						Expect(fakeRoutingAPIEmitter.EmitCallCount()).Should(Equal(1))
						events := fakeRoutingAPIEmitter.EmitArgsForCall(0)
						Expect(events).Should(Equal(routingEvents))
					})
				})
			})

			Context("when after state is not Running and before state is Running", func() {
				BeforeEach(func() {
					actualLRP = &models.ActualLRPGroup{
						Instance: &models.ActualLRP{
							ActualLRPKey:         models.NewActualLRPKey("process-guid", 0, "domain"),
							ActualLRPInstanceKey: models.NewActualLRPInstanceKey("instance-guid", "cell-id"),
							ActualLRPNetInfo: models.NewActualLRPNetInfo(
								"some-ip",
								"container-ip",
								models.NewPortMapping(611006, 5222),
							),
							State: models.ActualLRPStateRunning,
						},
						Evacuating: nil,
					}

					afterLRP = &models.ActualLRPGroup{
						Instance: &models.ActualLRP{
							ActualLRPKey:         models.NewActualLRPKey("process-guid", 0, "domain"),
							ActualLRPInstanceKey: models.NewActualLRPInstanceKey("instance-guid", "cell-id"),
							ActualLRPNetInfo: models.NewActualLRPNetInfo(
								"",
								"",
							),
							State: models.ActualLRPStateCrashed,
						},
						Evacuating: nil,
					}
				})

				It("invokes RemoveEndpoint on RoutingTable", func() {
					Expect(fakeRoutingTable.RemoveEndpointCallCount()).Should(Equal(1))
					lrp := fakeRoutingTable.RemoveEndpointArgsForCall(0)
					Expect(lrp).Should(Equal(routingtable.NewActualLRPRoutingInfo(actualLRP)))
				})

				Context("when there are routing events", func() {
					BeforeEach(func() {
						fakeRoutingTable.RemoveEndpointReturns(routingEvents, emptyNatsMessages)
					})

					It("invokes Emit on Emitter", func() {
						Expect(fakeRoutingAPIEmitter.EmitCallCount()).Should(Equal(1))
						events := fakeRoutingAPIEmitter.EmitArgsForCall(0)
						Expect(events).Should(Equal(routingEvents))
					})
				})
			})

			Context("when both after and before state is not Running", func() {
				BeforeEach(func() {
					actualLRP = &models.ActualLRPGroup{
						Instance: &models.ActualLRP{
							ActualLRPKey:         models.NewActualLRPKey("process-guid", 0, "domain"),
							ActualLRPInstanceKey: models.NewActualLRPInstanceKey("instance-guid", ""),
							ActualLRPNetInfo: models.NewActualLRPNetInfo(
								"",
								"",
							),
							State: models.ActualLRPStateUnclaimed,
						},
						Evacuating: nil,
					}

					afterLRP = &models.ActualLRPGroup{
						Instance: &models.ActualLRP{
							ActualLRPKey:         models.NewActualLRPKey("process-guid", 0, "domain"),
							ActualLRPInstanceKey: models.NewActualLRPInstanceKey("instance-guid", "cell-id"),
							ActualLRPNetInfo: models.NewActualLRPNetInfo(
								"",
								"",
							),
							State: models.ActualLRPStateClaimed,
						},
						Evacuating: nil,
					}
				})

				It("does not invoke AddEndpoint on RoutingTable", func() {
					Expect(fakeRoutingTable.AddEndpointCallCount()).Should(Equal(0))
				})

				It("does not invoke RemoveEndpoint on RoutingTable", func() {
					Expect(fakeRoutingTable.RemoveEndpointCallCount()).Should(Equal(0))
				})
			})
		})

		Describe("HandleActualDelete", func() {
			JustBeforeEach(func() {
				routeHandler.HandleEvent(logger, models.NewActualLRPRemovedEvent(actualLRP))
			})

			Context("when state is Running", func() {
				BeforeEach(func() {
					actualLRP = &models.ActualLRPGroup{
						Instance: &models.ActualLRP{
							ActualLRPKey:         models.NewActualLRPKey("process-guid", 0, "domain"),
							ActualLRPInstanceKey: models.NewActualLRPInstanceKey("instance-guid", "cell-id"),
							ActualLRPNetInfo: models.NewActualLRPNetInfo(
								"some-ip",
								"container-ip",
								models.NewPortMapping(611006, 5222),
							),
							State: models.ActualLRPStateRunning,
						},
						Evacuating: nil,
					}
				})

				It("invokes RemoveEndpoint on RoutingTable", func() {
					Expect(fakeRoutingTable.RemoveEndpointCallCount()).Should(Equal(1))
					lrp := fakeRoutingTable.RemoveEndpointArgsForCall(0)
					Expect(lrp).Should(Equal(routingtable.NewActualLRPRoutingInfo(actualLRP)))
				})

				Context("when there are routing events", func() {
					BeforeEach(func() {
						fakeRoutingTable.RemoveEndpointReturns(routingEvents, emptyNatsMessages)
					})

					It("invokes Emit on Emitter", func() {
						Expect(fakeRoutingAPIEmitter.EmitCallCount()).Should(Equal(1))
						events := fakeRoutingAPIEmitter.EmitArgsForCall(0)
						Expect(events).Should(Equal(routingEvents))
					})
				})
			})

			Context("when state is not in Running", func() {
				BeforeEach(func() {
					actualLRP = &models.ActualLRPGroup{
						Instance: &models.ActualLRP{
							ActualLRPKey:         models.NewActualLRPKey("process-guid", 0, "domain"),
							ActualLRPInstanceKey: models.NewActualLRPInstanceKey("instance-guid", "cell-id"),
							ActualLRPNetInfo: models.NewActualLRPNetInfo(
								"",
								"",
							),
							State: models.ActualLRPStateClaimed,
						},
						Evacuating: nil,
					}
				})

				It("does not invoke RemoveEndpoint on RoutingTable", func() {
					Expect(fakeRoutingTable.RemoveEndpointCallCount()).Should(Equal(0))
				})

				It("does not invoke Emit on Emitter", func() {
					Expect(fakeRoutingAPIEmitter.EmitCallCount()).Should(Equal(0))
				})
			})
		})
	})

	Describe("ShouldRefreshDesired", func() {
		var actualInfo *routingtable.ActualLRPRoutingInfo
		BeforeEach(func() {
			actualInfo = &routingtable.ActualLRPRoutingInfo{
				ActualLRP: &models.ActualLRP{
					ActualLRPKey:         models.NewActualLRPKey("process-guid-1", 0, "domain"),
					ActualLRPInstanceKey: models.NewActualLRPInstanceKey("instance-guid", "cell-id"),
					ActualLRPNetInfo: models.NewActualLRPNetInfo(
						"some-ip",
						"container-ip",
						models.NewPortMapping(61006, 5222),
						models.NewPortMapping(61007, 5223),
					),
					State:           models.ActualLRPStateRunning,
					ModificationTag: models.ModificationTag{Epoch: "abc", Index: 1},
				},
				Evacuating: false,
			}
		})

		Context("when corresponding desired state exists in the table", func() {
			BeforeEach(func() {
				fakeRoutingTable.HasExternalRoutesReturns(false)
			})

			It("returns false", func() {
				Expect(routeHandler.ShouldRefreshDesired(actualInfo)).To(BeTrue())
			})
		})

		Context("when corresponding desired state does not exist in the table", func() {
			BeforeEach(func() {
				fakeRoutingTable.HasExternalRoutesReturns(true)
			})

			It("returns true", func() {
				Expect(routeHandler.ShouldRefreshDesired(actualInfo)).To(BeFalse())
			})
		})
	})

	Describe("RefreshDesired", func() {
		BeforeEach(func() {
			fakeRoutingTable.SetRoutesReturns(routingtable.TCPRouteMappings{}, emptyNatsMessages)
		})

		It("adds the desired info to the routing table", func() {
			modificationTag := models.ModificationTag{Epoch: "abc", Index: 1}
			externalPort := uint32(61000)
			containerPort := uint32(5222)
			tcpRoutes := tcp_routes.TCPRoutes{
				tcp_routes.TCPRoute{
					RouterGroupGuid: "router-group-guid",
					ExternalPort:    externalPort,
					ContainerPort:   containerPort,
				},
			}
			desiredInfo := &models.DesiredLRPSchedulingInfo{
				DesiredLRPKey: models.DesiredLRPKey{
					ProcessGuid: "process-guid-1",
					LogGuid:     "log-guid",
				},
				Routes:          *tcpRoutes.RoutingInfo(),
				ModificationTag: modificationTag,
			}
			routeHandler.RefreshDesired(logger, []*models.DesiredLRPSchedulingInfo{desiredInfo})

			Expect(fakeRoutingTable.SetRoutesCallCount()).To(Equal(1))
			_, after := fakeRoutingTable.SetRoutesArgsForCall(0)
			Expect(after).To(Equal(desiredInfo))
			Expect(fakeRoutingAPIEmitter.EmitCallCount()).Should(Equal(1))
		})
	})

	Describe("Sync", func() {
		Context("when bbs server returns desired and actual lrps", func() {
			var (
				desiredInfo     []*models.DesiredLRPSchedulingInfo
				actualInfo      []*routingtable.ActualLRPRoutingInfo
				modificationTag models.ModificationTag
			)

			BeforeEach(func() {
				modificationTag = models.ModificationTag{Epoch: "abc", Index: 1}
				externalPort := uint32(61000)
				containerPort := uint32(5222)
				tcpRoutes := tcp_routes.TCPRoutes{
					tcp_routes.TCPRoute{
						RouterGroupGuid: "router-group-guid",
						ExternalPort:    externalPort,
						ContainerPort:   containerPort,
					},
				}

				desiredInfo = []*models.DesiredLRPSchedulingInfo{
					&models.DesiredLRPSchedulingInfo{
						DesiredLRPKey: models.DesiredLRPKey{
							ProcessGuid: "process-guid-1",
							LogGuid:     "log-guid",
						},
						Routes:          *tcpRoutes.RoutingInfo(),
						ModificationTag: modificationTag,
					},
				}

				actualInfo = []*routingtable.ActualLRPRoutingInfo{
					&routingtable.ActualLRPRoutingInfo{
						ActualLRP: &models.ActualLRP{
							ActualLRPKey:         models.NewActualLRPKey("process-guid-1", 0, "domain"),
							ActualLRPInstanceKey: models.NewActualLRPInstanceKey("instance-guid", "cell-id"),
							ActualLRPNetInfo: models.NewActualLRPNetInfo(
								"some-ip",
								"container-ip",
								models.NewPortMapping(61006, containerPort),
							),
							State:           models.ActualLRPStateRunning,
							ModificationTag: modificationTag,
						},
						Evacuating: false,
					},
				}

				fakeRoutingTable.SwapStub = func(t routingtable.RoutingTable, domains models.DomainSet) (routingtable.TCPRouteMappings, routingtable.MessagesToEmit) {
					return routingtable.TCPRouteMappings{}, emptyNatsMessages
				}
			})

			Context("when emitting metrics in localMode", func() {
				BeforeEach(func() {
					routeHandler = routehandlers.NewHandler(fakeRoutingTable, nil, fakeRoutingAPIEmitter, true)
					fakeRoutingTable.TCPRouteCountReturns(1)
				})

				It("emits the TCPRouteCount", func() {
					routeHandler.Sync(logger, desiredInfo, actualInfo, nil, nil)
					Expect(fakeMetricSender.GetValue("TCPRouteCount").Value).To(BeEquivalentTo(1))
				})
			})

			It("updates the routing table", func() {
				domains := models.DomainSet{}
				domains.Add("foo")
				routeHandler.Sync(logger, desiredInfo, actualInfo, domains, nil)
				Expect(fakeRoutingTable.SwapCallCount()).Should(Equal(1))
				tempRoutingTable, actualDomains := fakeRoutingTable.SwapArgsForCall(0)
				Expect(actualDomains).To(Equal(domains))
				Expect(tempRoutingTable.TCPAssociationsCount()).To(Equal(1))
				routingEvents, _ := tempRoutingTable.GetRoutingEvents()
				ttl := 0
				Expect(routingEvents.Registrations).To(ConsistOf(tcpmodels.TcpRouteMapping{
					TcpMappingEntity: tcpmodels.TcpMappingEntity{
						RouterGroupGuid: "router-group-guid",
						ExternalPort:    61000,
						HostPort:        61006,
						HostIP:          "some-ip",
						TTL:             &ttl,
					},
				}))

				Expect(fakeRoutingAPIEmitter.EmitCallCount()).Should(Equal(1))
			})

			Context("when events are cached", func() {
				BeforeEach(func() {
					tcpRoutes := tcp_routes.TCPRoutes{
						tcp_routes.TCPRoute{
							RouterGroupGuid: "router-group-guid",
							ExternalPort:    61007,
							ContainerPort:   5222,
						},
					}
					desiredLRPEvent := models.NewDesiredLRPCreatedEvent(&models.DesiredLRP{
						ProcessGuid: "process-guid-2",
						Routes:      tcpRoutes.RoutingInfo(),
						Instances:   1,
					})

					actualLRPEvent := models.NewActualLRPCreatedEvent(&models.ActualLRPGroup{
						Instance: &models.ActualLRP{
							ActualLRPKey:         models.NewActualLRPKey("process-guid-2", 0, "domain"),
							ActualLRPInstanceKey: models.NewActualLRPInstanceKey("instance-guid-1", "cell-id"),
							ActualLRPNetInfo: models.NewActualLRPNetInfo(
								"some-ip-2",
								"container-ip-2",
								models.NewPortMapping(61006, 5222),
							),
							State:           models.ActualLRPStateRunning,
							ModificationTag: modificationTag,
						},
					})

					cachedEvents := map[string]models.Event{
						desiredLRPEvent.Key(): desiredLRPEvent,
						actualLRPEvent.Key():  actualLRPEvent,
					}
					routeHandler.Sync(
						logger,
						desiredInfo,
						actualInfo,
						nil,
						cachedEvents,
					)
				})

				It("updates the routing table and emit cached events", func() {
					Expect(fakeRoutingTable.SwapCallCount()).Should(Equal(1))
					tempRoutingTable, _ := fakeRoutingTable.SwapArgsForCall(0)
					Expect(tempRoutingTable.TCPAssociationsCount()).Should(Equal(2))
					Expect(fakeRoutingAPIEmitter.EmitCallCount()).To(Equal(1))
				})
			})
		})
	})

	Describe("Emit", func() {
		var events routingtable.TCPRouteMappings
		BeforeEach(func() {
			events = routingtable.TCPRouteMappings{}
			fakeRoutingTable.GetRoutingEventsReturns(events, emptyNatsMessages)
		})

		It("emits all valid registration events", func() {
			routeHandler.Emit(logger)
			Expect(fakeRoutingTable.GetRoutingEventsCallCount()).To(Equal(1))
			Expect(fakeRoutingAPIEmitter.EmitCallCount()).To(Equal(1))
			Expect(fakeRoutingAPIEmitter.EmitArgsForCall(0)).To(Equal(events))
		})
	})
})
