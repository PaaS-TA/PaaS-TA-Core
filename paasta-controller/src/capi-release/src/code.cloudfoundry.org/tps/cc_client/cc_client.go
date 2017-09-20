package cc_client

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"time"

	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/runtimeschema/cc_messages"
	"code.cloudfoundry.org/urljoiner"
)

const (
	appCrashedPath           = "/internal/apps/%s/crashed"
	appCrashedRequestTimeout = 5 * time.Second
)

//go:generate counterfeiter -o fakes/fake_cc_client.go . CcClient
type CcClient interface {
	AppCrashed(guid string, appCrashed cc_messages.AppCrashedRequest, logger lager.Logger) error
}

type ccClient struct {
	ccURI      string
	username   string
	password   string
	httpClient *http.Client
}

type BadResponseError struct {
	StatusCode int
}

func (b *BadResponseError) Error() string {
	return fmt.Sprintf("Crashed response POST failed with %d", b.StatusCode)
}

func NewCcClient(baseURI string, username string, password string, skipCertVerify bool) CcClient {
	httpClient := &http.Client{
		Timeout: appCrashedRequestTimeout,
		Transport: &http.Transport{
			Proxy: http.ProxyFromEnvironment,
			Dial: (&net.Dialer{
				Timeout:   10 * time.Second,
				KeepAlive: 30 * time.Second,
			}).Dial,
			TLSHandshakeTimeout: 10 * time.Second,
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: skipCertVerify,
				MinVersion:         tls.VersionTLS10,
			},
		},
	}

	return &ccClient{
		ccURI:      urljoiner.Join(baseURI, appCrashedPath),
		username:   username,
		password:   password,
		httpClient: httpClient,
	}
}

func (cc *ccClient) AppCrashed(guid string, appCrashed cc_messages.AppCrashedRequest, logger lager.Logger) error {
	logger = logger.Session("cc-client")
	logger.Debug("delivering-app-crashed-response", lager.Data{"app_crashed": appCrashed})

	payload, err := json.Marshal(appCrashed)
	if err != nil {
		return err
	}

	request, err := http.NewRequest("POST", fmt.Sprintf(cc.ccURI, guid), bytes.NewReader(payload))
	if err != nil {
		return err
	}

	request.SetBasicAuth(cc.username, cc.password)
	request.Header.Set("content-type", "application/json")

	response, err := cc.httpClient.Do(request)
	if err != nil {
		logger.Error("deliver-app-crashed-response-failed", err)
		return err
	}

	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		return &BadResponseError{response.StatusCode}
	}

	logger.Debug("delivered-app-crashed-response")
	return nil
}
