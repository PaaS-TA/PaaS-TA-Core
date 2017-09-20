package upload_build_artifacts

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

func New(uploader ccclient.Uploader, logger lager.Logger) http.Handler {
	return &buildArtifactUploader{
		uploader: uploader,
		logger:   logger,
	}
}

type buildArtifactUploader struct {
	uploader ccclient.Uploader
	logger   lager.Logger
}

var MissingCCBuildArtifactsUploadUriKeyError = errors.New(fmt.Sprintf("missing %s parameter", cc_messages.CcBuildArtifactsUploadUriKey))

func (h *buildArtifactUploader) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	requestLogger := h.logger.Session("build-artifacts.upload")

	uploadUriParameter := r.URL.Query().Get(cc_messages.CcBuildArtifactsUploadUriKey)
	if uploadUriParameter == "" {
		requestLogger.Error("failed", MissingCCBuildArtifactsUploadUriKeyError)
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(MissingCCBuildArtifactsUploadUriKeyError.Error()))
		return
	}

	uploadUrl, err := url.Parse(uploadUriParameter)
	if err != nil {
		requestLogger.Error("failed: Invalid upload uri", err)
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(err.Error()))
		return
	}

	timeout := 5 * time.Minute
	timeoutParameter := r.URL.Query().Get(cc_messages.CcTimeoutKey)
	if timeoutParameter != "" {
		t, err := strconv.Atoi(timeoutParameter)
		if err != nil {
			requestLogger.Error("failed: Invalid timeout", err)
			w.WriteHeader(http.StatusBadRequest)
			w.Write([]byte(err.Error()))
			return
		}
		timeout = time.Duration(t) * time.Second
	}

	requestLogger.Info("start", lager.Data{
		"upload-url":     uploadUrl,
		"content-length": r.ContentLength,
	})

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

	uploadResponse, err := h.uploader.Upload(uploadUrl, "buildpack_cache.tgz", r, cancelChan)
	close(done)
	if err != nil {
		requestLogger.Error("failed", err)
		if uploadResponse == nil {
			w.WriteHeader(http.StatusInternalServerError)
		} else {
			w.WriteHeader(uploadResponse.StatusCode)
		}
		w.Write([]byte(err.Error()))
		return
	}

	w.WriteHeader(http.StatusOK)
	requestLogger.Info("success", lager.Data{
		"upload-url":     uploadUrl,
		"content-length": r.ContentLength,
	})
}
