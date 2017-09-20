package test_helpers

import (
	"errors"
	"net/http"
	"sync"
)

type RespErrorPair struct {
	Resp *http.Response
	Err  error
}

type fakeRoundTripper struct {
	reqChan chan *http.Request
	respMap map[string]RespErrorPair

	mu       sync.Mutex
	requests map[*http.Request]chan struct{}
}

func (f *fakeRoundTripper) RoundTrip(r *http.Request) (*http.Response, error) {
	barrier := make(chan struct{})
	f.mu.Lock()
	f.requests[r] = barrier
	pair := f.respMap[r.URL.Host]
	f.mu.Unlock()

	defer func() {
		f.mu.Lock()
		delete(f.requests, r)
		f.mu.Unlock()
	}()

	select {
	case f.reqChan <- r:
		return pair.Resp, pair.Err
	case <-barrier:
		return nil, errors.New("cancelled")
	}
}

func (f *fakeRoundTripper) CancelRequest(req *http.Request) {
	f.mu.Lock()
	barrier := f.requests[req]
	f.mu.Unlock()

	if barrier != nil {
		close(barrier)
	}
}

func NewFakeRoundTripper(reqChan chan *http.Request, responses map[string]RespErrorPair) *fakeRoundTripper {
	return &fakeRoundTripper{
		reqChan:  reqChan,
		respMap:  responses,
		requests: make(map[*http.Request]chan struct{})}
}
