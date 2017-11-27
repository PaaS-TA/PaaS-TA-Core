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

type members struct {
	Members []member `json:"members"`
}

type member struct {
	ClientURLs []string `json:"clientURLs"`
	ID         string   `json:"id"`
}

type self struct {
	LeaderInfo leader `json:"leaderInfo"`
}

type leader struct {
	Leader string `json:"leader"`
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
		leaders := map[string]string{}
		for _, member := range a.config.ClusterMembers {
			leader, err := leaderFromSelf(httpClient, member)
			if err != nil {
				a.logger.Printf("[ERR] %s\n", err)
				return err
			}

			leaderURL, err := getNodeURL(httpClient, member, leader)
			if err != nil {
				a.logger.Printf("[ERR] %s\n", err)
				return err
			}

			if leader != "" {
				leaders[leader] = leaderURL
			}
		}

		var leaderURLs []string
		for _, leaderURL := range leaders {
			leaderURLs = append(leaderURLs, leaderURL)
		}

		if len(leaderURLs) > 1 {
			err := fmt.Errorf("more than one leader exists: %v", leaderURLs)
			a.logger.Printf("[ERR] %s\n", err)
			return err
		}

		fmt.Printf("[INFO] the leader is %v\n", leaderURLs)
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

func leaderFromSelf(client *http.Client, url string) (string, error) {
	resp, err := client.Get(fmt.Sprintf("%s/v2/stats/self", url))
	if err != nil {
		switch {
		case strings.Contains(err.Error(), "no route to host"):
			// not tested
			return "", nil
		case strings.Contains(err.Error(), "no such host"):
			return "", nil
		case strings.Contains(err.Error(), "connection refused"):
			return "", nil
		default:
			return "", err
		}
	}
	defer resp.Body.Close()

	buf, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		// not tested
		return "", err
	}

	switch resp.StatusCode {
	case http.StatusOK:
		var self self
		if err := json.Unmarshal(buf, &self); err != nil {
			return "", err
		}
		return self.LeaderInfo.Leader, nil
	}

	return "", fmt.Errorf("unexpected status code %d - %s", resp.StatusCode, string(buf))
}

func getNodeURL(client *http.Client, url, leaderID string) (string, error) {
	resp, err := client.Get(fmt.Sprintf("%s/v2/members", url))
	if err != nil {
		switch {
		case strings.Contains(err.Error(), "no route to host"):
			// not tested
			return "", nil
		case strings.Contains(err.Error(), "no such host"):
			return "", nil
		case strings.Contains(err.Error(), "connection refused"):
			return "", nil
		default:
			return "", err
		}
	}
	defer resp.Body.Close()

	buf, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		// not tested
		return "", err
	}

	switch resp.StatusCode {
	case http.StatusOK:
		var members members
		if err := json.Unmarshal(buf, &members); err != nil {
			return "", err
		}

		for _, member := range members.Members {
			if member.ID == leaderID {
				return member.ClientURLs[0], nil
			}
		}
	}

	return "", fmt.Errorf("unexpected status code %d - %s", resp.StatusCode, string(buf))
}
