package syncer_test

import (
	"os"
	"time"

	"code.cloudfoundry.org/bbs/fake_bbs"
	"code.cloudfoundry.org/bbs/models"
	"code.cloudfoundry.org/clock/fakeclock"
	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/lager/lagertest"
	"code.cloudfoundry.org/route-emitter/diegonats"
	"code.cloudfoundry.org/route-emitter/syncer"
	"code.cloudfoundry.org/routing-info/cfroutes"
	fake_metrics_sender "github.com/cloudfoundry/dropsonde/metric_sender/fake"
	"github.com/cloudfoundry/dropsonde/metrics"
	"github.com/nats-io/nats"
	"github.com/tedsuo/ifrit"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

const logGuid = "some-log-guid"

var _ = Describe("Syncer", func() {
	const (
		processGuid   = "process-guid-1"
		containerPort = 8080
		instanceGuid  = "instance-guid-1"
		lrpHost       = "1.2.3.4"
	)

	var (
		bbsClient    *fake_bbs.FakeClient
		natsClient   *diegonats.FakeNATSClient
		syncerRunner *syncer.Syncer
		process      ifrit.Process
		clock        *fakeclock.FakeClock
		clockStep    time.Duration
		syncInterval time.Duration

		shutdown chan struct{}

		schedulingInfoResponse *models.DesiredLRPSchedulingInfo
		actualResponses        []*models.ActualLRPGroup

		routerStartMessages chan<- *nats.Msg
		fakeMetricSender    *fake_metrics_sender.FakeMetricSender
	)

	BeforeEach(func() {
		bbsClient = new(fake_bbs.FakeClient)
		natsClient = diegonats.NewFakeClient()

		clock = fakeclock.NewFakeClock(time.Now())
		clockStep = 1 * time.Second
		syncInterval = 10 * time.Second

		startMessages := make(chan *nats.Msg)
		routerStartMessages = startMessages

		natsClient.WhenSubscribing("router.start", func(callback nats.MsgHandler) error {
			go func() {
				for msg := range startMessages {
					callback(msg)
				}
			}()

			return nil
		})

		schedulingInfoResponse = &models.DesiredLRPSchedulingInfo{
			DesiredLRPKey: models.NewDesiredLRPKey(processGuid, "domain", logGuid),
			Routes:        cfroutes.CFRoutes{{Hostnames: []string{"route-1", "route-2"}, Port: containerPort}}.RoutingInfo(),
		}

		actualResponses = []*models.ActualLRPGroup{
			{
				Instance: &models.ActualLRP{
					ActualLRPKey:         models.NewActualLRPKey(processGuid, 1, "domain"),
					ActualLRPInstanceKey: models.NewActualLRPInstanceKey(instanceGuid, "cell-id"),
					ActualLRPNetInfo:     models.NewActualLRPNetInfo(lrpHost, models.NewPortMapping(1234, containerPort)),
					State:                models.ActualLRPStateRunning,
				},
			},
			{
				Instance: &models.ActualLRP{
					ActualLRPKey: models.NewActualLRPKey("", 1, ""),
					State:        models.ActualLRPStateUnclaimed,
				},
			},
		}

		bbsClient.DesiredLRPSchedulingInfosReturns([]*models.DesiredLRPSchedulingInfo{schedulingInfoResponse}, nil)
		bbsClient.ActualLRPGroupsReturns(actualResponses, nil)

		fakeMetricSender = fake_metrics_sender.NewFakeMetricSender()
		metrics.Initialize(fakeMetricSender, nil)
	})

	JustBeforeEach(func() {
		logger := lagertest.NewTestLogger("test")
		syncerRunner = syncer.NewSyncer(clock, syncInterval, natsClient, logger)

		shutdown = make(chan struct{})

		go func(clock *fakeclock.FakeClock, clockStep time.Duration, shutdown chan struct{}) {
			for {
				select {
				case <-time.After(100 * time.Millisecond):
					clock.Increment(clockStep)
				case <-shutdown:
					return
				}
			}
		}(clock, clockStep, shutdown)

		process = ifrit.Invoke(syncerRunner)
	})

	AfterEach(func() {
		process.Signal(os.Interrupt)
		Eventually(process.Wait()).Should(Receive(BeNil()))
		close(shutdown)
		close(routerStartMessages)
	})

	Describe("getting the heartbeat interval from the router", func() {
		var greetings chan *nats.Msg
		BeforeEach(func() {
			greetings = make(chan *nats.Msg, 3)
			natsClient.WhenPublishing("router.greet", func(msg *nats.Msg) error {
				greetings <- msg
				return nil
			})
		})

		Context("when the router emits a router.start", func() {
			Context("using an interval", func() {
				JustBeforeEach(func() {
					routerStartMessages <- &nats.Msg{
						Data: []byte(`{
						"minimumRegisterIntervalInSeconds":1,
						"pruneThresholdInSeconds": 3
						}`),
					}
				})

				It("should emit routes with the frequency of the passed-in-interval", func() {
					Eventually(syncerRunner.Events().Emit, 2).Should(Receive())
					t1 := clock.Now()

					Eventually(syncerRunner.Events().Emit, 2).Should(Receive())
					t2 := clock.Now()

					Expect(t2.Sub(t1)).To(BeNumerically("~", 1*time.Second, 200*time.Millisecond))
				})

				It("should only greet the router once", func() {
					Eventually(greetings).Should(Receive())
					Consistently(greetings, 1).ShouldNot(Receive())
				})
			})
		})

		Context("when the router does not emit a router.start", func() {
			It("should keep greeting the router until it gets an interval", func() {
				//get the first greeting
				Eventually(greetings, 2).Should(Receive())

				//get the second greeting, and respond
				var msg *nats.Msg
				Eventually(greetings, 2).Should(Receive(&msg))
				go natsClient.Publish(msg.Reply, []byte(`{"minimumRegisterIntervalInSeconds":1, "pruneThresholdInSeconds": 3}`))

				//should no longer be greeting the router
				Consistently(greetings).ShouldNot(Receive())
			})
		})

		Context("after getting the first interval, when a second interval arrives", func() {
			JustBeforeEach(func() {
				routerStartMessages <- &nats.Msg{
					Data: []byte(`{"minimumRegisterIntervalInSeconds":1, "pruneThresholdInSeconds": 3}`),
				}
			})

			It("should modify its update rate", func() {
				routerStartMessages <- &nats.Msg{
					Data: []byte(`{"minimumRegisterIntervalInSeconds":2, "pruneThresholdInSeconds": 6}`),
				}

				//first emit should be pretty quick, it is in response to the incoming heartbeat interval
				Eventually(syncerRunner.Events().Emit).Should(Receive())
				t1 := clock.Now()

				//subsequent emit should follow the interval
				Eventually(syncerRunner.Events().Emit).Should(Receive())
				t2 := clock.Now()

				Expect(t2.Sub(t1)).To(BeNumerically("~", 2*time.Second, 200*time.Millisecond))
			})
		})

		Context("if it never hears anything from a router anywhere", func() {
			It("should still be able to shutdown", func() {
				process.Signal(os.Interrupt)
				Eventually(process.Wait()).Should(Receive(BeNil()))
			})
		})
	})

	Describe("syncing", func() {
		BeforeEach(func() {
			bbsClient.ActualLRPGroupsStub = func(logger lager.Logger, f models.ActualLRPFilter) ([]*models.ActualLRPGroup, error) {
				return nil, nil
			}
			syncInterval = 500 * time.Millisecond

			clockStep = 250 * time.Millisecond
		})

		JustBeforeEach(func() {
			//we set the emit interval real high to avoid colliding with our sync interval
			routerStartMessages <- &nats.Msg{
				Data: []byte(`{"minimumRegisterIntervalInSeconds":10, "pruneThresholdInSeconds": 20}`),
			}
		})

		Context("after the router greets", func() {
			BeforeEach(func() {
				syncInterval = 10 * time.Minute
			})

			It("syncs", func() {
				Eventually(syncerRunner.Events().Sync).Should(Receive())
			})
		})

		Context("on a specified interval", func() {
			It("should sync", func() {
				var t1 time.Time
				var t2 time.Time

				select {
				case <-syncerRunner.Events().Sync:
					t1 = clock.Now()
				case <-time.After(500 * time.Millisecond):
					Fail("did not receive a sync event")
				}

				select {
				case <-syncerRunner.Events().Sync:
					t2 = clock.Now()
				case <-time.After(500 * time.Millisecond):
					Fail("did not receive a sync event")
				}

				Expect(t2.Sub(t1)).To(BeNumerically("~", syncInterval, 100*time.Millisecond))
			})
		})
	})
})
