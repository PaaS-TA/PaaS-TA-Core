package handlers

import (
	"errors"
	"net/http"

	"code.cloudfoundry.org/executor"
	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/rep"
)

type StopLRPInstanceHandler struct {
	client executor.Client
}

func NewStopLRPInstanceHandler(client executor.Client) *StopLRPInstanceHandler {
	return &StopLRPInstanceHandler{
		client: client,
	}
}

func (h StopLRPInstanceHandler) ServeHTTP(w http.ResponseWriter, r *http.Request, logger lager.Logger) {
	processGuid := r.FormValue(":process_guid")
	instanceGuid := r.FormValue(":instance_guid")

	logger = logger.Session("handling-stop-lrp-instance", lager.Data{
		"process-guid":  processGuid,
		"instance-guid": instanceGuid,
	})

	if processGuid == "" {
		logger.Error("missing-process-guid", errors.New("process_guid missing from request"))
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	if instanceGuid == "" {
		logger.Error("missing-instance-guid", errors.New("instance_guid missing from request"))
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	err := h.client.StopContainer(logger, rep.LRPContainerGuid(processGuid, instanceGuid))
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		logger.Error("failed-to-stop-container", err)
		return
	}

	w.WriteHeader(http.StatusAccepted)
}
