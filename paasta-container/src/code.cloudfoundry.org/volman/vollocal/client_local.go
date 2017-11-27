package vollocal

import (
	"errors"
	"time"

	"github.com/tedsuo/ifrit"

	"context"

	"fmt"
	"os"
	"strings"

	"code.cloudfoundry.org/clock"
	loggregator_v2 "code.cloudfoundry.org/go-loggregator/compatibility"
	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/voldriver"
	"code.cloudfoundry.org/voldriver/driverhttp"
	"code.cloudfoundry.org/volman"
	"github.com/tedsuo/ifrit/grouper"
)

const (
	volmanMountErrorsCounter   = "VolmanMountErrors"
	volmanMountDuration        = "VolmanMountDuration"
	volmanUnmountErrorsCounter = "VolmanUnmountErrors"
	volmanUnmountDuration      = "VolmanUnmountDuration"
)

var (
	driverMountDurations   = map[string]string{}
	driverUnmountDurations = map[string]string{}
)

type DriverConfig struct {
	DriverPaths  []string
	SyncInterval time.Duration
}

func NewDriverConfig() DriverConfig {
	return DriverConfig{
		SyncInterval: time.Second * 30,
	}
}

type localClient struct {
	driverRegistry DriverRegistry
	metronClient   loggregator_v2.IngressClient
	clock          clock.Clock
}

func NewServer(logger lager.Logger, metronClient loggregator_v2.IngressClient, config DriverConfig) (volman.Manager, ifrit.Runner) {
	clock := clock.NewClock()
	registry := NewDriverRegistry()

	syncer := NewDriverSyncer(logger, registry, config.DriverPaths, config.SyncInterval, clock)
	purger := NewMountPurger(logger, registry)

	grouper := grouper.NewOrdered(os.Kill, grouper.Members{grouper.Member{"volman-syncer", syncer.Runner()}, grouper.Member{"volman-purger", purger.Runner()}})

	return NewLocalClient(logger, registry, metronClient, clock), grouper
}

func NewLocalClient(logger lager.Logger, registry DriverRegistry, metronClient loggregator_v2.IngressClient, clock clock.Clock) volman.Manager {
	return &localClient{
		driverRegistry: registry,
		metronClient:   metronClient,
		clock:          clock,
	}
}

func (client *localClient) ListDrivers(logger lager.Logger) (volman.ListDriversResponse, error) {
	logger = logger.Session("list-drivers")
	logger.Info("start")
	defer logger.Info("end")

	var infoResponses []volman.InfoResponse
	drivers := client.driverRegistry.Drivers()

	for name, _ := range drivers {
		infoResponses = append(infoResponses, volman.InfoResponse{Name: name})
	}

	logger.Debug("listing-drivers", lager.Data{"drivers": infoResponses})
	return volman.ListDriversResponse{infoResponses}, nil
}

func (client *localClient) Mount(logger lager.Logger, driverId string, volumeId string, config map[string]interface{}) (volman.MountResponse, error) {
	logger = logger.Session("mount")
	logger.Info("start")
	defer logger.Info("end")

	mountStart := client.clock.Now()

	defer func() {
		sendMountDurationMetrics(logger, client.metronClient, time.Since(mountStart), driverId)
	}()

	logger.Debug("driver-mounting-volume", lager.Data{"driverId": driverId, "volumeId": volumeId})

	driver, found := client.driverRegistry.Driver(driverId)
	if !found {
		err := errors.New("Driver '" + driverId + "' not found in list of known drivers")
		logger.Error("mount-driver-lookup-error", err)
		client.metronClient.IncrementCounter(volmanMountErrorsCounter)
		return volman.MountResponse{}, err
	}

	err := client.create(logger, driverId, volumeId, config)
	if err != nil {
		client.metronClient.IncrementCounter(volmanMountErrorsCounter)
		return volman.MountResponse{}, err
	}

	env := driverhttp.NewHttpDriverEnv(logger, context.TODO())

	mountRequest := voldriver.MountRequest{Name: volumeId}
	logger.Debug("calling-driver-with-mount-request", lager.Data{"driverId": driverId, "mountRequest": mountRequest})
	mountResponse := driver.Mount(env, mountRequest)
	logger.Debug("response-from-driver", lager.Data{"response": mountResponse})

	if !strings.HasPrefix(mountResponse.Mountpoint, "/var/vcap/data") {
		logger.Info("invalid-mountpath", lager.Data{"detail": fmt.Sprintf("Invalid or dangerous mountpath %s outside of /var/vcap/data", mountResponse.Mountpoint)})
	}

	if mountResponse.Err != "" {
		client.metronClient.IncrementCounter(volmanMountErrorsCounter)
		return volman.MountResponse{}, errors.New(mountResponse.Err)
	}

	return volman.MountResponse{mountResponse.Mountpoint}, nil
}

func sendMountDurationMetrics(logger lager.Logger, metronClient loggregator_v2.IngressClient, duration time.Duration, driverId string) {
	err := metronClient.SendDuration(volmanMountDuration, duration)
	if err != nil {
		logger.Error("failed-to-send-volman-mount-duration-metric", err)
	}

	m, ok := driverMountDurations[driverId]
	if !ok {
		m = "VolmanMountDurationFor" + driverId
		driverMountDurations[driverId] = m
	}
	err = metronClient.SendDuration(m, duration)
	if err != nil {
		logger.Error("failed-to-send-volman-mount-duration-metric", err)
	}
}

func sendUnmountDurationMetrics(logger lager.Logger, metronClient loggregator_v2.IngressClient, duration time.Duration, driverId string) {
	err := metronClient.SendDuration(volmanUnmountDuration, duration)
	if err != nil {
		logger.Error("failed-to-send-volman-unmount-duration-metric", err)
	}

	m, ok := driverUnmountDurations[driverId]
	if !ok {
		m = "VolmanUnmountDurationFor" + driverId
		driverUnmountDurations[driverId] = m
	}
	err = metronClient.SendDuration(m, duration)
	if err != nil {
		logger.Error("failed-to-send-volman-unmount-duration-metric", err)
	}
}

func (client *localClient) Unmount(logger lager.Logger, driverId string, volumeName string) error {
	logger = logger.Session("unmount")
	logger.Info("start")
	defer logger.Info("end")
	logger.Debug("unmounting-volume", lager.Data{"volumeName": volumeName})

	unmountStart := client.clock.Now()

	defer func() {
		sendUnmountDurationMetrics(logger, client.metronClient, time.Since(unmountStart), driverId)
	}()

	driver, found := client.driverRegistry.Driver(driverId)
	if !found {
		err := errors.New("Driver '" + driverId + "' not found in list of known drivers")
		logger.Error("mount-driver-lookup-error", err)
		client.metronClient.IncrementCounter(volmanUnmountErrorsCounter)
		return err
	}

	env := driverhttp.NewHttpDriverEnv(logger, context.TODO())

	if response := driver.Unmount(env, voldriver.UnmountRequest{Name: volumeName}); response.Err != "" {
		err := errors.New(response.Err)
		logger.Error("unmount-failed", err)
		client.metronClient.IncrementCounter(volmanUnmountErrorsCounter)
		return err
	}

	return nil
}

func (client *localClient) create(logger lager.Logger, driverId string, volumeName string, opts map[string]interface{}) error {
	logger = logger.Session("create")
	logger.Info("start")
	defer logger.Info("end")
	driver, found := client.driverRegistry.Driver(driverId)
	if !found {
		err := errors.New("Driver '" + driverId + "' not found in list of known drivers")
		logger.Error("mount-driver-lookup-error", err)
		return err
	}

	env := driverhttp.NewHttpDriverEnv(logger, context.TODO())

	logger.Debug("creating-volume", lager.Data{"volumeName": volumeName, "driverId": driverId})
	response := driver.Create(env, voldriver.CreateRequest{Name: volumeName, Opts: opts})
	if response.Err != "" {
		return errors.New(response.Err)
	}
	return nil
}
