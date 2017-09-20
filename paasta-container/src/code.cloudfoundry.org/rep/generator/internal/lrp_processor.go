package internal

import (
	"code.cloudfoundry.org/bbs"
	"code.cloudfoundry.org/bbs/models"
	"code.cloudfoundry.org/executor"
	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/rep/evacuation/evacuation_context"
)

type lrpContainer struct {
	*models.ActualLRPKey
	*models.ActualLRPInstanceKey
	executor.Container
}

func newLRPContainer(lrpKey *models.ActualLRPKey, instanceKey *models.ActualLRPInstanceKey, container executor.Container) *lrpContainer {
	return &lrpContainer{
		ActualLRPKey:         lrpKey,
		ActualLRPInstanceKey: instanceKey,
		Container:            container,
	}
}

//go:generate counterfeiter -o fake_internal/fake_lrp_processor.go lrp_processor.go LRPProcessor

type LRPProcessor interface {
	Process(lager.Logger, executor.Container)
}

type lrpProcessor struct {
	evacuationReporter  evacuation_context.EvacuationReporter
	ordinaryProcessor   LRPProcessor
	evacuationProcessor LRPProcessor
}

func NewLRPProcessor(
	bbsClient bbs.InternalClient,
	containerDelegate ContainerDelegate,
	cellID string,
	evacuationReporter evacuation_context.EvacuationReporter,
	evacuationTTLInSeconds uint64,
) LRPProcessor {
	ordinaryProcessor := newOrdinaryLRPProcessor(bbsClient, containerDelegate, cellID)
	evacuationProcessor := newEvacuationLRPProcessor(bbsClient, containerDelegate, cellID, evacuationTTLInSeconds)
	return &lrpProcessor{
		evacuationReporter:  evacuationReporter,
		ordinaryProcessor:   ordinaryProcessor,
		evacuationProcessor: evacuationProcessor,
	}
}

func (p *lrpProcessor) Process(logger lager.Logger, container executor.Container) {
	if p.evacuationReporter.Evacuating() {
		p.evacuationProcessor.Process(logger, container)
	} else {
		p.ordinaryProcessor.Process(logger, container)
	}
}
