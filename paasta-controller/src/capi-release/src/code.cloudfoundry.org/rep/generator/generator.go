// generator creates operations for container processing
package generator

import (
	"fmt"

	"code.cloudfoundry.org/bbs"
	"code.cloudfoundry.org/bbs/models"
	"code.cloudfoundry.org/executor"
	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/operationq"
	"code.cloudfoundry.org/rep"
	"code.cloudfoundry.org/rep/evacuation/evacuation_context"
	"code.cloudfoundry.org/rep/generator/internal"
)

//go:generate counterfeiter -o fake_generator/fake_generator.go . Generator

// Generator encapsulates operation creation in the Rep.
type Generator interface {
	// BatchOperations creates a set of operations across all containers the Rep is managing.
	BatchOperations(lager.Logger) (map[string]operationq.Operation, error)

	// OperationStream creates an operation every time a container lifecycle event is observed.
	OperationStream(lager.Logger) (<-chan operationq.Operation, error)
}

type generator struct {
	cellID            string
	bbs               bbs.InternalClient
	executorClient    executor.Client
	lrpProcessor      internal.LRPProcessor
	taskProcessor     internal.TaskProcessor
	containerDelegate internal.ContainerDelegate
}

func New(
	cellID string,
	bbs bbs.InternalClient,
	executorClient executor.Client,
	evacuationReporter evacuation_context.EvacuationReporter,
	evacuationTTLInSeconds uint64,
) Generator {
	containerDelegate := internal.NewContainerDelegate(executorClient)
	lrpProcessor := internal.NewLRPProcessor(bbs, containerDelegate, cellID, evacuationReporter, evacuationTTLInSeconds)
	taskProcessor := internal.NewTaskProcessor(bbs, containerDelegate, cellID)

	return &generator{
		cellID:            cellID,
		bbs:               bbs,
		executorClient:    executorClient,
		lrpProcessor:      lrpProcessor,
		taskProcessor:     taskProcessor,
		containerDelegate: containerDelegate,
	}
}

func (g *generator) BatchOperations(logger lager.Logger) (map[string]operationq.Operation, error) {
	logger = logger.Session("batch-operations")
	logger.Info("started")

	containers := make(map[string]executor.Container)
	instanceLRPs := make(map[string]models.ActualLRP)
	evacuatingLRPs := make(map[string]models.ActualLRP)
	tasks := make(map[string]*models.Task)

	errChan := make(chan error, 3)

	logger.Info("getting-containers-lrps-and-tasks")
	go func() {
		foundContainers, err := g.executorClient.ListContainers(logger)
		if err != nil {
			logger.Error("failed-to-list-containers", err)
			err = fmt.Errorf("failed to list containers: %s", err.Error())
		}

		for _, c := range foundContainers {
			containers[c.Guid] = c
		}

		errChan <- err
	}()

	go func() {
		filter := models.ActualLRPFilter{CellID: g.cellID}
		groups, err := g.bbs.ActualLRPGroups(logger, filter)
		if err != nil {
			logger.Error("failed-to-retrieve-lrp-groups", err)
			err = fmt.Errorf("failed to retrieve lrps: %s", err.Error())
		}

		for _, group := range groups {
			if group.Instance != nil {
				instanceLRPs[rep.LRPContainerGuid(group.Instance.GetProcessGuid(), group.Instance.GetInstanceGuid())] = *group.Instance
			}
			if group.Evacuating != nil {
				evacuatingLRPs[rep.LRPContainerGuid(group.Evacuating.GetProcessGuid(), group.Evacuating.GetInstanceGuid())] = *group.Evacuating
			}
		}
		errChan <- err
	}()

	go func() {
		foundTasks, err := g.bbs.TasksByCellID(logger, g.cellID)
		if err != nil {
			logger.Error("failed-to-retrieve-tasks", err)
			err = fmt.Errorf("failed to retrieve tasks: %s", err.Error())
		}

		for _, task := range foundTasks {
			tasks[task.TaskGuid] = task
		}
		errChan <- err
	}()

	var err error
	for i := 0; i < 3; i++ {
		e := <-errChan
		if err == nil && e != nil {
			err = e
		}
	}

	if err != nil {
		logger.Error("failed-getting-containers-lrps-and-tasks", err)
		return nil, err
	}
	logger.Info("succeeded-getting-containers-lrps-and-tasks")

	batch := make(map[string]operationq.Operation)

	// create operations for processes with containers
	for guid, _ := range containers {
		batch[guid] = g.operationFromContainer(logger, guid)
	}

	// create operations for instance lrps with no containers
	for guid, lrp := range instanceLRPs {
		if _, foundContainer := batch[guid]; foundContainer {
			continue
		}
		if _, foundEvacuatingLRP := evacuatingLRPs[guid]; foundEvacuatingLRP {
			batch[guid] = NewResidualJointLRPOperation(logger, g.bbs, g.containerDelegate, lrp.ActualLRPKey, lrp.ActualLRPInstanceKey)
		} else {
			batch[guid] = NewResidualInstanceLRPOperation(logger, g.bbs, g.containerDelegate, lrp.ActualLRPKey, lrp.ActualLRPInstanceKey)
		}
	}

	// create operations for evacuating lrps with no containers
	for guid, lrp := range evacuatingLRPs {
		_, found := batch[guid]
		if !found {
			batch[guid] = NewResidualEvacuatingLRPOperation(logger, g.bbs, g.containerDelegate, lrp.ActualLRPKey, lrp.ActualLRPInstanceKey)
		}
	}

	// create operations for tasks with no containers
	for guid, _ := range tasks {
		_, found := batch[guid]
		if !found {
			batch[guid] = NewResidualTaskOperation(logger, guid, g.bbs, g.containerDelegate)
		}
	}

	logger.Info("succeeded", lager.Data{"batch-size": len(batch)})
	return batch, nil
}

func (g *generator) OperationStream(logger lager.Logger) (<-chan operationq.Operation, error) {
	streamLogger := logger.Session("operation-stream")

	streamLogger.Info("subscribing")
	events, err := g.executorClient.SubscribeToEvents(logger)
	if err != nil {
		streamLogger.Error("failed-subscribing", err)
		return nil, err
	}

	streamLogger.Info("succeeded-subscribing")

	opChan := make(chan operationq.Operation)

	go func() {
		defer events.Close()

		for {
			e, err := events.Next()
			if err != nil {
				streamLogger.Debug("event-stream-closed")
				close(opChan)
				return
			}

			lifecycle, ok := e.(executor.LifecycleEvent)
			if !ok {
				streamLogger.Debug("received-non-lifecycle-event")
				continue
			}

			container := lifecycle.Container()
			opChan <- g.operationFromContainer(logger, container.Guid)
		}
	}()

	return opChan, nil
}

func (g *generator) operationFromContainer(logger lager.Logger, guid string) operationq.Operation {
	return NewContainerOperation(logger, g.lrpProcessor, g.taskProcessor, g.containerDelegate, guid)
}
