package metricbatcher_test

import (
	"github.com/cloudfoundry/dropsonde/metricbatcher"

	"time"

	"github.com/cloudfoundry/dropsonde/metrics"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Deadlock", func() {
	var (
		metricBatcher *metricbatcher.MetricBatcher
		done          chan struct{}
	)

	BeforeEach(func() {
		done = make(chan struct{})

		metricSender := NewFakeMetricSender(&done)
		metricBatcher = metricbatcher.New(metricSender, 50*time.Millisecond)
		metrics.Initialize(metricSender, metricBatcher)
	})

	It("doesn't deadlock when Batch functions are called while batch sending", func() {
		metricBatcher.BatchAddCounter("count1", 2)
		Eventually(done).Should(BeClosed())
	}, 1)
})

type FakeMetricSender struct {
	done *chan struct{}
}

func NewFakeMetricSender(done *chan struct{}) *FakeMetricSender {
	return &FakeMetricSender{
		done: done,
	}
}

func (fms *FakeMetricSender) SendValue(name string, value float64, unit string) error {
	return nil
}

func (fms *FakeMetricSender) IncrementCounter(name string) error {
	return nil
}

func (fms *FakeMetricSender) AddToCounter(name string, delta uint64) error {
	metrics.BatchAddCounter("bogus-counter", 1)
	if name == "count1" {
		close(*fms.done)
	}
	return nil
}

func (fms *FakeMetricSender) SendContainerMetric(applicationId string, instanceIndex int32, cpuPercentage float64, memoryBytes uint64, diskBytes uint64) error {
	return nil
}
