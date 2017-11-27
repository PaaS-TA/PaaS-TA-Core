package testhelpers

import (
	"crypto/tls"
	"crypto/x509"
	"io/ioutil"
	"net"

	"code.cloudfoundry.org/go-loggregator/rpc/loggregator_v2"
	"code.cloudfoundry.org/go-loggregator/testhelpers/fakes"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
)

type TestEgressServer struct {
	addr       string
	cn         string
	tlsConfig  *tls.Config
	grpcServer *grpc.Server
}

type EgressServerOption func(*TestEgressServer)

func WithCN(cn string) EgressServerOption {
	return func(s *TestEgressServer) {
		s.cn = cn
	}
}

func WithAddr(addr string) EgressServerOption {
	return func(s *TestEgressServer) {
		s.addr = addr
	}
}

func NewTestEgressServer(serverCert, serverKey, caCert string, opts ...EgressServerOption) (*TestEgressServer, error) {
	s := &TestEgressServer{
		addr: "localhost:0",
	}

	for _, o := range opts {
		o(s)
	}

	cert, err := tls.LoadX509KeyPair(serverCert, serverKey)
	if err != nil {
		return nil, err
	}

	s.tlsConfig = &tls.Config{
		Certificates:       []tls.Certificate{cert},
		ClientAuth:         tls.RequestClientCert,
		InsecureSkipVerify: false,
		ServerName:         s.cn,
	}
	caCertBytes, err := ioutil.ReadFile(caCert)
	if err != nil {
		return nil, err
	}

	caCertPool := x509.NewCertPool()
	caCertPool.AppendCertsFromPEM(caCertBytes)
	s.tlsConfig.RootCAs = caCertPool

	return s, nil
}

func (t *TestEgressServer) Addr() string {
	return t.addr
}

func (t *TestEgressServer) Start(rxCallback func(*loggregator_v2.EgressRequest, loggregator_v2.Egress_ReceiverServer) error) error {
	listener, err := net.Listen("tcp4", t.addr)
	if err != nil {
		return err
	}
	t.addr = listener.Addr().String()

	var opts []grpc.ServerOption
	if t.tlsConfig != nil {
		opts = append(opts, grpc.Creds(credentials.NewTLS(t.tlsConfig)))
	}
	t.grpcServer = grpc.NewServer(opts...)

	senderServer := &fakes.FakeEgressServer{}
	senderServer.ReceiverStub = rxCallback
	loggregator_v2.RegisterEgressServer(t.grpcServer, senderServer)

	go t.grpcServer.Serve(listener)

	return nil
}

func (t *TestEgressServer) Stop() {
	t.grpcServer.Stop()
}
