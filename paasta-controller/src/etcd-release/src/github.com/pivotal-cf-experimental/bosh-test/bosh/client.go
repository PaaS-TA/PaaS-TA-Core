package bosh

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"time"

	"gopkg.in/yaml.v2"
)

var (
	client     = http.DefaultClient
	transport  = http.DefaultTransport
	bodyReader = ioutil.ReadAll
)

type deploymentManifest struct {
	Manifest string `json:"manifest"`
}

type manifest struct {
	DirectorUUID  interface{} `yaml:"director_uuid"`
	Name          interface{} `yaml:"name"`
	Compilation   interface{} `yaml:"compilation"`
	Update        interface{} `yaml:"update"`
	Networks      interface{} `yaml:"networks"`
	ResourcePools []struct {
		Name            interface{} `yaml:"name"`
		Network         interface{} `yaml:"network"`
		Size            interface{} `yaml:"size,omitempty"`
		CloudProperties interface{} `yaml:"cloud_properties,omitempty"`
		Env             interface{} `yaml:"env,omitempty"`
		Stemcell        struct {
			Name    string `yaml:"name"`
			Version string `yaml:"version"`
		} `yaml:"stemcell"`
	} `yaml:"resource_pools"`
	Jobs       interface{} `yaml:"jobs"`
	Properties interface{} `yaml:"properties"`
	Releases   []struct {
		Name    string `yaml:"name"`
		Version string `yaml:"version"`
	} `yaml:"releases"`
}

type Config struct {
	URL                 string
	Username            string
	Password            string
	TaskPollingInterval time.Duration
	AllowInsecureSSL    bool
}

type Client struct {
	config Config
}

type DirectorInfo struct {
	UUID string
	CPI  string
}

type Deployment struct {
	Name string
}

type Task struct {
	Id     int
	State  string
	Result string
}

type TaskOutput struct {
	Time     int64
	Error    TaskError
	Stage    string
	Tags     []string
	Total    int
	Task     string
	Index    int
	State    string
	Progress int
}

type TaskError struct {
	Code    int
	Message string
}

func NewClient(config Config) Client {
	if config.TaskPollingInterval == time.Duration(0) {
		config.TaskPollingInterval = 5 * time.Second
	}

	if config.AllowInsecureSSL {
		transport = &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		}

		client = &http.Client{
			Transport: transport,
		}
	}

	return Client{
		config: config,
	}
}

func (c Client) GetTaskOutput(taskId int) ([]TaskOutput, error) {
	request, err := http.NewRequest("GET", fmt.Sprintf("%s/tasks/%d/output?type=event", c.config.URL, taskId), nil)
	if err != nil {
		return []TaskOutput{}, err
	}
	request.SetBasicAuth(c.config.Username, c.config.Password)

	response, err := transport.RoundTrip(request)
	if err != nil {
		return []TaskOutput{}, err
	}

	if response.StatusCode != http.StatusOK {
		return []TaskOutput{}, fmt.Errorf("unexpected response %d %s", response.StatusCode, http.StatusText(response.StatusCode))
	}

	body, err := bodyReader(response.Body)
	if err != nil {
		return []TaskOutput{}, err
	}
	defer response.Body.Close()

	body = bytes.TrimSpace(body)
	parts := bytes.Split(body, []byte("\n"))

	var taskOutputs []TaskOutput
	for _, part := range parts {
		var taskOutput TaskOutput
		err = json.Unmarshal(part, &taskOutput)
		if err != nil {
			return []TaskOutput{}, err
		}

		taskOutputs = append(taskOutputs, taskOutput)
	}

	return taskOutputs, nil
}

func (c Client) rewriteURL(uri string) (string, error) {
	parsedURL, err := url.Parse(uri)
	if err != nil {
		return "", err
	}

	parsedURL.Scheme = ""
	parsedURL.Host = ""

	return c.config.URL + parsedURL.String(), nil
}

func (c Client) checkTask(location string) (Task, error) {
	location, err := c.rewriteURL(location)
	if err != nil {
		return Task{}, err
	}

	var task Task
	request, err := http.NewRequest("GET", location, nil)
	if err != nil {
		return task, err
	}
	request.SetBasicAuth(c.config.Username, c.config.Password)

	response, err := transport.RoundTrip(request)
	if err != nil {
		return task, err
	}

	err = json.NewDecoder(response.Body).Decode(&task)
	if err != nil {
		return task, err
	}

	return task, nil
}

func (c Client) checkTaskStatus(location string) (int, error) {
	for {
		task, err := c.checkTask(location)
		if err != nil {
			return 0, err
		}

		switch task.State {
		case "done":
			return task.Id, nil
		case "error":
			taskOutputs, err := c.GetTaskOutput(task.Id)
			if err != nil {
				return task.Id, fmt.Errorf("failed to get full bosh task event log, bosh task failed with an error status %q", task.Result)
			}
			errorMessage := taskOutputs[len(taskOutputs)-1].Error.Message
			return task.Id, fmt.Errorf("bosh task failed with an error status %q", errorMessage)
		case "errored":
			taskOutputs, err := c.GetTaskOutput(task.Id)
			if err != nil {
				return task.Id, fmt.Errorf("failed to get full bosh task event log, bosh task failed with an errored status %q", task.Result)
			}
			errorMessage := taskOutputs[len(taskOutputs)-1].Error.Message
			return task.Id, fmt.Errorf("bosh task failed with an errored status %q", errorMessage)
		case "cancelled":
			return task.Id, errors.New("bosh task was cancelled")
		default:
			time.Sleep(c.config.TaskPollingInterval)
		}
	}
}

func (c Client) Stemcell(name string) (Stemcell, error) {
	request, err := http.NewRequest("GET", fmt.Sprintf("%s/stemcells", c.config.URL), nil)
	if err != nil {
		return Stemcell{}, err
	}

	request.SetBasicAuth(c.config.Username, c.config.Password)
	response, err := client.Do(request)
	if err != nil {
		return Stemcell{}, err
	}

	if response.StatusCode == http.StatusNotFound {
		return Stemcell{}, fmt.Errorf("stemcell %s could not be found", name)
	}

	if response.StatusCode != http.StatusOK {
		return Stemcell{}, fmt.Errorf("unexpected response %d %s", response.StatusCode, http.StatusText(response.StatusCode))
	}

	stemcells := []struct {
		Name    string
		Version string
	}{}

	err = json.NewDecoder(response.Body).Decode(&stemcells)
	if err != nil {
		return Stemcell{}, err
	}

	stemcell := NewStemcell()
	stemcell.Name = name

	for _, s := range stemcells {
		if s.Name == name {
			stemcell.Versions = append(stemcell.Versions, s.Version)
		}
	}

	return stemcell, nil
}

func (c Client) Release(name string) (Release, error) {
	request, err := http.NewRequest("GET", fmt.Sprintf("%s/releases/%s", c.config.URL, name), nil)
	if err != nil {
		return Release{}, err
	}

	request.SetBasicAuth(c.config.Username, c.config.Password)
	response, err := client.Do(request)
	if err != nil {
		return Release{}, err
	}

	if response.StatusCode == http.StatusNotFound {
		return Release{}, fmt.Errorf("release %s could not be found", name)
	}

	if response.StatusCode != http.StatusOK {
		return Release{}, fmt.Errorf("unexpected response %d %s", response.StatusCode, http.StatusText(response.StatusCode))
	}

	release := NewRelease()
	err = json.NewDecoder(response.Body).Decode(&release)
	if err != nil {
		return Release{}, err
	}

	release.Name = name

	return release, nil
}

type VM struct {
	Index   int    `json:"index"`
	State   string `json:"job_state"`
	JobName string `json:"job_name"`
}

func (c Client) DeploymentVMs(name string) ([]VM, error) {
	request, err := http.NewRequest("GET", fmt.Sprintf("%s/deployments/%s/vms?format=full", c.config.URL, name), nil)
	if err != nil {
		return []VM{}, err
	}

	request.SetBasicAuth(c.config.Username, c.config.Password)
	response, err := transport.RoundTrip(request)
	if err != nil {
		return []VM{}, err
	}

	if response.StatusCode != http.StatusFound {
		return []VM{}, fmt.Errorf("unexpected response %d %s", response.StatusCode, http.StatusText(response.StatusCode))
	}

	location := response.Header.Get("Location")

	_, err = c.checkTaskStatus(location)
	if err != nil {
		return []VM{}, err
	}

	location, err = c.rewriteURL(location)
	if err != nil {
		return []VM{}, err
	}

	request, err = http.NewRequest("GET", fmt.Sprintf("%s/output?type=result", location), nil)
	if err != nil {
		return []VM{}, err
	}

	request.SetBasicAuth(c.config.Username, c.config.Password)
	response, err = transport.RoundTrip(request)
	if err != nil {
		return []VM{}, err
	}

	body, err := bodyReader(response.Body)
	if err != nil {
		return []VM{}, err
	}
	defer response.Body.Close()

	body = bytes.TrimSpace(body)
	parts := bytes.Split(body, []byte("\n"))

	var vms []VM
	for _, part := range parts {
		var vm VM
		err = json.Unmarshal(part, &vm)
		if err != nil {
			return vms, err
		}

		vms = append(vms, vm)
	}

	return vms, nil
}

func (c Client) Info() (DirectorInfo, error) {
	response, err := client.Get(fmt.Sprintf("%s/info", c.config.URL))
	if err != nil {
		return DirectorInfo{}, err
	}

	if response.StatusCode != http.StatusOK {
		return DirectorInfo{}, fmt.Errorf("unexpected response %d %s", response.StatusCode, http.StatusText(response.StatusCode))
	}

	info := DirectorInfo{}
	err = json.NewDecoder(response.Body).Decode(&info)
	if err != nil {
		return DirectorInfo{}, err
	}

	return info, nil
}

func (c Client) Deploy(manifest []byte) (int, error) {
	if len(manifest) == 0 {
		return 0, errors.New("a valid manifest is required to deploy")
	}

	request, err := http.NewRequest("POST", fmt.Sprintf("%s/deployments", c.config.URL), bytes.NewBuffer(manifest))
	if err != nil {
		return 0, err
	}

	request.Header.Set("Content-Type", "text/yaml")
	request.SetBasicAuth(c.config.Username, c.config.Password)

	response, err := transport.RoundTrip(request)
	if err != nil {
		return 0, err
	}

	if response.StatusCode != http.StatusFound {
		return 0, fmt.Errorf("unexpected response %d %s", response.StatusCode, http.StatusText(response.StatusCode))
	}

	return c.checkTaskStatus(response.Header.Get("Location"))
}

func (c Client) ScanAndFix(manifestYAML []byte) error {
	var manifest struct {
		Name string
		Jobs []struct {
			Name      string
			Instances int
		}
	}
	err := yaml.Unmarshal(manifestYAML, &manifest)
	if err != nil {
		return err
	}

	jobs := make(map[string][]int)
	for _, j := range manifest.Jobs {
		if j.Instances > 0 {
			var indices []int
			for i := 0; i < j.Instances; i++ {
				indices = append(indices, i)
			}
			jobs[j.Name] = indices
		}
	}

	requestBody, err := json.Marshal(map[string]interface{}{
		"jobs": jobs,
	})
	if err != nil {
		return err
	}

	request, err := http.NewRequest("PUT", fmt.Sprintf("%s/deployments/%s/scan_and_fix", c.config.URL, manifest.Name), bytes.NewBuffer(requestBody))
	if err != nil {
		return err
	}
	request.SetBasicAuth(c.config.Username, c.config.Password)
	request.Header.Set("Content-Type", "application/json")

	response, err := transport.RoundTrip(request)
	if err != nil {
		return err
	}

	if response.StatusCode != http.StatusFound {
		return fmt.Errorf("unexpected response %d %s", response.StatusCode, http.StatusText(response.StatusCode))
	}

	_, err = c.checkTaskStatus(response.Header.Get("Location"))
	if err != nil {
		return err
	}

	return nil
}

func (c Client) DeleteDeployment(name string) error {
	if name == "" {
		return errors.New("a valid deployment name is required")
	}

	request, err := http.NewRequest("DELETE", fmt.Sprintf("%s/deployments/%s", c.config.URL, name), nil)
	if err != nil {
		return err
	}
	request.Header.Set("Content-Type", "text/yaml")
	request.SetBasicAuth(c.config.Username, c.config.Password)

	response, err := transport.RoundTrip(request)
	if err != nil {
		return err
	}

	if response.StatusCode != http.StatusFound {
		return fmt.Errorf("unexpected response %d %s", response.StatusCode, http.StatusText(response.StatusCode))
	}

	_, err = c.checkTaskStatus(response.Header.Get("Location"))
	return err
}

func (c Client) ResolveManifestVersions(manifestYAML []byte) ([]byte, error) {
	m := manifest{}
	err := yaml.Unmarshal(manifestYAML, &m)
	if err != nil {
		return nil, err
	}

	for i, r := range m.Releases {
		if r.Version == "latest" {
			release, err := c.Release(r.Name)
			if err != nil {
				return nil, err
			}
			r.Version = release.Latest()
			m.Releases[i] = r
		}
	}

	for i, pool := range m.ResourcePools {
		if pool.Stemcell.Version == "latest" {
			stemcell, err := c.Stemcell(pool.Stemcell.Name)
			if err != nil {
				return nil, err
			}
			pool.Stemcell.Version, err = stemcell.Latest()
			if err != nil {
				return nil, err
			}
			m.ResourcePools[i] = pool
		}
	}

	return yaml.Marshal(m)
}

func (c Client) Deployments() ([]Deployment, error) {
	request, err := http.NewRequest("GET", fmt.Sprintf("%s/deployments", c.config.URL), nil)
	if err != nil {
		return nil, err
	}
	request.SetBasicAuth(c.config.Username, c.config.Password)

	response, err := transport.RoundTrip(request)
	if err != nil {
		return nil, err
	}

	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected response %d %s", response.StatusCode, http.StatusText(response.StatusCode))
	}

	var deployments []Deployment
	err = json.NewDecoder(response.Body).Decode(&deployments)
	if err != nil {
		return nil, err
	}

	return deployments, nil
}

func (c Client) UpdateCloudConfig(cloudConfig []byte) error {
	request, err := http.NewRequest("POST", fmt.Sprintf("%s/cloud_configs", c.config.URL), bytes.NewBuffer(cloudConfig))
	if err != nil {
		return err
	}

	request.SetBasicAuth(c.config.Username, c.config.Password)
	request.Header.Set("Content-Type", "text/yaml")

	response, err := transport.RoundTrip(request)
	if err != nil {
		return err
	}

	if response.StatusCode != http.StatusCreated {
		return fmt.Errorf("unexpected response %d %s", response.StatusCode, http.StatusText(response.StatusCode))
	}

	return nil
}

func (c Client) DownloadManifest(deploymentName string) ([]byte, error) {
	request, err := http.NewRequest("GET", fmt.Sprintf("%s/deployments/%s", c.config.URL, deploymentName), nil)
	if err != nil {
		return nil, err
	}

	request.SetBasicAuth(c.config.Username, c.config.Password)

	response, err := transport.RoundTrip(request)
	if err != nil {
		return nil, err
	}

	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected response %d %s", response.StatusCode, http.StatusText(response.StatusCode))
	}

	var result deploymentManifest
	err = json.NewDecoder(response.Body).Decode(&result)
	if err != nil {
		return nil, err
	}

	return []byte(result.Manifest), nil
}
