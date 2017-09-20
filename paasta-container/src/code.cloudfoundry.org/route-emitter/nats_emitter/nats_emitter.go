package nats_emitter

import (
	"encoding/json"
	"sync"

	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/route-emitter/diegonats"
	"code.cloudfoundry.org/route-emitter/routing_table"
	"code.cloudfoundry.org/runtimeschema/metric"
	"code.cloudfoundry.org/workpool"
)

var messagesEmitted = metric.Counter("MessagesEmitted")

//go:generate counterfeiter -o fake_nats_emitter/fake_nats_emitter.go . NATSEmitter
type NATSEmitter interface {
	Emit(messagesToEmit routing_table.MessagesToEmit) error
}

type natsEmitter struct {
	natsClient diegonats.NATSClient
	workPool   *workpool.WorkPool
	logger     lager.Logger
}

func New(natsClient diegonats.NATSClient, workPool *workpool.WorkPool, logger lager.Logger) NATSEmitter {
	return &natsEmitter{
		natsClient: natsClient,
		workPool:   workPool,
		logger:     logger.Session("nats-emitter"),
	}
}

func (n *natsEmitter) Emit(messagesToEmit routing_table.MessagesToEmit) error {
	errors := make(chan error, 1)
	var wg sync.WaitGroup
	wg.Add(len(messagesToEmit.RegistrationMessages))
	for _, message := range messagesToEmit.RegistrationMessages {
		n.emit("router.register", message, &wg, errors)
	}

	wg.Add(len(messagesToEmit.UnregistrationMessages))
	for _, message := range messagesToEmit.UnregistrationMessages {
		n.emit("router.unregister", message, &wg, errors)
	}

	wg.Wait()

	select {
	case finalError := <-errors:
		return finalError
	default:
	}

	numberOfMessages := uint64(len(messagesToEmit.RegistrationMessages) + len(messagesToEmit.UnregistrationMessages))
	messagesEmitted.Add(numberOfMessages)

	return nil
}

func (n *natsEmitter) emit(subject string, message routing_table.RegistryMessage, wg *sync.WaitGroup, errors chan error) {
	n.workPool.Submit(func() {
		var err error
		defer func() {
			if err != nil {
				select {
				case errors <- err:
				default:
				}
			}
			wg.Done()
		}()

		n.logger.Debug("emit", lager.Data{
			"subject": subject,
			"message": message,
		})

		payload, err := json.Marshal(message)
		if err != nil {
			n.logger.Error("failed-to-marshal", err, lager.Data{
				"message": message,
				"subject": subject,
			})
		}

		err = n.natsClient.Publish(subject, payload)
		if err != nil {
			n.logger.Error("failed-to-publish", err, lager.Data{
				"message": message,
				"subject": subject,
			})
		}
	})
}
