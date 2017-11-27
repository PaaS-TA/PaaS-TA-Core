// v1 provides a client to connect with the loggregtor v1 API
//
// Loggregator's v1 client library is better known to the Cloud Foundry
// community as Dropsonde (github.com/cloudfoundry/dropsonde). The code here
// wraps that library in the interest of consolidating all client code into
// a single library which includes both v1 and v2 clients.
package v1

import (
	"io/ioutil"
	"log"
	"time"

	loggregator "code.cloudfoundry.org/go-loggregator"
	"code.cloudfoundry.org/go-loggregator/rpc/loggregator_v2"
	"code.cloudfoundry.org/go-loggregator/v1/conversion"

	"github.com/cloudfoundry/dropsonde"
	"github.com/cloudfoundry/dropsonde/logs"
	"github.com/cloudfoundry/dropsonde/metrics"
	"github.com/cloudfoundry/sonde-go/events"
)

type ClientOption func(*Client)

// WithStringTag allows for the configuration of arbitrary string value
// metadata which will be included in all data sent to Loggregator
func WithStringTag(name, value string) ClientOption {
	return func(c *Client) {
		c.tags[name] = &loggregator_v2.Value{
			Data: &loggregator_v2.Value_Text{Text: value},
		}
	}
}

// WithDecimalTag allows for the configuration of arbitrary decimal value
// metadata which will be included in all data sent to Loggregator
func WithDecimalTag(name string, value float64) ClientOption {
	return func(c *Client) {
		c.tags[name] = &loggregator_v2.Value{
			Data: &loggregator_v2.Value_Decimal{Decimal: value},
		}
	}
}

// WithIntegerTag allows for the configuration of arbitrary integer value
// metadata which will be included in all data sent to Loggregator
func WithIntegerTag(name string, value int64) ClientOption {
	return func(c *Client) {
		c.tags[name] = &loggregator_v2.Value{
			Data: &loggregator_v2.Value_Integer{Integer: value},
		}
	}
}

// WithLogger allows for the configuration of a logger.
// By default, the logger is disabled.
func WithLogger(l loggregator.Logger) ClientOption {
	return func(c *Client) {
		c.logger = l
	}
}

// NewClient creates a v1 loggregator client. This is a wrapper around the
// dropsonde package that will write envelopes to loggregator over UDP. Before
// calling NewClient you should call dropsonde.Initialize.
func NewClient(opts ...ClientOption) (*Client, error) {
	c := &Client{
		tags:   make(map[string]*loggregator_v2.Value),
		logger: log.New(ioutil.Discard, "", 0),
	}

	for _, o := range opts {
		o(c)
	}

	return c, nil
}

// Client represents an emitter into loggregator. It should be created with
// the NewClient constructor.
type Client struct {
	tags   map[string]*loggregator_v2.Value
	logger loggregator.Logger
}

// EmitLog sends a message to loggregator.
func (c *Client) EmitLog(message string, opts ...loggregator.EmitLogOption) {
	v2Envelope := &loggregator_v2.Envelope{
		Timestamp: time.Now().UnixNano(),
		Message: &loggregator_v2.Envelope_Log{
			Log: &loggregator_v2.Log{
				Payload: []byte(message),
				Type:    loggregator_v2.Log_ERR,
			},
		},
		Tags: make(map[string]*loggregator_v2.Value),
	}

	for _, o := range opts {
		o(v2Envelope)
	}
	c.emitEnvelopes(v2Envelope)
}

// EmitGauge sends the configured gauge values to loggregator.
// If no EmitGaugeOption values are present, no envelopes will be emitted.
func (c *Client) EmitGauge(opts ...loggregator.EmitGaugeOption) {
	v2Envelope := &loggregator_v2.Envelope{
		Timestamp: time.Now().UnixNano(),
		Message: &loggregator_v2.Envelope_Gauge{
			Gauge: &loggregator_v2.Gauge{
				Metrics: make(map[string]*loggregator_v2.GaugeValue),
			},
		},
		Tags: make(map[string]*loggregator_v2.Value),
	}

	for _, o := range opts {
		o(v2Envelope)
	}
	c.emitEnvelopes(v2Envelope)
}

// EmitCounter sends a counter envelope with a delta of 1.
func (c *Client) EmitCounter(name string, opts ...loggregator.EmitCounterOption) {
	v2Envelope := &loggregator_v2.Envelope{
		Timestamp: time.Now().UnixNano(),
		Message: &loggregator_v2.Envelope_Counter{
			Counter: &loggregator_v2.Counter{
				Name: name,
				Value: &loggregator_v2.Counter_Delta{
					Delta: uint64(1),
				},
			},
		},
		Tags: make(map[string]*loggregator_v2.Value),
	}

	for _, o := range opts {
		o(v2Envelope)
	}
	c.emitEnvelopes(v2Envelope)
}

func (c *Client) Send() error {
	return nil
}

func (c *Client) IncrementCounter(name string) error {
	return metrics.IncrementCounter(name)
}

func (c *Client) IncrementCounterWithDelta(name string, value uint64) error {
	return metrics.AddToCounter(name, value)
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

func (c *Client) emitEnvelopes(v2Envelope *loggregator_v2.Envelope) {
	for k, v := range c.tags {
		v2Envelope.Tags[k] = v
	}
	v2Envelope.Tags["origin"] = &loggregator_v2.Value{
		Data: &loggregator_v2.Value_Text{
			Text: dropsonde.DefaultEmitter.Origin(),
		},
	}

	for _, e := range conversion.ToV1(v2Envelope) {
		err := dropsonde.DefaultEmitter.EmitEnvelope(e)
		if err != nil {
			c.logger.Printf("Failed to emit envelope: %s", err)
		}
	}
}
