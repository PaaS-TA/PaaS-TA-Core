package messagebus

import (
	"encoding/json"
	"fmt"
	"time"

	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/route-registrar/config"
	"github.com/nats-io/nats"
)

//go:generate counterfeiter . MessageBus

type MessageBus interface {
	Connect(servers []config.MessageBusServer) error
	SendMessage(subject string, host string, route config.Route, privateInstanceId string) error
	Close()
}

type msgBus struct {
	natsConn *nats.Conn
	logger   lager.Logger
}

type Message struct {
	URIs              []string          `json:"uris"`
	Host              string            `json:"host"`
	Port              int               `json:"port"`
	Tags              map[string]string `json:"tags"`
	RouteServiceUrl   string            `json:"route_service_url,omitempty"`
	PrivateInstanceId string            `json:"private_instance_id"`
}

func NewMessageBus(logger lager.Logger) MessageBus {
	return &msgBus{
		logger: logger,
	}
}

func (m *msgBus) Connect(servers []config.MessageBusServer) error {
	m.logger.Debug("Connecting to nats", lager.Data{"servers": servers})

	var natsServers []string
	for _, server := range servers {
		m.logger.Info(
			"Adding NATS server",
			lager.Data{"server": server},
		)
		natsServers = append(
			natsServers,
			fmt.Sprintf("nats://%s:%s@%s", server.User, server.Password, server.Host),
		)
	}

	opts := nats.DefaultOptions
	opts.Servers = natsServers
	opts.PingInterval = 20 * time.Second
	natsConn, err := opts.Connect()

	if err != nil {
		return err
	}

	m.natsConn = natsConn

	return nil
}

func (m msgBus) SendMessage(subject string, host string, route config.Route, privateInstanceId string) error {
	m.logger.Debug("Creating message", lager.Data{"subject": subject, "host": host, "route": route, "privateInstanceId": privateInstanceId})

	msg := &Message{
		URIs:              route.URIs,
		Host:              host,
		Port:              route.Port,
		Tags:              route.Tags,
		RouteServiceUrl:   route.RouteServiceUrl,
		PrivateInstanceId: privateInstanceId,
	}

	json, err := json.Marshal(msg)
	if err != nil {
		// Untested as we cannot force json.Marshal to return error.
		return err
	}

	m.logger.Debug("Publishing message", lager.Data{"msg": string(json)})

	return m.natsConn.Publish(subject, json)
}

func (m msgBus) Close() {
	m.natsConn.Close()
}
