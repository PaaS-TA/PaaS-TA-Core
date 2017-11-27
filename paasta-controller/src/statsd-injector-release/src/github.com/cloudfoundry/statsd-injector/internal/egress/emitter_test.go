package egress_test

import (
	"log"
	"net"

	"github.com/cloudfoundry/statsd-injector/internal/egress"
	v2 "github.com/cloudfoundry/statsd-injector/internal/plumbing/v2"
	"google.golang.org/grpc"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Statsdemitter", func() {
	var (
		serverAddr string
		mockServer *mockMetronIngressServer
		inputChan  chan *v2.Envelope
		message    *v2.Envelope
	)

	BeforeEach(func() {
		inputChan = make(chan *v2.Envelope, 100)
		message = &v2.Envelope{
			Message: &v2.Envelope_Counter{
				Counter: &v2.Counter{
					Name: "a-name",
					Value: &v2.Counter_Delta{
						Delta: 48,
					},
				},
			},
		}
	})

	Context("when the server is already listening", func() {
		BeforeEach(func() {
			serverAddr, mockServer = startServer()
			emitter := egress.New(serverAddr, grpc.WithInsecure())

			go emitter.Run(inputChan)
		})

		It("emits envelope", func() {
			go keepWriting(inputChan, message)
			var receiver v2.Ingress_SenderServer
			Eventually(mockServer.SenderInput.Arg0).Should(Receive(&receiver))

			f := func() bool {
				env, err := receiver.Recv()
				if err != nil {
					return false
				}

				return env.GetCounter().GetDelta() == 48
			}
			Eventually(f).Should(BeTrue())
		})

		It("reconnects when the stream has been closed", func() {
			go keepWriting(inputChan, message)
			close(mockServer.SenderOutput.Ret0)

			f := func() int {
				return len(mockServer.SenderCalled)
			}
			Eventually(f).Should(BeNumerically(">", 1))
		})
	})
})

func keepWriting(c chan<- *v2.Envelope, e *v2.Envelope) {
	for {
		c <- e
	}
}

func startServer() (string, *mockMetronIngressServer) {
	lis, err := net.Listen("tcp", ":0")
	if err != nil {
		panic(err)
	}
	s := grpc.NewServer()
	mockMetronIngressServer := newMockMetronIngressServer()
	v2.RegisterIngressServer(s, mockMetronIngressServer)

	go func() {
		if err := s.Serve(lis); err != nil {
			log.Printf("Failed to start server: %s", err)
		}
	}()

	return lis.Addr().String(), mockMetronIngressServer
}
