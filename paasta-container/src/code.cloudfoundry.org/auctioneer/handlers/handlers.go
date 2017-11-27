package handlers

import (
	"net/http"
	"time"

	"code.cloudfoundry.org/auction/auctiontypes"
	"code.cloudfoundry.org/auctioneer"
	"code.cloudfoundry.org/bbs/handlers/middleware"
	loggregator_v2 "code.cloudfoundry.org/go-loggregator/compatibility"
	"code.cloudfoundry.org/lager"
	"github.com/tedsuo/rata"
)

func New(logger lager.Logger, runner auctiontypes.AuctionRunner, metronClient loggregator_v2.IngressClient) http.Handler {
	taskAuctionHandler := logWrap(NewTaskAuctionHandler(runner).Create, logger)
	lrpAuctionHandler := logWrap(NewLRPAuctionHandler(runner).Create, logger)

	emitter := &auctioneerEmitter{
		logger:       logger,
		metronClient: metronClient,
	}

	actions := rata.Handlers{
		auctioneer.CreateTaskAuctionsRoute: middleware.RecordLatency(taskAuctionHandler, emitter),
		auctioneer.CreateLRPAuctionsRoute:  middleware.RecordLatency(lrpAuctionHandler, emitter),
	}

	handler, err := rata.NewRouter(auctioneer.Routes, actions)
	if err != nil {
		panic("unable to create router: " + err.Error())
	}

	return middleware.RecordRequestCount(handler, emitter)
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

type auctioneerEmitter struct {
	logger       lager.Logger
	metronClient loggregator_v2.IngressClient
}

func (e *auctioneerEmitter) IncrementCounter(delta int) {
	e.metronClient.IncrementCounter(middleware.RequestCount)
}

func (e *auctioneerEmitter) UpdateLatency(latency time.Duration) {
	err := e.metronClient.SendDuration(middleware.RequestLatency, latency)
	if err != nil {
		e.logger.Error("failed-to-send-latency", err)
	}
}
