package backend

import (
	"encoding/json"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
)

type EtcdBackendServer struct {
	server *httptest.Server

	callCount int32
	args      []string
	fastFail  bool

	mutex sync.Mutex
}

func NewEtcdBackendServer() *EtcdBackendServer {
	etcdBackendServer := &EtcdBackendServer{}
	etcdBackendServer.server = httptest.NewServer(http.HandlerFunc(func(responseWriter http.ResponseWriter, request *http.Request) {
		etcdBackendServer.ServeHTTP(responseWriter, request)
	}))

	return etcdBackendServer
}

func (e *EtcdBackendServer) ServeHTTP(responseWriter http.ResponseWriter, request *http.Request) {
	switch request.URL.Path {
	case "/call":
		e.call(responseWriter, request)
	}
}

func (e *EtcdBackendServer) call(responseWriter http.ResponseWriter, request *http.Request) {
	atomic.AddInt32(&e.callCount, 1)
	argsJSON, err := ioutil.ReadAll(request.Body)
	if err != nil {
		panic(err)
	}

	var args []string
	err = json.Unmarshal(argsJSON, &args)
	if err != nil {
		panic(err)
	}

	e.setArgs(args)

	if e.FastFail() {
		responseWriter.WriteHeader(http.StatusInternalServerError)
	} else {
		responseWriter.WriteHeader(http.StatusOK)
	}
}

func (e *EtcdBackendServer) setArgs(args []string) {
	e.mutex.Lock()
	defer e.mutex.Unlock()
	e.args = args
}

func (e *EtcdBackendServer) EnableFastFail() {
	e.mutex.Lock()
	defer e.mutex.Unlock()
	e.fastFail = true
}

func (e *EtcdBackendServer) DisableFastFail() {
	e.mutex.Lock()
	defer e.mutex.Unlock()
	e.fastFail = false
}

func (e *EtcdBackendServer) FastFail() bool {
	e.mutex.Lock()
	defer e.mutex.Unlock()
	return e.fastFail
}

func (e *EtcdBackendServer) ServerURL() string {
	return e.server.URL
}

func (e *EtcdBackendServer) GetCallCount() int {
	return int(atomic.LoadInt32(&e.callCount))
}

func (e *EtcdBackendServer) GetArgs() []string {
	e.mutex.Lock()
	defer e.mutex.Unlock()
	return e.args
}
