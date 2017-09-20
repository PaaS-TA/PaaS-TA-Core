package handlers

import (
	"net/http"

	"code.cloudfoundry.org/bbs"
	"code.cloudfoundry.org/clock"
	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/stager"
	"code.cloudfoundry.org/stager/backend"
	"code.cloudfoundry.org/stager/cc_client"
	"github.com/tedsuo/rata"
)

func New(logger lager.Logger, ccClient cc_client.CcClient, bbsClient bbs.Client, backends map[string]backend.Backend, clock clock.Clock) http.Handler {

	stagingHandler := NewStagingHandler(logger, backends, bbsClient)
	stagingCompletedHandler := NewStagingCompletionHandler(logger, ccClient, backends, clock)

	actions := rata.Handlers{
		stager.StageRoute:            http.HandlerFunc(stagingHandler.Stage),
		stager.StopStagingRoute:      http.HandlerFunc(stagingHandler.StopStaging),
		stager.StagingCompletedRoute: http.HandlerFunc(stagingCompletedHandler.StagingComplete),
	}

	handler, err := rata.NewRouter(stager.Routes, actions)
	if err != nil {
		panic("unable to create router: " + err.Error())
	}

	return handler
}
