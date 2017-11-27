package auctionrunner

import (
	"sync"

	"code.cloudfoundry.org/auction/auctiontypes"
	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/rep"
	"code.cloudfoundry.org/workpool"
)

func FetchStateAndBuildZones(logger lager.Logger, workPool *workpool.WorkPool, clients map[string]rep.Client, metricEmitter auctiontypes.AuctionMetricEmitterDelegate) map[string]Zone {
	var zones map[string]Zone
	for i := 0; ; i++ {
		zones = fetchStateAndBuildZones(logger, workPool, clients, metricEmitter)
		if len(zones) > 0 {
			break
		}
		if i == 3 {
			logger.Info("failed-to-communicate-to-cells-abort")
			break
		}
		logger.Info("failed-to-communicate-to-cells-retry")
	}
	return zones
}

func fetchStateAndBuildZones(logger lager.Logger, workPool *workpool.WorkPool, clients map[string]rep.Client, metricEmitter auctiontypes.AuctionMetricEmitterDelegate) map[string]Zone {
	wg := &sync.WaitGroup{}
	zones := map[string]Zone{}
	lock := &sync.Mutex{}

	wg.Add(len(clients))
	for guid, client := range clients {
		guid, client := guid, client
		workPool.Submit(func() {
			defer wg.Done()
			state, err := client.State(logger)
			if err != nil {
				metricEmitter.FailedCellStateRequest()
				logger.Error("failed-to-get-state", err, lager.Data{"cell-guid": guid})
				return
			}

			if state.Evacuating {
				return
			}

			cell := NewCell(logger, guid, client, state)
			lock.Lock()
			zones[state.Zone] = append(zones[state.Zone], cell)
			lock.Unlock()
		})
	}

	wg.Wait()

	return zones
}
