// v1 provides a client to connect with the loggregtor v1 API
//
// Loggregator's v1 client library is better known to the Cloud Foundry
// community as Dropsonde (github.com/cloudfoundry/dropsonde). The code here
// wraps that library in the interest of consolidating all client code into
// a single library which includes both v1 and v2 clients.
package v1

import (
	"time"

	"github.com/cloudfoundry/dropsonde/logs"
	"github.com/cloudfoundry/dropsonde/metrics"
	"github.com/cloudfoundry/sonde-go/events"
)

func NewClient() (*Client, error) {
	return &Client{}, nil
}

type Client struct{}

func (c *Client) Send() error {
	return nil
}

func (c *Client) IncrementCounter(name string) error {
	return metrics.IncrementCounter(name)
}
func (c *Client) SendAppLog(appID, message, sourceType, sourceInstance string) error {
	return logs.SendAppLog(appID, message, sourceType, sourceInstance)
}

func (c *Client) SendAppErrorLog(appID, message, sourceType, sourceInstance string) error {
	return logs.SendAppErrorLog(appID, message, sourceType, sourceInstance)
}

func (c *Client) SendAppMetrics(m *events.ContainerMetric) error {
	return metrics.Send(m)
}

func (c *Client) SendDuration(name string, duration time.Duration) error {
	return c.SendComponentMetric(name, float64(duration), "nanos")
}

func (c *Client) SendMebiBytes(name string, mebibytes int) error {
	return c.SendComponentMetric(name, float64(mebibytes), "MiB")
}

func (c *Client) SendMetric(name string, value int) error {
	return c.SendComponentMetric(name, float64(value), "Metric")
}

func (c *Client) SendBytesPerSecond(name string, value float64) error {
	return c.SendComponentMetric(name, value, "B/s")
}

func (c *Client) SendRequestsPerSecond(name string, value float64) error {
	return c.SendComponentMetric(name, value, "Req/s")
}

func (c *Client) SendComponentMetric(name string, value float64, unit string) error {
	return metrics.SendValue(name, value, unit)
}
