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

type Client struct {
	baseURL          string
	operationTimeout time.Duration
	pollingInterval  time.Duration
}

type Response struct {
	ID                   string           `json:"ID"`
	ExecutionCompletedAt string           `json:"ExecutionCompletedAt"`
	Events               []*ResponseEvent `json:"Events"`
}

type ResponseEvent struct {
	Error string `json:"Error"`
}

type deployment struct {
	Name string
	Jobs []job
}

type job struct {
	Name    string
	Indices []int
}

type killTask struct {
	Type string
}

type killCommand struct {
	Tasks       []interface{}
	Deployments []deployment
}

func NewClient(baseURL string, operationTimeout time.Duration, pollingInterval time.Duration) Client {
	return Client{
		baseURL:          baseURL,
		operationTimeout: operationTimeout,
		pollingInterval:  pollingInterval,
	}
}

func (c Client) KillIndices(deploymentName, jobName string, indices []int) error {
	command := killCommand{
		Tasks: []interface{}{
			killTask{Type: "kill"},
		},
		Deployments: []deployment{{
			Name: deploymentName,
			Jobs: []job{{Name: jobName, Indices: indices}},
		}},
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
		incidentCompleted, err := c.isIncidentCompleted(id)
		if err != nil {
			return err
		}

		if incidentCompleted {
			return nil
		}

		if time.Now().Sub(startTime) > c.operationTimeout {
			return errors.New(fmt.Sprintf("Did not finish deleting VM in time: %d", c.operationTimeout))
		}

		time.Sleep(c.pollingInterval)
	}

	return nil
}

func (c Client) isIncidentCompleted(id string) (bool, error) {
	turbulenceResponse, err := c.makeRequest("GET", fmt.Sprintf("%s/api/v1/incidents/%s", c.baseURL, id), nil)
	if err != nil {
		return false, err
	}

	if turbulenceResponse.ExecutionCompletedAt != "" {
		if len(turbulenceResponse.Events) == 0 {
			return false, errors.New("There should at least be one Event in response from turbulence.")
		}

		for _, event := range turbulenceResponse.Events {
			if event.Error != "" {
				return false, errors.New(event.Error)
			}
		}

		return true, nil
	}

	return false, nil
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
