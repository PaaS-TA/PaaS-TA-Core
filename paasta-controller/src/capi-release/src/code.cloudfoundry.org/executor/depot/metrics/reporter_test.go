package metrics_test

import (
	"errors"
	"os"
	"sync"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"code.cloudfoundry.org/clock/fakeclock"
	"code.cloudfoundry.org/executor"
	"code.cloudfoundry.org/executor/depot/metrics"
	"code.cloudfoundry.org/executor/fakes"
	"code.cloudfoundry.org/lager/lagertest"
	mfakes "code.cloudfoundry.org/loggregator_v2/fakes"
	"github.com/tedsuo/ifrit"
)

var _ = Describe("Reporter", func() {
	var (
		reportInterval   time.Duration
		executorClient   *fakes.FakeClient
		fakeClock        *fakeclock.FakeClock
		fakeMetronClient *mfakes.FakeClient

		reporter  ifrit.Process
		logger    *lagertest.TestLogger
		metricMap map[string]int
		m         sync.RWMutex
	)

	BeforeEach(func() {
		logger = lagertest.NewTestLogger("test")
		reportInterval = 1 * time.Millisecond
		executorClient = new(fakes.FakeClient)

		fakeClock = fakeclock.NewFakeClock(time.Now())
		fakeMetronClient = new(mfakes.FakeClient)

		executorClient.TotalResourcesReturns(executor.ExecutorResources{
			MemoryMB:   1024,
			DiskMB:     2048,
			Containers: 4096,
		}, nil)

		executorClient.RemainingResourcesReturns(executor.ExecutorResources{
			MemoryMB:   128,
			DiskMB:     256,
			Containers: 512,
		}, nil)

		executorClient.ListContainersReturns([]executor.Container{
			{Guid: "container-1"},
			{Guid: "container-2"},
			{Guid: "container-3"},
		}, nil)

		m = sync.RWMutex{}
	})

	JustBeforeEach(func() {
		metricMap = make(map[string]int)
		fakeMetronClient.SendMetricStub = func(name string, value int) error {
			m.Lock()
			metricMap[name] = value
			m.Unlock()
			return nil
		}
		fakeMetronClient.SendMebiBytesStub = func(name string, value int) error {
			m.Lock()
			metricMap[name] = value
			m.Unlock()
			return nil
		}

		reporter = ifrit.Invoke(&metrics.Reporter{
			ExecutorSource: executorClient,
			Interval:       reportInterval,
			Clock:          fakeClock,
			Logger:         logger,
			MetronClient:   fakeMetronClient,
		})
		fakeClock.WaitForWatcherAndIncrement(reportInterval)

	})

	AfterEach(func() {
		reporter.Signal(os.Interrupt)
		Eventually(reporter.Wait()).Should(Receive())
	})

	It("reports the current capacity on the given interval", func() {
		Eventually(fakeMetronClient.SendMebiBytesCallCount).Should(Equal(4))

		m.RLock()
		Eventually(metricMap["CapacityTotalMemory"]).Should(Equal(1024))
		Eventually(metricMap["CapacityTotalContainers"]).Should(Equal(4096))
		Eventually(metricMap["CapacityTotalDisk"]).Should(Equal(2048))

		Eventually(metricMap["CapacityRemainingMemory"]).Should(Equal(128))
		Eventually(metricMap["CapacityRemainingDisk"]).Should(Equal(256))
		Eventually(metricMap["CapacityRemainingContainers"]).Should(Equal(512))

		Eventually(metricMap["ContainerCount"]).Should(Equal(3))

		executorClient.RemainingResourcesReturns(executor.ExecutorResources{
			MemoryMB:   129,
			DiskMB:     257,
			Containers: 513,
		}, nil)

		executorClient.ListContainersReturns([]executor.Container{
			{Guid: "container-1"},
			{Guid: "container-2"},
		}, nil)

		fakeClock.Increment(reportInterval)

		m.RUnlock()

		Eventually(fakeMetronClient.SendMebiBytesCallCount).Should(Equal(8))

		m.RLock()
		Eventually(metricMap["CapacityRemainingMemory"]).Should(Equal(129))
		Eventually(metricMap["CapacityRemainingDisk"]).Should(Equal(257))
		Eventually(metricMap["CapacityRemainingContainers"]).Should(Equal(513))
		Eventually(metricMap["ContainerCount"]).Should(Equal(2))

		m.RUnlock()
	})

	Context("when getting remaining resources fails", func() {
		BeforeEach(func() {
			executorClient.RemainingResourcesReturns(executor.ExecutorResources{}, errors.New("oh no!"))
		})

		It("sends missing remaining resources", func() {
			Eventually(fakeMetronClient.SendMebiBytesCallCount).Should(Equal(4))

			m.RLock()
			Eventually(metricMap["CapacityRemainingMemory"]).Should(Equal(-1))
			Eventually(metricMap["CapacityRemainingDisk"]).Should(Equal(-1))
			Eventually(metricMap["CapacityRemainingContainers"]).Should(Equal(-1))
			m.RUnlock()
		})
	})

	Context("when getting total resources fails", func() {
		BeforeEach(func() {
			executorClient.TotalResourcesReturns(executor.ExecutorResources{}, errors.New("oh no!"))
		})

		It("sends missing total resources", func() {
			Eventually(fakeMetronClient.SendMebiBytesCallCount).Should(Equal(4))

			m.RLock()
			Eventually(metricMap["CapacityTotalMemory"]).Should(Equal(-1))
			Eventually(metricMap["CapacityTotalContainers"]).Should(Equal(-1))
			Eventually(metricMap["CapacityTotalDisk"]).Should(Equal(-1))
			m.RUnlock()
		})
	})

	Context("when getting the containers fails", func() {
		BeforeEach(func() {
			executorClient.ListContainersReturns(nil, errors.New("oh no!"))
		})

		It("reports garden.containers as -1", func() {
			logger.Info("checking this stuff")
			Eventually(fakeMetronClient.SendMetricCallCount).Should(Equal(3))

			m.RLock()
			Eventually(metricMap["ContainerCount"]).Should(Equal(-1))
			m.RUnlock()
		})
	})
})
