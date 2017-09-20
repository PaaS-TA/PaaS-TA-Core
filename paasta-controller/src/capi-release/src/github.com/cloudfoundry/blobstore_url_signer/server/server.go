package server

import (
	"net"
	"net/http"
	"os"
)

type Server interface {
	Start()
	Stop()
}

type server struct {
	protocol   string
	unixSocket string
	listener   net.Listener
	handlers   ServerHandlers
}

func NewServer(protocol string, socket string, handlers ServerHandlers) Server {
	listener, err := net.Listen(protocol, socket)
	if err != nil {
		panic(err)
	}

	return server{
		unixSocket: socket,
		listener:   listener,
		handlers:   handlers,
	}
}

func (s server) Start() {
	http.Serve(s.listener, http.HandlerFunc(s.handlers.SignUrl))
}

func (s server) Stop() {
	s.listener.Close()
	if s.protocol == "unix" {
		os.Remove(s.unixSocket)
	}
}
