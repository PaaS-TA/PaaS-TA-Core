package ccclient

import (
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"net/url"

	"code.cloudfoundry.org/lager"
)

const MAX_UPLOAD_RETRIES = 3

type uploader struct {
	logger    lager.Logger
	client    *http.Client
	tlsClient *http.Client
}

func NewUploader(logger lager.Logger, httpClient *http.Client) Uploader {
	return &uploader{
		client: httpClient,
		logger: logger.Session("uploader"),
	}
}

const contentMD5Header = "Content-MD5"

func (u *uploader) Upload(uploadURL *url.URL, filename string, r *http.Request, cancelChan <-chan struct{}) (*http.Response, error) {
	if r.ContentLength <= 0 {
		return &http.Response{StatusCode: http.StatusLengthRequired}, fmt.Errorf("Missing Content Length")
	}
	defer r.Body.Close()

	uploadReq, err := newMultipartRequestFromReader(r.ContentLength, r.Body, filename)
	if err != nil {
		return nil, err
	}

	uploadReq.Header.Set(contentMD5Header, r.Header.Get(contentMD5Header))
	uploadReq.URL = uploadURL

	var rsp *http.Response
	var uploadErr error
	for attempt := 0; attempt < MAX_UPLOAD_RETRIES; attempt++ {
		logger := u.logger.WithData(lager.Data{"attempt-number": attempt})
		logger.Info("uploading")
		rsp, uploadErr = u.do(uploadReq, cancelChan)
		if uploadErr == nil {
			logger.Info("succeeded-uploading")
			break
		}
		logger.Error("failed-uploading", err)

		// not a connect (dial) error
		var nestedErr error = uploadErr
		if urlErr, ok := nestedErr.(*url.Error); ok {
			nestedErr = urlErr.Err
		}

		if netErr, ok := nestedErr.(*net.OpError); !ok || netErr.Op != "dial" {
			break
		}
	}

	return rsp, uploadErr
}

func (u *uploader) do(req *http.Request, cancelChan <-chan struct{}) (*http.Response, error) {
	completion := make(chan struct{})
	defer close(completion)

	go func() {
		select {
		case <-cancelChan:
			if canceller, ok := u.client.Transport.(requestCanceller); ok {
				canceller.CancelRequest(req)
			} else {
				u.logger.Error("Invalid transport, does not support CancelRequest", nil, lager.Data{"transport": u.client.Transport})
			}
		case <-completion:
		}
	}()

	rsp, err := u.client.Do(req)

	req.Body.Close()
	if err != nil {
		return nil, err
	}

	switch rsp.StatusCode {
	case http.StatusOK, http.StatusCreated:
		return rsp, nil
	}

	respBody, _ := ioutil.ReadAll(rsp.Body)
	rsp.Body.Close()
	return rsp, fmt.Errorf("status code: %d\n%s", rsp.StatusCode, string(respBody))
}
