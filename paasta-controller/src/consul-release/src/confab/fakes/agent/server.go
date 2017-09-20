package main

import (
	"encoding/json"
	"net"
	"net/http"
	"os"
	"time"

	"github.com/hashicorp/consul/api"
	"github.com/hashicorp/consul/command/agent"
)

type Server struct {
	HTTPAddr     string
	HTTPListener net.Listener

	TCPAddr     string
	TCPListener *net.TCPListener

	OutputWriter *OutputWriter

	Members           []string
	DidLeave          bool
	FailStatsEndpoint bool
}

func (s *Server) Serve() error {
	var err error
	s.HTTPListener, err = net.Listen("tcp", s.HTTPAddr)
	if err != nil {
		return err
	}

	tcpAddr, err := net.ResolveTCPAddr("tcp", s.TCPAddr)
	if err != nil {
		return err
	}

	s.TCPListener, err = net.ListenTCP("tcp", tcpAddr)
	if err != nil {
		return err
	}

	go s.ServeTCP()
	go s.ServeHTTP()

	return nil
}

func (s *Server) ServeHTTP() {
	mux := http.NewServeMux()
	mux.HandleFunc("/v1/agent/members", func(w http.ResponseWriter, req *http.Request) {
		var members []api.AgentMember
		for _, member := range s.Members {
			members = append(members, api.AgentMember{
				Addr: member,
				Tags: map[string]string{
					"role": "consul",
				},
			})
		}
		json.NewEncoder(w).Encode(members)
	})
	mux.HandleFunc("/v1/agent/self", func(w http.ResponseWriter, req *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{}`))
	})
	mux.HandleFunc("/v1/agent/join/", func(w http.ResponseWriter, req *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	mux.HandleFunc("/v1/status/leader", func(w http.ResponseWriter, req *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`""`)) //s.Members[0]
	})

	server := &http.Server{
		Addr:    s.HTTPAddr,
		Handler: mux,
	}

	server.Serve(s.HTTPListener)
}

func (s *Server) ServeTCP() {
	mockAgent := new(FakeAgentBackend)
	if s.FailStatsEndpoint {
		mockAgent.StatsReturns(map[string]map[string]string{
			"raft": {
				"commit_index":   "5",
				"last_log_index": "2",
			},
		})
	}

	agentRPCServer := agent.NewAgentRPC(mockAgent, s.TCPListener, os.Stderr, agent.NewLogWriter(42))

	var (
		useKeyCallCount     int
		installKeyCallCount int
		leaveCallCount      int
		statsCallCount      int
	)

	for {
		switch {
		case mockAgent.UseKeyCallCount() > useKeyCallCount:
			useKeyCallCount++
			s.OutputWriter.UseKeyCalled()
		case mockAgent.InstallKeyCallCount() > installKeyCallCount:
			installKeyCallCount++
			s.OutputWriter.InstallKeyCalled()
		case mockAgent.LeaveCallCount() > leaveCallCount:
			leaveCallCount++
			s.OutputWriter.LeaveCalled()
			agentRPCServer.Shutdown()
			s.DidLeave = true
		case mockAgent.StatsCallCount() > statsCallCount:
			statsCallCount++
			s.OutputWriter.StatsCalled()
		}

		time.Sleep(10 * time.Millisecond)
	}
}

func (s Server) Exit() error {
	err := s.HTTPListener.Close()
	if err != nil {
		return err
	}

	err = s.TCPListener.Close()
	if err != nil {
		return err
	}

	return nil
}
