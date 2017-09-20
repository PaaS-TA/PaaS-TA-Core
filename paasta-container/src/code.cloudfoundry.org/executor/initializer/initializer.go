package initializer

import (
	"errors"
	"math"
	"os"
	"path/filepath"
	"time"

	"code.cloudfoundry.org/archiver/compressor"
	"code.cloudfoundry.org/archiver/extractor"
	"code.cloudfoundry.org/cacheddownloader"
	"code.cloudfoundry.org/clock"
	"code.cloudfoundry.org/executor"
	"code.cloudfoundry.org/executor/containermetrics"
	"code.cloudfoundry.org/executor/depot"
	"code.cloudfoundry.org/executor/depot/containerstore"
	"code.cloudfoundry.org/executor/depot/event"
	"code.cloudfoundry.org/executor/depot/metrics"
	"code.cloudfoundry.org/executor/depot/transformer"
	"code.cloudfoundry.org/executor/depot/uploader"
	"code.cloudfoundry.org/executor/gardenhealth"
	"code.cloudfoundry.org/executor/guidgen"
	"code.cloudfoundry.org/executor/initializer/configuration"
	"code.cloudfoundry.org/garden"
	GardenClient "code.cloudfoundry.org/garden/client"
	GardenConnection "code.cloudfoundry.org/garden/client/connection"
	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/runtimeschema/metric"
	"code.cloudfoundry.org/volman/vollocal"
	"code.cloudfoundry.org/workpool"
	"github.com/cloudfoundry/systemcerts"
	"github.com/google/shlex"
	"github.com/tedsuo/ifrit"
	"github.com/tedsuo/ifrit/grouper"
)

const (
	PingGardenInterval             = time.Second
	StalledMetricHeartbeatInterval = 5 * time.Second
	stalledDuration                = metric.Duration("StalledGardenDuration")
	maxConcurrentUploads           = 5
	metricsReportInterval          = 1 * time.Minute
)

type executorContainers struct {
	gardenClient garden.Client
	owner        string
}

func (containers *executorContainers) Containers() ([]garden.Container, error) {
	return containers.gardenClient.Containers(garden.Properties{
		containerstore.ContainerOwnerProperty: containers.owner,
	})
}

type Configuration struct {
	GardenNetwork string
	GardenAddr    string

	ContainerOwnerName            string
	HealthCheckContainerOwnerName string

	TempDir              string
	CachePath            string
	MaxCacheSizeInBytes  uint64
	SkipCertVerify       bool
	CACertsForDownloads  []byte
	ExportNetworkEnvVars bool

	VolmanDriverPaths []string

	ContainerMaxCpuShares       uint64
	ContainerInodeLimit         uint64
	HealthyMonitoringInterval   time.Duration
	UnhealthyMonitoringInterval time.Duration
	HealthCheckWorkPoolSize     int

	MaxConcurrentDownloads int

	CreateWorkPoolSize  int
	DeleteWorkPoolSize  int
	ReadWorkPoolSize    int
	MetricsWorkPoolSize int

	ReservedExpirationTime time.Duration
	ContainerReapInterval  time.Duration

	GardenHealthcheckRootFS            string
	GardenHealthcheckInterval          time.Duration
	GardenHealthcheckEmissionInterval  time.Duration
	GardenHealthcheckTimeout           time.Duration
	GardenHealthcheckCommandRetryPause time.Duration

	GardenHealthcheckProcessPath string
	GardenHealthcheckProcessArgs []string
	GardenHealthcheckProcessUser string
	GardenHealthcheckProcessEnv  []string
	GardenHealthcheckProcessDir  string

	MemoryMB string
	DiskMB   string

	PostSetupHook string
	PostSetupUser string

	TrustedSystemCertificatesPath  string
	ContainerMetricsReportInterval time.Duration
}

const (
	defaultMaxConcurrentDownloads  = 5
	defaultCreateWorkPoolSize      = 32
	defaultDeleteWorkPoolSize      = 32
	defaultReadWorkPoolSize        = 64
	defaultMetricsWorkPoolSize     = 8
	defaultHealthCheckWorkPoolSize = 64
)

var DefaultConfiguration = Configuration{
	GardenNetwork:                      "unix",
	GardenAddr:                         "/tmp/garden.sock",
	MemoryMB:                           configuration.Automatic,
	DiskMB:                             configuration.Automatic,
	TempDir:                            "/tmp",
	ReservedExpirationTime:             time.Minute,
	ContainerReapInterval:              time.Minute,
	ContainerInodeLimit:                200000,
	ContainerMaxCpuShares:              0,
	CachePath:                          "/tmp/cache",
	MaxCacheSizeInBytes:                10 * 1024 * 1024 * 1024,
	SkipCertVerify:                     false,
	HealthyMonitoringInterval:          30 * time.Second,
	UnhealthyMonitoringInterval:        500 * time.Millisecond,
	ExportNetworkEnvVars:               false,
	ContainerOwnerName:                 "executor",
	HealthCheckContainerOwnerName:      "executor-health-check",
	CreateWorkPoolSize:                 defaultCreateWorkPoolSize,
	DeleteWorkPoolSize:                 defaultDeleteWorkPoolSize,
	ReadWorkPoolSize:                   defaultReadWorkPoolSize,
	MetricsWorkPoolSize:                defaultMetricsWorkPoolSize,
	HealthCheckWorkPoolSize:            defaultHealthCheckWorkPoolSize,
	MaxConcurrentDownloads:             defaultMaxConcurrentDownloads,
	GardenHealthcheckInterval:          10 * time.Minute,
	GardenHealthcheckEmissionInterval:  30 * time.Second,
	GardenHealthcheckTimeout:           10 * time.Minute,
	GardenHealthcheckCommandRetryPause: time.Second,
	GardenHealthcheckProcessArgs:       []string{},
	GardenHealthcheckProcessEnv:        []string{},
	ContainerMetricsReportInterval:     15 * time.Second,
}

//=====================================
//Add - parameter GardenClient.Client
//=====================================
func Initialize(logger lager.Logger, config Configuration, clock clock.Clock) (executor.Client, grouper.Members, GardenClient.Client, error) {
	postSetupHook, err := shlex.Split(config.PostSetupHook)
	if err != nil {
		logger.Error("failed-to-parse-post-setup-hook", err)
		return nil, grouper.Members{}, nil, err			//original : nil, grouper.Members{}, err
	}

	gardenClient := GardenClient.New(GardenConnection.New(config.GardenNetwork, config.GardenAddr))
	err = waitForGarden(logger, gardenClient, clock)
	if err != nil {
		return nil, nil, nil, err				//original : nil, nil, err
	}

	containersFetcher := &executorContainers{
		gardenClient: gardenClient,
		owner:        config.ContainerOwnerName,
	}

	destroyContainers(gardenClient, containersFetcher, logger)

	workDir := setupWorkDir(logger, config.TempDir)

	healthCheckWorkPool, err := workpool.NewWorkPool(config.HealthCheckWorkPoolSize)
	if err != nil {
		return nil, grouper.Members{}, nil, err			//original : nil, grouper.Members{], err
	}

	caCertPool := systemcerts.SystemRootsPool()
	if caCertPool == nil {
		caCertPool = systemcerts.NewCertPool()
	}

	if len(config.CACertsForDownloads) > 0 {
		if ok := caCertPool.AppendCertsFromPEM(config.CACertsForDownloads); !ok {
			return nil, grouper.Members{}, nil, errors.New("unable to load CA certificate")		//original : nil, grouper.Members{}, errors.New("unable to load CA certificate")
		}
	}

	cache := cacheddownloader.New(
		config.CachePath,
		workDir,
		int64(config.MaxCacheSizeInBytes),
		10*time.Minute,
		int(math.MaxInt8),
		config.SkipCertVerify,
		caCertPool,
		cacheddownloader.TarTransform,
	)

	err = cache.RecoverState()
	if err != nil {
		return nil, grouper.Members{}, nil, err			//original : nil, grouper.Members{}, err
	}

	downloadRateLimiter := make(chan struct{}, uint(config.MaxConcurrentDownloads))

	transformer := initializeTransformer(
		logger,
		cache,
		workDir,
		downloadRateLimiter,
		maxConcurrentUploads,
		config.SkipCertVerify,
		config.ExportNetworkEnvVars,
		config.HealthyMonitoringInterval,
		config.UnhealthyMonitoringInterval,
		healthCheckWorkPool,
		clock,
		postSetupHook,
		config.PostSetupUser,
	)

	hub := event.NewHub()

	totalCapacity := fetchCapacity(logger, gardenClient, config)

	containerConfig := containerstore.ContainerConfig{
		OwnerName:              config.ContainerOwnerName,
		INodeLimit:             config.ContainerInodeLimit,
		MaxCPUShares:           config.ContainerMaxCpuShares,
		ReservedExpirationTime: config.ReservedExpirationTime,
		ReapInterval:           config.ContainerReapInterval,
	}

	driverConfig := vollocal.NewDriverConfig()
	driverConfig.DriverPaths = config.VolmanDriverPaths
	volmanClient, volmanDriverSyncer := vollocal.NewServer(logger, driverConfig)

	containerStore := containerstore.New(
		containerConfig,
		&totalCapacity,
		gardenClient,
		containerstore.NewDependencyManager(cache, downloadRateLimiter),
		volmanClient,
		clock,
		hub,
		transformer,
		config.TrustedSystemCertificatesPath,
	)

	workPoolSettings := executor.WorkPoolSettings{
		CreateWorkPoolSize:  config.CreateWorkPoolSize,
		DeleteWorkPoolSize:  config.DeleteWorkPoolSize,
		ReadWorkPoolSize:    config.ReadWorkPoolSize,
		MetricsWorkPoolSize: config.MetricsWorkPoolSize,
	}

	depotClient := depot.NewClient(
		totalCapacity,
		containerStore,
		gardenClient,
		volmanClient,
		hub,
		workPoolSettings,
	)

	healthcheckSpec := garden.ProcessSpec{
		Path: config.GardenHealthcheckProcessPath,
		Args: config.GardenHealthcheckProcessArgs,
		User: config.GardenHealthcheckProcessUser,
		Env:  config.GardenHealthcheckProcessEnv,
		Dir:  config.GardenHealthcheckProcessDir,
	}

	gardenHealthcheck := gardenhealth.NewChecker(
		config.GardenHealthcheckRootFS,
		config.HealthCheckContainerOwnerName,
		config.GardenHealthcheckCommandRetryPause,
		healthcheckSpec,
		gardenClient,
		guidgen.DefaultGenerator,
	)

	return depotClient,
		grouper.Members{
			{"volman-driver-syncer", volmanDriverSyncer},
			{"metrics-reporter", &metrics.Reporter{
				ExecutorSource: depotClient,
				Interval:       metricsReportInterval,
				Clock:          clock,
				Logger:         logger,
			}},
			{"hub-closer", closeHub(hub)},
			{"container-metrics-reporter", containermetrics.NewStatsReporter(
				logger,
				config.ContainerMetricsReportInterval,
				clock,
				depotClient,
			)},
			{"garden_health_checker", gardenhealth.NewRunner(
				config.GardenHealthcheckInterval,
				config.GardenHealthcheckEmissionInterval,
				config.GardenHealthcheckTimeout,
				logger,
				gardenHealthcheck,
				depotClient,
				clock,
			)},
			{"registry-pruner", containerStore.NewRegistryPruner(logger)},
			{"container-reaper", containerStore.NewContainerReaper(logger)},
		},
		gardenClient,	//<-- Added
		nil
}

// Until we get a successful response from garden,
// periodically emit metrics saying how long we've been trying
// while retrying the connection indefinitely.
func waitForGarden(logger lager.Logger, gardenClient GardenClient.Client, clock clock.Clock) error {
	pingStart := clock.Now()
	logger = logger.Session("wait-for-garden", lager.Data{"initialTime:": pingStart})
	pingRequest := clock.NewTimer(0)
	pingResponse := make(chan error)
	heartbeatTimer := clock.NewTimer(StalledMetricHeartbeatInterval)

	for {
		select {
		case <-pingRequest.C():
			go func() {
				logger.Info("ping-garden", lager.Data{"wait-time-ns:": clock.Since(pingStart)})
				pingResponse <- gardenClient.Ping()
			}()

		case err := <-pingResponse:
			switch err.(type) {
			case nil:
				logger.Info("ping-garden-success", lager.Data{"wait-time-ns:": clock.Since(pingStart)})
				// send 0 to indicate ping responded successfully
				sendError := stalledDuration.Send(0)
				if sendError != nil {
					logger.Error("failed-to-send-stalled-duration-metric", sendError)
				}
				return nil
			case garden.UnrecoverableError:
				logger.Error("failed-to-ping-garden-with-unrecoverable-error", err)
				return err
			default:
				logger.Error("failed-to-ping-garden", err)
				pingRequest.Reset(PingGardenInterval)
			}

		case <-heartbeatTimer.C():
			logger.Info("emitting-stalled-garden-heartbeat", lager.Data{"wait-time-ns:": clock.Since(pingStart)})
			sendError := stalledDuration.Send(clock.Since(pingStart))
			if sendError != nil {
				logger.Error("failed-to-send-stalled-duration-heartbeat-metric", sendError)
			}

			heartbeatTimer.Reset(StalledMetricHeartbeatInterval)
		}
	}
}

func fetchCapacity(logger lager.Logger, gardenClient GardenClient.Client, config Configuration) executor.ExecutorResources {
	capacity, err := configuration.ConfigureCapacity(gardenClient, config.MemoryMB, config.DiskMB)
	if err != nil {
		logger.Error("failed-to-configure-capacity", err)
		os.Exit(1)
	}

	logger.Info("initial-capacity", lager.Data{
		"capacity": capacity,
	})

	return capacity
}

func destroyContainers(gardenClient garden.Client, containersFetcher *executorContainers, logger lager.Logger) {
	logger.Info("executor-fetching-containers-to-destroy")
	containers, err := containersFetcher.Containers()
	if err != nil {
		logger.Fatal("executor-failed-to-get-containers", err)
		return
	} else {
		logger.Info("executor-fetched-containers-to-destroy", lager.Data{"num-containers": len(containers)})
	}

	for _, container := range containers {
		logger.Info("executor-destroying-container", lager.Data{"container-handle": container.Handle()})
		err := gardenClient.Destroy(container.Handle())
		if err != nil {
			logger.Fatal("executor-failed-to-destroy-container", err, lager.Data{
				"handle": container.Handle(),
			})
		} else {
			logger.Info("executor-destroyed-stray-container", lager.Data{
				"handle": container.Handle(),
			})
		}
	}
}

func setupWorkDir(logger lager.Logger, tempDir string) string {
	workDir := filepath.Join(tempDir, "executor-work")

	err := os.RemoveAll(workDir)
	if err != nil {
		logger.Error("working-dir.cleanup-failed", err)
		os.Exit(1)
	}

	err = os.MkdirAll(workDir, 0755)
	if err != nil {
		logger.Error("working-dir.create-failed", err)
		os.Exit(1)
	}

	return workDir
}

func initializeTransformer(
	logger lager.Logger,
	cache cacheddownloader.CachedDownloader,
	workDir string,
	downloadRateLimiter chan struct{},
	maxConcurrentUploads uint,
	skipSSLVerification bool,
	exportNetworkEnvVars bool,
	healthyMonitoringInterval time.Duration,
	unhealthyMonitoringInterval time.Duration,
	healthCheckWorkPool *workpool.WorkPool,
	clock clock.Clock,
	postSetupHook []string,
	postSetupUser string,
) transformer.Transformer {
	uploader := uploader.New(10*time.Minute, skipSSLVerification, logger)
	extractor := extractor.NewDetectable()
	compressor := compressor.NewTgz()

	return transformer.NewTransformer(
		cache,
		uploader,
		extractor,
		compressor,
		downloadRateLimiter,
		make(chan struct{}, maxConcurrentUploads),
		workDir,
		exportNetworkEnvVars,
		healthyMonitoringInterval,
		unhealthyMonitoringInterval,
		healthCheckWorkPool,
		clock,
		postSetupHook,
		postSetupUser,
	)
}

func closeHub(hub event.Hub) ifrit.Runner {
	return ifrit.RunFunc(func(signals <-chan os.Signal, ready chan<- struct{}) error {
		close(ready)
		<-signals
		hub.Close()
		return nil
	})
}

func (config *Configuration) Validate(logger lager.Logger) bool {
	valid := true

	if config.ContainerMaxCpuShares == 0 {
		logger.Error("max-cpu-shares-invalid", nil)
		valid = false
	}

	if config.HealthyMonitoringInterval <= 0 {
		logger.Error("healthy-monitoring-interval-invalid", nil)
		valid = false
	}

	if config.UnhealthyMonitoringInterval <= 0 {
		logger.Error("unhealthy-monitoring-interval-invalid", nil)
		valid = false
	}

	if config.GardenHealthcheckInterval <= 0 {
		logger.Error("garden-healthcheck-interval-invalid", nil)
		valid = false
	}

	if config.GardenHealthcheckProcessUser == "" {
		logger.Error("garden-healthcheck-process-user-invalid", nil)
		valid = false
	}

	if config.GardenHealthcheckProcessPath == "" {
		logger.Error("garden-healthcheck-process-path-invalid", nil)
		valid = false
	}

	if config.PostSetupHook != "" && config.PostSetupUser == "" {
		logger.Error("post-setup-hook-requires-a-user", nil)
		valid = false
	}

	return valid
}
