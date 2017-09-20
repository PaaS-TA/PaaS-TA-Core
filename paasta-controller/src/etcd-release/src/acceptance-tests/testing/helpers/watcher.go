package helpers

import (
	"sync"

	goetcd "github.com/coreos/go-etcd/etcd"
)

type etcdWatcher interface {
	Watch(prefix string, waitIndex uint64, recursive bool,
		receiver chan *goetcd.Response, stop chan bool) (*goetcd.Response, error)
}

type Watcher struct {
	Response          chan *goetcd.Response
	Stop              chan bool
	stopListening     chan bool
	stopMutex         sync.Mutex
	dataMutex         sync.Mutex
	data              map[string]string
	stopped           bool
	lastModifiedIndex uint64
}

func Watch(watcher etcdWatcher, prefix string) *Watcher {
	w := &Watcher{
		data:          map[string]string{},
		Response:      make(chan *goetcd.Response),
		Stop:          make(chan bool),
		stopListening: make(chan bool),
	}
	go func() {
		for {
			go w.listenForResponses()

			_, err := watcher.Watch(prefix, w.getLastModifiedIndex()+1, true, w.Response, w.Stop)
			<-w.stopListening
			if err == nil {
				w.setStopped(true)
				return
			} else {
				w.Response = make(chan *goetcd.Response)
				w.Stop = make(chan bool)
			}
		}
	}()

	return w
}

func (w *Watcher) listenForResponses() {
	for {
		r, ok := <-w.Response
		if !ok {
			w.stopListening <- true
			return
		}
		if r != nil && r.Node != nil {
			w.AddData(r.Node.Key, r.Node.Value, r.Node.ModifiedIndex)
		}
	}
}

func (w *Watcher) getLastModifiedIndex() uint64 {
	w.dataMutex.Lock()
	defer w.dataMutex.Unlock()

	return w.lastModifiedIndex
}

func (w *Watcher) setStopped(stopped bool) {
	w.stopMutex.Lock()
	defer w.stopMutex.Unlock()

	w.stopped = stopped
}

func (w *Watcher) IsStopped() bool {
	w.stopMutex.Lock()
	defer w.stopMutex.Unlock()

	return w.stopped
}

func (w *Watcher) AddData(key, value string, modifiedIndex uint64) {
	w.dataMutex.Lock()
	defer w.dataMutex.Unlock()

	w.data[key] = value
	w.lastModifiedIndex = modifiedIndex
}

func (w *Watcher) Data() map[string]string {
	w.dataMutex.Lock()
	defer w.dataMutex.Unlock()

	data := map[string]string{}
	for k, v := range w.data {
		data[k] = v
	}

	return data
}
