package handlers

import (
	"encoding/json"
	"net/http"
	"time"

	"code.cloudfoundry.org/bbs/models"
	"code.cloudfoundry.org/clock"
	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/runtimeschema/cc_messages"
	"code.cloudfoundry.org/runtimeschema/metric"
	"code.cloudfoundry.org/stager/backend"
	"code.cloudfoundry.org/stager/cc_client"
)

const (
	// Metrics
	stagingSuccessCounter  = metric.Counter("StagingRequestsSucceeded")
	stagingSuccessDuration = metric.Duration("StagingRequestSucceededDuration")
	stagingFailureCounter  = metric.Counter("StagingRequestsFailed")
	stagingFailureDuration = metric.Duration("StagingRequestFailedDuration")
)

type CompletionHandler interface {
	StagingComplete(resp http.ResponseWriter, req *http.Request)
}

type completionHandler struct {
	ccClient cc_client.CcClient
	backends map[string]backend.Backend
	logger   lager.Logger
	clock    clock.Clock
}

func NewStagingCompletionHandler(logger lager.Logger, ccClient cc_client.CcClient, backends map[string]backend.Backend, clock clock.Clock) CompletionHandler {
	return &completionHandler{
		ccClient: ccClient,
		backends: backends,
		logger:   logger.Session("completion-handler"),
		clock:    clock,
	}
}

func (handler *completionHandler) StagingComplete(res http.ResponseWriter, req *http.Request) {
	taskGuid := req.FormValue(":staging_guid")
	logger := handler.logger.Session("task-complete-callback-received", lager.Data{
		"guid": taskGuid,
	})

	task := &models.TaskCallbackResponse{}
	err := json.NewDecoder(req.Body).Decode(task)
	if err != nil {
		handler.logger.Error("parsing-incoming-task-failed", err)
		res.WriteHeader(http.StatusBadRequest)
		return
	}

	if taskGuid != task.TaskGuid {
		logger.Error("task-guid-mismatch", err, lager.Data{"body-task-guid": task.TaskGuid})
		res.WriteHeader(http.StatusBadRequest)
		return
	}

	var annotation cc_messages.StagingTaskAnnotation
	err = json.Unmarshal([]byte(task.Annotation), &annotation)
	if err != nil {
		res.WriteHeader(http.StatusBadRequest)
		logger.Error("parsing-annotation-failed", err)
		return
	}

	backend := handler.backends[annotation.Lifecycle]
	if backend == nil {
		res.WriteHeader(http.StatusNotFound)
		logger.Error("get-staging-response-failed-backend-not-found", err)
		return
	}

	response, err := backend.BuildStagingResponse(task)
	if err != nil {
		res.WriteHeader(http.StatusBadRequest)
		logger.Error("get-staging-response-failed", err)
		return
	}

	responseJson, err := json.Marshal(response)
	if err != nil {
		res.WriteHeader(http.StatusBadRequest)
		logger.Error("get-staging-response-failed", err)
		return
	}

	if responseJson == nil {
		res.WriteHeader(http.StatusNotFound)
		res.Write([]byte("Unknown task domain"))
		return
	}

	logger.Info("posting-staging-complete", lager.Data{
		"payload": responseJson,
	})

	err = handler.ccClient.StagingComplete(taskGuid, annotation.CompletionCallback, responseJson, logger)
	if err != nil {
		logger.Error("cc-staging-complete-failed", err)
		if responseErr, ok := err.(*cc_client.BadResponseError); ok {
			res.WriteHeader(responseErr.StatusCode)
		} else {
			res.WriteHeader(http.StatusServiceUnavailable)
		}
		return
	}

	handler.reportMetrics(task)

	logger.Info("posted-staging-complete")
	res.WriteHeader(http.StatusOK)
}

func (handler *completionHandler) reportMetrics(task *models.TaskCallbackResponse) {
	duration := handler.clock.Now().Sub(time.Unix(0, task.CreatedAt))
	if task.Failed {
		stagingFailureCounter.Increment()
		err := stagingFailureDuration.Send(duration)
		if err != nil {
			handler.logger.Error("failed-to-send-staging-failed-duration-metric", err)
		}
	} else {
		err := stagingSuccessDuration.Send(duration)
		if err != nil {
			handler.logger.Error("failed-to-send-staging-success-duration-metric", err)
		}
		stagingSuccessCounter.Increment()
	}
}
