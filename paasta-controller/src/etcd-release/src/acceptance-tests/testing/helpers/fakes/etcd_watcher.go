package fakes

import (
	goetcd "github.com/coreos/go-etcd/etcd"
)

type EtcdWatcher struct {
	WatchCall struct {
		CallCount int
		Started   chan bool
		Receives  struct {
			Prefix    string
			WaitIndex uint64
			Recursive bool
			Receiver  chan *goetcd.Response
			Stop      chan bool
		}
		Returns struct {
			Response *goetcd.Response
			Error    error
		}
	}
}

func (w *EtcdWatcher) Watch(prefix string, waitIndex uint64, recursive bool,
	receiver chan *goetcd.Response, stop chan bool) (*goetcd.Response, error) {
	w.WatchCall.CallCount++
	w.WatchCall.Receives.Prefix = prefix
	w.WatchCall.Receives.WaitIndex = waitIndex
	w.WatchCall.Receives.Recursive = recursive
	w.WatchCall.Receives.Receiver = receiver
	w.WatchCall.Receives.Stop = stop

	defer close(w.WatchCall.Receives.Receiver)

	w.WatchCall.Started <- true
	<-w.WatchCall.Receives.Stop

	return w.WatchCall.Returns.Response, w.WatchCall.Returns.Error
}
