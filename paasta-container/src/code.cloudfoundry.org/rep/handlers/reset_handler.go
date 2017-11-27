package handlers

import (
	"net/http"

	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/rep/auctioncellrep"
)

type reset struct {
	rep auctioncellrep.AuctionCellClient
}

func (h *reset) ServeHTTP(w http.ResponseWriter, r *http.Request, logger lager.Logger) {
	logger = logger.Session("sim-reset")

	err := h.rep.Reset()
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		logger.Error("failed-to-reset", err)
		return
	}
}
