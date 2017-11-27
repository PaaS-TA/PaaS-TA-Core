package v2shim

import (
	"time"

	"code.cloudfoundry.org/go-loggregator"
	"github.com/cloudfoundry/sonde-go/events"
)

type client struct {
	client *loggregator.IngressClient
}

func NewIngressClient(c *loggregator.IngressClient) client {
	return client{client: c}
}

func (c client) SendDuration(name string, value time.Duration) error {
	c.client.EmitGauge(
		loggregator.WithGaugeValue(name, float64(value), "nanos"),
	)
	return nil
}

func (c client) SendMebiBytes(name string, value int) error {
	c.client.EmitGauge(
		loggregator.WithGaugeValue(name, float64(value), "MiB"),
	)
	return nil
}

func (c client) SendMetric(name string, value int) error {
	c.client.EmitGauge(
		loggregator.WithGaugeValue(name, float64(value), "Metric"),
	)

	return nil
}

func (c client) SendBytesPerSecond(name string, value float64) error {
	c.client.EmitGauge(
		loggregator.WithGaugeValue(name, value, "B/s"),
	)
	return nil
}

func (c client) SendRequestsPerSecond(name string, value float64) error {
	c.client.EmitGauge(
		loggregator.WithGaugeValue(name, value, "Req/s"),
	)
	return nil
}

func (c client) IncrementCounter(name string) error {
	c.client.EmitCounter(name)

	return nil
}

func (c client) SendAppLog(appID, message, sourceType, sourceInstance string) error {
	c.client.EmitLog(
		message,
		loggregator.WithAppInfo(appID, sourceType, sourceInstance),
		loggregator.WithStdout(),
	)
	return nil
}

func (c client) SendAppErrorLog(appID, message, sourceType, sourceInstance string) error {
	c.client.EmitLog(
		message,
		loggregator.WithAppInfo(appID, sourceType, sourceInstance),
	)
	return nil
}

func (c client) SendAppMetrics(m *events.ContainerMetric) error {
	c.client.EmitGauge(
		loggregator.WithGaugeValue("instance_index", float64(m.GetInstanceIndex()), ""),
		loggregator.WithGaugeValue("cpu", m.GetCpuPercentage(), "percentage"),
		loggregator.WithGaugeValue("memory", float64(m.GetMemoryBytes()), "bytes"),
		loggregator.WithGaugeValue("disk", float64(m.GetDiskBytes()), "bytes"),
		loggregator.WithGaugeValue("memory_quota", float64(m.GetMemoryBytesQuota()), "bytes"),
		loggregator.WithGaugeValue("disk_quota", float64(m.GetDiskBytesQuota()), "bytes"),
		loggregator.WithGaugeAppInfo(m.GetApplicationId()),
	)

	return nil
}
