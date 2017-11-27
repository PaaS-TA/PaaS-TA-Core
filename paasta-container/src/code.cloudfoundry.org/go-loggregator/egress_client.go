package loggregator

import (
	"crypto/tls"
	"io"

	"golang.org/x/net/context"

	"code.cloudfoundry.org/go-loggregator/rpc/loggregator_v2"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
)

type EgressClient struct {
	c loggregator_v2.EgressClient
}

// NewEgressClient creates a new EgressClient for the given addr and TLS
// configuration.
func NewEgressClient(addr string, c *tls.Config) (*EgressClient, io.Closer, error) {
	conn, err := grpc.Dial(addr,
		grpc.WithTransportCredentials(credentials.NewTLS(c)),
	)
	if err != nil {
		return nil, nil, err
	}

	return &EgressClient{c: loggregator_v2.NewEgressClient(conn)}, conn, nil
}

// Receiver wraps the created EgressClient's Receiver method.
func (c *EgressClient) Receiver(ctx context.Context, in *loggregator_v2.EgressRequest) (loggregator_v2.Egress_ReceiverClient, error) {
	return c.c.Receiver(ctx, in)
}
