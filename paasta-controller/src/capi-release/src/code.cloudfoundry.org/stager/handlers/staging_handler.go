package handlers

import (
	"encoding/json"
	"io/ioutil"
	"net/http"

	"code.cloudfoundry.org/bbs"
	"code.cloudfoundry.org/bbs/models"
	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/runtimeschema/cc_messages"
	"code.cloudfoundry.org/runtimeschema/metric"
	"code.cloudfoundry.org/stager/backend"
)

const (
	StagingStartRequestsReceivedCounter = metric.Counter("StagingStartRequestsReceived")
	StagingStopRequestsReceivedCounter  = metric.Counter("StagingStopRequestsReceived")
)

type StagingHandler interface {
	Stage(resp http.ResponseWriter, req *http.Request)
	StopStaging(resp http.ResponseWriter, req *http.Request)
}

type stagingHandler struct {
	logger      lager.Logger
	backends    map[string]backend.Backend
	diegoClient bbs.Client
}

func NewStagingHandler(
	logger lager.Logger,
	backends map[string]backend.Backend,
	bbsClient bbs.Client,
) StagingHandler {
	logger = logger.Session("staging-handler")

	return &stagingHandler{
		logger:      logger,
		backends:    backends,
		diegoClient: bbsClient,
	}
}

func (handler *stagingHandler) Stage(resp http.ResponseWriter, req *http.Request) {
	stagingGuid := req.FormValue(":staging_guid")
	logger := handler.logger.Session("staging-request", lager.Data{"staging-guid": stagingGuid})

	requestBody, err := ioutil.ReadAll(req.Body)
	if err != nil {
		logger.Error("read-body-failed", err)
		resp.WriteHeader(http.StatusInternalServerError)
		return
	}

	var stagingRequest cc_messages.StagingRequestFromCC
	err = json.Unmarshal(requestBody, &stagingRequest)
	if err != nil {
		logger.Error("unmarshal-request-failed", err)
		resp.WriteHeader(http.StatusBadRequest)
		return
	}

	envNames := []string{}
	for _, envVar := range stagingRequest.Environment {
		envNames = append(envNames, envVar.Name)
	}
	logger.Info("environment", lager.Data{"keys": envNames})

	backend, ok := handler.backends[stagingRequest.Lifecycle]
	if !ok {
		logger.Error("backend-not-found", err, lager.Data{"backend": stagingRequest.Lifecycle})
		resp.WriteHeader(http.StatusNotFound)
		return
	}

	StagingStartRequestsReceivedCounter.Increment()

	taskDef, guid, domain, err := backend.BuildRecipe(stagingGuid, stagingRequest)
	if err != nil {
		logger.Error("recipe-building-failed", err, lager.Data{"staging-request": stagingRequest})
		handler.doErrorResponse(resp, err.Error())
		return
	}

	logger.Info("desiring-task", lager.Data{
		"task_guid":    guid,
		"callback_url": taskDef.CompletionCallbackUrl,
	})

	err = handler.diegoClient.DesireTask(logger, guid, domain, taskDef)
	if models.ErrResourceExists.Equal(err) {
		err = nil
	}

	if err != nil {
		logger.Error("staging-failed", err, lager.Data{"staging-request": stagingRequest})
		handler.doErrorResponse(resp, err.Error())
		return
	}

	resp.WriteHeader(http.StatusAccepted)
}

func (handler *stagingHandler) doErrorResponse(resp http.ResponseWriter, message string) {
	response := cc_messages.StagingResponseForCC{
		Error: backend.SanitizeErrorMessage(message),
	}
	responseJson, _ := json.Marshal(response)

	resp.WriteHeader(http.StatusInternalServerError)
	resp.Write(responseJson)
}

func (handler *stagingHandler) StopStaging(resp http.ResponseWriter, req *http.Request) {
	taskGuid := req.FormValue(":staging_guid")
	logger := handler.logger.Session("stop-staging-request", lager.Data{"staging-guid": taskGuid})

	task, err := handler.diegoClient.TaskByGuid(logger, taskGuid)
	if err != nil {
		if models.ErrResourceNotFound.Equal(err) {
			resp.WriteHeader(http.StatusNotFound)
			return
		}

		logger.Error("failed-to-get-task", err)
		resp.WriteHeader(http.StatusInternalServerError)
		return
	}

	var annotation cc_messages.StagingTaskAnnotation
	err = json.Unmarshal([]byte(task.Annotation), &annotation)
	if err != nil {
		logger.Error("failed-to-unmarshal-task-annotation", err)
		resp.WriteHeader(http.StatusInternalServerError)
		return
	}

	resp.WriteHeader(http.StatusAccepted)
	StagingStopRequestsReceivedCounter.Increment()

	logger.Info("cancelling", lager.Data{"task_guid": taskGuid})

	err = handler.diegoClient.CancelTask(logger, taskGuid)
	if err != nil {
		logger.Error("stop-staging-failed", err)
	}
}
