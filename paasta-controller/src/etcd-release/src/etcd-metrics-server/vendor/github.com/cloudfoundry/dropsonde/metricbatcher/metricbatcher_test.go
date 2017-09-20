package metricbatcher_test

import (
	"time"

	"github.com/cloudfoundry/dropsonde/metric_sender/fake"
	"github.com/cloudfoundry/dropsonde/metricbatcher"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("MetricBatcher", func() {
	var (
		fakeMetricSender *fake.FakeMetricSender
		metricBatcher    *metricbatcher.MetricBatcher
	)

	BeforeEach(func() {
		fakeMetricSender = fake.NewFakeMetricSender()
	})

	Context("BatchIncrementCounter", func() {

		BeforeEach(func() {
			metricBatcher = metricbatcher.New(fakeMetricSender, 50*time.Millisecond)
		})

		It("batches and then sends a single metric", func() {
			metricBatcher.BatchIncrementCounter("count")
			Expect(fakeMetricSender.GetCounter("count")).To(BeEquivalentTo(0)) // should not increment.

			metricBatcher.BatchIncrementCounter("count")
			metricBatcher.BatchIncrementCounter("count")

			time.Sleep(75 * time.Millisecond)
			Expect(fakeMetricSender.GetCounter("count")).To(BeEquivalentTo(3)) // should update counter only when time out

			metricBatcher.BatchIncrementCounter("count")
			metricBatcher.BatchIncrementCounter("count")
			Expect(fakeMetricSender.GetCounter("count")).To(BeEquivalentTo(3)) // should update counter only when time out

			time.Sleep(75 * time.Millisecond)
			Expect(fakeMetricSender.GetCounter("count")).To(BeEquivalentTo(5)) // should update counter only when time out
		})

		It("batches and then sends multiple metrics", func() {
			metricBatcher.BatchIncrementCounter("count1")
			metricBatcher.BatchIncrementCounter("count2")
			metricBatcher.BatchIncrementCounter("count2")
			Expect(fakeMetricSender.GetCounter("count1")).To(BeEquivalentTo(0)) // should not increment.
			Expect(fakeMetricSender.GetCounter("count2")).To(BeEquivalentTo(0)) // should not increment.

			time.Sleep(75 * time.Millisecond)
			Expect(fakeMetricSender.GetCounter("count1")).To(BeEquivalentTo(1)) // should update counter only when time out
			Expect(fakeMetricSender.GetCounter("count2")).To(BeEquivalentTo(2)) // should update counter only when time out

			metricBatcher.BatchIncrementCounter("count1")
			metricBatcher.BatchIncrementCounter("count2")
			Expect(fakeMetricSender.GetCounter("count1")).To(BeEquivalentTo(1)) // should update counter only when time out
			Expect(fakeMetricSender.GetCounter("count2")).To(BeEquivalentTo(2)) // should update counter only when time out

			time.Sleep(75 * time.Millisecond)
			Expect(fakeMetricSender.GetCounter("count1")).To(BeEquivalentTo(2)) // should update counter only when time out
			Expect(fakeMetricSender.GetCounter("count2")).To(BeEquivalentTo(3)) // should update counter only when time out
		})
	})

	Context("BatchAddCounter", func() {

		BeforeEach(func() {
			metricBatcher = metricbatcher.New(fakeMetricSender, 50*time.Millisecond)
		})

		It("batches and then sends a single metric", func() {
			metricBatcher.BatchAddCounter("count", 2)
			Expect(fakeMetricSender.GetCounter("count")).To(BeEquivalentTo(0)) // should not increment.

			metricBatcher.BatchAddCounter("count", 2)
			metricBatcher.BatchAddCounter("count", 3)

			time.Sleep(75 * time.Millisecond)
			Expect(fakeMetricSender.GetCounter("count")).To(BeEquivalentTo(7)) // should update counter only when time out

			metricBatcher.BatchAddCounter("count", 1)
			metricBatcher.BatchAddCounter("count", 2)
			Expect(fakeMetricSender.GetCounter("count")).To(BeEquivalentTo(7)) // should update counter only when time out

			time.Sleep(75 * time.Millisecond)
			Expect(fakeMetricSender.GetCounter("count")).To(BeEquivalentTo(10)) // should update counter only when time out
		})

		It("batches and then sends multiple metrics", func() {
			metricBatcher.BatchAddCounter("count1", 2)
			metricBatcher.BatchAddCounter("count2", 1)
			metricBatcher.BatchAddCounter("count2", 2)
			Expect(fakeMetricSender.GetCounter("count1")).To(BeEquivalentTo(0)) // should not increment.
			Expect(fakeMetricSender.GetCounter("count2")).To(BeEquivalentTo(0)) // should not increment.

			time.Sleep(75 * time.Millisecond)
			Expect(fakeMetricSender.GetCounter("count1")).To(BeEquivalentTo(2)) // should update counter only when time out
			Expect(fakeMetricSender.GetCounter("count2")).To(BeEquivalentTo(3)) // should update counter only when time out

			metricBatcher.BatchAddCounter("count1", 2)
			metricBatcher.BatchAddCounter("count2", 2)
			Expect(fakeMetricSender.GetCounter("count1")).To(BeEquivalentTo(2)) // should update counter only when time out
			Expect(fakeMetricSender.GetCounter("count2")).To(BeEquivalentTo(3)) // should update counter only when time out

			time.Sleep(75 * time.Millisecond)
			Expect(fakeMetricSender.GetCounter("count1")).To(BeEquivalentTo(4)) // should update counter only when time out
			Expect(fakeMetricSender.GetCounter("count2")).To(BeEquivalentTo(5)) // should update counter only when time out
		})
	})

	Context("Reset", func() {
		It("cancels any scheduled counter emission", func() {
			metricBatcher = metricbatcher.New(fakeMetricSender, 50*time.Millisecond)

			metricBatcher.BatchAddCounter("count1", 2)
			metricBatcher.BatchIncrementCounter("count1")

			metricBatcher.Reset()

			Consistently(func() uint64 { return fakeMetricSender.GetCounter("count1") }).Should(BeZero())
		})
	})

	Context("Close", func() {
		BeforeEach(func() {
			// Sets ticker to a longer time so that the Flush isn't called automatically from the go routine
			metricBatcher = metricbatcher.New(fakeMetricSender, 5*time.Second)
		})

		It("flushes remaining metrics", func() {
			metricBatcher.BatchAddCounter("count2", 1)
			metricBatcher.Close()

			Eventually(fakeMetricSender.GetCounter("count2")).Should(BeEquivalentTo(1))
		})

		It("panics when sending metrics after closing", func() {
			metricBatcher.Close()
			Expect(func() { metricBatcher.BatchAddCounter("count3", 3) }).To(Panic())
		})
	})

})
