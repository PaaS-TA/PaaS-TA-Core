package containerstore

import (
	"os"

	"code.cloudfoundry.org/clock"
	"code.cloudfoundry.org/garden"
	"code.cloudfoundry.org/lager"
)

type containerReaper struct {
	logger       lager.Logger
	config       *ContainerConfig
	clock        clock.Clock
	containers   *nodeMap
	gardenClient garden.Client
}

func newContainerReaper(logger lager.Logger, config *ContainerConfig, clock clock.Clock, containers *nodeMap, gardenClient garden.Client) *containerReaper {
	return &containerReaper{
		logger:       logger,
		config:       config,
		clock:        clock,
		containers:   containers,
		gardenClient: gardenClient,
	}
}

func (r *containerReaper) Run(signals <-chan os.Signal, ready chan<- struct{}) error {
	logger := r.logger.Session("container-reaper")
	timer := r.clock.NewTimer(r.config.ReapInterval)

	close(ready)

	for {
		select {
		case <-timer.C():
			err := r.reapExtraGardenContainers(logger)
			if err != nil {
				logger.Error("failed-to-reap-extra-containers", err)
			}

			err = r.reapMissingGardenContainers(logger)
			if err != nil {
				logger.Error("failed-to-reap-missing-containers", err)
			}

		case <-signals:
			return nil
		}

		timer.Reset(r.config.ReapInterval)
	}

	return nil
}

func (r *containerReaper) reapExtraGardenContainers(logger lager.Logger) error {
	handles, err := r.fetchGardenContainerHandles(logger)
	if err != nil {
		return err
	}

	for key := range handles {
		if !r.containers.Contains(key) {
			err := r.gardenClient.Destroy(key)
			if err != nil {
				logger.Error("failed-to-destroy-container", err, lager.Data{"handle": key})
			}
		}
	}

	return nil
}

func (r *containerReaper) reapMissingGardenContainers(logger lager.Logger) error {
	logger.Info("starting")
	defer logger.Info("complete")

	handles, err := r.fetchGardenContainerHandles(logger)
	if err != nil {
		return err
	}

	r.containers.CompleteMissing(logger, handles)

	return nil
}

func (r *containerReaper) fetchGardenContainerHandles(logger lager.Logger) (map[string]struct{}, error) {
	properties := garden.Properties{
		ContainerOwnerProperty: r.config.OwnerName,
	}

	gardenContainers, err := r.gardenClient.Containers(properties)
	if err != nil {
		logger.Error("failed-to-fetch-containers", err)
		return nil, err
	}

	handles := make(map[string]struct{})
	for _, gardenContainer := range gardenContainers {
		handles[gardenContainer.Handle()] = struct{}{}
	}
	return handles, nil
}
