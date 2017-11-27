package watcher_test

import (
	"errors"
	"os"
	"time"

	"code.cloudfoundry.org/bbs/events"
	"code.cloudfoundry.org/bbs/events/eventfakes"
	"code.cloudfoundry.org/bbs/fake_bbs"
	"code.cloudfoundry.org/bbs/models"
	"code.cloudfoundry.org/clock/fakeclock"
	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/lager/lagertest"
	"code.cloudfoundry.org/route-emitter/routingtable"
	"code.cloudfoundry.org/route-emitter/syncer"
	"code.cloudfoundry.org/route-emitter/watcher"
	"code.cloudfoundry.org/route-emitter/watcher/fakes"
	"code.cloudfoundry.org/routing-info/cfroutes"
	"code.cloudfoundry.org/routing-info/tcp_routes"
	fake_metrics_sender "github.com/cloudfoundry/dropsonde/metric_sender/fake"
	"github.com/cloudfoundry/dropsonde/metrics"
	"github.com/tedsuo/ifrit"
	"github.com/tedsuo/ifrit/ginkgomon"
	"github.com/vito/go-sse/sse"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
)

type EventHolder struct {
	event models.Event
}

var nilEventHolder = EventHolder{}

var _ = Describe("Watcher", func() {

	getDesiredLRP := func(processGuid, logGuid string,
		containerPort, externalPort uint32) *models.DesiredLRP {
		var desiredLRP models.DesiredLRP
		desiredLRP.ProcessGuid = processGuid
		desiredLRP.Ports = []uint32{containerPort}
		desiredLRP.LogGuid = logGuid
		tcpRoutes := tcp_routes.TCPRoutes{
			tcp_routes.TCPRoute{
				ExternalPort:  externalPort,
				ContainerPort: containerPort,
			},
		}
		desiredLRP.Routes = tcpRoutes.RoutingInfo()
		return &desiredLRP
	}

	getActualLRP := func(processGuid, instanceGuid, hostAddress, instanceAddress string,
		hostPort, containerPort uint32, evacuating bool) *models.ActualLRPGroup {
		if evacuating {
			return &models.ActualLRPGroup{
				Instance: nil,
				Evacuating: &models.ActualLRP{
					ActualLRPKey:         models.NewActualLRPKey(processGuid, 0, "domain"),
					ActualLRPInstanceKey: models.NewActualLRPInstanceKey(instanceGuid, "cell-id-1"),
					ActualLRPNetInfo: models.NewActualLRPNetInfo(
						hostAddress,
						instanceAddress,
						models.NewPortMapping(hostPort, containerPort),
					),
					State: models.ActualLRPStateRunning,
				},
			}
		} else {
			return &models.ActualLRPGroup{
				Instance: &models.ActualLRP{
					ActualLRPKey:         models.NewActualLRPKey(processGuid, 0, "domain"),
					ActualLRPInstanceKey: models.NewActualLRPInstanceKey(instanceGuid, "cell-id-1"),
					ActualLRPNetInfo: models.NewActualLRPNetInfo(
						hostAddress,
						instanceAddress,
						models.NewPortMapping(hostPort, containerPort),
					),
					State: models.ActualLRPStateRunning,
				},
				Evacuating: nil,
			}
		}
	}

	var (
		logger       *lagertest.TestLogger
		eventSource  *eventfakes.FakeEventSource
		bbsClient    *fake_bbs.FakeClient
		routeHandler *fakes.FakeRouteHandler
		testWatcher  *watcher.Watcher
		clock        *fakeclock.FakeClock
		process      ifrit.Process
		cellID       string
		syncEvents   syncer.Events
	)

	BeforeEach(func() {
		logger = lagertest.NewTestLogger("test-watcher")
		eventSource = new(eventfakes.FakeEventSource)
		bbsClient = new(fake_bbs.FakeClient)
		routeHandler = new(fakes.FakeRouteHandler)

		clock = fakeclock.NewFakeClock(time.Now())
		bbsClient.SubscribeToEventsByCellIDReturns(eventSource, nil)

		syncEvents = syncer.Events{
			Sync: make(chan struct{}),
			Emit: make(chan struct{}),
		}
		cellID = ""
	})

	JustBeforeEach(func() {
		testWatcher = watcher.NewWatcher(cellID, bbsClient, clock, routeHandler, syncEvents, logger)
		process = ifrit.Invoke(testWatcher)
	})

	AfterEach(func() {
		process.Signal(os.Interrupt)
		Eventually(process.Wait()).Should(Receive())
	})

	Context("event subscriptions", func() {
		Context("when cell id is set", func() {
			BeforeEach(func() {
				cellID = "some-cell-id"
			})

			It("subscribes to events for the current cell", func() {
				Eventually(bbsClient.SubscribeToEventsByCellIDCallCount).Should(Equal(1))
				_, cellid := bbsClient.SubscribeToEventsByCellIDArgsForCall(0)
				Expect(cellid).To(Equal("some-cell-id"))
			})
		})

		Context("when the cell id is not set", func() {
			BeforeEach(func() {
				cellID = ""
			})

			It("subscribes to all events", func() {
				Eventually(bbsClient.SubscribeToEventsByCellIDCallCount).Should(Equal(1))
				_, actualCellID := bbsClient.SubscribeToEventsByCellIDArgsForCall(0)
				Expect(actualCellID).To(Equal(""))
			})
		})
	})

	Context("handle DesiredLRPCreatedEvent", func() {
		var (
			event models.Event
		)

		BeforeEach(func() {
			desiredLRP := getDesiredLRP("process-guid-1", "log-guid-1", 5222, 61000)
			event = models.NewDesiredLRPCreatedEvent(desiredLRP)
			eventSource.NextReturns(event, nil)
		})

		It("calls routeHandler HandleEvent", func() {
			Eventually(routeHandler.HandleEventCallCount).Should(BeNumerically(">=", 1))
			_, createEvent := routeHandler.HandleEventArgsForCall(0)
			Expect(createEvent).Should(Equal(event))
		})
	})

	Context("handle DesiredLRPChangedEvent", func() {
		var (
			event models.Event
		)

		BeforeEach(func() {
			beforeLRP := getDesiredLRP("process-guid-1", "log-guid-1", 5222, 61000)
			afterLRP := getDesiredLRP("process-guid-1", "log-guid-1", 5222, 61001)
			event = models.NewDesiredLRPChangedEvent(beforeLRP, afterLRP)
			eventSource.NextReturns(event, nil)
		})

		It("calls routeHandler HandleEvent", func() {
			Eventually(routeHandler.HandleEventCallCount).Should(BeNumerically(">=", 1))
			_, changeEvent := routeHandler.HandleEventArgsForCall(0)
			Expect(changeEvent).Should(Equal(event))
		})
	})

	Context("handle DesiredLRPRemovedEvent", func() {
		var (
			event models.Event
		)

		BeforeEach(func() {
			desiredLRP := getDesiredLRP("process-guid-1", "log-guid-1", 5222, 61000)
			event = models.NewDesiredLRPRemovedEvent(desiredLRP)
			eventSource.NextReturns(event, nil)
		})

		It("calls routeHandler HandleDesiredDelete", func() {
			Eventually(routeHandler.HandleEventCallCount).Should(BeNumerically(">=", 1))
			_, deleteEvent := routeHandler.HandleEventArgsForCall(0)
			Expect(deleteEvent).Should(Equal(event))
		})
	})

	Context("handle ActualLRPRemovedEvent", func() {
		var (
			event models.Event
		)

		BeforeEach(func() {
			actualLRP := getActualLRP("process-guid-1", "instance-guid-1", "some-ip", "container-ip", 61000, 5222, false)
			event = models.NewActualLRPRemovedEvent(actualLRP)
			eventSource.NextReturns(event, nil)
		})

		It("calls routeHandler HandleActualCreate", func() {
			Eventually(routeHandler.HandleEventCallCount).Should(BeNumerically(">=", 1))
			_, createEvent := routeHandler.HandleEventArgsForCall(0)
			Expect(createEvent).Should(Equal(event))
		})

		Context("when the cell id is set", func() {
			Context("and doesn't match the event cell id", func() {
				BeforeEach(func() {
					cellID = "random-cell-id"
				})

				It("ignores the event", func() {
					Consistently(routeHandler.HandleEventCallCount).Should(BeZero())
				})
			})

			Context("and matches the event cell id", func() {
				BeforeEach(func() {
					cellID = "cell-id-1"
				})

				It("handles the event", func() {
					Eventually(routeHandler.HandleEventCallCount).Should(BeNumerically(">=", 1))
				})
			})
		})
	})

	Context("handle ActualLRPCreatedEvent", func() {
		var (
			event models.Event
		)

		BeforeEach(func() {
			actualLRP := getActualLRP("process-guid-1", "instance-guid-1", "some-ip", "container-ip", 61000, 5222, false)
			event = models.NewActualLRPCreatedEvent(actualLRP)
			eventSource.NextReturns(event, nil)
		})

		It("calls routeHandler HandleActualCreate", func() {
			Eventually(routeHandler.HandleEventCallCount).Should(BeNumerically(">=", 1))
			_, createEvent := routeHandler.HandleEventArgsForCall(0)
			Expect(createEvent).Should(Equal(event))
		})

		Context("when the cell id is set", func() {
			Context("and doesn't match the event cell id", func() {
				BeforeEach(func() {
					cellID = "random-cell-id"
				})

				It("ignores the event", func() {
					Consistently(routeHandler.HandleEventCallCount).Should(BeZero())
				})
			})

			Context("and matches the event cell id", func() {
				BeforeEach(func() {
					cellID = "cell-id-1"
				})

				It("handles the event", func() {
					Eventually(routeHandler.HandleEventCallCount).Should(BeNumerically(">=", 1))
				})
			})
		})
	})

	Context("handle ActualLRPChangedEvent", func() {
		var (
			event models.Event
		)

		BeforeEach(func() {
			beforeLRP := getActualLRP("process-guid-1", "instance-guid-1", "some-ip", "container-ip", 61000, 5222, false)
			afterLRP := getActualLRP("process-guid-1", "instance-guid-1", "some-ip", "container-ip", 61001, 5222, false)
			event = models.NewActualLRPChangedEvent(beforeLRP, afterLRP)
			eventSource.NextReturns(event, nil)
		})

		It("calls routeHandler HandleActualUpdate", func() {
			Eventually(routeHandler.HandleEventCallCount).Should(BeNumerically(">=", 1))
			_, changeEvent := routeHandler.HandleEventArgsForCall(0)
			Expect(changeEvent).Should(Equal(event))
		})

		Context("when the cell id is set", func() {
			Context("and doesn't match the event cell id", func() {
				BeforeEach(func() {
					cellID = "random-cell-id"
				})

				It("ignores the event", func() {
					Consistently(routeHandler.HandleEventCallCount).Should(BeZero())
				})
			})

			Context("and matches the event cell id", func() {
				BeforeEach(func() {
					cellID = "cell-id-1"
				})

				It("handles the event", func() {
					Eventually(routeHandler.HandleEventCallCount).Should(BeNumerically(">=", 1))
				})
			})
		})
	})

	Context("when an unrecognized event is received", func() {
		var (
			fakeRawEventSource *eventfakes.FakeRawEventSource
		)
		BeforeEach(func() {
			fakeRawEventSource = new(eventfakes.FakeRawEventSource)
			fakeEventSource := events.NewEventSource(fakeRawEventSource)

			fakeRawEventSource.NextReturns(
				sse.Event{
					ID:   "sup",
					Name: "unrecognized-event-type",
					Data: []byte("c3Nzcw=="),
				},
				nil,
			)

			bbsClient.SubscribeToEventsByCellIDReturns(fakeEventSource, nil)
			testWatcher = watcher.NewWatcher(cellID, bbsClient, clock, routeHandler, syncEvents, logger)
		})

		It("should not close the current connection", func() {
			Consistently(fakeRawEventSource.CloseCallCount).Should(Equal(0))
		})
	})

	Context("when eventSource returns error", func() {
		BeforeEach(func() {
			eventSource.NextReturns(nil, errors.New("bazinga..."))
		})

		It("closes the current event source", func() {
			Eventually(eventSource.CloseCallCount).Should(BeNumerically(">=", 1))
		})

		It("resubscribes to SSE from bbs", func() {
			Eventually(bbsClient.SubscribeToEventsByCellIDCallCount, 5*time.Second, 300*time.Millisecond).Should(BeNumerically(">=", 2))
			Eventually(logger).Should(gbytes.Say("event-source-error"))
		})
	})

	Context("when subscribe to events fails", func() {
		var (
			bbsErrorChannel chan error
		)
		BeforeEach(func() {
			bbsErrorChannel = make(chan error)

			bbsClient.SubscribeToEventsByCellIDStub = func(logger lager.Logger, cellID string) (events.EventSource, error) {
				select {
				case err := <-bbsErrorChannel:
					if err != nil {
						return nil, err
					}
				}
				return eventSource, nil
			}

			testWatcher = watcher.NewWatcher(cellID, bbsClient, clock, routeHandler, syncEvents, logger)
		})

		JustBeforeEach(func() {
			bbsErrorChannel <- errors.New("kaboom")
		})

		It("retries to subscribe", func() {
			close(bbsErrorChannel)
			Eventually(bbsClient.SubscribeToEventsByCellIDCallCount, 5*time.Second, 300*time.Millisecond).Should(Equal(2))
			Eventually(logger).Should(gbytes.Say("kaboom"))
		})
	})

	Describe("emit event", func() {
		It("emits registrations", func() {
			syncEvents.Emit <- struct{}{}
			Eventually(routeHandler.EmitCallCount).Should(Equal(1))
		})
	})

	Describe("Sync Events", func() {
		var (
			errCh            chan error
			eventCh          chan EventHolder
			fakeMetricSender *fake_metrics_sender.FakeMetricSender
		)

		BeforeEach(func() {
			errCh = make(chan error, 10)
			eventCh = make(chan EventHolder, 1)
			// make the variables local to avoid race detection
			nextErr := errCh
			nextEventValue := eventCh

			eventSource.CloseStub = func() error {
				nextErr <- errors.New("closed")
				return nil
			}

			eventSource.NextStub = func() (models.Event, error) {
				t := time.After(10 * time.Millisecond)
				select {
				case err := <-nextErr:
					return nil, err
				case x := <-nextEventValue:
					return x.event, nil
				case <-t:
					return nil, nil
				}
			}
			fakeMetricSender = fake_metrics_sender.NewFakeMetricSender()
			metrics.Initialize(fakeMetricSender, nil)
		})

		currentTag := &models.ModificationTag{Epoch: "abc", Index: 1}
		hostname1 := "foo.example.com"
		hostname2 := "bar.example.com"
		hostname3 := "baz.example.com"
		endpoint1 := routingtable.Endpoint{InstanceGUID: "ig-1", Host: "1.1.1.1", Index: 0, Port: 11, ContainerPort: 8080, Evacuating: false, ModificationTag: currentTag}
		endpoint2 := routingtable.Endpoint{InstanceGUID: "ig-2", Host: "2.2.2.2", Index: 0, Port: 22, ContainerPort: 8080, Evacuating: false, ModificationTag: currentTag}
		endpoint3 := routingtable.Endpoint{InstanceGUID: "ig-3", Host: "2.2.2.2", Index: 1, Port: 23, ContainerPort: 8080, Evacuating: false, ModificationTag: currentTag}

		schedulingInfo1 := &models.DesiredLRPSchedulingInfo{
			DesiredLRPKey: models.NewDesiredLRPKey("pg-1", "tests", "lg1"),
			Routes: cfroutes.CFRoutes{
				cfroutes.CFRoute{
					Hostnames:       []string{hostname1},
					Port:            8080,
					RouteServiceUrl: "https://rs.example.com",
				},
			}.RoutingInfo(),
			Instances: 1,
		}

		schedulingInfo2 := &models.DesiredLRPSchedulingInfo{
			DesiredLRPKey: models.NewDesiredLRPKey("pg-2", "tests", "lg2"),
			Routes: cfroutes.CFRoutes{
				cfroutes.CFRoute{
					Hostnames: []string{hostname2},
					Port:      8080,
				},
			}.RoutingInfo(),
			Instances: 1,
		}

		schedulingInfo3 := &models.DesiredLRPSchedulingInfo{
			DesiredLRPKey: models.NewDesiredLRPKey("pg-3", "tests", "lg3"),
			Routes: cfroutes.CFRoutes{
				cfroutes.CFRoute{
					Hostnames: []string{hostname3},
					Port:      8080,
				},
			}.RoutingInfo(),
			Instances: 1,
		}

		actualLRPGroup1 := &models.ActualLRPGroup{
			Instance: &models.ActualLRP{
				ActualLRPKey:         models.NewActualLRPKey("pg-1", 0, "domain"),
				ActualLRPInstanceKey: models.NewActualLRPInstanceKey(endpoint1.InstanceGUID, "cell-id"),
				ActualLRPNetInfo:     models.NewActualLRPNetInfo(endpoint1.Host, "container-ip", models.NewPortMapping(endpoint1.Port, endpoint1.ContainerPort)),
				State:                models.ActualLRPStateRunning,
			},
		}

		actualLRPGroup2 := &models.ActualLRPGroup{
			Instance: &models.ActualLRP{
				ActualLRPKey:         models.NewActualLRPKey("pg-2", 0, "domain"),
				ActualLRPInstanceKey: models.NewActualLRPInstanceKey(endpoint2.InstanceGUID, "cell-id"),
				ActualLRPNetInfo:     models.NewActualLRPNetInfo(endpoint2.Host, "container-ip", models.NewPortMapping(endpoint2.Port, endpoint2.ContainerPort)),
				State:                models.ActualLRPStateRunning,
			},
		}

		actualLRPGroup3 := &models.ActualLRPGroup{
			Instance: &models.ActualLRP{
				ActualLRPKey:         models.NewActualLRPKey("pg-3", 1, "domain"),
				ActualLRPInstanceKey: models.NewActualLRPInstanceKey(endpoint3.InstanceGUID, "cell-id"),
				ActualLRPNetInfo:     models.NewActualLRPNetInfo(endpoint3.Host, "container-ip", models.NewPortMapping(endpoint3.Port, endpoint3.ContainerPort)),
				State:                models.ActualLRPStateRunning,
			},
		}

		sendEvent := func() {
			Eventually(eventCh).Should(BeSent(EventHolder{models.NewActualLRPRemovedEvent(actualLRPGroup1)}))
		}

		JustBeforeEach(func() {
			syncEvents.Sync <- struct{}{}
		})

		Describe("bbs events", func() {
			BeforeEach(func() {
				bbsClient.ActualLRPGroupsStub = func(lager.Logger, models.ActualLRPFilter) ([]*models.ActualLRPGroup, error) {
					defer GinkgoRecover()
					sendEvent()
					Eventually(logger).Should(gbytes.Say("caching-event"))
					return nil, nil
				}
			})
			Context("when cell id is set", func() {
				BeforeEach(func() {
					cellID = "cell-id"
					endpoint4 := routingtable.Endpoint{InstanceGUID: "ig-4", Host: "2.2.2.3", Index: 1, Port: 23, ContainerPort: 8080, Evacuating: false, ModificationTag: currentTag}
					actualLRPGroup4 := &models.ActualLRPGroup{
						Instance: &models.ActualLRP{
							ActualLRPKey:         models.NewActualLRPKey("pg-2", 1, "domain"),
							ActualLRPInstanceKey: models.NewActualLRPInstanceKey(endpoint4.InstanceGUID, "cell-id4"),
							ActualLRPNetInfo:     models.NewActualLRPNetInfo(endpoint4.Host, "container-ip", models.NewPortMapping(endpoint4.Port, endpoint4.ContainerPort)),
							State:                models.ActualLRPStateRunning,
						},
					}
					bbsClient.ActualLRPGroupsStub = func(lager.Logger, models.ActualLRPFilter) ([]*models.ActualLRPGroup, error) {
						defer GinkgoRecover()
						Eventually(eventCh).Should(BeSent(EventHolder{models.NewActualLRPRemovedEvent(actualLRPGroup1)}))
						Eventually(eventCh).Should(BeSent(EventHolder{models.NewActualLRPRemovedEvent(actualLRPGroup4)}))
						Eventually(logger).Should(gbytes.Say("caching-event"))
						return nil, nil
					}
				})

				It("applies cached events associated only with the cell after syncing is complete", func() {
					Eventually(routeHandler.SyncCallCount).Should(Equal(1))
					_, _, _, _, event := routeHandler.SyncArgsForCall(0)

					Expect(event).To(HaveLen(1))
					expectedEvent := models.NewActualLRPRemovedEvent(actualLRPGroup1)
					Expect(event[actualLRPGroup1.Instance.InstanceGuid]).To(Equal(expectedEvent))
				})
			})

			It("caches events", func() {
				Consistently(routeHandler.HandleEventCallCount).Should(Equal(0))
			})

			It("applies cached events after syncing is complete", func() {
				Eventually(routeHandler.SyncCallCount).Should(Equal(1))
				_, _, _, _, event := routeHandler.SyncArgsForCall(0)

				expectedEvent := models.NewActualLRPRemovedEvent(actualLRPGroup1)
				Expect(event[actualLRPGroup1.Instance.InstanceGuid]).To(Equal(expectedEvent))
			})
		})

		Context("during sync", func() {
			var (
				unblock chan struct{}
			)

			BeforeEach(func() {
				unblock = make(chan struct{})
				bbsClient.ActualLRPGroupsStub = func(lager.Logger, models.ActualLRPFilter) ([]*models.ActualLRPGroup, error) {
					<-unblock
					return nil, nil
				}
			})

			AfterEach(func() {
				close(unblock)
			})

			It("ignores a sync event", func() {
				Eventually(syncEvents.Sync).Should(BeSent(struct{}{}))
				Eventually(logger).Should(gbytes.Say("sync-already-in-progress"))
			})

			It("can be signaled", func() {
				ginkgomon.Interrupt(process)
			})
		})

		Context("when fetching actuals fails", func() {
			var (
				errCh chan error
			)

			BeforeEach(func() {
				errCh = make(chan error, 1)
				errCh <- errors.New("bam")
				bbsClient.ActualLRPGroupsStub = func(lager.Logger, models.ActualLRPFilter) ([]*models.ActualLRPGroup, error) {
					return []*models.ActualLRPGroup{}, <-errCh
				}
			})

			It("should not call sync until the error resolves", func() {
				Eventually(bbsClient.ActualLRPGroupsCallCount).Should(Equal(1))
				Consistently(routeHandler.SyncCallCount).Should(Equal(0))

				// return no errors
				close(errCh)
				syncEvents.Sync <- struct{}{}

				Eventually(routeHandler.SyncCallCount).Should(Equal(1))
				Expect(bbsClient.ActualLRPGroupsCallCount()).To(Equal(2))
			})
		})

		Context("when fetching desireds fails", func() {
			var (
				errCh chan error
			)

			BeforeEach(func() {
				errCh = make(chan error, 1)
				errCh <- errors.New("bam")

				bbsClient.DesiredLRPSchedulingInfosStub = func(lager.Logger, models.DesiredLRPFilter) ([]*models.DesiredLRPSchedulingInfo, error) {
					return []*models.DesiredLRPSchedulingInfo{}, <-errCh
				}
			})

			It("should not call sync until the error resolves", func() {
				Eventually(bbsClient.DesiredLRPSchedulingInfosCallCount).Should(Equal(1))
				Consistently(routeHandler.SyncCallCount).Should(Equal(0))

				// return no errors
				close(errCh)
				syncEvents.Sync <- struct{}{}

				Eventually(routeHandler.SyncCallCount).Should(Equal(1))
				Expect(bbsClient.DesiredLRPSchedulingInfosCallCount()).To(Equal(2))
			})
		})

		Context("when fetching domains fails", func() {
			var (
				errCh chan error
			)

			BeforeEach(func() {
				errCh = make(chan error, 1)
				errCh <- errors.New("bam")

				bbsClient.DomainsStub = func(lager.Logger) ([]string, error) {
					return []string{}, <-errCh
				}
			})

			It("should not call sync until the error resolves", func() {
				Eventually(bbsClient.DomainsCallCount).Should(Equal(1))
				Consistently(routeHandler.SyncCallCount).Should(Equal(0))

				// return no errors
				close(errCh)
				syncEvents.Sync <- struct{}{}

				Eventually(routeHandler.SyncCallCount).Should(Equal(1))
				Expect(bbsClient.DomainsCallCount()).To(Equal(2))
			})

			It("does not emit the sync duration metric", func() {
				Consistently(func() float64 {
					return fakeMetricSender.GetValue("RouteEmitterSyncDuration").Value
				}).Should(BeZero())
			})
		})

		Context("when desired lrps are retrieved", func() {
			BeforeEach(func() {
				bbsClient.ActualLRPGroupsStub = func(logger lager.Logger, f models.ActualLRPFilter) ([]*models.ActualLRPGroup, error) {
					clock.IncrementBySeconds(1)

					return []*models.ActualLRPGroup{
						actualLRPGroup1,
						actualLRPGroup2,
						actualLRPGroup3,
					}, nil
				}

				bbsClient.DesiredLRPSchedulingInfosStub = func(logger lager.Logger, f models.DesiredLRPFilter) ([]*models.DesiredLRPSchedulingInfo, error) {
					defer GinkgoRecover()

					return []*models.DesiredLRPSchedulingInfo{schedulingInfo1, schedulingInfo2}, nil
				}
			})

			It("calls RouteHandler Sync with correct arguments", func() {
				expectedDesired := []*models.DesiredLRPSchedulingInfo{
					schedulingInfo1,
					schedulingInfo2,
				}
				expectedActuals := []*routingtable.ActualLRPRoutingInfo{
					routingtable.NewActualLRPRoutingInfo(actualLRPGroup1),
					routingtable.NewActualLRPRoutingInfo(actualLRPGroup2),
					routingtable.NewActualLRPRoutingInfo(actualLRPGroup3),
				}

				expectedDomains := models.DomainSet{}
				Eventually(routeHandler.SyncCallCount).Should(Equal(1))
				_, desired, actuals, domains, _ := routeHandler.SyncArgsForCall(0)

				Expect(domains).To(Equal(expectedDomains))
				Expect(desired).To(Equal(expectedDesired))
				Expect(actuals).To(Equal(expectedActuals))
			})

			It("should emit the sync duration, and allow event processing", func() {
				Eventually(func() float64 {
					return fakeMetricSender.GetValue("RouteEmitterSyncDuration").Value
				}).Should(BeNumerically(">=", 100*time.Millisecond))

				By("completing, events are no longer cached")
				sendEvent()

				Eventually(routeHandler.HandleEventCallCount).Should(Equal(1))
			})

			It("gets all the desired lrps", func() {
				Eventually(bbsClient.DesiredLRPSchedulingInfosCallCount).Should(Equal(1))
				_, filter := bbsClient.DesiredLRPSchedulingInfosArgsForCall(0)
				Expect(filter.ProcessGuids).To(BeEmpty())
			})
		})

		Context("when the cell id is set", func() {
			BeforeEach(func() {
				cellID = "cell-id"
				actualLRPGroup2.Instance.ActualLRPInstanceKey.CellId = cellID

				testWatcher = watcher.NewWatcher(cellID, bbsClient, clock, routeHandler, syncEvents, logger)
			})

			Context("when the cell has actual lrps running", func() {
				BeforeEach(func() {
					bbsClient.ActualLRPGroupsStub = func(lager.Logger, models.ActualLRPFilter) ([]*models.ActualLRPGroup, error) {
						clock.IncrementBySeconds(1)

						return []*models.ActualLRPGroup{
							actualLRPGroup2,
						}, nil
					}

					bbsClient.DesiredLRPSchedulingInfosStub = func(lager.Logger, models.DesiredLRPFilter) ([]*models.DesiredLRPSchedulingInfo, error) {
						return []*models.DesiredLRPSchedulingInfo{schedulingInfo2}, nil
					}
				})

				It("calls Sync method with correct desired lrps", func() {
					Eventually(routeHandler.SyncCallCount).Should(Equal(1))
					_, desired, _, _, _ := routeHandler.SyncArgsForCall(0)
					Expect(desired).To(ContainElement(schedulingInfo2))
					Eventually(bbsClient.DesiredLRPSchedulingInfosCallCount).Should(Equal(1))
				})

				It("registers endpoints for lrps on this cell", func() {
					Eventually(routeHandler.SyncCallCount).Should(Equal(1))
					_, _, actual, _, _ := routeHandler.SyncArgsForCall(0)
					routingInfo2 := routingtable.NewActualLRPRoutingInfo(actualLRPGroup2)
					Expect(actual).To(ContainElement(routingInfo2))
				})

				It("fetches actual lrps that match the cell id", func() {
					Eventually(bbsClient.ActualLRPGroupsCallCount).Should(Equal(1))
					_, filter := bbsClient.ActualLRPGroupsArgsForCall(0)
					Expect(filter.CellID).To(Equal(cellID))
				})

				It("fetches desired lrp scheduling info that match the cell id", func() {
					Eventually(bbsClient.DesiredLRPSchedulingInfosCallCount).Should(Equal(1))
					_, filter := bbsClient.DesiredLRPSchedulingInfosArgsForCall(0)
					lrp, _ := actualLRPGroup2.Resolve()
					Expect(filter.ProcessGuids).To(ConsistOf(lrp.ProcessGuid))
				})
			})

			Context("when desired lrp for the actual lrp is missing", func() {
				sendEvent := func() {
					beforeActualLRPGroup3 := &models.ActualLRPGroup{
						Instance: &models.ActualLRP{
							ActualLRPKey:         models.NewActualLRPKey("pg-3", 1, "domain"),
							ActualLRPInstanceKey: models.NewActualLRPInstanceKey(endpoint3.InstanceGUID, "cell-id"),
							State:                models.ActualLRPStateClaimed,
						},
					}
					Eventually(eventCh).Should(BeSent(EventHolder{models.NewActualLRPChangedEvent(
						beforeActualLRPGroup3,
						actualLRPGroup3,
					)}))
				}

				BeforeEach(func() {
					bbsClient.DesiredLRPSchedulingInfosStub = func(_ lager.Logger, f models.DesiredLRPFilter) ([]*models.DesiredLRPSchedulingInfo, error) {
						defer GinkgoRecover()
						if len(f.ProcessGuids) == 1 && f.ProcessGuids[0] == "pg-3" {
							return []*models.DesiredLRPSchedulingInfo{schedulingInfo3}, nil
						}
						return []*models.DesiredLRPSchedulingInfo{schedulingInfo1}, nil
					}

					routeHandler.ShouldRefreshDesiredReturns(true)
				})

				Context("when a running actual lrp event is received", func() {
					BeforeEach(func() {
						cellID = "cell-id"

						bbsClient.ActualLRPGroupsStub = func(logger lager.Logger, f models.ActualLRPFilter) ([]*models.ActualLRPGroup, error) {
							clock.IncrementBySeconds(1)
							return []*models.ActualLRPGroup{actualLRPGroup1}, nil
						}
					})

					JustBeforeEach(func() {
						Eventually(routeHandler.SyncCallCount).Should(Equal(1))
						sendEvent()
					})

					It("fetches the desired lrp and passes it to the route handler", func() {
						Eventually(bbsClient.DesiredLRPSchedulingInfosCallCount).Should(Equal(2))

						_, filter := bbsClient.DesiredLRPSchedulingInfosArgsForCall(1)
						lrp, _ := actualLRPGroup3.Resolve()

						Expect(filter.ProcessGuids).To(HaveLen(1))
						Expect(filter.ProcessGuids).To(ConsistOf(lrp.ProcessGuid))

						Eventually(routeHandler.ShouldRefreshDesiredCallCount).Should(Equal(1))
						Eventually(routeHandler.RefreshDesiredCallCount).Should(Equal(1))
						_, desiredInfo := routeHandler.RefreshDesiredArgsForCall(0)
						Expect(desiredInfo).To(ContainElement(schedulingInfo3))

						Eventually(routeHandler.HandleEventCallCount).Should(Equal(1))
					})
				})

				Context("and the event is cached", func() {
					BeforeEach(func() {
						bbsClient.ActualLRPGroupsStub = func(lager.Logger, models.ActualLRPFilter) ([]*models.ActualLRPGroup, error) {
							clock.IncrementBySeconds(1)
							defer GinkgoRecover()
							sendEvent()
							Eventually(logger).Should(gbytes.Say("caching-event"))
							return []*models.ActualLRPGroup{actualLRPGroup1}, nil
						}
					})

					It("fetches the desired lrp and refreshes the handler", func() {
						Eventually(bbsClient.DesiredLRPSchedulingInfosCallCount).Should(Equal(2))

						_, filter := bbsClient.DesiredLRPSchedulingInfosArgsForCall(1)
						lrp, _ := actualLRPGroup3.Resolve()

						Expect(filter.ProcessGuids).To(HaveLen(1))
						Expect(filter.ProcessGuids).To(ConsistOf(lrp.ProcessGuid))

						Eventually(routeHandler.ShouldRefreshDesiredCallCount).Should(Equal(1))
						Eventually(routeHandler.SyncCallCount).Should(Equal(1))
						_, desiredInfo, _, _, _ := routeHandler.SyncArgsForCall(0)
						Expect(desiredInfo).To(ContainElement(schedulingInfo3))
					})

					Context("and fetching desired scheduling info fails", func() {
						BeforeEach(func() {
							bbsClient.DesiredLRPSchedulingInfosStub = func(lager.Logger, models.DesiredLRPFilter) ([]*models.DesiredLRPSchedulingInfo, error) {
								return nil, errors.New("boom")
							}
						})

						It("does not refresh the desired state", func() {
							Eventually(routeHandler.ShouldRefreshDesiredCallCount).Should(Equal(1))
							Eventually(bbsClient.DesiredLRPSchedulingInfosCallCount).Should(Equal(2))
							Consistently(routeHandler.RefreshDesiredCallCount).Should(Equal(0))
						})
					})
				})

				Context("when fetching desired scheduling info fails", func() {
					BeforeEach(func() {
						bbsClient.ActualLRPGroupsStub = func(lager.Logger, models.ActualLRPFilter) ([]*models.ActualLRPGroup, error) {
							defer GinkgoRecover()
							sendEvent()
							Eventually(logger).Should(gbytes.Say("caching-event"))
							return []*models.ActualLRPGroup{actualLRPGroup1}, nil
						}
						bbsClient.DesiredLRPSchedulingInfosStub = func(lager.Logger, models.DesiredLRPFilter) ([]*models.DesiredLRPSchedulingInfo, error) {
							return nil, errors.New("blam")
						}
					})

					It("does not refresh the desired state", func() {
						Eventually(routeHandler.ShouldRefreshDesiredCallCount).Should(Equal(1))
						Eventually(bbsClient.DesiredLRPSchedulingInfosCallCount).Should(Equal(2))
						Consistently(routeHandler.RefreshDesiredCallCount).Should(Equal(0))
					})
				})
			})

			Context("when there are no running actual lrps on the cell", func() {
				BeforeEach(func() {
					bbsClient.ActualLRPGroupsStub = func(logger lager.Logger, f models.ActualLRPFilter) ([]*models.ActualLRPGroup, error) {
						return []*models.ActualLRPGroup{}, nil
					}
				})

				It("does not fetch any desired lrp scheduling info", func() {
					Consistently(bbsClient.DesiredLRPSchedulingInfosCallCount).Should(Equal(0))
				})
			})
		})

		Context("when actual lrp state is not running", func() {
			BeforeEach(func() {
				actualLRPGroup4 := &models.ActualLRPGroup{
					Instance: &models.ActualLRP{
						ActualLRPKey:         models.NewActualLRPKey("pg-4", 1, "domain"),
						ActualLRPInstanceKey: models.NewActualLRPInstanceKey(endpoint3.InstanceGUID, "cell-id"),
						State:                models.ActualLRPStateClaimed,
					},
				}

				Eventually(eventCh).Should(BeSent(EventHolder{models.NewActualLRPCreatedEvent(
					actualLRPGroup4,
				)}))
			})

			It("should not refresh desired lrps", func() {
				Consistently(routeHandler.ShouldRefreshDesiredCallCount).Should(Equal(0))
				Consistently(routeHandler.RefreshDesiredCallCount).Should(Equal(0))
			})
		})
	})
})
