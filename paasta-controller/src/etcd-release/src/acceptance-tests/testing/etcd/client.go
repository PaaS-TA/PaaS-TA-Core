package etcd

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"
)

var bodyReader = ioutil.ReadAll

type Client struct {
	testConsumerURL string
}

func NewClient(testConsumerURL string) Client {
	return Client{
		testConsumerURL: testConsumerURL,
	}
}

func (c Client) Address() string {
	return c.testConsumerURL
}

func (c Client) Get(key string) (string, error) {
	resp, err := http.Get(fmt.Sprintf("%s/kv/%s", c.testConsumerURL, key))
	if err != nil {
		return "", err
	}

	body, err := bodyReader(resp.Body)
	if err != nil {
		return "", err
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("unexpected status: %d %s %s", resp.StatusCode, http.StatusText(resp.StatusCode), string(body))
	}

	return string(body), nil
}

func (c Client) Set(key, value string) error {
	request, err := http.NewRequest("PUT", fmt.Sprintf("%s/kv/%s", c.testConsumerURL, key), strings.NewReader(value))
	if err != nil {
		return err
	}

	resp, err := http.DefaultClient.Do(request)
	if err != nil {
		return err
	}

	body, err := bodyReader(resp.Body)
	if err != nil {
		return err
	}

	if resp.StatusCode != http.StatusCreated {
		return fmt.Errorf("unexpected status: %d %s %s", resp.StatusCode, http.StatusText(resp.StatusCode), string(body))
	}

	return nil
}

func (c Client) Leader() (string, error) {
	endpoint := fmt.Sprintf("%s/leader", c.testConsumerURL)

	resp, err := http.Get(endpoint)
	if err != nil {
		return "", err
	}

	body, err := bodyReader(resp.Body)
	if err != nil {
		return "", err
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("unexpected status: %d %s %s", resp.StatusCode, http.StatusText(resp.StatusCode), string(body))
	}

	return string(body), nil
}

func (c Client) LeaderByNodeURL(nodeURL string) (string, error) {
	endpoint := fmt.Sprintf("%s/leader?node=%s", c.testConsumerURL, nodeURL)

	resp, err := http.Get(endpoint)
	if err != nil {
		return "", err
	}

	body, err := bodyReader(resp.Body)
	if err != nil {
		return "", err
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("unexpected status: %d %s %s", resp.StatusCode, http.StatusText(resp.StatusCode), string(body))
	}

	return string(body), nil
}
