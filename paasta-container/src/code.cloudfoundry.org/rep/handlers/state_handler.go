package handlers

import (
	"encoding/json"
	"net/http"

	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/rep/auctioncellrep"
)

type state struct {
	rep auctioncellrep.AuctionCellClient
}

func (h *state) ServeHTTP(w http.ResponseWriter, r *http.Request, logger lager.Logger) {
	logger = logger.Session("auction-fetch-state")

	state, healthy, err := h.rep.State(logger)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		logger.Error("failed-to-fetch-state", err)
		return
	}

	if !healthy {
		logger.Info("cell-not-healthy")
		w.WriteHeader(http.StatusServiceUnavailable)
	}

	json.NewEncoder(w).Encode(state)
}
