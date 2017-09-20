package rep

import (
	"encoding/json"
	"errors"
	"net/url"
	"strconv"

	"code.cloudfoundry.org/bbs/models"
	"code.cloudfoundry.org/executor"
)

const (
	LifecycleTag  = "lifecycle"
	ResultFileTag = "result-file"
	DomainTag     = "domain"

	TaskLifecycle = "task"
	LRPLifecycle  = "lrp"

	ProcessGuidTag  = "process-guid"
	InstanceGuidTag = "instance-guid"
	ProcessIndexTag = "process-index"
)

var (
	ErrContainerMissingTags = errors.New("container is missing tags")
	ErrInvalidProcessIndex  = errors.New("container does not have a valid process index")
)

func ActualLRPKeyFromTags(tags executor.Tags) (*models.ActualLRPKey, error) {
	if tags == nil {
		return &models.ActualLRPKey{}, ErrContainerMissingTags
	}

	processIndex, err := strconv.Atoi(tags[ProcessIndexTag])
	if err != nil {
		return &models.ActualLRPKey{}, ErrInvalidProcessIndex
	}

	actualLRPKey := models.NewActualLRPKey(
		tags[ProcessGuidTag],
		int32(processIndex),
		tags[DomainTag],
	)

	err = actualLRPKey.Validate()
	if err != nil {
		return &models.ActualLRPKey{}, err
	}

	return &actualLRPKey, nil
}

func ActualLRPInstanceKeyFromContainer(container executor.Container, cellID string) (*models.ActualLRPInstanceKey, error) {
	if container.Tags == nil {
		return &models.ActualLRPInstanceKey{}, ErrContainerMissingTags
	}

	actualLRPInstanceKey := models.NewActualLRPInstanceKey(
		container.Tags[InstanceGuidTag],
		cellID,
	)

	err := actualLRPInstanceKey.Validate()
	if err != nil {
		return &models.ActualLRPInstanceKey{}, err
	}

	return &actualLRPInstanceKey, nil
}

func ActualLRPNetInfoFromContainer(container executor.Container) (*models.ActualLRPNetInfo, error) {
	ports := []*models.PortMapping{}
	for _, portMapping := range container.Ports {
		ports = append(ports, models.NewPortMapping(uint32(portMapping.HostPort), uint32(portMapping.ContainerPort)))
	}

	actualLRPNetInfo := models.NewActualLRPNetInfo(container.ExternalIP, ports...)

	err := actualLRPNetInfo.Validate()
	if err != nil {
		return nil, err
	}

	return &actualLRPNetInfo, nil
}

func LRPContainerGuid(processGuid, instanceGuid string) string {
	return instanceGuid
}

func NewRunRequestFromDesiredLRP(
	containerGuid string,
	desiredLRP *models.DesiredLRP,
	lrpKey *models.ActualLRPKey,
	lrpInstanceKey *models.ActualLRPInstanceKey,
) (executor.RunRequest, error) {
	diskScope, err := diskScopeForRootFS(desiredLRP.RootFs)
	if err != nil {
		return executor.RunRequest{}, err
	}

	mounts, err := convertVolumeMounts(desiredLRP.VolumeMounts)
	if err != nil {
		return executor.RunRequest{}, err
	}

	runInfo := executor.RunInfo{
		CPUWeight: uint(desiredLRP.CpuWeight),
		DiskScope: diskScope,
		Ports:     ConvertPortMappings(desiredLRP.Ports),
		LogConfig: executor.LogConfig{
			Guid:       desiredLRP.LogGuid,
			Index:      int(lrpKey.Index),
			SourceName: desiredLRP.LogSource,
		},

		MetricsConfig: executor.MetricsConfig{
			Guid:  desiredLRP.MetricsGuid,
			Index: int(lrpKey.Index),
		},
		StartTimeoutMs:     uint(desiredLRP.StartTimeoutMs),
		Privileged:         desiredLRP.Privileged,
		CachedDependencies: ConvertCachedDependencies(desiredLRP.CachedDependencies),
		Setup:              desiredLRP.Setup,
		Action:             desiredLRP.Action,
		Monitor:            desiredLRP.Monitor,
		EgressRules:        desiredLRP.EgressRules,
		Env: append([]executor.EnvironmentVariable{
			{Name: "INSTANCE_GUID", Value: lrpInstanceKey.InstanceGuid},
			{Name: "INSTANCE_INDEX", Value: strconv.Itoa(int(lrpKey.Index))},
			{Name: "CF_INSTANCE_GUID", Value: lrpInstanceKey.InstanceGuid},
			{Name: "CF_INSTANCE_INDEX", Value: strconv.Itoa(int(lrpKey.Index))},
		}, executor.EnvironmentVariablesFromModel(desiredLRP.EnvironmentVariables)...),
		TrustedSystemCertificatesPath: desiredLRP.TrustedSystemCertificatesPath,
		VolumeMounts:                  mounts,
		Network:                       convertNetwork(desiredLRP.Network),
	}
	tags := executor.Tags{}
	return executor.NewRunRequest(containerGuid, &runInfo, tags), nil
}

func NewRunRequestFromTask(task *models.Task) (executor.RunRequest, error) {
	diskScope, err := diskScopeForRootFS(task.RootFs)
	if err != nil {
		return executor.RunRequest{}, err
	}

	mounts, err := convertVolumeMounts(task.VolumeMounts)
	if err != nil {
		return executor.RunRequest{}, err
	}

	tags := executor.Tags{
		ResultFileTag: task.ResultFile,
	}
	runInfo := executor.RunInfo{
		DiskScope:  diskScope,
		CPUWeight:  uint(task.CpuWeight),
		Privileged: task.Privileged,
		LogConfig: executor.LogConfig{
			Guid:       task.LogGuid,
			SourceName: task.LogSource,
		},
		MetricsConfig: executor.MetricsConfig{
			Guid: task.MetricsGuid,
		},
		CachedDependencies:            ConvertCachedDependencies(task.CachedDependencies),
		Action:                        task.Action,
		Env:                           executor.EnvironmentVariablesFromModel(task.EnvironmentVariables),
		EgressRules:                   task.EgressRules,
		TrustedSystemCertificatesPath: task.TrustedSystemCertificatesPath,
		VolumeMounts:                  mounts,
		Network:                       convertNetwork(task.Network),
	}
	return executor.NewRunRequest(task.TaskGuid, &runInfo, tags), nil
}

func ConvertCachedDependencies(modelDeps []*models.CachedDependency) []executor.CachedDependency {
	execDeps := make([]executor.CachedDependency, len(modelDeps))
	for i := range modelDeps {
		execDeps[i] = ConvertCachedDependency(modelDeps[i])
	}
	return execDeps
}

func ConvertCachedDependency(modelDep *models.CachedDependency) executor.CachedDependency {
	return executor.CachedDependency{
		Name:              modelDep.Name,
		From:              modelDep.From,
		To:                modelDep.To,
		CacheKey:          modelDep.CacheKey,
		LogSource:         modelDep.LogSource,
		ChecksumValue:     modelDep.ChecksumValue,
		ChecksumAlgorithm: modelDep.ChecksumAlgorithm,
	}
}

func convertVolumeMounts(volumeMounts []*models.VolumeMount) ([]executor.VolumeMount, error) {
	execMnts := make([]executor.VolumeMount, len(volumeMounts))
	for i := range volumeMounts {
		var err error
		execMnts[i], err = convertVolumeMount(volumeMounts[i])
		if err != nil {
			return nil, err
		}
	}
	return execMnts, nil
}

func convertVolumeMount(volumeMnt *models.VolumeMount) (executor.VolumeMount, error) {
	var config map[string]interface{}

	if len(volumeMnt.Shared.MountConfig) > 0 {
		err := json.Unmarshal([]byte(volumeMnt.Shared.MountConfig), &config)
		if err != nil {
			return executor.VolumeMount{}, err
		}
	}

	var mode executor.BindMountMode
	switch volumeMnt.Mode {
	case "r":
		mode = executor.BindMountModeRO
	case "rw":
		mode = executor.BindMountModeRW
	default:
		return executor.VolumeMount{}, errors.New("unrecognized volume mount mode")
	}

	return executor.VolumeMount{
		Driver:        volumeMnt.Driver,
		VolumeId:      volumeMnt.Shared.VolumeId,
		ContainerPath: volumeMnt.ContainerDir,
		Mode:          mode,
		Config:        config,
	}, nil
}

func convertNetwork(network *models.Network) *executor.Network {
	if network == nil {
		return nil
	}

	return &executor.Network{
		Properties: network.Properties,
	}
}

func ConvertPortMappings(containerPorts []uint32) []executor.PortMapping {
	out := []executor.PortMapping{}
	for _, port := range containerPorts {
		out = append(out, executor.PortMapping{
			ContainerPort: uint16(port),
		})
	}

	return out
}

func diskScopeForRootFS(rootFS string) (executor.DiskLimitScope, error) {
	preloaded := false

	url, err := url.Parse(rootFS)
	if err != nil {
		return executor.TotalDiskLimit, err
	}

	if url.Scheme == models.PreloadedRootFSScheme {
		preloaded = true
	}

	if preloaded {
		return executor.ExclusiveDiskLimit, nil
	}
	return executor.TotalDiskLimit, nil
}
