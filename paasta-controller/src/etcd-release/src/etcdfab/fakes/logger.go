package fakes

import (
	"sync"

	"code.cloudfoundry.org/lager"
)

type LoggerMessage struct {
	Action string
	Error  error
	Data   []lager.Data
}

type Logger struct {
	sync.Mutex
	messages []LoggerMessage
}

func (l *Logger) Info(action string, data ...lager.Data) {
	l.Lock()
	defer l.Unlock()

	l.messages = append(l.messages, LoggerMessage{
		Action: action,
		Data:   data,
	})
}

func (l *Logger) Error(action string, err error, data ...lager.Data) {
	l.Lock()
	defer l.Unlock()

	l.messages = append(l.messages, LoggerMessage{
		Action: action,
		Error:  err,
		Data:   data,
	})
}

func (l *Logger) Messages() []LoggerMessage {
	l.Lock()
	defer l.Unlock()

	return l.messages
}
