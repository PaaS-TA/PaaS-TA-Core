package handlers

import (
	"encoding/json"
	"net/http"

	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/rep"
)

type state struct {
	rep rep.AuctionCellClient
}

func (h *state) ServeHTTP(w http.ResponseWriter, r *http.Request, logger lager.Logger) {
	logger = logger.Session("auction-fetch-state")

	state, err := h.rep.State(logger)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		logger.Error("failed-to-fetch-state", err)
		return
	}

	json.NewEncoder(w).Encode(state)
}
