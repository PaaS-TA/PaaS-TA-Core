package evacuation

import (
	"errors"
	"os"
	"time"

	"code.cloudfoundry.org/bbs"
	"code.cloudfoundry.org/bbs/models"
	"code.cloudfoundry.org/clock"
	"code.cloudfoundry.org/executor"
	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/runtimeschema/metric"
)

const (
	ExitTimeout = 15 * time.Second
)

var strandedEvacuatingActualLRPs = metric.Metric("StrandedEvacuatingActualLRPs")

type EvacuationCleanup struct {
	clock          clock.Clock
	logger         lager.Logger
	cellID         string
	bbsClient      bbs.InternalClient
	executorClient executor.Client
}

func NewEvacuationCleanup(
	logger lager.Logger,
	cellID string,
	bbsClient bbs.InternalClient,
	executorClient executor.Client,
	clock clock.Clock,
) *EvacuationCleanup {
	return &EvacuationCleanup{
		logger:         logger,
		cellID:         cellID,
		bbsClient:      bbsClient,
		executorClient: executorClient,
		clock:          clock,
	}
}

func (e *EvacuationCleanup) Run(signals <-chan os.Signal, ready chan<- struct{}) error {
	logger := e.logger.Session("evacuation-cleanup")

	logger.Info("started")
	defer logger.Info("complete")

	close(ready)

	select {
	case signal := <-signals:
		logger.Info("signalled", lager.Data{"signal": signal})
	}

	actualLRPGroups, err := e.bbsClient.ActualLRPGroups(logger, models.ActualLRPFilter{CellID: e.cellID})
	if err != nil {
		logger.Error("failed-fetching-actual-lrp-groups", err)
		return err
	}

	strandedEvacuationCount := 0
	for _, group := range actualLRPGroups {
		if group.Evacuating == nil {
			continue
		}

		strandedEvacuationCount++
		actualLRP := group.Evacuating
		err = e.bbsClient.RemoveEvacuatingActualLRP(logger, &actualLRP.ActualLRPKey, &actualLRP.ActualLRPInstanceKey)
		if err != nil {
			logger.Error("failed-removing-evacuating-actual-lrp", err, lager.Data{"lrp-key": actualLRP.ActualLRPKey})
		}
	}

	err = strandedEvacuatingActualLRPs.Send(strandedEvacuationCount)
	if err != nil {
		logger.Error("failed-sending-stranded-evacuating-lrp-metric", err, lager.Data{"count": strandedEvacuationCount})
	}

	logger.Info("stopping-all-containers")

	exitTimer := e.clock.NewTimer(ExitTimeout)
	checkRunningContainersTimer := e.clock.NewTicker(1 * time.Second)
	containersSignalled := make(chan struct{})
	containersStopped := make(chan struct{})
	go e.signalRunningContainers(logger, containersSignalled)
	go e.checkRunningContainers(logger, checkRunningContainersTimer.C(), containersSignalled, containersStopped)

	select {
	case <-exitTimer.C():
		// exit after ExitTimeout has passed
		logger.Info("failed-to-cleanup-all-containers")
		return errors.New("failed-to-cleanup-all-containers")
	case <-containersStopped:
		logger.Info("stopped-containers-successfully")
		return nil
	}
}

func (e *EvacuationCleanup) checkRunningContainers(
	logger lager.Logger,
	ticker <-chan time.Time,
	containersSignalled <-chan struct{},
	containersStopped chan<- struct{},
) {
	hasRunningContainers := func() bool {
		containers, err := e.executorClient.ListContainers(logger)
		if err != nil {
			logger.Error("failed-listing-containers", err)
			// assume no container is running if we can't list them
			return false
		}

		for _, container := range containers {
			if container.State == executor.StateRunning {
				return true
			}
		}

		return false
	}

	defer close(containersStopped)

	// wait for all containers to be signalled, this only makes the tests easier
	// to write since they depend on the signalling and checking to happen
	// sequentially, but isn't necessary for the operation of the cleanup
	<-containersSignalled
	for hasRunningContainers() {
		logger.Info("waiting-for-containers-to-stop")
		<-ticker
	}
}

func (e *EvacuationCleanup) signalRunningContainers(logger lager.Logger, containersSignalled chan<- struct{}) {
	defer close(containersSignalled)

	containers, err := e.executorClient.ListContainers(logger)
	if err != nil {
		logger.Error("failed-listing-containers", err)
		return
	}

	logger.Info("sending-signal-to-containers")

	for _, container := range containers {
		e.executorClient.StopContainer(logger, container.Guid)
	}

	logger.Info("sent-signal-to-containers")
}
