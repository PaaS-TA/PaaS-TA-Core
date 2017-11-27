package pulseemitter

import (
	"fmt"
	"sync/atomic"

	loggregator "code.cloudfoundry.org/go-loggregator"
)

type MetricOption func(map[string]string)

// WithVersion will apply a `metric_version` tag to all envelopes sent about
// the metric.
func WithVersion(major, minor uint) MetricOption {
	return WithTags(map[string]string{
		"metric_version": fmt.Sprintf("%d.%d", major, minor),
	})
}

// WithTags will set the tags to apply to every envelopes sent about the
// metric..
func WithTags(tags map[string]string) MetricOption {
	return func(c map[string]string) {
		for k, v := range tags {
			c[k] = v
		}
	}
}

// CounterMetric is used by the pulse emitter to emit counter metrics to the
// LoggClient.
type CounterMetric struct {
	name  string
	delta uint64
	tags  map[string]string
}

// NewCounterMetric returns a new CounterMetric that can be incremented and
// emitted via a LoggClient.
func NewCounterMetric(name string, opts ...MetricOption) *CounterMetric {
	m := &CounterMetric{
		name: name,
		tags: make(map[string]string),
	}

	for _, opt := range opts {
		opt(m.tags)
	}

	return m
}

// Increment will add the given uint64 to the current delta.
func (m *CounterMetric) Increment(c uint64) {
	atomic.AddUint64(&m.delta, c)
}

// GetDelta will return the current value of the delta.
func (m *CounterMetric) GetDelta() uint64 {
	return atomic.LoadUint64(&m.delta)
}

// Emit will send the current delta and tagging options to the LoggClient to
// be emitted. The delta on the CounterMetric will be reset to 0.
func (m *CounterMetric) Emit(c LoggClient) {
	d := atomic.SwapUint64(&m.delta, 0)
	options := []loggregator.EmitCounterOption{loggregator.WithDelta(d)}

	for k, v := range m.tags {
		options = append(options, loggregator.WithEnvelopeStringTag(k, v))
	}

	c.EmitCounter(m.name, options...)
}
