package app

import (
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"strings"
	"time"
)

type Config struct {
	ClusterMembers []string
	CA             string
	Cert           string
	Key            string
}

type App struct {
	config  Config
	sleeper func(time.Duration)
	logger  *log.Logger
}

func New(config Config, sleeper func(time.Duration)) App {
	return App{
		config:  config,
		sleeper: sleeper,
		logger:  log.New(os.Stdout, "", log.LstdFlags),
	}
}

func (a App) Run() error {
	if len(a.config.ClusterMembers) == 0 {
		return errors.New("at least one cluster member is required")
	}

	httpClient, err := newHttpClient(a.config.CA, a.config.Cert, a.config.Key)
	if err != nil {
		return err
	}

	for {
		leaders := []string{}
		for _, member := range a.config.ClusterMembers {
			isLeader, err := leaderInfo(httpClient, member)
			if err != nil {
				a.logger.Printf("[ERR] %s\n", err)
				return err
			}

			if isLeader {
				leaders = append(leaders, member)
			}
		}

		if len(leaders) > 1 {
			err := fmt.Errorf("more than one leader exists: %v", leaders)
			a.logger.Printf("[ERR] %s\n", err)
			return err
		}

		fmt.Printf("[INFO] the leader is %v\n", leaders)
		a.sleeper(1 * time.Second)
	}

	return nil
}

func newHttpClient(ca, cert, key string) (*http.Client, error) {
	var client *http.Client
	if ca != "" && cert != "" && key != "" {
		keypair, err := tls.LoadX509KeyPair(cert, key)
		if err != nil {
			return nil, err
		}

		caCert, err := ioutil.ReadFile(ca)
		if err != nil {
			return nil, err
		}

		caCertPool := x509.NewCertPool()
		caCertPool.AppendCertsFromPEM(caCert)

		tlsConfig := &tls.Config{
			Certificates: []tls.Certificate{keypair},
			RootCAs:      caCertPool,
		}
		tlsConfig.BuildNameToCertificate()
		transport := &http.Transport{TLSClientConfig: tlsConfig}
		client = &http.Client{Transport: transport}
	} else {
		client = &http.Client{}
	}

	return client, nil
}

func leaderInfo(client *http.Client, url string) (bool, error) {
	resp, err := client.Get(fmt.Sprintf("%s/v2/stats/leader", url))
	if err != nil {
		switch {
		case strings.Contains(err.Error(), "no route to host"):
			// not tested
			return false, nil
		case strings.Contains(err.Error(), "no such host"):
			return false, nil
		case strings.Contains(err.Error(), "connection refused"):
			return false, nil
		default:
			return false, err
		}
	}
	defer resp.Body.Close()

	buf, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		// not tested
		return false, err
	}

	switch resp.StatusCode {
	case http.StatusOK:
		var leader leader
		if err := json.Unmarshal(buf, &leader); err != nil {
			return false, err
		}
		return true, nil
	case http.StatusForbidden:
		var message message
		if err := json.Unmarshal(buf, &message); err != nil {
			return false, err
		}

		if message.Message == "not current leader" {
			return false, nil
		}
	}

	return false, fmt.Errorf("unexpected status code %d - %s", resp.StatusCode, string(buf))
}

type message struct {
	Message string `json:"message"`
}

type leader struct {
	Leader string `json:"leader"`
}
