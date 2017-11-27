package pulseemitter_test

import (
	"time"

	"code.cloudfoundry.org/go-loggregator/pulseemitter"
	"code.cloudfoundry.org/go-loggregator/rpc/loggregator_v2"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Pulse EmitterClient", func() {
	It("emits a counter with a zero delta", func() {
		spyLoggClient := newSpyLoggClient()
		client := pulseemitter.New(
			spyLoggClient,
			pulseemitter.WithPulseInterval(50*time.Millisecond),
		)

		client.NewCounterMetric("some-name")
		Eventually(spyLoggClient.CounterName).Should(Equal("some-name"))

		e := &loggregator_v2.Envelope{
			Message: &loggregator_v2.Envelope_Counter{
				Counter: &loggregator_v2.Counter{},
			},
			Tags: make(map[string]*loggregator_v2.Value),
		}
		for _, o := range spyLoggClient.CounterOpts() {
			o(e)
		}
		Expect(e.GetCounter().GetDelta()).To(Equal(uint64(0)))
	})

	It("emits a gauge with a zero value", func() {
		spyLoggClient := newSpyLoggClient()
		client := pulseemitter.New(
			spyLoggClient,
			pulseemitter.WithPulseInterval(50*time.Millisecond),
		)

		client.NewGaugeMetric("some-name", "some-unit")
		Eventually(spyLoggClient.GaugeOpts).Should(HaveLen(1))

		e := &loggregator_v2.Envelope{
			Message: &loggregator_v2.Envelope_Gauge{
				Gauge: &loggregator_v2.Gauge{
					Metrics: make(map[string]*loggregator_v2.GaugeValue),
				},
			},
			Tags: make(map[string]*loggregator_v2.Value),
		}
		for _, o := range spyLoggClient.GaugeOpts() {
			o(e)
		}
		Expect(e.GetGauge().GetMetrics()).To(HaveLen(1))
		Expect(e.GetGauge().GetMetrics()).To(HaveKey("some-name"))
		Expect(e.GetGauge().GetMetrics()["some-name"].GetUnit()).To(Equal("some-unit"))
		Expect(e.GetGauge().GetMetrics()["some-name"].GetValue()).To(Equal(0.0))
	})

	It("pulses", func() {
		spyLoggClient := newSpyLoggClient()
		client := pulseemitter.New(
			spyLoggClient,
			pulseemitter.WithPulseInterval(time.Millisecond),
		)

		client.NewGaugeMetric("some-name", "some-unit")
		Eventually(spyLoggClient.GaugeCallCount).Should(BeNumerically(">", 1))
	})

})
