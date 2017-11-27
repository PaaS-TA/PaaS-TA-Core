// Package v2 provides a client to send data to the Loggregator v2 API.
//
// The v2 API distinguishes itself from the v1 API on three counts:
//
// 1) it uses gRPC,
// 2) it uses a streaming connection, and
// 3) it supports batching to improve performance.
//
// The code here provides a generic interface into the v2 API. Clients who
// prefer more fine grained control may generate their own code using
// the protobuf and gRPC service definitions found at:
// github.com/cloudfoundry/loggregator-api.
//
// Note that on account of the client using batching wherein multiple
// messages may be sent at once, there is no meaningful error return value
// available. Each of the methods below make a best-effort at message
// delivery. Even in the event of a failed send, the client will not block
// callers.
package loggregator

import (
	"crypto/tls"
	"io/ioutil"
	"log"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"

	"code.cloudfoundry.org/go-loggregator/rpc/loggregator_v2"
	"golang.org/x/net/context"
)

// IngressClient represents an emitter into loggregator. It should be created with the
// NewIngressClient constructor.
type IngressClient struct {
	conn      loggregator_v2.IngressClient
	sender    loggregator_v2.Ingress_BatchSenderClient
	envelopes chan *loggregator_v2.Envelope
	tags      map[string]*loggregator_v2.Value

	batchMaxSize       uint
	batchFlushInterval time.Duration
	addr               string

	logger Logger
}

// IngressOption is the type of a configurable client option.
type IngressOption func(*IngressClient)

// WithStringTag allows for the configuration of arbitrary string value
// metadata which will be included in all data sent to Loggregator
func WithStringTag(name, value string) IngressOption {
	return func(c *IngressClient) {
		c.tags[name] = &loggregator_v2.Value{
			Data: &loggregator_v2.Value_Text{Text: value},
		}
	}
}

// WithDecimalTag allows for the configuration of arbitrary decimal value
// metadata which will be included in all data sent to Loggregator
func WithDecimalTag(name string, value float64) IngressOption {
	return func(c *IngressClient) {
		c.tags[name] = &loggregator_v2.Value{
			Data: &loggregator_v2.Value_Decimal{Decimal: value},
		}
	}
}

// WithIntegerTag allows for the configuration of arbitrary integer value
// metadata which will be included in all data sent to Loggregator
func WithIntegerTag(name string, value int64) IngressOption {
	return func(c *IngressClient) {
		c.tags[name] = &loggregator_v2.Value{
			Data: &loggregator_v2.Value_Integer{Integer: value},
		}
	}
}

// WithBatchMaxSize allows for the configuration of the number of messages to
// collect before emitting them into loggregator. By default, its value is 100
// messages.
//
// Note that aside from batch size, messages will be flushed from
// the client into loggregator at a fixed interval to ensure messages are not
// held for an undue amount of time before being sent. In other words, even if
// the client has not yet achieved the maximum batch size, the batch interval
// may trigger the messages to be sent.
func WithBatchMaxSize(maxSize uint) IngressOption {
	return func(c *IngressClient) {
		c.batchMaxSize = maxSize
	}
}

// WithBatchFlushInterval allows for the configuration of the maximum time to
// wait before sending a batch of messages. Note that the batch interval
// may be triggered prior to the batch reaching the configured maximum size.
func WithBatchFlushInterval(d time.Duration) IngressOption {
	return func(c *IngressClient) {
		c.batchFlushInterval = d
	}
}

// WithAddr allows for the configuration of the loggregator v2 address.
// The value to defaults to localhost:3458, which happens to be the default
// address in the loggregator server.
func WithAddr(addr string) IngressOption {
	return func(c *IngressClient) {
		c.addr = addr
	}
}

// Logger declares the minimal logging interface used within the v2 client
type Logger interface {
	Printf(string, ...interface{})
}

// WithLogger allows for the configuration of a logger.
// By default, the logger is disabled.
func WithLogger(l Logger) IngressOption {
	return func(c *IngressClient) {
		c.logger = l
	}
}

// NewIngressClient creates a v2 loggregator client. Its TLS configuration
// must share a CA with the loggregator server.
func NewIngressClient(tlsConfig *tls.Config, opts ...IngressOption) (*IngressClient, error) {
	client := &IngressClient{
		envelopes:          make(chan *loggregator_v2.Envelope, 100),
		tags:               make(map[string]*loggregator_v2.Value),
		batchMaxSize:       100,
		batchFlushInterval: time.Second,
		addr:               "localhost:3458",
		logger:             log.New(ioutil.Discard, "", 0),
	}

	for _, o := range opts {
		o(client)
	}

	conn, err := grpc.Dial(
		client.addr,
		grpc.WithTransportCredentials(credentials.NewTLS(tlsConfig)),
	)
	if err != nil {
		return nil, err
	}
	client.conn = loggregator_v2.NewIngressClient(conn)

	go client.startSender()

	return client, nil
}

// EmitLogOption is the option type passed into EmitLog
type EmitLogOption func(*loggregator_v2.Envelope)

// WithAppInfo configures the meta data associated with emitted data
func WithAppInfo(appID, sourceType, sourceInstance string) EmitLogOption {
	return func(e *loggregator_v2.Envelope) {
		e.SourceId = appID
		e.InstanceId = sourceInstance
		e.Tags["source_type"] = &loggregator_v2.Value{
			Data: &loggregator_v2.Value_Text{Text: sourceType},
		}
		e.Tags["source_instance"] = &loggregator_v2.Value{
			Data: &loggregator_v2.Value_Text{Text: sourceInstance},
		}
	}
}

// WithStdout sets the output type to stdout. Without using this option,
// all data is assumed to be stderr output.
func WithStdout() EmitLogOption {
	return func(e *loggregator_v2.Envelope) {
		e.GetLog().Type = loggregator_v2.Log_OUT
	}
}

// EmitLog sends a message to loggregator.
func (c *IngressClient) EmitLog(message string, opts ...EmitLogOption) {
	e := &loggregator_v2.Envelope{
		Timestamp: time.Now().UnixNano(),
		Message: &loggregator_v2.Envelope_Log{
			Log: &loggregator_v2.Log{
				Payload: []byte(message),
				Type:    loggregator_v2.Log_ERR,
			},
		},
		Tags: make(map[string]*loggregator_v2.Value),
	}

	for k, v := range c.tags {
		e.Tags[k] = v
	}

	for _, o := range opts {
		o(e)
	}

	c.envelopes <- e
}

// EmitGaugeOption is the option type passed into EmitGauge
type EmitGaugeOption func(*loggregator_v2.Envelope)

// WithGaugeAppInfo configures an ID associated with the gauge
func WithGaugeAppInfo(appID string) EmitGaugeOption {
	return func(e *loggregator_v2.Envelope) {
		e.SourceId = appID
	}
}

// WithGaugeValue adds a gauge information. For example,
// to send information about current CPU usage, one might use:
//
// WithGaugeValue("cpu", 3.0, "percent")
//
// An number of calls to WithGaugeValue may be passed into EmitGauge.
// If there are duplicate names in any of the options, i.e., "cpu" and "cpu",
// then the last EmitGaugeOption will take precedence.
func WithGaugeValue(name string, value float64, unit string) EmitGaugeOption {
	return func(e *loggregator_v2.Envelope) {
		e.GetGauge().Metrics[name] = &loggregator_v2.GaugeValue{Value: value, Unit: unit}
	}
}

// WithGaugeTags adds tag information that can be text, integer, or decimal to
// the envelope.  WithGaugeTags expects a single call with a complete map
// and will overwrite if called a second time.
func WithGaugeTags(tags map[string]string) EmitGaugeOption {
	return func(e *loggregator_v2.Envelope) {
		for name, value := range tags {
			e.Tags[name] = &loggregator_v2.Value{
				Data: &loggregator_v2.Value_Text{
					Text: value,
				},
			}
		}
	}
}

// EmitGauge sends the configured gauge values to loggregator.
// If no EmitGaugeOption values are present, the client will emit
// an empty gauge.
func (c *IngressClient) EmitGauge(opts ...EmitGaugeOption) {
	e := &loggregator_v2.Envelope{
		Timestamp: time.Now().UnixNano(),
		Message: &loggregator_v2.Envelope_Gauge{
			Gauge: &loggregator_v2.Gauge{
				Metrics: make(map[string]*loggregator_v2.GaugeValue),
			},
		},
		Tags: make(map[string]*loggregator_v2.Value),
	}

	for k, v := range c.tags {
		e.Tags[k] = v
	}

	for _, o := range opts {
		o(e)
	}

	c.envelopes <- e
}

// EmitCounterOption is the option type passed into EmitCounter.
type EmitCounterOption func(*loggregator_v2.Envelope)

// WithDelta is an option that sets the delta for a counter.
func WithDelta(d uint64) EmitCounterOption {
	return func(e *loggregator_v2.Envelope) {
		e.GetCounter().Value = &loggregator_v2.Counter_Delta{Delta: d}
	}
}

// EmitCounter sends a counter envelope with a delta of 1.
func (c *IngressClient) EmitCounter(name string, opts ...EmitCounterOption) {
	e := &loggregator_v2.Envelope{
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

	for k, v := range c.tags {
		e.Tags[k] = v
	}

	for _, o := range opts {
		o(e)
	}

	c.envelopes <- e
}

func (c *IngressClient) startSender() {
	t := time.NewTimer(c.batchFlushInterval)

	var batch []*loggregator_v2.Envelope
	for {
		select {
		case env := <-c.envelopes:
			batch = append(batch, env)

			if len(batch) >= int(c.batchMaxSize) {
				c.flush(batch)
				batch = nil
				if !t.Stop() {
					<-t.C
				}
				t.Reset(c.batchFlushInterval)
			}
		case <-t.C:
			if len(batch) > 0 {
				c.flush(batch)
				batch = nil
			}
			t.Reset(c.batchFlushInterval)
		}
	}
}

func (c *IngressClient) flush(batch []*loggregator_v2.Envelope) {
	if c.sender == nil {
		var err error
		c.sender, err = c.conn.BatchSender(context.TODO())
		if err != nil {
			c.logger.Printf("Error while flushing: %s", err)
			return
		}
	}

	err := c.sender.Send(&loggregator_v2.EnvelopeBatch{Batch: batch})
	if err != nil {
		c.logger.Printf("Error while flushing: %s", err)
		c.sender = nil
		return
	}

	return
}

// WithEnvelopeStringTag adds a string tag to the envelope.
func WithEnvelopeStringTag(name, value string) func(*loggregator_v2.Envelope) {
	return func(e *loggregator_v2.Envelope) {
		e.Tags[name] = &loggregator_v2.Value{
			Data: &loggregator_v2.Value_Text{
				Text: value,
			},
		}
	}
}

// WithEnvelopeDecimalTag adds a decimal tag to the envelope.
func WithEnvelopeDecimalTag(name string, value float64) func(*loggregator_v2.Envelope) {
	return func(e *loggregator_v2.Envelope) {
		e.Tags[name] = &loggregator_v2.Value{
			Data: &loggregator_v2.Value_Decimal{
				Decimal: value,
			},
		}
	}
}

// WithEnvelopeIntegerTag adds a integer tag to the envelope.
func WithEnvelopeIntegerTag(name string, value int64) func(*loggregator_v2.Envelope) {
	return func(e *loggregator_v2.Envelope) {
		e.Tags[name] = &loggregator_v2.Value{
			Data: &loggregator_v2.Value_Integer{
				Integer: value,
			},
		}
	}
}
