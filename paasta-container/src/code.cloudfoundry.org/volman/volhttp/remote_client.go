package volhttp

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"

	"code.cloudfoundry.org/cfhttp"
	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/volman"
	"github.com/tedsuo/rata"
)

type remoteClient struct {
	HttpClient *http.Client
	reqGen     *rata.RequestGenerator
}

func NewRemoteClient(volmanURL string) *remoteClient {
	return &remoteClient{
		HttpClient: cfhttp.NewClient(),
		reqGen:     rata.NewRequestGenerator(volmanURL, volman.Routes),
	}
}

func (r *remoteClient) ListDrivers(logger lager.Logger) (volman.ListDriversResponse, error) {
	logger = logger.Session("list-drivers")
	logger.Debug("start")
	defer logger.Debug("end")

	request, err := r.reqGen.CreateRequest(volman.ListDriversRoute, nil, nil)
	if err != nil {
		return volman.ListDriversResponse{}, r.clientError(logger, err, fmt.Sprintf("Error creating request to %s", volman.ListDriversRoute))
	}

	response, err := r.HttpClient.Do(request)
	if err != nil {
		return volman.ListDriversResponse{}, r.clientError(logger, err, "Error in Listing Drivers remote call")
	}
	var drivers volman.ListDriversResponse
	err = unmarshallJSON(logger, response.Body, &drivers)

	if err != nil {
		return volman.ListDriversResponse{}, r.clientError(logger, err, "Error in Parsing JSON Response of List Drivers")
	}

	return drivers, err
}

func (r *remoteClient) Mount(logger lager.Logger, driverId string, volumeId string, config map[string]interface{}) (volman.MountResponse, error) {
	logger = logger.Session("mount")
	logger.Info("start")
	defer logger.Info("end")

	MountRequest := volman.MountRequest{driverId, volumeId, config}

	sendingJson, err := json.Marshal(MountRequest)
	if err != nil {
		return volman.MountResponse{}, r.clientError(logger, err, fmt.Sprintf("Error marshalling JSON request %#v", MountRequest))
	}

	request, err := r.reqGen.CreateRequest(volman.MountRoute, nil, bytes.NewReader(sendingJson))

	if err != nil {
		return volman.MountResponse{}, r.clientError(logger, err, fmt.Sprintf("Error creating request to %s", volman.MountRoute))
	}

	response, err := r.HttpClient.Do(request)
	if err != nil {
		return volman.MountResponse{}, r.clientError(logger, err, fmt.Sprintf("Error mounting volume %s", volumeId))
	}

	if response.StatusCode == 500 {
		var remoteError volman.Error
		if err := unmarshallJSON(logger, response.Body, &remoteError); err != nil {
			return volman.MountResponse{}, r.clientError(logger, err, fmt.Sprintf("Error parsing 500 response from %s", volman.MountRoute))
		}
		return volman.MountResponse{}, remoteError
	}

	var mountPoint volman.MountResponse
	if err := unmarshallJSON(logger, response.Body, &mountPoint); err != nil {
		return volman.MountResponse{}, r.clientError(logger, err, fmt.Sprintf("Error parsing response from %s", volman.MountRoute))
	}

	return mountPoint, err
}

func (r *remoteClient) Unmount(logger lager.Logger, driverId string, volumeId string) error {
	logger = logger.Session("mount")
	logger.Info("start")
	defer logger.Info("end")

	unmountRequest := volman.UnmountRequest{driverId, volumeId}
	payload, err := json.Marshal(unmountRequest)
	if err != nil {
		return r.clientError(logger, err, fmt.Sprintf("Error marshalling JSON request %#v", unmountRequest))
	}

	request, err := r.reqGen.CreateRequest(volman.UnmountRoute, nil, bytes.NewReader(payload))

	if err != nil {
		return r.clientError(logger, err, fmt.Sprintf("Error creating request to %s", volman.UnmountRoute))
	}

	response, err := r.HttpClient.Do(request)
	if err != nil {
		return r.clientError(logger, err, fmt.Sprintf("Error unmounting volume %s", volumeId))
	}

	if response.StatusCode == 500 {
		var remoteError volman.Error
		if err := unmarshallJSON(logger, response.Body, &remoteError); err != nil {
			return r.clientError(logger, err, fmt.Sprintf("Error parsing 500 response from %s", volman.UnmountRoute))
		}
		return remoteError
	}

	return nil
}

func unmarshallJSON(logger lager.Logger, reader io.ReadCloser, jsonResponse interface{}) error {
	body, err := ioutil.ReadAll(reader)
	if err != nil {
		logger.Error("Error in Reading HTTP Response body from remote.", err)
	}
	err = json.Unmarshal(body, jsonResponse)

	return err
}

func (r *remoteClient) clientError(logger lager.Logger, err error, msg string) error {
	logger.Error(msg, err)
	return err
}
