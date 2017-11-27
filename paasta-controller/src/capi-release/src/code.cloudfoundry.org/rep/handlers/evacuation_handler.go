package handlers

import (
	"encoding/json"
	"net/http"
	"strconv"

	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/rep/evacuation/evacuation_context"
)

type EvacuationHandler struct {
	evacuatable evacuation_context.Evacuatable
}

// Evacuation Handler serves a route that is called by the rep drain script
func NewEvacuationHandler(
	evacuatable evacuation_context.Evacuatable,
) *EvacuationHandler {
	return &EvacuationHandler{
		evacuatable: evacuatable,
	}
}

func (h *EvacuationHandler) ServeHTTP(w http.ResponseWriter, r *http.Request, logger lager.Logger) {
	logger = logger.Session("handling-evacuation")

	h.evacuatable.Evacuate()

	jsonBytes, err := json.Marshal(map[string]string{"ping_path": "/ping"})
	if err != nil {
		logger.Error("failed-to-marshal-response-payload", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Length", strconv.Itoa(len(jsonBytes)))
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusAccepted)
	w.Write(jsonBytes)
}
