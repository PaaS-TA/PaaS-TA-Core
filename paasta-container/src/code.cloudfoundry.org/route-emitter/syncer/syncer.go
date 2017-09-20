package syncer

import (
	"encoding/json"
	"os"
	"time"

	"code.cloudfoundry.org/clock"
	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/route-emitter/diegonats"
	"code.cloudfoundry.org/route-emitter/routing_table"
	"github.com/nats-io/nats"
	uuid "github.com/nu7hatch/gouuid"
)

type Syncer struct {
	natsClient   diegonats.NATSClient
	clock        clock.Clock
	syncInterval time.Duration
	events       Events
	routerGreet  chan time.Duration

	logger lager.Logger
}

func NewSyncer(
	clock clock.Clock,
	syncInterval time.Duration,
	natsClient diegonats.NATSClient,
	logger lager.Logger,
) *Syncer {
	return &Syncer{
		natsClient: natsClient,

		clock:        clock,
		syncInterval: syncInterval,
		events: Events{
			Sync: make(chan struct{}, 1),
			Emit: make(chan struct{}, 1),
		},

		routerGreet: make(chan time.Duration),

		logger: logger.Session("syncer"),
	}
}

func (s *Syncer) Run(signals <-chan os.Signal, ready chan<- struct{}) error {
	s.logger.Info("starting")
	replyUuid, err := uuid.NewV4()
	if err != nil {
		return err
	}

	err = s.listenForRouter(replyUuid.String())
	if err != nil {
		return err
	}

	close(ready)
	s.logger.Info("started")

	var routerPruneInterval time.Duration
	retryGreetingTicker := s.clock.NewTicker(time.Second)

	//keep trying to greet until we hear from the router
GREET_LOOP:
	for {
		s.logger.Info("greeting-router")
		err := s.greetRouter(replyUuid.String())
		if err != nil {
			s.logger.Error("failed-to-greet-router", err)
			return err
		}

		select {
		case routerPruneInterval = <-s.routerGreet:
			s.logger.Info("received-router-prune-interval", lager.Data{"interval": routerPruneInterval.String()})
			break GREET_LOOP
		case <-retryGreetingTicker.C():
		case <-signals:
			s.logger.Info("stopping")
			return nil
		}
	}
	retryGreetingTicker.Stop()

	s.sync()

	//now keep emitting at the desired interval, syncing with etcd every syncInterval
	syncTicker := s.clock.NewTicker(s.syncInterval)
	routerTicker := s.clock.NewTicker(routerPruneInterval)

	for {
		select {
		case routerPruneInterval = <-s.routerGreet:
			s.logger.Info("received-new-router-prune-interval", lager.Data{"interval": routerPruneInterval.String()})
			routerTicker.Stop()
			routerTicker = s.clock.NewTicker(routerPruneInterval)
			s.emit()
		case <-routerTicker.C():
			s.logger.Info("emitting-routes")
			s.emit()
		case <-syncTicker.C():
			s.logger.Info("syncing")
			s.sync()
		case <-signals:
			s.logger.Info("stopping")
			syncTicker.Stop()
			routerTicker.Stop()
			return nil
		}
	}

	return nil
}

func (s *Syncer) Events() Events {
	return s.events
}

func (s *Syncer) emit() {
	select {
	case s.events.Emit <- struct{}{}:
	default:
		s.logger.Debug("emit-already-in-progress")
	}
}

func (s *Syncer) sync() {
	select {
	case s.events.Sync <- struct{}{}:
	default:
		s.logger.Debug("sync-already-in-progress")
	}
}

func (s *Syncer) listenForRouter(replyUUID string) error {
	_, err := s.natsClient.Subscribe("router.start", s.handleRouterGreet)
	if err != nil {
		return err
	}

	sub, err := s.natsClient.Subscribe(replyUUID, s.handleRouterGreet)
	if err != nil {
		return err
	}
	sub.AutoUnsubscribe(1)

	return nil
}

func (s *Syncer) greetRouter(replyUUID string) error {
	err := s.natsClient.PublishRequest("router.greet", replyUUID, []byte{})
	if err != nil {
		return err
	}

	return nil
}

func (s *Syncer) handleRouterGreet(msg *nats.Msg) {
	var response routing_table.RouterGreetingMessage

	err := json.Unmarshal(msg.Data, &response)
	if err != nil {
		s.logger.Error("received-invalid-router-start", err, lager.Data{
			"payload": msg.Data,
		})
		return
	}

	greetInterval := response.MinimumRegisterInterval
	s.routerGreet <- time.Duration(greetInterval) * time.Second
}
