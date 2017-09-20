package bosh

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	"github.com/blang/semver"
)

type Stemcell struct {
	Name     string
	Versions []string
}

func NewStemcell() Stemcell {
	return Stemcell{}
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
		body, err := bodyReader(response.Body)
		if err != nil {
			return Stemcell{}, err
		}
		defer response.Body.Close()

		return Stemcell{}, fmt.Errorf("unexpected response %d %s:\n%s", response.StatusCode, http.StatusText(response.StatusCode), body)
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

func (s Stemcell) Latest() (string, error) {
	latestVersion := "0"

	if len(s.Versions) == 0 {
		return "", errors.New("no stemcell versions found, cannot get latest")
	}

	for _, version := range s.Versions {

		semVersion, err := semver.ParseTolerant(version)
		if err != nil {
			return "", err
		}

		semLatestVersion, err := semver.ParseTolerant(latestVersion)
		if err != nil {
			// Not tested
			return "", err
		}

		if semVersion.GT(semLatestVersion) {
			latestVersion = version
		}
	}

	return latestVersion, nil
}
