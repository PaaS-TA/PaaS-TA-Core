package generator

import (
	"fmt"

	"code.cloudfoundry.org/bbs"
	"code.cloudfoundry.org/bbs/models"
	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/rep"
	"code.cloudfoundry.org/rep/generator/internal"
)

// ResidualInstanceLRPOperation processes an instance ActualLRP with no matching container.
type ResidualInstanceLRPOperation struct {
	logger            lager.Logger
	bbsClient         bbs.InternalClient
	containerDelegate internal.ContainerDelegate
	models.ActualLRPKey
	models.ActualLRPInstanceKey
}

func NewResidualInstanceLRPOperation(logger lager.Logger,
	bbsClient bbs.InternalClient,
	containerDelegate internal.ContainerDelegate,
	lrpKey models.ActualLRPKey,
	instanceKey models.ActualLRPInstanceKey,
) *ResidualInstanceLRPOperation {
	return &ResidualInstanceLRPOperation{
		logger:               logger,
		bbsClient:            bbsClient,
		containerDelegate:    containerDelegate,
		ActualLRPKey:         lrpKey,
		ActualLRPInstanceKey: instanceKey,
	}
}

func (o *ResidualInstanceLRPOperation) Key() string {
	return o.GetInstanceGuid()
}

func (o *ResidualInstanceLRPOperation) Execute() {
	logger := o.logger.Session("executing-residual-instance-lrp-operation", lager.Data{
		"lrp-key":          o.ActualLRPKey,
		"lrp-instance-key": o.ActualLRPInstanceKey,
	})
	logger.Info("starting")
	defer logger.Info("finished")

	_, exists := o.containerDelegate.GetContainer(logger, rep.LRPContainerGuid(o.GetProcessGuid(), o.GetInstanceGuid()))
	if exists {
		logger.Info("skipped-because-container-exists")
		return
	}

	o.bbsClient.RemoveActualLRP(logger, o.ProcessGuid, int(o.Index), &models.ActualLRPInstanceKey{
		InstanceGuid: o.InstanceGuid,
		CellId:       o.CellId,
	})
}

// ResidualEvacuatingLRPOperation processes an evacuating ActualLRP with no matching container.
type ResidualEvacuatingLRPOperation struct {
	logger            lager.Logger
	bbsClient         bbs.InternalClient
	containerDelegate internal.ContainerDelegate
	models.ActualLRPKey
	models.ActualLRPInstanceKey
}

func NewResidualEvacuatingLRPOperation(logger lager.Logger,
	bbsClient bbs.InternalClient,
	containerDelegate internal.ContainerDelegate,
	lrpKey models.ActualLRPKey,
	instanceKey models.ActualLRPInstanceKey,
) *ResidualEvacuatingLRPOperation {
	return &ResidualEvacuatingLRPOperation{
		logger:               logger,
		bbsClient:            bbsClient,
		containerDelegate:    containerDelegate,
		ActualLRPKey:         lrpKey,
		ActualLRPInstanceKey: instanceKey,
	}
}

func (o *ResidualEvacuatingLRPOperation) Key() string {
	return o.GetInstanceGuid()
}

func (o *ResidualEvacuatingLRPOperation) Execute() {
	logger := o.logger.Session("executing-residual-evacuating-lrp-operation", lager.Data{
		"lrp-key":          o.ActualLRPKey,
		"lrp-instance-key": o.ActualLRPInstanceKey,
	})
	logger.Info("starting")
	defer logger.Info("finished")

	_, exists := o.containerDelegate.GetContainer(logger, rep.LRPContainerGuid(o.GetProcessGuid(), o.GetInstanceGuid()))
	if exists {
		logger.Info("skipped-because-container-exists")
		return
	}

	o.bbsClient.RemoveEvacuatingActualLRP(logger, &o.ActualLRPKey, &o.ActualLRPInstanceKey)
}

// ResidualJointLRPOperation processes an evacuating ActualLRP with no matching container.
type ResidualJointLRPOperation struct {
	logger            lager.Logger
	bbsClient         bbs.InternalClient
	containerDelegate internal.ContainerDelegate
	models.ActualLRPKey
	models.ActualLRPInstanceKey
}

func NewResidualJointLRPOperation(logger lager.Logger,
	bbsClient bbs.InternalClient,
	containerDelegate internal.ContainerDelegate,
	lrpKey models.ActualLRPKey,
	instanceKey models.ActualLRPInstanceKey,
) *ResidualJointLRPOperation {
	return &ResidualJointLRPOperation{
		bbsClient:            bbsClient,
		logger:               logger,
		containerDelegate:    containerDelegate,
		ActualLRPKey:         lrpKey,
		ActualLRPInstanceKey: instanceKey,
	}
}

func (o *ResidualJointLRPOperation) Key() string {
	return o.GetInstanceGuid()
}

func (o *ResidualJointLRPOperation) Execute() {
	logger := o.logger.Session("executing-residual-joint-lrp-operation", lager.Data{
		"lrp-key":          o.ActualLRPKey,
		"lrp-instance-key": o.ActualLRPInstanceKey,
	})
	logger.Info("starting")
	defer logger.Info("finished")

	_, exists := o.containerDelegate.GetContainer(logger, rep.LRPContainerGuid(o.GetProcessGuid(), o.GetInstanceGuid()))
	if exists {
		logger.Info("skipped-because-container-exists")
		return
	}

	actualLRPKey := models.NewActualLRPKey(o.ProcessGuid, int32(o.Index), o.Domain)
	actualLRPInstanceKey := models.NewActualLRPInstanceKey(o.InstanceGuid, o.CellId)
	o.bbsClient.RemoveActualLRP(logger, o.ProcessGuid, int(o.Index), &o.ActualLRPInstanceKey)
	o.bbsClient.RemoveEvacuatingActualLRP(logger, &actualLRPKey, &actualLRPInstanceKey)
}

// ResidualTaskOperation processes a Task with no matching container.
type ResidualTaskOperation struct {
	logger            lager.Logger
	TaskGuid          string
	bbsClient         bbs.InternalClient
	containerDelegate internal.ContainerDelegate
}

func NewResidualTaskOperation(
	logger lager.Logger,
	taskGuid string,
	bbsClient bbs.InternalClient,
	containerDelegate internal.ContainerDelegate,
) *ResidualTaskOperation {
	return &ResidualTaskOperation{
		logger:            logger,
		TaskGuid:          taskGuid,
		bbsClient:         bbsClient,
		containerDelegate: containerDelegate,
	}
}

func (o *ResidualTaskOperation) Key() string {
	return o.TaskGuid
}

func (o *ResidualTaskOperation) Execute() {
	logger := o.logger.Session("executing-residual-task-operation", lager.Data{
		"task-guid": o.TaskGuid,
	})
	logger.Info("starting")
	defer logger.Info("finished")

	_, exists := o.containerDelegate.GetContainer(logger, o.TaskGuid)
	if exists {
		logger.Info("skipped-because-container-exists")
		return
	}

	err := o.bbsClient.FailTask(logger, o.TaskGuid, internal.TaskCompletionReasonMissingContainer)
	if err != nil {
		logger.Error("failed-to-fail-task", err)
	}
}

// ContainerOperation acquires the current state of a container and performs any
// bbs or container operations necessary to harmonize the state of the world.
type ContainerOperation struct {
	logger            lager.Logger
	lrpProcessor      internal.LRPProcessor
	taskProcessor     internal.TaskProcessor
	containerDelegate internal.ContainerDelegate
	Guid              string
}

func NewContainerOperation(
	logger lager.Logger,
	lrpProcessor internal.LRPProcessor,
	taskProcessor internal.TaskProcessor,
	containerDelegate internal.ContainerDelegate,
	guid string,
) *ContainerOperation {
	return &ContainerOperation{
		logger:            logger,
		lrpProcessor:      lrpProcessor,
		taskProcessor:     taskProcessor,
		containerDelegate: containerDelegate,
		Guid:              guid,
	}
}

func (o *ContainerOperation) Key() string {
	return o.Guid
}

func (o *ContainerOperation) Execute() {
	logger := o.logger.Session("executing-container-operation", lager.Data{
		"container-guid": o.Guid,
	})
	logger.Info("starting")
	defer logger.Info("finished")

	container, ok := o.containerDelegate.GetContainer(logger, o.Guid)
	if !ok {
		logger.Info("skipped-because-container-does-not-exist")
		return
	}

	logger = logger.WithData(lager.Data{
		"container-state": container.State,
	})

	lifecycle := container.Tags[rep.LifecycleTag]

	switch lifecycle {
	case rep.LRPLifecycle:
		o.lrpProcessor.Process(logger, container)
		return

	case rep.TaskLifecycle:
		o.taskProcessor.Process(logger, container)
		return

	default:
		logger.Error("failed-to-process-container-with-unknown-lifecycle", fmt.Errorf("unknown lifecycle: %s", lifecycle))
		return
	}
}
