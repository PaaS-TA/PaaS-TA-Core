package handlers

import (
	"net/http"

	"code.cloudfoundry.org/lager"
)

type PingHandler struct{}

// Ping Handler serves a route that is called by the rep ctl script
func NewPingHandler() *PingHandler {
	return &PingHandler{}
}

func (h PingHandler) ServeHTTP(w http.ResponseWriter, r *http.Request, logger lager.Logger) {
	w.WriteHeader(http.StatusOK)
}
