package turbulence

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"
)

const million int64 = 1000000

type Client struct {
	baseURL          string
	operationTimeout time.Duration
	pollingInterval  time.Duration
}

type Response struct {
	ID                   string          `json:"ID"`
	ExecutionStartedAt   string          `json:"ExecutionStartedAt"`
	ExecutionCompletedAt string          `json:"ExecutionCompletedAt"`
	Events               []ResponseEvent `json:"Events"`
}

type ResponseEvent struct {
	Error                string `json:"Error"`
	ID                   string `json:"ID"`
	Type                 string `json:"Type"`
	DeploymentName       string `json:"DeploymentName"`
	JobName              string `json:"JobName"`
	JobNameMatch         string `json:"JobNameMatch"`
	JobIndex             int    `json:"JobIndex"`
	ExecutionStartedAt   string `json:"ExecutionStartedAt"`
	ExecutionCompletedAt string `json:"ExecutionCompletedAt"`
}

type killTask struct {
	Type string
}

type controlNetTask struct {
	Type    string
	Timeout string
	Delay   string
}

type id struct {
	Values []string `json:"Values"`
}

type selector struct {
	ID id `json:"ID"`
}

type command struct {
	Tasks    []interface{}
	Selector selector `json:"Selector"`
}

func NewClient(baseURL string, operationTimeout time.Duration, pollingInterval time.Duration) Client {
	return Client{
		baseURL:          baseURL,
		operationTimeout: operationTimeout,
		pollingInterval:  pollingInterval,
	}
}

func (c Client) Delay(ids []string, delay time.Duration, timeout time.Duration) (Response, error) {
	command := command{
		Tasks: []interface{}{
			controlNetTask{Type: "control-net",
				Timeout: fmt.Sprintf("%dms", timeout.Nanoseconds()/million),
				Delay:   fmt.Sprintf("%dms", delay.Nanoseconds()/million)}},
		Selector: selector{
			ID: id{
				Values: ids,
			},
		},
	}

	jsonCommand, err := json.Marshal(command)
	if err != nil {
		return Response{}, err
	}

	resp, err := c.makeRequest("POST", c.baseURL+"/api/v1/incidents", bytes.NewBuffer(jsonCommand))
	if err != nil {
		return Response{}, err
	}

	return c.pollControlNetStarted(resp.ID)
}

func (c Client) pollControlNetStarted(id string) (Response, error) {
	startTime := time.Now()
	for {
		turbulenceResponse, err := c.makeRequest("GET", fmt.Sprintf("%s/api/v1/incidents/%s", c.baseURL, id), nil)
		if err != nil {
			return turbulenceResponse, err
		}

		for _, event := range turbulenceResponse.Events {
			if event.Error != "" {
				return turbulenceResponse, errors.New(event.Error)
			}
			if event.Type == "ControlNet" && event.ExecutionStartedAt != "" {
				return turbulenceResponse, nil
			}
		}

		if time.Now().Sub(startTime) > c.operationTimeout {
			return turbulenceResponse, errors.New(fmt.Sprintf("Did not start control-net event in time: %d", c.operationTimeout))
		}

		time.Sleep(c.pollingInterval)
	}
}

func (c Client) KillIDs(ids []string) error {
	command := command{
		Tasks: []interface{}{
			killTask{Type: "kill"},
		},
		Selector: selector{
			ID: id{
				Values: ids,
			},
		},
	}

	jsonCommand, err := json.Marshal(command)
	if err != nil {
		return err
	}

	turbulenceResponse, err := c.makeRequest("POST", c.baseURL+"/api/v1/incidents", bytes.NewBuffer(jsonCommand))
	if err != nil {
		return err
	}

	return c.pollRequestCompletedDeletingVM(turbulenceResponse.ID)
}

func (c Client) pollRequestCompletedDeletingVM(id string) error {
	startTime := time.Now()
	for {
		turbulenceResponse, err := c.makeRequest("GET", fmt.Sprintf("%s/api/v1/incidents/%s", c.baseURL, id), nil)
		if err != nil {
			return err
		}

		if turbulenceResponse.ExecutionCompletedAt != "" {
			if len(turbulenceResponse.Events) == 0 {
				return errors.New("There should at least be one Event in response from turbulence.")
			}

			for _, event := range turbulenceResponse.Events {
				if event.Error != "" {
					return fmt.Errorf("There was a turbulence event error. Check out the turbulence events (response id: %s) for more information.", id)
				}
			}

			return nil
		}

		if time.Now().Sub(startTime) > c.operationTimeout {
			return errors.New(fmt.Sprintf("Did not finish deleting VM in time: %d", c.operationTimeout))
		}

		time.Sleep(c.pollingInterval)
	}
}

func (c Client) makeRequest(method string, path string, body io.Reader) (Response, error) {
	request, err := http.NewRequest(method, path, body)
	if err != nil {
		return Response{}, err
	}

	tr := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}
	client := &http.Client{Transport: tr}

	resp, err := client.Do(request)
	if err != nil {
		return Response{}, err
	}

	var turbulenceResponse Response
	err = json.NewDecoder(resp.Body).Decode(&turbulenceResponse)
	if err != nil {
		return Response{}, errors.New("Unable to decode turbulence response.")
	}

	return turbulenceResponse, nil
}

func (c Client) Incident(id string) (Response, error) {
	return c.makeRequest("GET", fmt.Sprintf("%s/api/v1/incidents/%s", c.baseURL, id), nil)
}
