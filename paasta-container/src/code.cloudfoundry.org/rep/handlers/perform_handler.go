package handlers

import (
	"encoding/json"
	"net/http"

	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/rep"
	"code.cloudfoundry.org/rep/auctioncellrep"
)

type perform struct {
	rep auctioncellrep.AuctionCellClient
}

func (h *perform) ServeHTTP(w http.ResponseWriter, r *http.Request, logger lager.Logger) {
	logger = logger.Session("auction-perform-work")
	var work rep.Work
	err := json.NewDecoder(r.Body).Decode(&work)

	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		logger.Error("failed-to-unmarshal", err)
		return
	}

	failedWork, err := h.rep.Perform(logger, work)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		logger.Error("failed-to-perform-work", err)
		return
	}

	json.NewEncoder(w).Encode(failedWork)
}
