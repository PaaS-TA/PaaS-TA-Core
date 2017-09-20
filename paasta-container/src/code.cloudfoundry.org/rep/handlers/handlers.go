package handlers

import (
	"net/http"

	"code.cloudfoundry.org/executor"
	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/rep"
	"code.cloudfoundry.org/rep/evacuation/evacuation_context"
	"github.com/tedsuo/rata"

	GardenClient "code.cloudfoundry.org/garden/client"
)

//====================================================
// Add - parameter gardenClient GardenClient.Client
//====================================================
func New(
	localCellClient rep.AuctionCellClient,
	executorClient executor.Client,
	gardenClient GardenClient.Client,
	evacuatable evacuation_context.Evacuatable,
	logger lager.Logger,
	secure bool,
) rata.Handlers {

	handlers := rata.Handlers{}
	if secure {
		stateHandler := &state{rep: localCellClient}
		performHandler := &perform{rep: localCellClient}
		resetHandler := &reset{rep: localCellClient}
		stopLrpHandler := NewStopLRPInstanceHandler(executorClient)
		cancelTaskHandler := NewCancelTaskHandler(executorClient)

		handlers[rep.StateRoute] = logWrap(stateHandler.ServeHTTP, logger)
		handlers[rep.PerformRoute] = logWrap(performHandler.ServeHTTP, logger)
		handlers[rep.Sim_ResetRoute] = logWrap(resetHandler.ServeHTTP, logger)

		handlers[rep.StopLRPInstanceRoute] = logWrap(stopLrpHandler.ServeHTTP, logger)
		handlers[rep.CancelTaskRoute] = logWrap(cancelTaskHandler.ServeHTTP, logger)
	} else {
		pingHandler := NewPingHandler()
		evacuationHandler := NewEvacuationHandler(evacuatable)
		containerHandler := NewContainerListHandler(logger, executorClient, gardenClient)

		handlers[rep.PingRoute] = logWrap(pingHandler.ServeHTTP, logger)
		handlers[rep.EvacuateRoute] = logWrap(evacuationHandler.ServeHTTP, logger)
		//===============================================================
		//Added : Get Container List
		handlers[rep.ContainerListRoute] = logWrap(containerHandler.ServeHTTP, logger)
		//===============================================================
	}

	return handlers
}

//====================================================
// Add - parameter gardenClient GardenClient.Client
//====================================================
func NewLegacy(
	localCellClient rep.AuctionCellClient,
	executorClient executor.Client,
	gardenClient GardenClient.Client,
	evacuatable evacuation_context.Evacuatable,
	logger lager.Logger,
) rata.Handlers {
	//insecureHandlers := New(localCellClient, executorClient, evacuatable, logger, false)
	insecureHandlers := New(localCellClient, executorClient, gardenClient, evacuatable, logger, false)
	//secureHandlers := New(localCellClient, executorClient, evacuatable, logger, true)
	secureHandlers := New(localCellClient, executorClient, gardenClient, evacuatable, logger, true)

	for name, handler := range secureHandlers {
		insecureHandlers[name] = handler
	}
	return insecureHandlers
}

func logWrap(loggable func(http.ResponseWriter, *http.Request, lager.Logger), logger lager.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		requestLog := logger.Session("request", lager.Data{
			"method":  r.Method,
			"request": r.URL.String(),
		})

		defer requestLog.Debug("done")
		requestLog.Debug("serving")

		loggable(w, r, requestLog)
	}
}
