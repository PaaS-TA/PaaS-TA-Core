package pulseemitter_test

import (
	"code.cloudfoundry.org/go-loggregator/pulseemitter"
	"code.cloudfoundry.org/go-loggregator/rpc/loggregator_v2"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("GaugeMetric", func() {
	It("emits gauges", func() {
		g := pulseemitter.NewGaugeMetric("some-gauge", "some-unit", pulseemitter.WithVersion(1, 2))

		g.Set(10)

		spy := newSpyLoggClient()
		g.Emit(spy)

		e := &loggregator_v2.Envelope{
			Message: &loggregator_v2.Envelope_Gauge{
				Gauge: &loggregator_v2.Gauge{
					Metrics: make(map[string]*loggregator_v2.GaugeValue),
				},
			},
			Tags: make(map[string]*loggregator_v2.Value),
		}

		for _, o := range spy.GaugeOpts() {
			o(e)
		}
		Expect(e.GetGauge().GetMetrics()).To(HaveLen(1))
		Expect(e.GetGauge().GetMetrics()).To(HaveKey("some-gauge"))
		Expect(e.GetGauge().GetMetrics()["some-gauge"].GetValue()).To(Equal(10.0))
		Expect(e.GetGauge().GetMetrics()["some-gauge"].GetUnit()).To(Equal("some-unit"))

		Expect(e.GetTags()).To(HaveKey("metric_version"))
		Expect(e.GetTags()["metric_version"].GetText()).To(Equal("1.2"))
	})
})
