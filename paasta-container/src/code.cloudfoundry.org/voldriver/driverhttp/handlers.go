package driverhttp

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"

	cf_http_handlers "code.cloudfoundry.org/cfhttp/handlers"
	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/voldriver"
	"context"
	"github.com/tedsuo/rata"
)

func NewHttpDriverEnv(logger lager.Logger, ctx context.Context) voldriver.Env {
	return &voldriverEnv{logger, ctx}
}

type voldriverEnv struct {
	logger   lager.Logger
	aContext context.Context
}

func (v *voldriverEnv) Logger() lager.Logger {
	return v.logger
}

func (v *voldriverEnv) Context() context.Context {
	return v.aContext
}

func EnvWithLogger(logger lager.Logger, env voldriver.Env) voldriver.Env {
	return &voldriverEnv{logger, env.Context()}
}

func EnvWithContext(ctx context.Context, env voldriver.Env) voldriver.Env {
	return &voldriverEnv{env.Logger(), ctx}
}

func EnvWithMonitor(logger lager.Logger, ctx context.Context, res http.ResponseWriter) voldriver.Env {
	logger = logger.Session("with-cancel")
	logger.Info("start")
	defer logger.Info("end")

	cancelCtx, cancel := context.WithCancel(ctx)

	env := NewHttpDriverEnv(logger, cancelCtx)

	if closer, ok := res.(http.CloseNotifier); ok {
		// Note: make calls in this thread to ensure reference on context
		doneOrTimeoutChannel := ctx.Done()
		cancelChannel := closer.CloseNotify()
		go func() {
			select {
			case <-doneOrTimeoutChannel:
			case <-cancelChannel:
				logger.Info("signalling-cancel")
				cancel()
			}
		}()
	}

	return env
}

// At present, Docker ignores HTTP status codes, and requires errors to be returned in the response body.  To
// comply with this API, we will return 200 in all cases
const (
	StatusInternalServerError = http.StatusOK
	StatusOK                  = http.StatusOK
)

func NewHandler(logger lager.Logger, client voldriver.Driver) (http.Handler, error) {
	logger = logger.Session("server")
	logger.Info("start")
	defer logger.Info("end")

	var handlers = rata.Handlers{
		voldriver.ActivateRoute:     newActivateHandler(logger, client),
		voldriver.GetRoute:          newGetHandler(logger, client),
		voldriver.ListRoute:         newListHandler(logger, client),
		voldriver.PathRoute:         newPathHandler(logger, client),
		voldriver.CreateRoute:       newCreateHandler(logger, client),
		voldriver.MountRoute:        newMountHandler(logger, client),
		voldriver.UnmountRoute:      newUnmountHandler(logger, client),
		voldriver.RemoveRoute:       newRemoveHandler(logger, client),
		voldriver.CapabilitiesRoute: newCapabilitiesHandler(logger, client),
	}

	return rata.NewRouter(voldriver.Routes, handlers)
}

func newActivateHandler(logger lager.Logger, client voldriver.Driver) http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		logger := logger.Session("handle-activate")
		logger.Info("start")
		defer logger.Info("end")

		activateResponse := client.Activate(EnvWithMonitor(logger, req.Context(), w))
		if activateResponse.Err != "" {
			logger.Error("failed-activating-driver", fmt.Errorf(activateResponse.Err))
			writeJSONResponse(w, StatusInternalServerError, activateResponse, req)
			return
		}

		logger.Debug("activate-response", lager.Data{"activation": activateResponse})
		writeJSONResponse(w, StatusOK, activateResponse, req)
	}
}

func newGetHandler(logger lager.Logger, client voldriver.Driver) http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		logger := logger.Session("handle-get")
		logger.Info("start")
		defer logger.Info("end")

		body, err := ioutil.ReadAll(req.Body)
		if err != nil {
			logger.Error("failed-reading-get-request-body", err)
			writeJSONResponse(w, StatusInternalServerError, voldriver.MountResponse{Err: err.Error()}, req)
			return
		}

		var getRequest voldriver.GetRequest
		if err = json.Unmarshal(body, &getRequest); err != nil {
			logger.Error("failed-unmarshalling-get-request-body", err)
			writeJSONResponse(w, StatusInternalServerError, voldriver.GetResponse{Err: err.Error()}, req)
			return
		}

		getResponse := client.Get(EnvWithMonitor(logger, req.Context(), w), getRequest)
		if getResponse.Err != "" {
			logger.Error("failed-getting-volume", err, lager.Data{"volume": getRequest.Name})
			writeJSONResponse(w, StatusInternalServerError, getResponse, req)
			return
		}

		writeJSONResponse(w, StatusOK, getResponse, req)
	}
}

func newListHandler(logger lager.Logger, client voldriver.Driver) http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		logger := logger.Session("handle-list")
		logger.Info("start")
		defer logger.Info("end")

		listResponse := client.List(EnvWithMonitor(logger, req.Context(), w))
		if listResponse.Err != "" {
			logger.Error("failed-listing-volumes", fmt.Errorf("%s", listResponse.Err))
			writeJSONResponse(w, StatusInternalServerError, listResponse, req)
			return
		}

		writeJSONResponse(w, StatusOK, listResponse, req)
	}
}

func newPathHandler(logger lager.Logger, client voldriver.Driver) http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		logger := logger.Session("handle-path")
		logger.Info("start")
		defer logger.Info("end")

		body, err := ioutil.ReadAll(req.Body)
		if err != nil {
			logger.Error("failed-reading-path-request-body", err)
			writeJSONResponse(w, StatusInternalServerError, voldriver.MountResponse{Err: err.Error()}, req)
			return
		}

		var pathRequest voldriver.PathRequest
		if err = json.Unmarshal(body, &pathRequest); err != nil {
			logger.Error("failed-unmarshalling-path-request-body", err)
			writeJSONResponse(w, StatusInternalServerError, voldriver.GetResponse{Err: err.Error()}, req)
			return
		}

		pathResponse := client.Path(EnvWithMonitor(logger, req.Context(), w), pathRequest)
		if pathResponse.Err != "" {
			logger.Error("failed-activating-driver", fmt.Errorf(pathResponse.Err))
			writeJSONResponse(w, StatusInternalServerError, pathResponse, req)
			return
		}

		writeJSONResponse(w, StatusOK, pathResponse, req)
	}
}

func newCapabilitiesHandler(logger lager.Logger, client voldriver.Driver) http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		logger := logger.Session("handle-capabilities")
		logger.Info("start", lager.Data{"request.RemoteAddr": req.RemoteAddr})
		defer logger.Info("end")

		capabilitiesResponse := client.Capabilities(EnvWithMonitor(logger, req.Context(), w))
		logger.Debug("capabilities-response", lager.Data{"capabilities": capabilitiesResponse})
		writeJSONResponse(w, StatusOK, capabilitiesResponse, req)
	}
}

func newCreateHandler(logger lager.Logger, client voldriver.Driver) http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		logger := logger.Session("handle-create")
		logger.Info("start")
		defer logger.Info("end")

		body, err := ioutil.ReadAll(req.Body)
		if err != nil {
			logger.Error("failed-reading-create-request-body", err)
			writeJSONResponse(w, StatusInternalServerError, voldriver.ErrorResponse{Err: err.Error()}, req)
			return
		}

		var createRequest voldriver.CreateRequest
		if err = json.Unmarshal(body, &createRequest); err != nil {
			logger.Error("failed-unmarshalling-create-request-body", err)
			writeJSONResponse(w, StatusInternalServerError, voldriver.ErrorResponse{Err: err.Error()}, req)
			return
		}

		createResponse := client.Create(EnvWithMonitor(logger, req.Context(), w), createRequest)
		if createResponse.Err != "" {
			logger.Error("failed-creating-volume", errors.New(createResponse.Err))
			writeJSONResponse(w, StatusInternalServerError, createResponse, req)
			return
		}

		writeJSONResponse(w, StatusOK, createResponse, req)
	}
}

func newMountHandler(logger lager.Logger, client voldriver.Driver) http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		logger := logger.Session("handle-mount")
		logger.Info("start")
		defer logger.Info("end")

		body, err := ioutil.ReadAll(req.Body)
		if err != nil {
			logger.Error("failed-reading-mount-request-body", err)
			writeJSONResponse(w, StatusInternalServerError, voldriver.MountResponse{Err: err.Error()}, req)
			return
		}

		var mountRequest voldriver.MountRequest
		if err = json.Unmarshal(body, &mountRequest); err != nil {
			logger.Error("failed-unmarshalling-mount-request-body", err)
			writeJSONResponse(w, StatusInternalServerError, voldriver.MountResponse{Err: err.Error()}, req)
			return
		}

		mountResponse := client.Mount(EnvWithMonitor(logger, req.Context(), w), mountRequest)
		if mountResponse.Err != "" {
			logger.Error("failed-mounting-volume", errors.New(mountResponse.Err), lager.Data{"volume": mountRequest.Name})
			writeJSONResponse(w, StatusInternalServerError, mountResponse, req)
			return
		}

		writeJSONResponse(w, StatusOK, mountResponse, req)
	}
}

func newUnmountHandler(logger lager.Logger, client voldriver.Driver) http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		logger := logger.Session("handle-unmount")
		logger.Info("start")
		defer logger.Info("end")

		body, err := ioutil.ReadAll(req.Body)
		if err != nil {
			logger.Error("failed-reading-unmount-request-body", err)
			writeJSONResponse(w, StatusInternalServerError, voldriver.ErrorResponse{Err: err.Error()}, req)
			return
		}

		var unmountRequest voldriver.UnmountRequest
		if err = json.Unmarshal(body, &unmountRequest); err != nil {
			logger.Error("failed-unmarshalling-unmount-request-body", err)
			writeJSONResponse(w, StatusInternalServerError, voldriver.ErrorResponse{Err: err.Error()}, req)
			return
		}

		unmountResponse := client.Unmount(EnvWithMonitor(logger, req.Context(), w), unmountRequest)
		if unmountResponse.Err != "" {
			logger.Error("failed-unmount-volume", errors.New(unmountResponse.Err), lager.Data{"volume": unmountRequest.Name})
			writeJSONResponse(w, StatusInternalServerError, unmountResponse, req)
			return
		}

		writeJSONResponse(w, StatusOK, unmountResponse, req)
	}
}

func newRemoveHandler(logger lager.Logger, client voldriver.Driver) http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		logger := logger.Session("handle-remove")
		logger.Info("start")
		defer logger.Info("end")

		body, err := ioutil.ReadAll(req.Body)
		if err != nil {
			logger.Error("failed-reading-remove-request-body", err)
			writeJSONResponse(w, StatusInternalServerError, voldriver.ErrorResponse{Err: err.Error()}, req)
			return
		}

		var removeRequest voldriver.RemoveRequest
		if err = json.Unmarshal(body, &removeRequest); err != nil {
			logger.Error("failed-unmarshalling-unmount-request-body", err)
			writeJSONResponse(w, StatusInternalServerError, voldriver.ErrorResponse{Err: err.Error()}, req)
			return
		}

		removeResponse := client.Remove(EnvWithMonitor(logger, req.Context(), w), removeRequest)
		if removeResponse.Err != "" {
			logger.Error("failed-remove-volume", errors.New(removeResponse.Err))
			writeJSONResponse(w, StatusInternalServerError, removeResponse, req)
			return
		}

		writeJSONResponse(w, StatusOK, removeResponse, req)
	}
}

func writeJSONResponse(w http.ResponseWriter, statusCode int, jsonObj interface{}, req *http.Request) {
	// We'd like to request connection close for tcp/http connections, but that causes problems for unix
	// sockets.  For Unix connections, there's no remote address, so we are programming by side effect
	// here, and swtiching off of that in the absence of a better way to know the transport. :-(
	if req.RemoteAddr != "" && req.RemoteAddr != "@" {
		w.Header().Set("Connection", "close")
	}
	cf_http_handlers.WriteJSONResponse(w, statusCode, jsonObj)
}
