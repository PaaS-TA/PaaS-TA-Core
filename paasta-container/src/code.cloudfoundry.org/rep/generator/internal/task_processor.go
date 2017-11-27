package internal

import (
	"code.cloudfoundry.org/bbs/models"
	"code.cloudfoundry.org/executor"
	"code.cloudfoundry.org/rep"

	"code.cloudfoundry.org/bbs"
	"code.cloudfoundry.org/lager"
)

const TaskCompletionReasonMissingContainer = "task container does not exist"
const TaskCompletionReasonFailedToRunContainer = "failed to run container"
const TaskCompletionReasonInvalidTransition = "invalid state transition"
const TaskCompletionReasonFailedToFetchResult = "failed to fetch result"

//go:generate counterfeiter -o fake_internal/fake_task_processor.go task_processor.go TaskProcessor

type TaskProcessor interface {
	Process(lager.Logger, executor.Container)
}

type taskProcessor struct {
	bbsClient         bbs.InternalClient
	containerDelegate ContainerDelegate
	cellID            string
}

func NewTaskProcessor(bbs bbs.InternalClient, containerDelegate ContainerDelegate, cellID string) TaskProcessor {
	return &taskProcessor{
		bbsClient:         bbs,
		containerDelegate: containerDelegate,
		cellID:            cellID,
	}
}

func (p *taskProcessor) Process(logger lager.Logger, container executor.Container) {
	logger = logger.Session("task-processor", lager.Data{
		"container-guid":  container.Guid,
		"container-state": container.State,
	})

	logger.Debug("starting")
	defer logger.Debug("finished")

	switch container.State {
	case executor.StateReserved:
		logger.Debug("processing-reserved-container")
		p.processActiveContainer(logger, container)
	case executor.StateInitializing:
		logger.Debug("processing-initializing-container")
		p.processActiveContainer(logger, container)
	case executor.StateCreated:
		logger.Debug("processing-created-container")
		p.processActiveContainer(logger, container)
	case executor.StateRunning:
		logger.Debug("processing-running-container")
		p.processActiveContainer(logger, container)
	case executor.StateCompleted:
		logger.Debug("processing-completed-container")
		p.processCompletedContainer(logger, container)
	}
}

func (p *taskProcessor) processActiveContainer(logger lager.Logger, container executor.Container) {
	ok := p.startTask(logger, container.Guid)
	if !ok {
		return
	}

	task, err := p.bbsClient.TaskByGuid(logger, container.Guid)
	if err != nil {
		logger.Error("failed-fetching-task", err)
		return
	}

	runReq, err := rep.NewRunRequestFromTask(task)
	if err != nil {
		logger.Error("failed-to-construct-run-request", err)
		return
	}

	ok = p.containerDelegate.RunContainer(logger, &runReq)
	if !ok {
		p.failTask(logger, container.Guid, TaskCompletionReasonFailedToRunContainer)
	}
}

func (p *taskProcessor) processCompletedContainer(logger lager.Logger, container executor.Container) {
	p.completeTask(logger, container)
	p.containerDelegate.DeleteContainer(logger, container.Guid)
}

func (p *taskProcessor) startTask(logger lager.Logger, guid string) bool {
	logger.Info("starting-task")
	changed, err := p.bbsClient.StartTask(logger, guid, p.cellID)
	if err != nil {
		logger.Error("failed-starting-task", err)

		bbsErr := models.ConvertError(err)
		switch bbsErr.Type {
		case models.Error_InvalidStateTransition:
			p.containerDelegate.DeleteContainer(logger, guid)
		case models.Error_ResourceNotFound:
			p.containerDelegate.DeleteContainer(logger, guid)
		}
		return false
	}

	if changed {
		logger.Info("succeeded-starting-task")
	} else {
		logger.Info("task-already-started")
	}

	return changed
}

func (p *taskProcessor) completeTask(logger lager.Logger, container executor.Container) {
	var result string
	var err error

	resultFile := container.Tags[rep.ResultFileTag]
	if !container.RunResult.Failed && resultFile != "" {
		result, err = p.containerDelegate.FetchContainerResultFile(logger, container.Guid, resultFile)
		if err != nil {
			p.failTask(logger, container.Guid, TaskCompletionReasonFailedToFetchResult)
			return
		}
	}

	logger.Info("completing-task")
	err = p.bbsClient.CompleteTask(logger, container.Guid, p.cellID, container.RunResult.Failed, container.RunResult.FailureReason, result)
	if err != nil {
		logger.Error("failed-completing-task", err)

		bbsErr := models.ConvertError(err)
		if bbsErr.Type == models.Error_InvalidStateTransition {
			p.failTask(logger, container.Guid, TaskCompletionReasonInvalidTransition)
		}
		return
	}

	logger.Info("succeeded-completing-task")
}

func (p *taskProcessor) failTask(logger lager.Logger, guid string, reason string) {
	logger.Info("failing-task")
	err := p.bbsClient.FailTask(logger, guid, reason)
	if err != nil {
		logger.Error("failed-failing-task", err)
		return
	}

	logger.Info("succeeded-failing-task")
}
