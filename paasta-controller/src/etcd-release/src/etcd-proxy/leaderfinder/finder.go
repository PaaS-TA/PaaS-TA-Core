package leaderfinder

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
)

var (
	NoAddressesProvided   = errors.New("no addresses have been provided")
	LeaderNotFound        = errors.New("leader not found")
	MembersNotFound       = errors.New("no etcd members could be found")
	NoClientURLs          = errors.New("no etcd member client urls could be found")
	NoClientURLsForLeader = errors.New("no etcd member client url for leader")
)

type Finder struct {
	address string
	client  getter
}

type members struct {
	Members []member `json:"members"`
}

type member struct {
	ClientURLs []string `json:"clientURLs"`
	ID         string   `json:"id"`
}

type self struct {
	LeaderInfo leaderInfo `json:"leaderInfo"`
}

type leaderInfo struct {
	Leader string `json:"leader"`
}

type getter interface {
	Get(url string) (resp *http.Response, err error)
}

func NewFinder(address string, client getter) Finder {
	return Finder{
		address: address,
		client:  client,
	}
}

func (f Finder) Find() (*url.URL, error) {
	if len(f.address) == 0 {
		return nil, errors.New("no address provided")
	}

	resp, err := f.client.Get(fmt.Sprintf("%s/v2/members", f.address))
	if err != nil {
		return nil, err
	}

	var members members
	err = json.NewDecoder(resp.Body).Decode(&members)
	if err != nil {
		return nil, err
	}

	if len(members.Members) == 0 {
		return nil, MembersNotFound
	}

	// fixme remove
	//if len(members.Members[0].ClientURLs) == 0 {
	//return nil, NoClientURLs
	//}

	resp, err = f.client.Get(fmt.Sprintf("%s/v2/stats/self", f.address))
	if err != nil {
		return nil, err
	}

	var self self
	err = json.NewDecoder(resp.Body).Decode(&self)
	if err != nil {
		return nil, err
	}

	leaderID := self.LeaderInfo.Leader

	var leaderURL string

	for _, member := range members.Members {
		if member.ID == leaderID {
			if len(member.ClientURLs) == 0 {
				return nil, NoClientURLsForLeader
			}

			leaderURL = member.ClientURLs[0]
			break
		}
	}

	if leaderURL == "" {
		return nil, LeaderNotFound
	}

	leader, err := url.Parse(leaderURL)
	if err != nil {
		return nil, err
	}

	return leader, nil
}
