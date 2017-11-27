package driverhttp

import (
	"bytes"
	"encoding/json"
	"io/ioutil"
	"net/http"

	"strings"

	"fmt"

	"code.cloudfoundry.org/cfhttp"
	"code.cloudfoundry.org/clock"
	"code.cloudfoundry.org/goshims/http_wrap"
	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/voldriver"
	"github.com/tedsuo/rata"

	os_http "net/http"

	"time"

	"code.cloudfoundry.org/voldriver/backoff"
	"context"
	"errors"
)

type reqFactory struct {
	reqGen  *rata.RequestGenerator
	route   string
	payload []byte
}

func newReqFactory(reqGen *rata.RequestGenerator, route string, payload []byte) *reqFactory {
	return &reqFactory{
		reqGen:  reqGen,
		route:   route,
		payload: payload,
	}
}

func (r *reqFactory) Request() (*os_http.Request, error) {
	return r.reqGen.CreateRequest(r.route, nil, bytes.NewBuffer(r.payload))
}

type remoteClient struct {
	HttpClient http_wrap.Client
	reqGen     *rata.RequestGenerator
	clock      clock.Clock
	url        string
	tls        *voldriver.TLSConfig
}

func NewRemoteClient(url string, tls *voldriver.TLSConfig) (*remoteClient, error) {
	client := cfhttp.NewClient()
	input_url := url

	if strings.Contains(url, ".sock") {
		client = cfhttp.NewUnixClient(url)
		url = fmt.Sprintf("unix://%s", url)
	} else {
		if tls != nil {
			tlsConfig, err := cfhttp.NewTLSConfig(tls.CertFile, tls.KeyFile, tls.CAFile)
			if err != nil {
				return nil, err
			}

			tlsConfig.InsecureSkipVerify = tls.InsecureSkipVerify

			if tr, ok := client.Transport.(*http.Transport); ok {
				tr.TLSClientConfig = tlsConfig
			} else {
				return nil, errors.New("Invalid transport")
			}
		}

	}

	driver := NewRemoteClientWithClient(url, client, clock.NewClock())
	driver.tls = tls
	driver.url = input_url
	return driver, nil
}

func NewRemoteClientWithClient(socketPath string, client http_wrap.Client, clock clock.Clock) *remoteClient {
	return &remoteClient{
		HttpClient: client,
		reqGen:     rata.NewRequestGenerator(socketPath, voldriver.Routes),
		clock:      clock,
	}
}

func (r *remoteClient) Matches(loggerIn lager.Logger, url string, tls *voldriver.TLSConfig) bool {
	logger := loggerIn.Session("matches")
	logger.Info("start")
	defer logger.Info("end")

	if url != r.url {
		return false
	}
	var tls1, tls2 []byte
	var err error
	if tls != nil {
		tls1, err = json.Marshal(tls)
		logger.Error("failed-json-marshall", err)
		return false
	}
	if r.tls != nil {
		tls2, err = json.Marshal(r.tls)
		logger.Error("failed-json-marshall", err)
		return false
	}
	return string(tls1) == string(tls2)
}

func (r *remoteClient) Activate(env voldriver.Env) voldriver.ActivateResponse {
	logger := env.Logger().Session("activate")
	logger.Info("start")
	defer logger.Info("end")

	request := newReqFactory(r.reqGen, voldriver.ActivateRoute, nil)

	response, err := r.do(env.Context(), logger, request)
	if err != nil {
		logger.Error("failed-activate", err)
		return voldriver.ActivateResponse{Err: err.Error()}
	}

	if response == nil {
		return voldriver.ActivateResponse{Err: "Invalid response from driver."}
	}

	var activate voldriver.ActivateResponse
	if err := json.Unmarshal(response, &activate); err != nil {
		logger.Error("failed-parsing-activate-response", err)
		return voldriver.ActivateResponse{Err: err.Error()}
	}

	return activate
}

func (r *remoteClient) Create(env voldriver.Env, createRequest voldriver.CreateRequest) voldriver.ErrorResponse {
	logger := env.Logger().Session("create", lager.Data{"create_request.Name": createRequest.Name})
	logger.Info("start")
	defer logger.Info("end")

	payload, err := json.Marshal(createRequest)
	if err != nil {
		logger.Error("failed-marshalling-request", err)
		return voldriver.ErrorResponse{Err: err.Error()}
	}

	request := newReqFactory(r.reqGen, voldriver.CreateRoute, payload)

	response, err := r.do(env.Context(), logger, request)
	if err != nil {
		logger.Error("failed-creating-volume", err)
		return voldriver.ErrorResponse{Err: err.Error()}
	}

	var remoteError voldriver.ErrorResponse
	if response == nil {
		return voldriver.ErrorResponse{Err: "Invalid response from driver."}
	}

	if err := json.Unmarshal(response, &remoteError); err != nil {
		logger.Error("failed-parsing-error-response", err)
		return voldriver.ErrorResponse{Err: err.Error()}
	}

	return voldriver.ErrorResponse{}
}

func (r *remoteClient) List(env voldriver.Env) voldriver.ListResponse {
	logger := env.Logger().Session("remoteclient-list")
	logger.Info("start")
	defer logger.Info("end")

	request := newReqFactory(r.reqGen, voldriver.ListRoute, nil)

	response, err := r.do(env.Context(), logger, request)
	if err != nil {
		logger.Error("failed-list", err)
		return voldriver.ListResponse{Err: err.Error()}
	}

	if response == nil {
		return voldriver.ListResponse{Err: "Invalid response from driver."}
	}

	var list voldriver.ListResponse
	if err := json.Unmarshal(response, &list); err != nil {
		logger.Error("failed-parsing-list-response", err)
		return voldriver.ListResponse{Err: err.Error()}
	}

	return list
}

func (r *remoteClient) Mount(env voldriver.Env, mountRequest voldriver.MountRequest) voldriver.MountResponse {
	logger := env.Logger().Session("remoteclient-mount", lager.Data{"mount_request": mountRequest})
	logger.Info("start")
	defer logger.Info("end")

	sendingJson, err := json.Marshal(mountRequest)
	if err != nil {
		logger.Error("failed-marshalling-request", err)
		return voldriver.MountResponse{Err: err.Error()}
	}

	request := newReqFactory(r.reqGen, voldriver.MountRoute, sendingJson)

	response, err := r.do(env.Context(), logger, request)
	if err != nil {
		logger.Error("failed-mounting-volume", err)
		return voldriver.MountResponse{Err: err.Error()}
	}

	if response == nil {
		return voldriver.MountResponse{Err: "Invalid response from driver."}
	}

	var mountPoint voldriver.MountResponse
	if err := json.Unmarshal(response, &mountPoint); err != nil {
		logger.Error("failed-parsing-mount-response", err)
		return voldriver.MountResponse{Err: err.Error()}
	}

	return mountPoint
}

func (r *remoteClient) Path(env voldriver.Env, pathRequest voldriver.PathRequest) voldriver.PathResponse {
	logger := env.Logger().Session("path")
	logger.Info("start")
	defer logger.Info("end")

	payload, err := json.Marshal(pathRequest)
	if err != nil {
		logger.Error("failed-marshalling-request", err)
		return voldriver.PathResponse{Err: err.Error()}
	}

	request := newReqFactory(r.reqGen, voldriver.PathRoute, payload)

	response, err := r.do(env.Context(), logger, request)
	if err != nil {
		logger.Error("failed-volume-path", err)
		return voldriver.PathResponse{Err: err.Error()}
	}

	if response == nil {
		return voldriver.PathResponse{Err: "Invalid response from driver."}
	}

	var mountPoint voldriver.PathResponse
	if err := json.Unmarshal(response, &mountPoint); err != nil {
		logger.Error("failed-parsing-path-response", err)
		return voldriver.PathResponse{Err: err.Error()}
	}

	return mountPoint
}

func (r *remoteClient) Unmount(env voldriver.Env, unmountRequest voldriver.UnmountRequest) voldriver.ErrorResponse {
	logger := env.Logger().Session("mount")
	logger.Info("start")
	defer logger.Info("end")

	payload, err := json.Marshal(unmountRequest)
	if err != nil {
		logger.Error("failed-marshalling-request", err)
		return voldriver.ErrorResponse{Err: err.Error()}
	}

	request := newReqFactory(r.reqGen, voldriver.UnmountRoute, payload)

	response, err := r.do(env.Context(), logger, request)
	if err != nil {
		logger.Error("failed-unmounting-volume", err)
		return voldriver.ErrorResponse{Err: err.Error()}
	}

	if response == nil {
		return voldriver.ErrorResponse{Err: "Invalid response from driver."}
	}

	var remoteErrorResponse voldriver.ErrorResponse
	if err := json.Unmarshal(response, &remoteErrorResponse); err != nil {
		logger.Error("failed-parsing-error-response", err)
		return voldriver.ErrorResponse{Err: err.Error()}
	}
	return remoteErrorResponse
}

func (r *remoteClient) Remove(env voldriver.Env, removeRequest voldriver.RemoveRequest) voldriver.ErrorResponse {
	logger := env.Logger().Session("remove")
	logger.Info("start")
	defer logger.Info("end")

	payload, err := json.Marshal(removeRequest)
	if err != nil {
		logger.Error("failed-marshalling-request", err)
		return voldriver.ErrorResponse{Err: err.Error()}
	}

	request := newReqFactory(r.reqGen, voldriver.RemoveRoute, payload)

	response, err := r.do(env.Context(), logger, request)
	if err != nil {
		logger.Error("failed-removing-volume", err)
		return voldriver.ErrorResponse{Err: err.Error()}
	}

	if response == nil {
		return voldriver.ErrorResponse{Err: "Invalid response from driver."}
	}

	var remoteErrorResponse voldriver.ErrorResponse
	if err := json.Unmarshal(response, &remoteErrorResponse); err != nil {
		logger.Error("failed-parsing-error-response", err)
		return voldriver.ErrorResponse{Err: err.Error()}
	}

	return remoteErrorResponse
}

func (r *remoteClient) Get(env voldriver.Env, getRequest voldriver.GetRequest) voldriver.GetResponse {
	logger := env.Logger().Session("get")
	logger.Info("start")
	defer logger.Info("end")

	payload, err := json.Marshal(getRequest)
	if err != nil {
		logger.Error("failed-marshalling-request", err)
		return voldriver.GetResponse{Err: err.Error()}
	}

	request := newReqFactory(r.reqGen, voldriver.GetRoute, payload)

	response, err := r.do(env.Context(), logger, request)
	if err != nil {
		logger.Error("failed-getting-volume", err)
		return voldriver.GetResponse{Err: err.Error()}
	}

	if response == nil {
		return voldriver.GetResponse{Err: "Invalid response from driver."}
	}

	var remoteResponse voldriver.GetResponse
	if err := json.Unmarshal(response, &remoteResponse); err != nil {
		logger.Error("failed-parsing-error-response", err)
		return voldriver.GetResponse{Err: err.Error()}
	}

	return remoteResponse
}

func (r *remoteClient) Capabilities(env voldriver.Env) voldriver.CapabilitiesResponse {
	logger := env.Logger().Session("capabilities")
	logger.Info("start")
	defer logger.Info("end")

	request := newReqFactory(r.reqGen, voldriver.CapabilitiesRoute, nil)

	response, err := r.do(env.Context(), logger, request)
	if err != nil {
		logger.Error("failed-capabilities", err)
		return voldriver.CapabilitiesResponse{}
	}

	var remoteError voldriver.CapabilitiesResponse
	if response == nil {
		return remoteError
	}

	var capabilities voldriver.CapabilitiesResponse
	if err := json.Unmarshal(response, &capabilities); err != nil {
		logger.Error("failed-parsing-capabilities-response", err)
		return voldriver.CapabilitiesResponse{}
	}

	return capabilities
}

func (r *remoteClient) do(ctx context.Context, logger lager.Logger, requestFactory *reqFactory) ([]byte, error) {
	var data []byte

	childContext, _ := context.WithDeadline(ctx, r.clock.Now().Add(30*time.Second))

	backoff := backoff.NewExponentialBackOff(childContext, r.clock)

	err := backoff.Retry(func(ctx context.Context) error {
		var (
			err      error
			request  *os_http.Request
			response *os_http.Response
		)

		request, err = requestFactory.Request()
		if err != nil {
			logger.Error("request-gen-failed", err)
			return err
		}
		request = request.WithContext(ctx)

		response, err = r.HttpClient.Do(request)
		if err != nil {
			logger.Error("request-failed", err)
			return err
		}
		logger.Debug("response", lager.Data{"response": response.Status})

		data, err = ioutil.ReadAll(response.Body)
		if err != nil {
			return err
		}

		var remoteErrorResponse voldriver.ErrorResponse
		if err := json.Unmarshal(data, &remoteErrorResponse); err != nil {
			logger.Error("failed-parsing-http-response-body", err)
			return err
		}

		if remoteErrorResponse.Err != "" {
			return errors.New(remoteErrorResponse.Err)
		}

		return nil
	})

	return data, err
}
