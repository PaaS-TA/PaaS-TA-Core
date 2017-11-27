package loggregator

import (
	"context"
	"crypto/tls"

	"code.cloudfoundry.org/go-loggregator/rpc/loggregator_v2"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
)

// RawIngressClient is an emitter of bare envelopes to loggregator.
// Only use this if you do not want the tagging and batching features of
// ingress client.
type RawIngressClient struct {
	conn   loggregator_v2.IngressClient
	sender loggregator_v2.Ingress_BatchSenderClient
}

// NewRawIngressClient creates a new RawIngressClient.
func NewRawIngressClient(addr string, tlsConfig *tls.Config) (*RawIngressClient, error) {
	conn, err := grpc.Dial(
		addr,
		grpc.WithTransportCredentials(credentials.NewTLS(tlsConfig)),
	)
	if err != nil {
		return nil, err
	}
	return &RawIngressClient{
		conn: loggregator_v2.NewIngressClient(conn),
	}, nil
}

// Emit will send the batch of envelopes down the current stream to the
// loggregator system. It will not alter any of the envelopes in any way.
func (c *RawIngressClient) Emit(e []*loggregator_v2.Envelope) error {
	if c.sender == nil {
		var err error
		c.sender, err = c.conn.BatchSender(context.TODO())
		if err != nil {
			return err
		}
	}

	err := c.sender.Send(&loggregator_v2.EnvelopeBatch{Batch: e})
	if err != nil {
		c.sender = nil
		return err
	}

	return nil
}
