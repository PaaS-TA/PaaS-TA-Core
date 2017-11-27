package metrics_test

import (
	"errors"
	"time"

	"code.cloudfoundry.org/clock/fakeclock"
	mfakes "code.cloudfoundry.org/go-loggregator/testhelpers/fakes/v1"
	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/lager/lagertest"
	"code.cloudfoundry.org/locket/db/dbfakes"
	"code.cloudfoundry.org/locket/metrics"
	"code.cloudfoundry.org/locket/models"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/tedsuo/ifrit"
	"github.com/tedsuo/ifrit/ginkgomon"
)

var _ = Describe("Metrics", func() {
	var (
		runner           ifrit.Runner
		process          ifrit.Process
		fakeMetronClient *mfakes.FakeIngressClient
		logger           *lagertest.TestLogger
		fakeClock        *fakeclock.FakeClock
		metricsInterval  time.Duration
		lockDB           *dbfakes.FakeLockDB
	)
	BeforeEach(func() {
		logger = lagertest.NewTestLogger("metrics")
		fakeMetronClient = new(mfakes.FakeIngressClient)
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
		runner = metrics.NewMetricsNotifier(logger, fakeClock, fakeMetronClient, metricsInterval, lockDB)
		process = ifrit.Background(runner)
		Eventually(process.Ready()).Should(BeClosed())
	})

	AfterEach(func() {
		ginkgomon.Interrupt(process)
	})

	Context("when there are no errors retrieving counts from database", func() {
		JustBeforeEach(func() {
			fakeClock.Increment(15 * time.Second)
		})

		It("emits metric for number of active locks", func() {
			Eventually(fakeMetronClient.SendMetricCallCount).Should(Equal(2))
			metric, value := fakeMetronClient.SendMetricArgsForCall(0)
			Expect(metric).To(Equal("ActiveLocks"))
			Expect(value).To(Equal(3))
		})

		It("emits metric for number of active presences", func() {
			Eventually(fakeMetronClient.SendMetricCallCount).Should(Equal(2))
			metric, value := fakeMetronClient.SendMetricArgsForCall(1)
			Expect(metric).To(Equal("ActivePresences"))
			Expect(value).To(Equal(2))
		})
	})

	Context("when there are errors retrieving counts from database", func() {
		BeforeEach(func() {
			lockDB.CountReturns(1, errors.New("DB error"))
		})

		JustBeforeEach(func() {
			fakeClock.Increment(15 * time.Second)
		})

		It("does not emit metrics", func() {
			Eventually(logger).Should(gbytes.Say("failed-to-retrieve-lock-count"))
			Consistently(fakeMetronClient.SendMetricCallCount()).Should(Equal(0))
		})
	})
})
