package watcher_test

import (
	"errors"
	"os"
	"time"

	"code.cloudfoundry.org/bbs/events/eventfakes"
	"code.cloudfoundry.org/bbs/fake_bbs"
	"code.cloudfoundry.org/bbs/models"
	"code.cloudfoundry.org/clock/fakeclock"
	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/lager/lagertest"
	"code.cloudfoundry.org/route-emitter/diegonats"
	"code.cloudfoundry.org/route-emitter/emitter"
	"code.cloudfoundry.org/route-emitter/routehandlers"
	"code.cloudfoundry.org/route-emitter/routingtable"
	"code.cloudfoundry.org/route-emitter/syncer"
	"code.cloudfoundry.org/route-emitter/watcher"
	"code.cloudfoundry.org/routing-api/fake_routing_api"
	"code.cloudfoundry.org/routing-info/cfroutes"
	uaaclient "code.cloudfoundry.org/uaa-go-client"
	"code.cloudfoundry.org/workpool"
	"github.com/nats-io/nats"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/tedsuo/ifrit"
)

var _ = Describe("Watcher Integration", func() {
	var (
		bbsClient        *fake_bbs.FakeClient
		eventSource      *eventfakes.FakeEventSource
		natsClient       *diegonats.FakeNATSClient
		routingApiClient *fake_routing_api.FakeClient
		syncEvents       syncer.Events
		cellID           string
		testWatcher      *watcher.Watcher
		process          ifrit.Process
		logger           *lagertest.TestLogger
	)

	BeforeEach(func() {
		bbsClient = new(fake_bbs.FakeClient)
		eventSource = new(eventfakes.FakeEventSource)
		bbsClient.SubscribeToEventsByCellIDReturns(eventSource, nil)

		natsClient = diegonats.NewFakeClient()
		routingApiClient = new(fake_routing_api.FakeClient)
		syncEvents = syncer.Events{
			Sync: make(chan struct{}),
			Emit: make(chan struct{}),
		}

		logger = lagertest.NewTestLogger("test")
		workPool, err := workpool.NewWorkPool(1)
		Expect(err).NotTo(HaveOccurred())
		natsEmitter := emitter.NewNATSEmitter(natsClient, workPool, logger)
		natsTable := routingtable.NewRoutingTable(logger, false)

		uaaClient := uaaclient.NewNoOpUaaClient()
		routingAPIEmitter := emitter.NewRoutingAPIEmitter(logger, routingApiClient, uaaClient, 100)
		handler := routehandlers.NewHandler(natsTable, natsEmitter, routingAPIEmitter, false)
		clock := fakeclock.NewFakeClock(time.Now())
		testWatcher = watcher.NewWatcher(
			cellID,
			bbsClient,
			clock,
			handler,
			syncEvents,
			logger,
		)
	})

	JustBeforeEach(func() {
		process = ifrit.Invoke(testWatcher)
	})

	AfterEach(func() {
		process.Signal(os.Interrupt)
		Eventually(process.Wait()).Should(Receive())
	})

	Describe("caching events", func() {
		var (
			errCh            chan error
			eventCh          chan EventHolder
			modTag           *models.ModificationTag
			schedulingInfo1  *models.DesiredLRPSchedulingInfo
			actualLRPGroup1  *models.ActualLRPGroup
			removedActualLRP *models.ActualLRPGroup
		)

		sendEvent := func() {
			Eventually(eventCh).Should(BeSent(EventHolder{models.NewActualLRPRemovedEvent(removedActualLRP)}))
			Eventually(logger).Should(gbytes.Say("caching-event"))
		}

		BeforeEach(func() {
			errCh = make(chan error, 10)
			eventCh = make(chan EventHolder, 1)
			// make the variables local to avoid race detection
			nextErr := errCh
			nextEventValue := eventCh

			modTag = &models.ModificationTag{Epoch: "abc", Index: 1}
			endpoint1 := routingtable.Endpoint{InstanceGUID: "ig-1", Host: "1.1.1.1", Index: 0, Port: 11, ContainerPort: 8080, Evacuating: false, ModificationTag: modTag}

			hostname1 := "foo.example.com"
			schedulingInfo1 = &models.DesiredLRPSchedulingInfo{
				ModificationTag: *modTag,
				DesiredLRPKey:   models.NewDesiredLRPKey("pg-1", "tests", "lg1"),
				Routes: cfroutes.CFRoutes{
					cfroutes.CFRoute{
						Hostnames:       []string{hostname1},
						Port:            8080,
						RouteServiceUrl: "https://rs.example.com",
					},
				}.RoutingInfo(),
				Instances: 1,
			}

			actualLRPGroup1 = &models.ActualLRPGroup{
				Instance: &models.ActualLRP{
					ActualLRPKey:         models.NewActualLRPKey("pg-1", 0, "domain"),
					ActualLRPInstanceKey: models.NewActualLRPInstanceKey(endpoint1.InstanceGUID, "cell-id"),
					ActualLRPNetInfo:     models.NewActualLRPNetInfo(endpoint1.Host, "container-ip", models.NewPortMapping(endpoint1.Port, endpoint1.ContainerPort)),
					State:                models.ActualLRPStateRunning,
					ModificationTag:      *modTag,
				},
			}

			removedActualLRP = &models.ActualLRPGroup{
				Instance: &models.ActualLRP{
					ActualLRPKey:         models.NewActualLRPKey("pg-1", 0, "domain"),
					ActualLRPInstanceKey: models.NewActualLRPInstanceKey(endpoint1.InstanceGUID, "cell-id"),
					ActualLRPNetInfo:     models.NewActualLRPNetInfo(endpoint1.Host, "container-ip", models.NewPortMapping(endpoint1.Port, endpoint1.ContainerPort)),
					State:                models.ActualLRPStateRunning,
					ModificationTag:      *modTag,
				},
			}

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

			bbsClient.ActualLRPGroupsStub = func(logger lager.Logger, filter models.ActualLRPFilter) ([]*models.ActualLRPGroup, error) {
				defer GinkgoRecover()

				sendEvent()

				return []*models.ActualLRPGroup{
					actualLRPGroup1,
				}, nil
			}

			bbsClient.DesiredLRPSchedulingInfosStub = func(logger lager.Logger, f models.DesiredLRPFilter) ([]*models.DesiredLRPSchedulingInfo, error) {
				defer GinkgoRecover()
				return []*models.DesiredLRPSchedulingInfo{schedulingInfo1}, nil
			}
		})

		JustBeforeEach(func() {
			syncEvents.Sync <- struct{}{}
			Eventually(bbsClient.ActualLRPGroupsCallCount).Should(Equal(1))
		})

		Context("when an old remove event is cached", func() {
			BeforeEach(func() {
				removedActualLRP.Instance.ModificationTag.Index = 0
			})

			It("registers the new route", func() {
				Eventually(func() []*nats.Msg {
					return natsClient.PublishedMessages("router.register")
				}).Should(HaveLen(1))
			})
		})

		Context("when a newer remove event is cached", func() {
			BeforeEach(func() {
				removedActualLRP.Instance.ModificationTag.Index = 2
			})

			It("does not register a new route", func() {
				Consistently(func() []*nats.Msg {
					return natsClient.PublishedMessages("router.register")
				}).Should(HaveLen(0))
			})
		})
	})
})
