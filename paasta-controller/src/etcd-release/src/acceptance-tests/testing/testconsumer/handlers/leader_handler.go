package handlers

import (
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
)

type LeaderHandler struct {
	etcdURL    string
	caCert     string
	clientCert string
	clientKey  string
}

func NewLeaderHandler(etcdURL string, caCert, clientCert, clientKey string) LeaderHandler {
	return LeaderHandler{
		etcdURL:    etcdURL,
		caCert:     caCert,
		clientCert: clientCert,
		clientKey:  clientKey,
	}
}

func (h *LeaderHandler) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	var client *http.Client

	if h.caCert != "" && h.clientCert != "" && h.clientKey != "" {
		keypair, err := tls.LoadX509KeyPair(h.clientCert, h.clientKey)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		caCert, err := ioutil.ReadFile(h.caCert)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
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

	var etcdURL string
	node, hasNode := req.URL.Query()["node"]
	if hasNode {
		etcdURL = node[0]
	} else {
		etcdURL = h.etcdURL
	}

	members, err := getMembers(client, etcdURL)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	member, err := getLeader(client, etcdURL, members)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}

	w.WriteHeader(http.StatusOK)
	w.Write([]byte(member.Name))
}

func getMembers(client *http.Client, url string) (Members, error) {
	resp, err := client.Get(fmt.Sprintf("%s/v2/members", url))
	if err != nil {
		return Members{}, err
	}
	defer resp.Body.Close()

	buf, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return Members{}, err
	}

	if resp.StatusCode != http.StatusOK {
		return Members{}, fmt.Errorf("unexpected status code %d - %s", resp.StatusCode, string(buf))
	}

	var members Members
	if err := json.Unmarshal(buf, &members); err != nil {
		return Members{}, err
	}

	return members, nil
}

func getLeader(client *http.Client, url string, members Members) (Member, error) {
	resp, err := client.Get(fmt.Sprintf("%s/v2/stats/self", url))
	if err != nil {
		return Member{}, err
	}
	defer resp.Body.Close()

	buf, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return Member{}, err
	}

	if resp.StatusCode != http.StatusOK {
		return Member{}, fmt.Errorf("unexpected status code %d - %s", resp.StatusCode, string(buf))
	}

	var node Node
	if err := json.Unmarshal(buf, &node); err != nil {
		return Member{}, err
	}

	for _, member := range members.Members {
		if member.ID == node.LeaderInfo.Leader {
			return member, nil
		}
	}

	return Member{}, errors.New("could not determine leader")
}

type LeaderInfo struct {
	Leader string `json:"leader"`
}

type Node struct {
	LeaderInfo LeaderInfo `json:"leaderInfo"`
}

type Members struct {
	Members []Member `json:"members"`
}

type Member struct {
	ID         string   `json:"id"`
	ClientURLs []string `json:"clientURLs"`
	PeerURLs   []string `json:"peerURLs"`
	Name       string   `json:"name"`
}
