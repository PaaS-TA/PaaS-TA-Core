package containerstore

import (
	"errors"
	"fmt"
	"io"
	"os"
	"sync"
	"time"

	"code.cloudfoundry.org/executor"
	"code.cloudfoundry.org/executor/depot/event"
	"code.cloudfoundry.org/executor/depot/transformer"
	"code.cloudfoundry.org/garden"
	"code.cloudfoundry.org/garden/server"
	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/runtimeschema/metric"
	"code.cloudfoundry.org/volman"
	"github.com/tedsuo/ifrit"
)

const DownloadCachedDependenciesFailed = "failed to download cached artifacts"
const ContainerInitializationFailedMessage = "failed to initialize container"
const ContainerExpirationMessage = "expired container"
const ContainerMissingMessage = "missing garden container"
const VolmanMountFailed = "failed to mount volume"
const BindMountCleanupFailed = "failed to cleanup bindmount artifacts"

// To be deprecated
const GardenContainerCreationDuration = metric.Duration("GardenContainerCreationDuration")

const GardenContainerCreationSucceededDuration = metric.Duration("GardenContainerCreationSucceededDuration")
const GardenContainerCreationFailedDuration = metric.Duration("GardenContainerCreationFailedDuration")

const GardenContainerDestructionSucceededDuration = metric.Duration("GardenContainerDestructionSucceededDuration")
const GardenContainerDestructionFailedDuration = metric.Duration("GardenContainerDestructionFailedDuration")

type storeNode struct {
	modifiedIndex               uint
	hostTrustedCertificatesPath string

	// infoLock protects modifying info and swapping gardenContainer pointers
	infoLock           *sync.Mutex
	info               executor.Container
	bindMountCacheKeys []BindMountCacheKey
	gardenContainer    garden.Container

	// opLock serializes public methods that involve garden interactions
	opLock            *sync.Mutex
	gardenClient      garden.Client
	dependencyManager DependencyManager
	volumeManager     volman.Manager
	eventEmitter      event.Hub
	transformer       transformer.Transformer
	process           ifrit.Process
	config            *ContainerConfig
}

func newStoreNode(
	config *ContainerConfig,
	container executor.Container,
	gardenClient garden.Client,
	dependencyManager DependencyManager,
	volumeManager volman.Manager,
	eventEmitter event.Hub,
	transformer transformer.Transformer,
	hostTrustedCertificatesPath string,
) *storeNode {
	return &storeNode{
		config:                      config,
		info:                        container,
		infoLock:                    &sync.Mutex{},
		opLock:                      &sync.Mutex{},
		gardenClient:                gardenClient,
		dependencyManager:           dependencyManager,
		volumeManager:               volumeManager,
		eventEmitter:                eventEmitter,
		transformer:                 transformer,
		modifiedIndex:               0,
		hostTrustedCertificatesPath: hostTrustedCertificatesPath,
	}
}

func (n *storeNode) acquireOpLock(logger lager.Logger) {
	startTime := time.Now()
	n.opLock.Lock()
	logger.Debug("ops-lock-aquired", lager.Data{"lock-wait-time": time.Now().Sub(startTime).String()})
}

func (n *storeNode) releaseOpLock(logger lager.Logger) {
	n.opLock.Unlock()
	logger.Debug("ops-lock-released")
}

func (n *storeNode) Info() executor.Container {
	n.infoLock.Lock()
	defer n.infoLock.Unlock()

	return n.info.Copy()
}

func (n *storeNode) GetFiles(logger lager.Logger, sourcePath string) (io.ReadCloser, error) {
	n.infoLock.Lock()
	gc := n.gardenContainer
	n.infoLock.Unlock()
	if gc == nil {
		return nil, executor.ErrContainerNotFound
	}
	return gc.StreamOut(garden.StreamOutSpec{Path: sourcePath, User: "root"})
}

func (n *storeNode) Initialize(logger lager.Logger, req *executor.RunRequest) error {
	logger = logger.Session("node-initialize")
	n.infoLock.Lock()
	defer n.infoLock.Unlock()

	err := n.info.TransistionToInitialize(req)
	if err != nil {
		logger.Error("failed-to-initialize", err)
		return err
	}
	return nil
}

func (n *storeNode) Create(logger lager.Logger) error {
	logger = logger.Session("node-create")
	n.acquireOpLock(logger)
	defer n.releaseOpLock(logger)

	n.infoLock.Lock()
	info := n.info.Copy()
	n.infoLock.Unlock()

	if !info.ValidateTransitionTo(executor.StateCreated) {
		logger.Error("failed-to-create", executor.ErrInvalidTransition)
		return executor.ErrInvalidTransition
	}

	logStreamer := logStreamerFromLogConfig(info.LogConfig)

	mounts, err := n.dependencyManager.DownloadCachedDependencies(logger, info.CachedDependencies, logStreamer)
	if err != nil {
		n.complete(logger, true, DownloadCachedDependenciesFailed)
		return err
	}

	if n.hostTrustedCertificatesPath != "" && info.TrustedSystemCertificatesPath != "" {
		mount := garden.BindMount{
			SrcPath: n.hostTrustedCertificatesPath,
			DstPath: info.TrustedSystemCertificatesPath,
			Mode:    garden.BindMountModeRO,
			Origin:  garden.BindMountOriginHost,
		}
		mounts.GardenBindMounts = append(mounts.GardenBindMounts, mount)
	}

	volumeMounts, err := n.mountVolumes(logger, info)
	if err != nil {
		logger.Error("failed-to-mount-volume", err)
		n.complete(logger, true, VolmanMountFailed)
		return err
	}
	mounts.GardenBindMounts = append(mounts.GardenBindMounts, volumeMounts...)

	fmt.Fprintf(logStreamer.Stdout(), "Creating container\n")
	gardenContainer, err := n.createGardenContainer(logger, &info, mounts.GardenBindMounts)
	if err != nil {
		logger.Error("failed-to-create-container", err)
		fmt.Fprintf(logStreamer.Stderr(), "Failed to create container\n")
		n.complete(logger, true, ContainerInitializationFailedMessage)
		return err
	}
	fmt.Fprintf(logStreamer.Stdout(), "Successfully created container\n")

	n.infoLock.Lock()
	n.gardenContainer = gardenContainer
	n.info = info
	n.bindMountCacheKeys = mounts.CacheKeys
	n.infoLock.Unlock()

	return nil
}

func (n *storeNode) mountVolumes(logger lager.Logger, info executor.Container) ([]garden.BindMount, error) {
	gardenMounts := []garden.BindMount{}
	for _, volume := range info.VolumeMounts {
		hostMount, err := n.volumeManager.Mount(logger, volume.Driver, volume.VolumeId, volume.Config)
		if err != nil {
			return nil, err
		}
		gardenMounts = append(gardenMounts,
			garden.BindMount{
				SrcPath: hostMount.Path,
				DstPath: volume.ContainerPath,
				Origin:  garden.BindMountOriginHost,
				Mode:    garden.BindMountMode(volume.Mode),
			})
	}
	return gardenMounts, nil
}

func (n *storeNode) gardenProperties(container *executor.Container) garden.Properties {
	properties := garden.Properties{}
	if container.Network != nil {
		for key, value := range container.Network.Properties {
			properties["network."+key] = value
		}
	}
	properties[ContainerOwnerProperty] = n.config.OwnerName

	return properties
}

func (n *storeNode) createGardenContainer(logger lager.Logger, info *executor.Container, mounts []garden.BindMount) (garden.Container, error) {
	containerSpec := garden.ContainerSpec{
		Handle:     info.Guid,
		Privileged: info.Privileged,
		RootFSPath: info.RootFSPath,
		Env:        convertEnvVars(info.Env),
		BindMounts: mounts,
		Limits: garden.Limits{
			Memory: garden.MemoryLimits{
				LimitInBytes: uint64(info.MemoryMB * 1024 * 1024),
			},
			Disk: garden.DiskLimits{
				ByteHard:  uint64(info.DiskMB * 1024 * 1024),
				InodeHard: n.config.INodeLimit,
				Scope:     convertDiskScope(info.DiskScope),
			},
			CPU: garden.CPULimits{
				LimitInShares: uint64(float64(n.config.MaxCPUShares) * float64(info.CPUWeight) / 100.0),
			},
		},
		Properties: n.gardenProperties(info),
	}

	netOutRules, err := convertEgressToNetOut(logger, info.EgressRules)
	if err != nil {
		return nil, err
	}

	gardenContainer, err := createContainer(logger, containerSpec, n.gardenClient)
	if err != nil {
		return nil, err
	}

	err = setupNetOutOnContainer(logger, netOutRules, gardenContainer)
	if err != nil {
		n.destroyContainer(logger)
		return nil, err
	}

	info.Ports, err = setupNetInOnContainer(logger, info.Ports, gardenContainer)
	if err != nil {
		n.destroyContainer(logger)
		return nil, err
	}

	externalIP, err := fetchExternalIp(logger, gardenContainer)
	if err != nil {
		n.destroyContainer(logger)
		return nil, err
	}
	info.ExternalIP = externalIP

	err = info.TransistionToCreate()
	if err != nil {
		return nil, err
	}

	info.MemoryLimit = containerSpec.Limits.Memory.LimitInBytes
	info.DiskLimit = containerSpec.Limits.Disk.ByteHard

	return gardenContainer, nil
}

func (n *storeNode) Run(logger lager.Logger) error {
	logger = logger.Session("node-run")

	n.acquireOpLock(logger)
	defer n.releaseOpLock(logger)

	if n.info.State != executor.StateCreated {
		logger.Error("failed-to-run", executor.ErrInvalidTransition)
		return executor.ErrInvalidTransition
	}

	logStreamer := logStreamerFromLogConfig(n.info.LogConfig)

	runner, err := n.transformer.StepsRunner(logger, n.info, n.gardenContainer, logStreamer)
	if err != nil {
		return err
	}

	n.process = ifrit.Background(runner)
	go n.run(logger)
	return nil
}

func (n *storeNode) run(logger lager.Logger) {
	logger.Debug("execute-process")
	<-n.process.Ready()
	logger.Debug("healthcheck-passed")

	n.infoLock.Lock()
	n.info.State = executor.StateRunning
	info := n.info.Copy()
	n.infoLock.Unlock()
	go n.eventEmitter.Emit(executor.NewContainerRunningEvent(info))

	err := <-n.process.Wait()
	if err != nil {
		n.complete(logger, true, err.Error())
	} else {
		n.complete(logger, false, "")
	}
}

func (n *storeNode) Stop(logger lager.Logger) error {
	logger = logger.Session("node-stop")
	n.acquireOpLock(logger)
	defer n.releaseOpLock(logger)

	return n.stop(logger)
}

func (n *storeNode) stop(logger lager.Logger) error {
	n.infoLock.Lock()
	n.info.RunResult.Stopped = true
	n.infoLock.Unlock()

	if n.process != nil {
		n.process.Signal(os.Interrupt)
		logger.Debug("signaled-process")
	} else {
		n.complete(logger, true, "stopped-before-running")
	}
	return nil
}

func (n *storeNode) Destroy(logger lager.Logger) error {
	logger = logger.Session("node-destroy")
	n.acquireOpLock(logger)
	defer n.releaseOpLock(logger)

	err := n.stop(logger)
	if err != nil {
		return err
	}

	if n.process != nil {
		<-n.process.Wait()
	}

	logStreamer := logStreamerFromLogConfig(n.info.LogConfig)

	fmt.Fprintf(logStreamer.Stdout(), "Destroying container\n")
	err = n.destroyContainer(logger)
	if err != nil {
		fmt.Fprintf(logStreamer.Stderr(), "Failed to destroy container\n")
		return err
	}
	fmt.Fprintf(logStreamer.Stdout(), "Successfully destroyed container\n")

	n.infoLock.Lock()
	cacheKeys := n.bindMountCacheKeys
	n.infoLock.Unlock()

	var bindMountCleanupErr error
	err = n.dependencyManager.ReleaseCachedDependencies(logger, cacheKeys)
	if err != nil {
		logger.Error("failed-to-release-cached-deps", err)
		bindMountCleanupErr = errors.New(BindMountCleanupFailed)
	}

	for _, volume := range n.info.VolumeMounts {
		err = n.volumeManager.Unmount(logger, volume.Driver, volume.VolumeId)
		if err != nil {
			logger.Error("failed-to-unmount-volume", err)
			bindMountCleanupErr = errors.New(BindMountCleanupFailed)
		}
	}

	return bindMountCleanupErr
}

func (n *storeNode) destroyContainer(logger lager.Logger) error {
	logger.Debug("destroying-garden-container")

	startTime := time.Now()
	err := n.gardenClient.Destroy(n.info.Guid)
	destroyDuration := time.Now().Sub(startTime)

	if err != nil {
		if _, ok := err.(garden.ContainerNotFoundError); ok {
			logger.Error("container-not-found-in-garden", err)
		} else if err.Error() == server.ErrConcurrentDestroy.Error() {
			logger.Error("container-destroy-in-progress", err)
		} else {
			logger.Error("failed-to-destroy-container-in-garden", err)
			logger.Info("failed-to-destroy-container-in-garden", lager.Data{
				"destroy-took": destroyDuration.String(),
			})
			sendMetricDuration(logger, GardenContainerDestructionFailedDuration, destroyDuration)
			return err
		}
	}

	logger.Info("destroyed-container-in-garden", lager.Data{
		"destroy-took": destroyDuration.String(),
	})
	sendMetricDuration(logger, GardenContainerDestructionSucceededDuration, destroyDuration)
	return nil
}

func (n *storeNode) Expire(logger lager.Logger, now time.Time) bool {
	n.infoLock.Lock()
	defer n.infoLock.Unlock()

	if n.info.State != executor.StateReserved {
		return false
	}

	lifespan := now.Sub(time.Unix(0, n.info.AllocatedAt))
	if lifespan >= n.config.ReservedExpirationTime {
		n.info.TransitionToComplete(true, ContainerExpirationMessage)
		go n.eventEmitter.Emit(executor.NewContainerCompleteEvent(n.info))
		return true
	}

	return false
}

func (n *storeNode) Reap(logger lager.Logger) bool {
	n.infoLock.Lock()
	defer n.infoLock.Unlock()

	if n.info.IsCreated() {
		n.info.TransitionToComplete(true, ContainerMissingMessage)
		go n.eventEmitter.Emit(executor.NewContainerCompleteEvent(n.info))
		return true
	}

	return false
}

func (n *storeNode) complete(logger lager.Logger, failed bool, failureReason string) {
	logger.Debug("node-complete", lager.Data{"failed": failed, "reason": failureReason})
	n.infoLock.Lock()
	defer n.infoLock.Unlock()
	n.info.TransitionToComplete(failed, failureReason)

	go n.eventEmitter.Emit(executor.NewContainerCompleteEvent(n.info))
}

func setupNetInOnContainer(logger lager.Logger, ports []executor.PortMapping, gardenContainer garden.Container) ([]executor.PortMapping, error) {
	actualPortMappings := make([]executor.PortMapping, len(ports))
	for i, portMapping := range ports {
		logger.Debug("net-in")
		actualHost, actualContainerPort, err := gardenContainer.NetIn(uint32(portMapping.HostPort), uint32(portMapping.ContainerPort))
		if err != nil {
			logger.Error("net-in-failed", err)
			return nil, err
		}
		logger.Debug("net-in-complete")
		actualPortMappings[i].ContainerPort = uint16(actualContainerPort)
		actualPortMappings[i].HostPort = uint16(actualHost)
	}
	return actualPortMappings, nil
}

func setupNetOutOnContainer(logger lager.Logger, netOutRules []garden.NetOutRule, gardenContainer garden.Container) error {
	logger.Debug("net-out")
	err := gardenContainer.BulkNetOut(netOutRules)
	if err != nil {
		logger.Error("net-out-failed", err)
		return err
	}
	logger.Debug("net-out-complete")
	return nil
}

func sendMetricDuration(logger lager.Logger, metric metric.Duration, value time.Duration) {
	err := metric.Send(value)
	if err != nil {
		switch metric {
		case GardenContainerCreationDuration:
			logger.Error("failed-to-send-garden-container-creation-duration-metric", err)
		case GardenContainerCreationSucceededDuration:
			logger.Error("failed-to-send-garden-container-creation-succeeded-duration-metric", err)
		case GardenContainerCreationFailedDuration:
			logger.Error("failed-to-send-garden-container-creation-failed-duration-metric", err)
		case GardenContainerDestructionSucceededDuration:
			logger.Error("failed-to-send-garden-container-destruction-succeeded-duration-metric", err)
		case GardenContainerDestructionFailedDuration:
			logger.Error("failed-to-send-garden-container-destruction-failed-duration-metric", err)
		default:
			logger.Error("failed-to-send-metric", err)
		}
	}
}

func createContainer(logger lager.Logger, spec garden.ContainerSpec, client garden.Client) (garden.Container, error) {
	logger.Info("creating-container-in-garden")
	startTime := time.Now()
	container, err := client.Create(spec)
	createDuration := time.Now().Sub(startTime)
	if err != nil {
		logger.Error("failed-to-create-container-in-garden", err)
		logger.Info("failed-to-create-container-in-garden", lager.Data{
			"create-took": createDuration.String(),
		})
		sendMetricDuration(logger, GardenContainerCreationFailedDuration, createDuration)
		return nil, err
	}
	logger.Info("created-container-in-garden", lager.Data{"create-took": createDuration.String()})
	sendMetricDuration(logger, GardenContainerCreationDuration, createDuration)
	sendMetricDuration(logger, GardenContainerCreationSucceededDuration, createDuration)
	return container, nil
}

func fetchExternalIp(logger lager.Logger, gardenContainer garden.Container) (string, error) {
	logger.Debug("container-info")
	gardenInfo, err := gardenContainer.Info()
	if err != nil {
		logger.Error("failed-container-info", err)
		return "", err
	}
	logger.Debug("container-info-complete")

	return gardenInfo.ExternalIP, nil
}
