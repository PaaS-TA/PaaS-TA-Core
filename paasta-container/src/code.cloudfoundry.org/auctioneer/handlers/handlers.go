package handlers

import (
	"net/http"

	"code.cloudfoundry.org/auction/auctiontypes"
	"code.cloudfoundry.org/auctioneer"
	"code.cloudfoundry.org/bbs/handlers/middleware"
	"code.cloudfoundry.org/lager"
	"github.com/tedsuo/rata"
)

func New(runner auctiontypes.AuctionRunner, logger lager.Logger) http.Handler {
	taskAuctionHandler := logWrap(NewTaskAuctionHandler(runner).Create, logger)
	lrpAuctionHandler := logWrap(NewLRPAuctionHandler(runner).Create, logger)

	emitter := middleware.NewLatencyEmitter(logger)
	actions := rata.Handlers{
		auctioneer.CreateTaskAuctionsRoute: emitter.EmitLatency(taskAuctionHandler),
		auctioneer.CreateLRPAuctionsRoute:  emitter.EmitLatency(lrpAuctionHandler),
	}

	handler, err := rata.NewRouter(auctioneer.Routes, actions)
	if err != nil {
		panic("unable to create router: " + err.Error())
	}

	return middleware.RequestCountWrap(handler)
}

func logWrap(loggable func(http.ResponseWriter, *http.Request, lager.Logger), logger lager.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		requestLog := logger.Session("request", lager.Data{
			"method":  r.Method,
			"request": r.URL.String(),
		})

		requestLog.Info("serving")
		loggable(w, r, requestLog)
		requestLog.Info("done")
	}
}
