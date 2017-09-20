package db

import (
	"encoding/json"
	"fmt"

	"github.com/coreos/etcd/client"
)

type Event struct {
	Type  EventType
	Value string
}

type EventType int

const (
	InvalidEvent = EventType(iota)
	CreateEvent
	DeleteEvent
	ExpireEvent
	UpdateEvent
)

func (e EventType) String() string {
	switch e {
	case CreateEvent:
		return "Upsert"
	case UpdateEvent:
		return "Upsert"
	case DeleteEvent, ExpireEvent:
		return "Delete"
	default:
		return "Invalid"
	}
}

func NewEventFromInterface(eventType EventType, obj interface{}) (Event, error) {
	data, err := json.Marshal(obj)
	if err != nil {
		return Event{}, err
	}

	return Event{
		Type:  eventType,
		Value: string(data),
	}, nil
}

func NewEventFromEtcd(event *client.Response) (Event, error) {
	var eventType EventType

	node := event.Node
	switch event.Action {
	case "delete", "compareAndDelete":
		eventType = DeleteEvent
		node = event.PrevNode
	case "create":
		eventType = CreateEvent
	case "set", "update", "compareAndSwap":
		eventType = UpdateEvent
	case "expire":
		eventType = ExpireEvent
		node = event.PrevNode
	default:
		return Event{}, fmt.Errorf("unknown event: %s", event.Action)
	}

	newEvent := Event{Type: eventType}

	if node != nil {
		newEvent.Value = node.Value
	}

	return newEvent, nil
}
