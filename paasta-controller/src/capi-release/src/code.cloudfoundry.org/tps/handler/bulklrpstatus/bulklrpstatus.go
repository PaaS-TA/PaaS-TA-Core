package bulklrpstatus

import (
	"encoding/json"
	"net/http"
	"regexp"
	"strings"
	"sync"

	"code.cloudfoundry.org/bbs"
	"code.cloudfoundry.org/bbs/models"
	"code.cloudfoundry.org/clock"
	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/runtimeschema/cc_messages"
	"code.cloudfoundry.org/tps/handler/lrpstatus"
	"code.cloudfoundry.org/workpool"
)

var processGuidPattern = regexp.MustCompile(`^([a-zA-Z0-9_-]+,)*[a-zA-Z0-9_-]+$`)

type handler struct {
	bbsClient                 bbs.Client
	clock                     clock.Clock
	logger                    lager.Logger
	bulkLRPStatusWorkPoolSize int
}

func NewHandler(bbsClient bbs.Client, clk clock.Clock, bulkLRPStatusWorkPoolSize int, logger lager.Logger) http.Handler {
	return &handler{
		bbsClient: bbsClient,
		clock:     clk,
		bulkLRPStatusWorkPoolSize: bulkLRPStatusWorkPoolSize,
		logger: logger,
	}
}

func (handler *handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	logger := handler.logger.Session("bulk-lrp-status")

	guidParameter := r.FormValue("guids")
	if !processGuidPattern.Match([]byte(guidParameter)) {
		logger.Error("failed-parsing-guids", nil, lager.Data{"guid-parameter": guidParameter})
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	guids := strings.Split(guidParameter, ",")
	works := []func(){}

	statusBundle := make(map[string][]cc_messages.LRPInstance)
	statusLock := sync.Mutex{}

	for _, processGuid := range guids {
		works = append(works, handler.getStatusForLRPWorkFunction(logger, processGuid, &statusLock, statusBundle))
	}

	throttler, err := workpool.NewThrottler(handler.bulkLRPStatusWorkPoolSize, works)
	if err != nil {
		logger.Error("failed-constructing-throttler", err, lager.Data{"max-workers": handler.bulkLRPStatusWorkPoolSize, "num-works": len(works)})
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	throttler.Work()

	w.WriteHeader(http.StatusOK)
	w.Header().Set("Content-Type", "application/json")

	err = json.NewEncoder(w).Encode(statusBundle)
	if err != nil {
		logger.Error("stream-response-failed", err, nil)
	}
}

func (handler *handler) getStatusForLRPWorkFunction(logger lager.Logger, processGuid string, statusLock *sync.Mutex, statusBundle map[string][]cc_messages.LRPInstance) func() {
	return func() {
		logger = logger.Session("fetching-actual-lrps-info", lager.Data{"process-guid": processGuid})
		logger.Info("start")
		defer logger.Info("complete")
		actualLRPGroups, err := handler.bbsClient.ActualLRPGroupsByProcessGuid(logger, processGuid)
		if err != nil {
			logger.Error("fetching-actual-lrps-info-failed", err)
			return
		}

		instances := lrpstatus.LRPInstances(actualLRPGroups,
			func(instance *cc_messages.LRPInstance, actual *models.ActualLRP) {
				instance.Details = actual.PlacementError
			},
			handler.clock,
		)

		statusLock.Lock()
		statusBundle[processGuid] = instances
		statusLock.Unlock()
	}
}
