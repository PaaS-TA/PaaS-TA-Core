package upload_droplet

import (
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"code.cloudfoundry.org/cc-uploader/ccclient"
	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/runtimeschema/cc_messages"
)

func New(uploader ccclient.Uploader, poller ccclient.Poller, logger lager.Logger) http.Handler {
	return &dropletUploader{
		uploader: uploader,
		poller:   poller,
		logger:   logger,
	}
}

type dropletUploader struct {
	uploader ccclient.Uploader
	poller   ccclient.Poller
	logger   lager.Logger
}

var MissingCCDropletUploadUriKeyError = errors.New(fmt.Sprintf("missing %s parameter", cc_messages.CcDropletUploadUriKey))

func (h *dropletUploader) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	logger := h.logger.Session("droplet.upload")

	logger.Info("extracting-droplet-upload-uri-key")
	uploadUriParameter := r.URL.Query().Get(cc_messages.CcDropletUploadUriKey)
	if uploadUriParameter == "" {
		logger.Error("failed-extracting-droplet-upload-uri-key", MissingCCDropletUploadUriKeyError)
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(MissingCCDropletUploadUriKeyError.Error()))
		return
	}
	logger.Info("succeeded-extracting-droplet-upload-uri-key")

	logger.Info("parsing-upload-uri-parameter")
	uploadUrl, err := url.Parse(uploadUriParameter)
	if err != nil {
		logger.Error("failed-parsing-upload-uri-parameter", err)
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(err.Error()))
		return
	}
	logger.Info("succeeded-parsing-upload-uri-parameter")

	timeout := 5 * time.Minute
	timeoutParameter := r.URL.Query().Get(cc_messages.CcTimeoutKey)
	if timeoutParameter != "" {
		t, err := strconv.Atoi(timeoutParameter)
		if err != nil {
			logger.Error("failed-converting-timeout-parameter", err)
			w.WriteHeader(http.StatusBadRequest)
			w.Write([]byte(err.Error()))
			return
		}
		timeout = time.Duration(t) * time.Second
	}

	query := uploadUrl.Query()
	query.Set("async", "true")
	uploadUrl.RawQuery = query.Encode()

	cancelChan := make(chan struct{})
	var writerClosed <-chan bool
	closeNotifier, ok := w.(http.CloseNotifier)
	if ok {
		writerClosed = closeNotifier.CloseNotify()
	}

	done := make(chan struct{})
	go func() {
		timer := time.NewTimer(timeout)
		select {
		case <-writerClosed:
			close(cancelChan)
		case <-timer.C:
			close(cancelChan)
		case <-done:
		}
		timer.Stop()
	}()
	defer close(done)

	logger = logger.WithData(lager.Data{"upload-url": uploadUrl, "content-length": r.ContentLength})
	logger.Info("uploading-droplet")
	uploadStart := time.Now()
	uploadResponse, err := h.uploader.Upload(uploadUrl, "droplet.tgz", r, cancelChan)
	if err != nil {
		logger.Error("failed-uploading-droplet", err)
		if uploadResponse == nil {
			w.WriteHeader(http.StatusInternalServerError)
		} else {
			w.WriteHeader(uploadResponse.StatusCode)
		}
		w.Write([]byte(err.Error()))
		return
	}
	uploadEnd := time.Now()
	logger.Info("succeeded-uploading-droplet", lager.Data{
		"upload-duration": uploadEnd.Sub(uploadStart).String(),
	})

	logger.Info("polling-cc-background-upload")
	err = h.poller.Poll(uploadUrl, uploadResponse, cancelChan)
	if err != nil {
		logger.Error("failed-polling-cc-background-upload", err)
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(err.Error()))
		return
	}
	pollEnd := time.Now()
	logger.Info("succeeded-polling-cc-background-upload", lager.Data{
		"poll-duration": pollEnd.Sub(uploadEnd).String(),
	})

	w.WriteHeader(http.StatusCreated)
}
