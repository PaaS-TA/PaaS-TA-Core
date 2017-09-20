package lrpstatus

import (
	"encoding/json"
	"net/http"

	"code.cloudfoundry.org/bbs"
	"code.cloudfoundry.org/bbs/models"
	"code.cloudfoundry.org/clock"
	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/tps/handler/cc_conv"

	"code.cloudfoundry.org/runtimeschema/cc_messages"
)

type handler struct {
	apiClient bbs.Client
	clock     clock.Clock
	logger    lager.Logger
}

func NewHandler(apiClient bbs.Client, clk clock.Clock, logger lager.Logger) http.Handler {
	return &handler{
		apiClient: apiClient,
		clock:     clk,
		logger:    logger,
	}
}

func (handler *handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	guid := r.FormValue(":guid")
	logger := handler.logger.Session("lrp-status", lager.Data{"process-guid": guid})

	logger.Info("fetching-actual-lrp-info")
	actualLRPGroups, err := handler.apiClient.ActualLRPGroupsByProcessGuid(logger, guid)
	if err != nil {
		logger.Error("failed-fetching-actual-lrp-info", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	instances := LRPInstances(actualLRPGroups,
		func(instance *cc_messages.LRPInstance, actual *models.ActualLRP) {
			instance.Details = actual.PlacementError
		},
		handler.clock,
	)

	err = json.NewEncoder(w).Encode(instances)
	if err != nil {
		logger.Error("stream-response-failed", err)
	}
}

func LRPInstances(
	actualLRPGroups []*models.ActualLRPGroup,
	addInfo func(*cc_messages.LRPInstance, *models.ActualLRP),
	clk clock.Clock,
) []cc_messages.LRPInstance {
	instances := make([]cc_messages.LRPInstance, len(actualLRPGroups))
	for i, actualLRPGroup := range actualLRPGroups {
		actual, _ := actualLRPGroup.Resolve()

		instance := cc_messages.LRPInstance{
			ProcessGuid:  actual.ProcessGuid,
			InstanceGuid: actual.InstanceGuid,
			Index:        uint(actual.Index),
			Since:        actual.Since / 1e9,
			Uptime:       (clk.Now().UnixNano() - actual.Since) / 1e9,
			State:        cc_conv.StateFor(actual.State, actual.PlacementError),
			NetInfo:      actual.ActualLRPNetInfo,
		}

		if addInfo != nil {
			addInfo(&instance, actual)
		}

		instances[i] = instance
	}

	return instances
}
