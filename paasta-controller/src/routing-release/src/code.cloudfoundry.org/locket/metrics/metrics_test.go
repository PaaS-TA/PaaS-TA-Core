package metrics_test

import (
	"errors"
	"time"

	"code.cloudfoundry.org/clock/fakeclock"
	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/lager/lagertest"
	"code.cloudfoundry.org/locket/db/dbfakes"
	"code.cloudfoundry.org/locket/metrics"
	"code.cloudfoundry.org/locket/models"

	"github.com/cloudfoundry/dropsonde/metric_sender/fake"
	dropsondemetrics "github.com/cloudfoundry/dropsonde/metrics"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/tedsuo/ifrit"
	"github.com/tedsuo/ifrit/ginkgomon"
)

var _ = Describe("Metrics", func() {
	var (
		runner          ifrit.Runner
		process         ifrit.Process
		sender          *fake.FakeMetricSender
		logger          *lagertest.TestLogger
		fakeClock       *fakeclock.FakeClock
		metricsInterval time.Duration
		lockDB          *dbfakes.FakeLockDB
	)
	BeforeEach(func() {
		logger = lagertest.NewTestLogger("metrics")
		sender = fake.NewFakeMetricSender()
		dropsondemetrics.Initialize(sender, nil)

		fakeClock = fakeclock.NewFakeClock(time.Now())
		metricsInterval = 10 * time.Second

		lockDB = &dbfakes.FakeLockDB{}

		lockDB.CountStub = func(l lager.Logger, lockType string) (int, error) {
			switch {
			case lockType == models.LockType:
				return 3, nil
			case lockType == models.PresenceType:
				return 2, nil
			default:
				return 0, errors.New("unknown type")
			}
		}
	})

	JustBeforeEach(func() {
		runner = metrics.NewMetricsNotifier(logger, fakeClock, metricsInterval, lockDB)
		process = ifrit.Background(runner)
		Eventually(process.Ready()).Should(BeClosed())
	})

	AfterEach(func() {
		ginkgomon.Interrupt(process)
	})

	It("emits metric for total number of active locks", func() {
		fakeClock.WaitForWatcherAndIncrement(10 * time.Second)

		Eventually(func() float64 {
			return sender.GetValue("ActiveLocks").Value
		}).Should(BeEquivalentTo(3))
	})

	It("emits metric for total number of active presences", func() {
		fakeClock.WaitForWatcherAndIncrement(10 * time.Second)

		Eventually(func() float64 {
			return sender.GetValue("ActivePresences").Value
		}).Should(BeEquivalentTo(2))
	})

	Context("when database returns an error", func() {
		BeforeEach(func() {
			lockDB.CountReturns(0, errors.New("failed"))
		})

		It("does not emit metrics", func() {
			fakeClock.WaitForWatcherAndIncrement(10 * time.Second)

			Consistently(func() bool {
				return sender.HasValue("ActivePresences")
			}).Should(BeFalse())
			Consistently(func() bool {
				return sender.HasValue("ActiveLocks")
			}).Should(BeFalse())
		})
	})
})
