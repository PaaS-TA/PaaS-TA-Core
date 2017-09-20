package client

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"strconv"
)

var bodyReader = ioutil.ReadAll

type Client struct {
	url string
}

func New(url string) Client {
	return Client{url: url}
}

func (c Client) DNS(serviceName string) ([]string, error) {
	resp, err := http.Get(fmt.Sprintf("%s/dns?service=%s", c.url, serviceName))
	if err != nil {
		return []string{}, err
	}

	body, err := readBodyAndResponse(resp)
	if err != nil {
		return []string{}, err
	}

	var addresses []string
	if err := json.Unmarshal(body, &addresses); err != nil {
		return []string{}, err
	}

	return addresses, nil
}

func (c Client) SetHealthCheck(health bool) error {
	resp, err := http.Post(fmt.Sprintf("%s/health_check", c.url),
		"application/json",
		bytes.NewBufferString(strconv.FormatBool(health)),
	)
	if err != nil {
		return err
	}

	if _, err := readBodyAndResponse(resp); err != nil {
		return err
	}

	return nil
}

func readBodyAndResponse(resp *http.Response) ([]byte, error) {
	body, err := bodyReader(resp.Body)
	if err != nil {
		return []byte{}, err
	}

	if resp.StatusCode != http.StatusOK {
		return []byte{}, fmt.Errorf("unexpected status: %s %s", resp.Status, string(body))
	}

	return body, nil
}
