package handlers

import (
	"net/http"

	"code.cloudfoundry.org/executor"
	"code.cloudfoundry.org/lager"
)

type CancelTaskHandler struct {
	executorClient executor.Client
}

func NewCancelTaskHandler(executorClient executor.Client) *CancelTaskHandler {
	return &CancelTaskHandler{
		executorClient: executorClient,
	}
}

func (h CancelTaskHandler) ServeHTTP(w http.ResponseWriter, r *http.Request, logger lager.Logger) {
	taskGuid := r.FormValue(":task_guid")

	logger = logger.Session("cancel-task", lager.Data{
		"instance-guid": taskGuid,
	})

	w.WriteHeader(http.StatusAccepted)

	go func() {
		logger.Info("deleting-container")
		err := h.executorClient.DeleteContainer(logger, taskGuid)
		if err == executor.ErrContainerNotFound {
			logger.Info("container-not-found")
			return
		}

		if err != nil {
			logger.Error("failed-deleting-container", err)
			return
		}

		logger.Info("succeeded-deleting-container")
	}()
}
