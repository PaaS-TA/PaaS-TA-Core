package auctionrunner

import (
	"os"
	"time"

	"code.cloudfoundry.org/clock"
	"code.cloudfoundry.org/lager"

	"code.cloudfoundry.org/auction/auctiontypes"
	"code.cloudfoundry.org/auctioneer"
	"code.cloudfoundry.org/workpool"
)

type auctionRunner struct {
	logger lager.Logger

	delegate                auctiontypes.AuctionRunnerDelegate
	metricEmitter           auctiontypes.AuctionMetricEmitterDelegate
	batch                   *Batch
	clock                   clock.Clock
	workPool                *workpool.WorkPool
	startingContainerWeight float64
}

func New(
	logger lager.Logger,
	delegate auctiontypes.AuctionRunnerDelegate,
	metricEmitter auctiontypes.AuctionMetricEmitterDelegate,
	clock clock.Clock,
	workPool *workpool.WorkPool,
	startingContainerWeight float64,
) *auctionRunner {
	return &auctionRunner{
		logger: logger,

		delegate:                delegate,
		metricEmitter:           metricEmitter,
		batch:                   NewBatch(clock),
		clock:                   clock,
		workPool:                workPool,
		startingContainerWeight: startingContainerWeight,
	}
}

func (a *auctionRunner) Run(signals <-chan os.Signal, ready chan<- struct{}) error {
	close(ready)

	var hasWork chan struct{}
	hasWork = a.batch.HasWork

	for {
		select {
		case <-hasWork:
			logger := a.logger.Session("auction")

			logger.Info("fetching-cell-reps")
			clients, err := a.delegate.FetchCellReps()
			if err != nil {
				logger.Error("failed-to-fetch-reps", err)
				time.Sleep(time.Second)
				hasWork = make(chan struct{}, 1)
				hasWork <- struct{}{}
				break
			}
			logger.Info("fetched-cell-reps", lager.Data{"cell-reps-count": len(clients)})

			hasWork = a.batch.HasWork

			logger.Info("fetching-zone-state")
			fetchStatesStartTime := time.Now()
			zones := FetchStateAndBuildZones(logger, a.workPool, clients, a.metricEmitter)
			fetchStateDuration := time.Since(fetchStatesStartTime)
			err = a.metricEmitter.FetchStatesCompleted(fetchStateDuration)
			if err != nil {
				logger.Error("failed-sending-fetch-states-completed-metric", err)
			}

			cellCount := 0
			for zone, cells := range zones {
				logger.Info("zone-state", lager.Data{"zone": zone, "cell-count": len(cells)})
				cellCount += len(cells)
			}
			logger.Info("fetched-zone-state", lager.Data{
				"cell-state-count":    cellCount,
				"num-failed-requests": len(clients) - cellCount,
				"duration":            fetchStateDuration.String(),
			})

			logger.Info("fetching-auctions")
			lrpAuctions, taskAuctions := a.batch.DedupeAndDrain()
			logger.Info("fetched-auctions", lager.Data{
				"lrp-start-auctions": len(lrpAuctions),
				"task-auctions":      len(taskAuctions),
			})
			if len(lrpAuctions) == 0 && len(taskAuctions) == 0 {
				logger.Info("nothing-to-auction")
				break
			}

			logger.Info("scheduling")
			auctionRequest := auctiontypes.AuctionRequest{
				LRPs:  lrpAuctions,
				Tasks: taskAuctions,
			}

			scheduler := NewScheduler(a.workPool, zones, a.clock, logger, a.startingContainerWeight)
			auctionResults := scheduler.Schedule(auctionRequest)
			logger.Info("scheduled", lager.Data{
				"successful-lrp-start-auctions": len(auctionResults.SuccessfulLRPs),
				"successful-task-auctions":      len(auctionResults.SuccessfulTasks),
				"failed-lrp-start-auctions":     len(auctionResults.FailedLRPs),
				"failed-task-auctions":          len(auctionResults.FailedTasks),
			})

			a.metricEmitter.AuctionCompleted(auctionResults)
			a.delegate.AuctionCompleted(auctionResults)
		case <-signals:
			return nil
		}
	}
}

func (a *auctionRunner) ScheduleLRPsForAuctions(lrpStarts []auctioneer.LRPStartRequest) {
	a.batch.AddLRPStarts(lrpStarts)
}

func (a *auctionRunner) ScheduleTasksForAuctions(tasks []auctioneer.TaskStartRequest) {
	a.batch.AddTasks(tasks)
}
