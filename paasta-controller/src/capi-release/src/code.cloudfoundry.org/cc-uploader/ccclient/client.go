package ccclient

import (
	"net/http"
	"net/url"
)

//go:generate counterfeiter -o fake_ccclient/fake_uploader.go . Uploader
type Uploader interface {
	Upload(uploadURL *url.URL, filename string, r *http.Request, cancelChan <-chan struct{}) (*http.Response, error)
}

//go:generate counterfeiter -o fake_ccclient/fake_poller.go . Poller
type Poller interface {
	Poll(fallbackURL *url.URL, res *http.Response, cancelChan <-chan struct{}) error
}

type requestCanceller interface {
	CancelRequest(req *http.Request)
}
