package vollocal

import (
	"bufio"
	"encoding/json"
	"fmt"
	"path"
	"strings"

	"net/url"

	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/voldriver"
	"code.cloudfoundry.org/voldriver/driverhttp"
	"code.cloudfoundry.org/goshims/os"
)

//go:generate counterfeiter -o ../volmanfakes/fake_driver_factory.go . DriverFactory

// DriverFactories are responsible for instantiating remote client implementations of the voldriver.Driver interface.
type DriverFactory interface {
	// Given a driver id, path and config filename returns a remote client implementation of the voldriver.Driver interface
	Driver(logger lager.Logger, driverId string, driverPath, driverFileName string) (voldriver.Driver, error)
}

type realDriverFactory struct {
	Factory         driverhttp.RemoteClientFactory
	useOs           osshim.Os
	DriversRegistry map[string]voldriver.Driver
}

func NewDriverFactory() DriverFactory {
	remoteClientFactory := driverhttp.NewRemoteClientFactory()
	return NewDriverFactoryWithRemoteClientFactory(remoteClientFactory)
}

func NewDriverFactoryWithRemoteClientFactory(remoteClientFactory driverhttp.RemoteClientFactory) DriverFactory {
	return &realDriverFactory{remoteClientFactory, &osshim.OsShim{}, nil}
}

func NewDriverFactoryWithOs(useOs osshim.Os) DriverFactory {
	remoteClientFactory := driverhttp.NewRemoteClientFactory()
	return &realDriverFactory{remoteClientFactory, useOs, nil}
}

func (r *realDriverFactory) Driver(logger lager.Logger, driverId string, driverPath string, driverFileName string) (voldriver.Driver, error) {
	logger = logger.Session("driver", lager.Data{"driverId": driverId, "driverFileName": driverFileName})
	logger.Info("start")
	defer logger.Info("end")

	var driver voldriver.Driver

	var address string
	var tls *voldriver.TLSConfig
	if strings.Contains(driverFileName, ".") {
		extension := strings.Split(driverFileName, ".")[1]
		switch extension {
		case "sock":
			address = path.Join(driverPath, driverFileName)
		case "spec":
			configFile, err := r.useOs.Open(path.Join(driverPath, driverFileName))
			if err != nil {
				logger.Error(fmt.Sprintf("error-opening-config-%s", driverFileName), err)
				return nil, err
			}
			reader := bufio.NewReader(configFile)
			addressBytes, _, err := reader.ReadLine()
			if err != nil { // no real value in faking this as bigger problems exist when this fails
				logger.Error(fmt.Sprintf("error-reading-%s", driverFileName), err)
				return nil, err
			}
			address = string(addressBytes)
		case "json":
			// extract url from json file
			var driverJsonSpec voldriver.DriverSpec
			configFile, err := r.useOs.Open(path.Join(driverPath, driverFileName))
			if err != nil {
				logger.Error(fmt.Sprintf("error-opening-config-%s", driverFileName), err)
				return nil, err
			}
			jsonParser := json.NewDecoder(configFile)
			if err = jsonParser.Decode(&driverJsonSpec); err != nil {
				logger.Error("parsing-config-file-error", err)
				return nil, err
			}
			address = driverJsonSpec.Address
			tls = driverJsonSpec.TLSConfig
		default:
			err := fmt.Errorf("unknown-driver-extension: %s", extension)
			logger.Error("driver", err)
			return nil, err

		}
		var err error

		address, err = r.canonicalize(logger, address)
		if err != nil {
			logger.Error(fmt.Sprintf("invalid-address: %s", address), err)
			return nil, err
		}

		logger.Info("getting-driver", lager.Data{"address": address})
		driver, err = r.Factory.NewRemoteClient(address, tls)
		if err != nil {
			logger.Error(fmt.Sprintf("error-building-driver-attached-to-%s", address), err)
			return nil, err
		}

		return driver, nil
	}

	return nil, fmt.Errorf("Driver '%s' not found in list of known drivers", driverId)
}

func (r *realDriverFactory) canonicalize(logger lager.Logger, address string) (string, error) {
	logger = logger.Session("canonicalize", lager.Data{"address": address})
	logger.Debug("start")
	defer logger.Debug("end")

	url, err := url.Parse(address)
	if err != nil {
		return address, err
	}

	switch url.Scheme {
	case "http", "https":
		return address, nil
	case "tcp":
		return fmt.Sprintf("http://%s%s", url.Host, url.Path), nil
	case "unix":
		return address, nil
	default:
		if strings.HasSuffix(url.Path, ".sock") {
			return fmt.Sprintf("%s%s", url.Host, url.Path), nil
		}
	}
	return fmt.Sprintf("http://%s", address), nil
}

func driverImplements(protocol string, activateResponseProtocols []string) bool {
	for _, nextProtocol := range activateResponseProtocols {
		if protocol == nextProtocol {
			return true
		}
	}
	return false
}
