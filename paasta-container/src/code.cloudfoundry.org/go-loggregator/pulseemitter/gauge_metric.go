package pulseemitter

import (
	"sync/atomic"

	loggregator "code.cloudfoundry.org/go-loggregator"
)

// GaugeMetric is used by the pulse emitter to emit gauge metrics to the
// LoggClient.
type GaugeMetric struct {
	name  string
	unit  string
	value int64
	tags  map[string]string
}

// NewGaugeMetric returns a new GaugeMetric that has a value that can be set
// and emitted via a LoggClient.
func NewGaugeMetric(name, unit string, opts ...MetricOption) *GaugeMetric {
	g := &GaugeMetric{
		name: name,
		unit: unit,
		tags: make(map[string]string),
	}

	for _, opt := range opts {
		opt(g.tags)
	}

	return g
}

// Set will set the current value of the gauge metric to the given number.
func (g *GaugeMetric) Set(n int64) {
	atomic.SwapInt64(&g.value, n)
}

// Emit will send the current value and tagging options to the LoggClient to
// be emitted.
func (g *GaugeMetric) Emit(c LoggClient) {
	options := []loggregator.EmitGaugeOption{
		loggregator.WithGaugeValue(
			g.name,
			float64(atomic.LoadInt64(&g.value)),
			g.unit,
		),
	}

	for k, v := range g.tags {
		options = append(options, loggregator.WithEnvelopeStringTag(k, v))
	}

	c.EmitGauge(options...)
}
