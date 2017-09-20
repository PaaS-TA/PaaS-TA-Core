package test_helpers

import "net/http"

type FakeResponseWriter struct {
	closeChan chan bool
	Code      int
}

func (rw *FakeResponseWriter) Header() http.Header {
	return nil
}

func (rw *FakeResponseWriter) Write([]byte) (int, error) {
	if rw.Code == 0 {
		rw.Code = http.StatusOK
	}
	return 0, nil
}

func (rw *FakeResponseWriter) WriteHeader(code int) {
	rw.Code = code
}

func (rw *FakeResponseWriter) CloseNotify() <-chan bool {
	return rw.closeChan
}

func NewFakeResponseWriter(closeNotifier chan bool) *FakeResponseWriter {
	return &FakeResponseWriter{
		closeChan: closeNotifier,
	}
}
