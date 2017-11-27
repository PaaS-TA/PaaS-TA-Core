package testhelper

import "code.cloudfoundry.org/go-loggregator/pulseemitter"

type SpyMetricClient struct {
	counterMetrics map[string]*pulseemitter.CounterMetric

	GaugeMetric *pulseemitter.GaugeMetric
}

func NewMetricClient() *SpyMetricClient {
	return &SpyMetricClient{
		counterMetrics: make(map[string]*pulseemitter.CounterMetric),
	}
}

func (s *SpyMetricClient) NewCounterMetric(name string, opts ...pulseemitter.MetricOption) *pulseemitter.CounterMetric {
	m := &pulseemitter.CounterMetric{}
	s.counterMetrics[name] = m

	return m
}

func (s *SpyMetricClient) NewGaugeMetric(name, unit string, opts ...pulseemitter.MetricOption) *pulseemitter.GaugeMetric {
	s.GaugeMetric = pulseemitter.NewGaugeMetric(name, unit, opts...)

	return s.GaugeMetric
}

func (s *SpyMetricClient) GetDelta(name string) uint64 {
	return s.counterMetrics[name].GetDelta()
}
