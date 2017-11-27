package depot

import (
	"io"
	"sync"

	"code.cloudfoundry.org/executor"
	"code.cloudfoundry.org/executor/depot/containerstore"
	"code.cloudfoundry.org/executor/depot/event"
	"code.cloudfoundry.org/garden"
	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/volman"
	"code.cloudfoundry.org/workpool"
)

const ContainerStoppedBeforeRunMessage = "Container stopped by user"

type client struct {
	totalCapacity    executor.ExecutorResources
	containerStore   containerstore.ContainerStore
	gardenClient     garden.Client
	volmanClient     volman.Manager
	eventHub         event.Hub
	creationWorkPool *workpool.WorkPool
	deletionWorkPool *workpool.WorkPool
	readWorkPool     *workpool.WorkPool
	metricsWorkPool  *workpool.WorkPool

	healthyLock sync.RWMutex
	healthy     bool
}

func NewClient(
	totalCapacity executor.ExecutorResources,
	containerStore containerstore.ContainerStore,
	gardenClient garden.Client,
	volmanClient volman.Manager,
	eventHub event.Hub,
	workPoolSettings executor.WorkPoolSettings,
) executor.Client {
	// A misconfigured WorkPool is non-recoverable, so we panic here
	creationWorkPool, err := workpool.NewWorkPool(workPoolSettings.CreateWorkPoolSize)
	if err != nil {
		panic(err)
	}
	deletionWorkPool, err := workpool.NewWorkPool(workPoolSettings.DeleteWorkPoolSize)
	if err != nil {
		panic(err)
	}
	readWorkPool, err := workpool.NewWorkPool(workPoolSettings.ReadWorkPoolSize)
	if err != nil {
		panic(err)
	}
	metricsWorkPool, err := workpool.NewWorkPool(workPoolSettings.MetricsWorkPoolSize)
	if err != nil {
		panic(err)
	}

	return &client{
		totalCapacity:    totalCapacity,
		containerStore:   containerStore,
		gardenClient:     gardenClient,
		volmanClient:     volmanClient,
		eventHub:         eventHub,
		creationWorkPool: creationWorkPool,
		deletionWorkPool: deletionWorkPool,
		readWorkPool:     readWorkPool,
		metricsWorkPool:  metricsWorkPool,
		healthy:          true,
	}
}

func (c *client) Cleanup(logger lager.Logger) {
	c.creationWorkPool.Stop()
	c.deletionWorkPool.Stop()
	c.readWorkPool.Stop()
	c.metricsWorkPool.Stop()
	c.containerStore.Cleanup(logger)
}

func (c *client) AllocateContainers(logger lager.Logger, requests []executor.AllocationRequest) ([]executor.AllocationFailure, error) {
	logger = logger.Session("allocate-containers")
	failures := make([]executor.AllocationFailure, 0)

	for i := range requests {
		req := &requests[i]
		err := req.Validate()
		if err != nil {
			logger.Error("invalid-request", err)
			failures = append(failures, executor.NewAllocationFailure(req, err.Error()))
			continue
		}

		_, err = c.containerStore.Reserve(logger, req)
		if err != nil {
			logger.Error("failed-to-allocate-container", err, lager.Data{"guid": req.Guid})
			failures = append(failures, executor.NewAllocationFailure(req, err.Error()))
			continue
		}
	}

	return failures, nil
}

func (c *client) GetContainer(logger lager.Logger, guid string) (executor.Container, error) {
	logger = logger.Session("get-container", lager.Data{
		"guid": guid,
	})

	container, err := c.containerStore.Get(logger, guid)
	if err != nil {
		logger.Error("failed-to-get-container", err)
	}

	return container, err
}

func (c *client) RunContainer(logger lager.Logger, request *executor.RunRequest) error {
	logger = logger.Session("run-container", lager.Data{
		"guid": request.Guid,
	})

	logger.Debug("initializing-container")
	err := c.containerStore.Initialize(logger, request)
	if err != nil {
		logger.Error("failed-initializing-container", err)
		return err
	}
	logger.Debug("succeeded-initializing-container")

	c.creationWorkPool.Submit(c.newRunContainerWorker(logger, request.Guid))
	return nil
}

func (c *client) newRunContainerWorker(logger lager.Logger, guid string) func() {
	return func() {
		logger.Info("creating-container")
		_, err := c.containerStore.Create(logger, guid)
		if err != nil {
			logger.Error("failed-creating-container", err)
			return
		}
		logger.Info("succeeded-creating-container-in-garden")

		logger.Info("running-container-in-garden")
		err = c.containerStore.Run(logger, guid)
		if err != nil {
			logger.Error("failed-running-container-in-garden", err)
		}
		logger.Info("succeeded-running-container-in-garden")
	}
}

func tagsMatch(needles, haystack executor.Tags) bool {
	for k, v := range needles {
		if haystack[k] != v {
			return false
		}
	}

	return true
}

func (c *client) ListContainers(logger lager.Logger) ([]executor.Container, error) {
	return c.containerStore.List(logger), nil
}

func (c *client) GetBulkMetrics(logger lager.Logger) (map[string]executor.Metrics, error) {
	errChannel := make(chan error, 1)
	metricsChannel := make(chan map[string]executor.Metrics, 1)

	logger = logger.Session("get-all-metrics")

	c.metricsWorkPool.Submit(func() {
		cmetrics, err := c.containerStore.Metrics(logger)
		if err != nil {
			logger.Error("failed-to-get-metrics", err)
			errChannel <- err
			return
		}

		metrics := make(map[string]executor.Metrics)
		for _, container := range c.containerStore.List(logger) {
			if container.MetricsConfig.Guid != "" {
				if cmetric, found := cmetrics[container.Guid]; found {
					metrics[container.Guid] = executor.Metrics{
						MetricsConfig:    container.MetricsConfig,
						ContainerMetrics: cmetric,
					}
				}
			}
		}
		metricsChannel <- metrics
	})

	var metrics map[string]executor.Metrics
	var err error
	select {
	case metrics = <-metricsChannel:
		err = nil
	case err = <-errChannel:
		metrics = make(map[string]executor.Metrics)
	}

	close(metricsChannel)
	close(errChannel)
	return metrics, err
}

func (c *client) StopContainer(logger lager.Logger, guid string) error {
	logger = logger.Session("stop-container")
	logger.Info("starting")
	defer logger.Info("complete")

	return c.containerStore.Stop(logger, guid)
}

func (c *client) DeleteContainer(logger lager.Logger, guid string) error {
	logger = logger.Session("delete-container", lager.Data{"guid": guid})

	logger.Info("starting")
	defer logger.Info("complete")

	errChannel := make(chan error, 1)
	c.deletionWorkPool.Submit(func() {
		errChannel <- c.containerStore.Destroy(logger, guid)
	})

	err := <-errChannel

	if err != nil {
		logger.Error("failed-to-delete-garden-container", err)
	}

	return err
}

func (c *client) RemainingResources(logger lager.Logger) (executor.ExecutorResources, error) {
	logger = logger.Session("remaining-resources")
	return c.containerStore.RemainingResources(logger), nil
}

func (c *client) Ping(logger lager.Logger) error {
	return c.gardenClient.Ping()
}

func (c *client) TotalResources(logger lager.Logger) (executor.ExecutorResources, error) {
	totalCapacity := c.totalCapacity

	return executor.ExecutorResources{
		MemoryMB:   totalCapacity.MemoryMB,
		DiskMB:     totalCapacity.DiskMB,
		Containers: totalCapacity.Containers,
	}, nil
}

func (c *client) GetFiles(logger lager.Logger, guid, sourcePath string) (io.ReadCloser, error) {
	logger = logger.Session("get-files", lager.Data{
		"guid": guid,
	})

	errChannel := make(chan error, 1)
	readChannel := make(chan io.ReadCloser, 1)
	c.readWorkPool.Submit(func() {
		readCloser, err := c.containerStore.GetFiles(logger, guid, sourcePath)
		if err != nil {
			errChannel <- err
		} else {
			readChannel <- readCloser
		}
	})

	var readCloser io.ReadCloser
	var err error
	select {
	case readCloser = <-readChannel:
		err = nil
	case err = <-errChannel:
	}
	return readCloser, err
}

func (c *client) VolumeDrivers(logger lager.Logger) ([]string, error) {
	logger = logger.Session("volume-drivers")

	response, err := c.volmanClient.ListDrivers(logger)
	if err != nil {
		logger.Error("cannot-fetch-drivers", err)
		return nil, err
	}

	actualDrivers := make([]string, 0, len(response.Drivers))
	for _, driver := range response.Drivers {
		actualDrivers = append(actualDrivers, driver.Name)
	}
	return actualDrivers, nil
}

func (c *client) SubscribeToEvents(logger lager.Logger) (executor.EventSource, error) {
	return c.eventHub.Subscribe()
}

func (c *client) Healthy(logger lager.Logger) bool {
	c.healthyLock.RLock()
	defer c.healthyLock.RUnlock()
	return c.healthy
}

func (c *client) SetHealthy(logger lager.Logger, healthy bool) {
	c.healthyLock.Lock()
	defer c.healthyLock.Unlock()
	c.healthy = healthy
}
