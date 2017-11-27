package internal

import (
	"code.cloudfoundry.org/bbs"
	"code.cloudfoundry.org/bbs/models"
	"code.cloudfoundry.org/executor"
	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/rep"
)

type ordinaryLRPProcessor struct {
	bbsClient         bbs.InternalClient
	containerDelegate ContainerDelegate
	cellID            string
}

func newOrdinaryLRPProcessor(
	bbsClient bbs.InternalClient,
	containerDelegate ContainerDelegate,
	cellID string,
) LRPProcessor {
	return &ordinaryLRPProcessor{
		bbsClient:         bbsClient,
		containerDelegate: containerDelegate,
		cellID:            cellID,
	}
}

func (p *ordinaryLRPProcessor) Process(logger lager.Logger, container executor.Container) {
	logger = logger.Session("ordinary-lrp-processor", lager.Data{
		"container-guid":  container.Guid,
		"container-state": container.State,
	})
	logger.Debug("starting")
	defer logger.Debug("finished")

	lrpKey, err := rep.ActualLRPKeyFromTags(container.Tags)
	if err != nil {
		logger.Error("failed-to-generate-lrp-key", err)
		return
	}
	logger = logger.WithData(lager.Data{"lrp-key": lrpKey})

	instanceKey, err := rep.ActualLRPInstanceKeyFromContainer(container, p.cellID)
	if err != nil {
		logger.Error("failed-to-generate-instance-key", err)
		return
	}
	logger = logger.WithData(lager.Data{"lrp-instance-key": instanceKey})

	lrpContainer := newLRPContainer(lrpKey, instanceKey, container)
	switch lrpContainer.Container.State {
	case executor.StateReserved:
		p.processReservedContainer(logger, lrpContainer)
	case executor.StateInitializing:
		p.processInitializingContainer(logger, lrpContainer)
	case executor.StateCreated:
		p.processCreatedContainer(logger, lrpContainer)
	case executor.StateRunning:
		p.processRunningContainer(logger, lrpContainer)
	case executor.StateCompleted:
		p.processCompletedContainer(logger, lrpContainer)
	default:
		p.processInvalidContainer(logger, lrpContainer)
	}
}

func (p *ordinaryLRPProcessor) processReservedContainer(logger lager.Logger, lrpContainer *lrpContainer) {
	logger = logger.Session("process-reserved-container")
	ok := p.claimLRPContainer(logger, lrpContainer)
	if !ok {
		return
	}

	desired, err := p.bbsClient.DesiredLRPByProcessGuid(logger, lrpContainer.ProcessGuid)
	if err != nil {
		logger.Error("failed-to-fetch-desired", err)
		return
	}

	runReq, err := rep.NewRunRequestFromDesiredLRP(lrpContainer.Guid, desired, lrpContainer.ActualLRPKey, lrpContainer.ActualLRPInstanceKey)
	if err != nil {
		logger.Error("failed-to-construct-run-request", err)
		return
	}
	ok = p.containerDelegate.RunContainer(logger, &runReq)
	if !ok {
		p.bbsClient.RemoveActualLRP(logger, lrpContainer.ProcessGuid, int(lrpContainer.Index), lrpContainer.ActualLRPInstanceKey)
		return
	}
}

func (p *ordinaryLRPProcessor) processInitializingContainer(logger lager.Logger, lrpContainer *lrpContainer) {
	logger = logger.Session("process-initializing-container")
	p.claimLRPContainer(logger, lrpContainer)
}

func (p *ordinaryLRPProcessor) processCreatedContainer(logger lager.Logger, lrpContainer *lrpContainer) {
	logger = logger.Session("process-created-container")
	p.claimLRPContainer(logger, lrpContainer)
}

func (p *ordinaryLRPProcessor) processRunningContainer(logger lager.Logger, lrpContainer *lrpContainer) {
	logger = logger.Session("process-running-container")

	logger.Debug("extracting-net-info-from-container")
	netInfo, err := rep.ActualLRPNetInfoFromContainer(lrpContainer.Container)
	if err != nil {
		logger.Error("failed-extracting-net-info-from-container", err)
		return
	}
	logger.Debug("succeeded-extracting-net-info-from-container")

	logger.Info("bbs-start-actual-lrp", lager.Data{"net_info": netInfo})
	err = p.bbsClient.StartActualLRP(logger, lrpContainer.ActualLRPKey, lrpContainer.ActualLRPInstanceKey, netInfo)
	bbsErr := models.ConvertError(err)
	if bbsErr != nil && bbsErr.Type == models.Error_ActualLRPCannotBeStarted {
		p.containerDelegate.StopContainer(logger, lrpContainer.Guid)
	}
}

func (p *ordinaryLRPProcessor) processCompletedContainer(logger lager.Logger, lrpContainer *lrpContainer) {
	logger = logger.Session("process-completed-container")

	if lrpContainer.RunResult.Stopped {
		err := p.bbsClient.RemoveActualLRP(logger, lrpContainer.ProcessGuid, int(lrpContainer.Index), lrpContainer.ActualLRPInstanceKey)
		if err != nil {
			logger.Info("failed-to-remove-actual-lrp", lager.Data{"error": err})
		}
	} else {
		err := p.bbsClient.CrashActualLRP(logger, lrpContainer.ActualLRPKey, lrpContainer.ActualLRPInstanceKey, lrpContainer.RunResult.FailureReason)
		if err != nil {
			logger.Info("failed-to-crash-actual-lrp", lager.Data{"error": err})
		}
	}

	p.containerDelegate.DeleteContainer(logger, lrpContainer.Guid)
}

func (p *ordinaryLRPProcessor) processInvalidContainer(logger lager.Logger, lrpContainer *lrpContainer) {
	logger = logger.Session("process-invalid-container")
	logger.Error("not-processing-container-in-invalid-state", nil)
}

func (p *ordinaryLRPProcessor) claimLRPContainer(logger lager.Logger, lrpContainer *lrpContainer) bool {
	err := p.bbsClient.ClaimActualLRP(logger, lrpContainer.ProcessGuid, int(lrpContainer.Index), lrpContainer.ActualLRPInstanceKey)
	bbsErr := models.ConvertError(err)
	if err != nil {
		if bbsErr.Type == models.Error_ActualLRPCannotBeClaimed {
			p.containerDelegate.DeleteContainer(logger, lrpContainer.Guid)
		}
		return false
	}
	return true
}
