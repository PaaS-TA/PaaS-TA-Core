package rep

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"code.cloudfoundry.org/bbs/models"
	"code.cloudfoundry.org/cfhttp"
	"code.cloudfoundry.org/lager"
	"github.com/tedsuo/rata"
)

//go:generate counterfeiter -o repfakes/fake_client_factory.go . ClientFactory

type ClientFactory interface {
	CreateClient(address, url string) (Client, error)
}

// capture the behavior described in the comment of this story
// https://www.pivotaltracker.com/story/show/130664747/comments/152863773
type TLSConfig struct {
	RequireTLS                    bool
	CertFile, KeyFile, CaCertFile string
	ClientCacheSize               int // the tls client cache size, 0 means use golang default value
}

// return true if all the certs files are set in the struct, i.e. not ""
func (config *TLSConfig) hasCreds() bool {
	return config.CaCertFile != "" &&
		config.KeyFile != "" &&
		config.CertFile != ""
}

// pick either the old address or the new rep_url depending on the announced
// addresses and the tls config
func (config *TLSConfig) pickURL(address, repURL string) (string, error) {
	secure := false
	if repURL != "" {
		url, err := url.Parse(repURL)
		if err != nil {
			return "", err
		}

		if url.Scheme == "https" {
			secure = true
		}
	}

	if !config.RequireTLS && !config.hasCreds() {
		// cannot use tls
		if secure {
			return "", errors.New("https scheme not supported since certificates aren't provided")
		}
		// prefer repURL
		if repURL != "" {
			return repURL, nil
		}
		return address, nil
	} else if !config.RequireTLS {
		// prefer tls but don't require it
		if repURL != "" {
			return repURL, nil
		}
		return address, nil
	} else {
		// must use tls
		if !secure {
			return "", errors.New("https scheme is required but none of the addresses support it")
		}
		return repURL, nil
	}
}

func (tlsConfig *TLSConfig) modifyTransport(client *http.Client) error {
	if !tlsConfig.hasCreds() {
		return nil
	}

	if transport, ok := client.Transport.(*http.Transport); ok {
		config, err := cfhttp.NewTLSConfig(tlsConfig.CertFile, tlsConfig.KeyFile, tlsConfig.CaCertFile)
		if err != nil {
			return err
		}

		config.ClientSessionCache = tls.NewLRUClientSessionCache(tlsConfig.ClientCacheSize)

		transport.TLSClientConfig = config
	}
	return nil
}

type clientFactory struct {
	httpClient  *http.Client
	stateClient *http.Client
	tlsConfig   *TLSConfig
}

func NewClientFactory(httpClient, stateClient *http.Client, tlsConfig *TLSConfig) (ClientFactory, error) {
	if tlsConfig == nil {
		// zero values tls config
		tlsConfig = &TLSConfig{}
	}

	if err := tlsConfig.modifyTransport(httpClient); err != nil {
		return nil, err
	}

	if err := tlsConfig.modifyTransport(stateClient); err != nil {
		return nil, err
	}

	return &clientFactory{
		httpClient:  httpClient,
		stateClient: stateClient,
		tlsConfig:   tlsConfig,
	}, nil
}

func (factory *clientFactory) CreateClient(address, url string) (Client, error) {
	urlToUse, err := factory.tlsConfig.pickURL(address, url)
	if err != nil {
		return nil, err
	}

	return newClient(factory.httpClient, factory.stateClient, urlToUse), nil
}

type AuctionCellClient interface {
	State(logger lager.Logger) (CellState, error)
	Perform(logger lager.Logger, work Work) (Work, error)
}

//go:generate counterfeiter -o repfakes/fake_client.go . Client

type Client interface {
	AuctionCellClient
	StopLRPInstance(key models.ActualLRPKey, instanceKey models.ActualLRPInstanceKey) error
	CancelTask(taskGuid string) error
	SetStateClient(stateClient *http.Client)
	StateClientTimeout() time.Duration
}

//go:generate counterfeiter -o repfakes/fake_sim_client.go . SimClient

type SimClient interface {
	Client
	Reset() error
}

type client struct {
	client           *http.Client
	stateClient      *http.Client
	address          string
	requestGenerator *rata.RequestGenerator
}

func newClient(httpClient, stateClient *http.Client, address string) Client {
	return &client{
		client:           httpClient,
		stateClient:      stateClient,
		address:          address,
		requestGenerator: rata.NewRequestGenerator(address, Routes),
	}
}

func (c *client) SetStateClient(stateClient *http.Client) {
	c.stateClient = stateClient
}

func (c *client) StateClientTimeout() time.Duration {
	return c.stateClient.Timeout
}

func (c *client) State(logger lager.Logger) (CellState, error) {
	req, err := c.requestGenerator.CreateRequest(StateRoute, nil, nil)
	if err != nil {
		return CellState{}, err
	}

	resp, err := c.stateClient.Do(req)
	if err != nil {
		return CellState{}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return CellState{}, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	var state CellState
	err = json.NewDecoder(resp.Body).Decode(&state)
	if err != nil {
		return CellState{}, err
	}

	return state, nil
}

func (c *client) Perform(logger lager.Logger, work Work) (Work, error) {
	body, err := json.Marshal(work)
	if err != nil {
		return Work{}, err
	}

	req, err := c.requestGenerator.CreateRequest(PerformRoute, nil, bytes.NewReader(body))
	if err != nil {
		return Work{}, err
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return Work{}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return Work{}, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	var failedWork Work
	err = json.NewDecoder(resp.Body).Decode(&failedWork)
	if err != nil {
		return Work{}, err
	}

	return failedWork, nil
}

func (c *client) Reset() error {
	req, err := c.requestGenerator.CreateRequest(Sim_ResetRoute, nil, nil)
	if err != nil {
		return err
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	return nil
}

func (c *client) StopLRPInstance(
	key models.ActualLRPKey,
	instanceKey models.ActualLRPInstanceKey,
) error {
	req, err := c.requestGenerator.CreateRequest(StopLRPInstanceRoute, stopParamsFromLRP(key, instanceKey), nil)
	if err != nil {
		return err
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := c.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusAccepted {
		return fmt.Errorf("http error: status code %d (%s)", resp.StatusCode, http.StatusText(resp.StatusCode))
	}

	return nil
}

func (c *client) CancelTask(taskGuid string) error {
	req, err := c.requestGenerator.CreateRequest(CancelTaskRoute, rata.Params{"task_guid": taskGuid}, nil)
	if err != nil {
		return err
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return err
	}

	defer resp.Body.Close()

	if resp.StatusCode != http.StatusAccepted {
		return fmt.Errorf("http error: status code %d (%s)", resp.StatusCode, http.StatusText(resp.StatusCode))
	}

	return nil
}

func stopParamsFromLRP(
	key models.ActualLRPKey,
	instanceKey models.ActualLRPInstanceKey,
) rata.Params {
	return rata.Params{
		"process_guid":  key.ProcessGuid,
		"instance_guid": instanceKey.InstanceGuid,
		"index":         strconv.Itoa(int(key.Index)),
	}
}
