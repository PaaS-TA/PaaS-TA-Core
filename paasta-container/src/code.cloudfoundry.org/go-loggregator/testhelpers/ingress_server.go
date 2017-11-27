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

type TestIngressServer struct {
	receivers  chan loggregator_v2.Ingress_BatchSenderServer
	addr       string
	tlsConfig  *tls.Config
	grpcServer *grpc.Server
}

func NewTestIngressServer(serverCert, serverKey, caCert string) (*TestIngressServer, error) {
	cert, err := tls.LoadX509KeyPair(serverCert, serverKey)
	if err != nil {
		return nil, err
	}

	tlsConfig := &tls.Config{
		Certificates:       []tls.Certificate{cert},
		ClientAuth:         tls.RequestClientCert,
		InsecureSkipVerify: false,
	}
	caCertBytes, err := ioutil.ReadFile(caCert)
	if err != nil {
		return nil, err
	}

	caCertPool := x509.NewCertPool()
	caCertPool.AppendCertsFromPEM(caCertBytes)
	tlsConfig.RootCAs = caCertPool

	return &TestIngressServer{
		tlsConfig: tlsConfig,
		receivers: make(chan loggregator_v2.Ingress_BatchSenderServer),
		addr:      "localhost:0",
	}, nil
}

func (t *TestIngressServer) Addr() string {
	return t.addr
}

func (t *TestIngressServer) Receivers() chan loggregator_v2.Ingress_BatchSenderServer {
	return t.receivers
}

func (t *TestIngressServer) Start() error {
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

	senderServer := &fakes.FakeIngressServer{}
	senderServer.BatchSenderStub = func(recv loggregator_v2.Ingress_BatchSenderServer) error {
		t.receivers <- recv
		return nil
	}
	loggregator_v2.RegisterIngressServer(t.grpcServer, senderServer)

	go t.grpcServer.Serve(listener)

	return nil
}

func (t *TestIngressServer) Stop() {
	t.grpcServer.Stop()
}
