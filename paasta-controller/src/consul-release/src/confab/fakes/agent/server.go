package main

import (
	"encoding/json"
	"net"
	"net/http"

	"github.com/hashicorp/consul/api"
)

type Server struct {
	HTTPAddr     string
	HTTPListener net.Listener

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

	go s.ServeHTTP()

	return nil
}

func (s *Server) ServeHTTP() {
	var (
		useKeyCallCount     int
		installKeyCallCount int
		leaveCallCount      int
		selfCallCount       int
	)

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
		if s.FailStatsEndpoint {
			w.Write([]byte(`{
				"Stats": {
					"raft": {
						"commit_index":   "5",
						"last_log_index": "2"
					}
				}
			}`))
		} else {
			w.Write([]byte(`{
				"Stats": {
					"raft": {
						"commit_index":   "2",
						"last_log_index": "2"
					}
				}
			}`))
		}
		selfCallCount++
		s.OutputWriter.SelfCalled()
	})
	mux.HandleFunc("/v1/agent/join/", func(w http.ResponseWriter, req *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	mux.HandleFunc("/v1/agent/leave", func(w http.ResponseWriter, req *http.Request) {
		w.WriteHeader(http.StatusOK)
		leaveCallCount++
		s.OutputWriter.LeaveCalled()
		s.DidLeave = true
	})
	mux.HandleFunc("/v1/status/leader", func(w http.ResponseWriter, req *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`""`)) //s.Members[0]
	})
	mux.HandleFunc("/v1/operator/keyring", func(w http.ResponseWriter, req *http.Request) {
		switch req.Method {
		case "GET":
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`[]`))
		case "POST":
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`[]`))
			installKeyCallCount++
			s.OutputWriter.InstallKeyCalled()
		case "PUT":
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`[]`))
			useKeyCallCount++
			s.OutputWriter.UseKeyCalled()
		case "DELETE":
			w.WriteHeader(http.StatusInternalServerError)
		}
	})

	server := &http.Server{
		Addr:    s.HTTPAddr,
		Handler: mux,
	}

	server.Serve(s.HTTPListener)
}

func (s Server) Exit() error {
	err := s.HTTPListener.Close()
	if err != nil {
		return err
	}

	return nil
}
