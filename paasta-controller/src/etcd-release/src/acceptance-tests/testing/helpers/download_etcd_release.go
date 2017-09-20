package helpers

import (
	"encoding/json"
	"errors"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"

	"github.com/pivotal-cf-experimental/bosh-test/bosh"
)

type release struct {
	Name    string
	Version string
	URL     string
}

func DownloadLatestEtcdRelease(client bosh.Client) (string, error) {
	resp, err := http.Get("http://bosh.io/api/v1/releases/github.com/cloudfoundry-incubator/etcd-release")
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	releases := []release{}
	json.NewDecoder(resp.Body).Decode(&releases)

	if len(releases) < 1 {
		return "", errors.New("no releases")
	}

	dir, err := ioutil.TempDir("", "")
	if err != nil {
		return "", err
	}

	out, err := os.Create(filepath.Join(dir, "etcd-release.tgz"))
	if err != nil {
		return "", err
	}
	defer out.Close()

	resp, err = http.Get(releases[0].URL)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	_, err = io.Copy(out, resp.Body)
	if err != nil {
		return "", err
	}

	_, err = out.Seek(0, 0)
	if err != nil {
		return "", err
	}

	info, err := out.Stat()
	if err != nil {
		return "", err
	}

	_, err = client.UploadRelease(bosh.NewSizeReader(out, info.Size()))
	if err != nil {
		return "", err
	}

	return releases[0].Version, nil
}
