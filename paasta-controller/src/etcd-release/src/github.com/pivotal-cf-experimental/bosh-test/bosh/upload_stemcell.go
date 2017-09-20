package bosh

import (
	"fmt"
	"net/http"
)

func (c Client) UploadStemcell(contents SizeReader) (int, error) {
	request, err := http.NewRequest("POST", fmt.Sprintf("%s/stemcells", c.config.URL), contents)
	if err != nil {
		return 0, err
	}

	request.SetBasicAuth(c.config.Username, c.config.Password)
	request.Header.Set("Content-Type", "application/x-compressed")
	request.ContentLength = contents.Size()

	response, err := transport.RoundTrip(request)
	if err != nil {
		return 0, err
	}

	if response.StatusCode != http.StatusFound {
		return 0, fmt.Errorf("unexpected response %s", response.Status)
	}

	return c.checkTaskStatus(response.Header.Get("Location"))
}
