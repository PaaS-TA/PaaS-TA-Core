package handlers

import (
	"net/http"

	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/rep"
)

type reset struct {
	rep rep.AuctionCellClient
}

func (h *reset) ServeHTTP(w http.ResponseWriter, r *http.Request, logger lager.Logger) {
	logger = logger.Session("sim-reset")

	simRep, ok := h.rep.(rep.SimClient)
	if !ok {
		w.WriteHeader(http.StatusInternalServerError)
		logger.Error("not-a-simulation-rep", nil)
		return
	}

	err := simRep.Reset()
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		logger.Error("failed-to-reset", err)
		return
	}
}
