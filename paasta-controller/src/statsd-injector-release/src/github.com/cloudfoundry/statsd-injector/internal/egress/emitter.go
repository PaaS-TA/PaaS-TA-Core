package egress

import (
	"context"
	"io"
	"log"
	"time"

	v2 "github.com/cloudfoundry/statsd-injector/internal/plumbing/v2"
	"google.golang.org/grpc"
)

type StatsdEmitter struct {
	addr string
	opts []grpc.DialOption
}

func New(addr string, opts ...grpc.DialOption) *StatsdEmitter {
	return &StatsdEmitter{
		addr: addr,
		opts: opts,
	}
}

func (s *StatsdEmitter) Run(inputChan chan *v2.Envelope) {
	client, closer := startClient(s.addr, s.opts)
	defer closer.Close()

	for {
		sender, err := client.Sender(context.Background())
		if err != nil {
			log.Printf("Unable to establish stream to server (%s): %s", s.addr, err)
			time.Sleep(time.Second)
			continue
		}

		for message := range inputChan {
			if err := sender.Send(message); err != nil {
				log.Printf("Error sending message (reconnecting...): %s", err)
				break
			}
		}
	}
}

func startClient(addr string, opts []grpc.DialOption) (v2.IngressClient, io.Closer) {
	conn, err := grpc.Dial(addr, opts...)
	if err != nil {
		log.Fatalf("unable to establish client (%s): %s", addr, err)
	}
	return v2.NewIngressClient(conn), conn
}
