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
	"code.cloudfoundry.org/routing-info/tcp_routes"
	"code.cloudfoundry.org/tcp-emitter/routing_table/fakes"
	"code.cloudfoundry.org/tcp-emitter/watcher"
	"github.com/tedsuo/ifrit"
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

	getActualLRP := func(processGuid, instanceGuid, hostAddress string,
		hostPort, containerPort uint32, evacuating bool) *models.ActualLRPGroup {
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
						"1.2.3.4",
						models.NewPortMapping(hostPort, containerPort),
					),
					State: models.ActualLRPStateRunning,
				},
				Evacuating: nil,
			}
		}
	}

	var (
		eventSource         *eventfakes.FakeEventSource
		bbsClient           *fake_bbs.FakeClient
		routingTableHandler *fakes.FakeRoutingTableHandler
		testWatcher         *watcher.Watcher
		clock               *fakeclock.FakeClock
		process             ifrit.Process
		syncChannel         chan struct{}
	)

	BeforeEach(func() {
		eventSource = new(eventfakes.FakeEventSource)
		bbsClient = new(fake_bbs.FakeClient)
		routingTableHandler = new(fakes.FakeRoutingTableHandler)

		clock = fakeclock.NewFakeClock(time.Now())
		bbsClient.SubscribeToEventsReturns(eventSource, nil)

		syncChannel = make(chan struct{})
		testWatcher = watcher.NewWatcher(bbsClient, clock, routingTableHandler, syncChannel, logger)
	})

	JustBeforeEach(func() {
		process = ifrit.Invoke(testWatcher)
	})

	AfterEach(func() {
		process.Signal(os.Interrupt)
		Eventually(process.Wait()).Should(Receive())
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

		It("calls routingTableHandler HandleEvent", func() {
			Eventually(routingTableHandler.HandleEventCallCount).Should(BeNumerically(">=", 1))
			createEvent := routingTableHandler.HandleEventArgsForCall(0)
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

		It("calls routingTableHandler HandleEvent", func() {
			Eventually(routingTableHandler.HandleEventCallCount).Should(BeNumerically(">=", 1))
			changeEvent := routingTableHandler.HandleEventArgsForCall(0)
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

		It("calls routingTableHandler HandleDesiredDelete", func() {
			Eventually(routingTableHandler.HandleEventCallCount).Should(BeNumerically(">=", 1))
			deleteEvent := routingTableHandler.HandleEventArgsForCall(0)
			Expect(deleteEvent).Should(Equal(event))
		})
	})

	Context("handle ActualLRPCreatedEvent", func() {
		var (
			event models.Event
		)

		BeforeEach(func() {
			actualLRP := getActualLRP("process-guid-1", "instance-guid-1", "some-ip", 61000, 5222, false)
			event = models.NewActualLRPCreatedEvent(actualLRP)
			eventSource.NextReturns(event, nil)
		})

		It("calls routingTableHandler HandleActualCreate", func() {
			Eventually(routingTableHandler.HandleEventCallCount).Should(BeNumerically(">=", 1))
			createEvent := routingTableHandler.HandleEventArgsForCall(0)
			Expect(createEvent).Should(Equal(event))
		})
	})

	Context("handle ActualLRPChangedEvent", func() {
		var (
			event models.Event
		)

		BeforeEach(func() {
			beforeLRP := getActualLRP("process-guid-1", "instance-guid-1", "some-ip", 61000, 5222, false)
			afterLRP := getActualLRP("process-guid-1", "instance-guid-1", "some-ip", 61001, 5222, false)
			event = models.NewActualLRPChangedEvent(beforeLRP, afterLRP)
			eventSource.NextReturns(event, nil)
		})

		It("calls routingTableHandler HandleActualUpdate", func() {
			Eventually(routingTableHandler.HandleEventCallCount).Should(BeNumerically(">=", 1))
			changeEvent := routingTableHandler.HandleEventArgsForCall(0)
			Expect(changeEvent).Should(Equal(event))
		})
	})

	Context("handle Sync Event", func() {
		JustBeforeEach(func() {
			syncChannel <- struct{}{}
		})

		It("calls routingTableHandler HandleSync", func() {
			Eventually(routingTableHandler.SyncCallCount).Should(Equal(1))
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

			bbsClient.SubscribeToEventsReturns(fakeEventSource, nil)
			testWatcher = watcher.NewWatcher(bbsClient, clock, routingTableHandler, syncChannel, logger)
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
			Eventually(bbsClient.SubscribeToEventsCallCount, 5*time.Second, 300*time.Millisecond).Should(BeNumerically(">=", 2))
			Eventually(logger).Should(gbytes.Say("event-source-error"))
		})
	})

	Context("when subscribe to events fails", func() {
		var (
			bbsErrorChannel chan error
		)
		BeforeEach(func() {
			bbsErrorChannel = make(chan error)

			bbsClient.SubscribeToEventsStub = func(logger lager.Logger) (events.EventSource, error) {
				select {
				case err := <-bbsErrorChannel:
					if err != nil {
						return nil, err
					}
				}
				return eventSource, nil
			}

			testWatcher = watcher.NewWatcher(bbsClient, clock, routingTableHandler, syncChannel, logger)
		})

		JustBeforeEach(func() {
			bbsErrorChannel <- errors.New("kaboom")
		})

		It("retries to subscribe", func() {
			close(bbsErrorChannel)
			Eventually(bbsClient.SubscribeToEventsCallCount, 5*time.Second, 300*time.Millisecond).Should(Equal(2))
			Eventually(logger).Should(gbytes.Say("kaboom"))
		})
	})
})
