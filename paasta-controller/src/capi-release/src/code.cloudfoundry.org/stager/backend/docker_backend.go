package backend

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"net/url"
	"path"
	"strings"
	"time"

	"code.cloudfoundry.org/bbs/models"
	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/runtimeschema/cc_messages"
	"code.cloudfoundry.org/stager/diego_errors"
	"code.cloudfoundry.org/urljoiner"
)

const (
	DockerLifecycleName         = "docker"
	MountCgroupsPath            = "/tmp/docker_app_lifecycle/mount_cgroups"
	DockerBuilderExecutablePath = "/tmp/docker_app_lifecycle/builder"
	DockerBuilderOutputPath     = "/tmp/docker-result/result.json"
)

var ErrMissingDockerImageUrl = errors.New(diego_errors.MISSING_DOCKER_IMAGE_URL)
var ErrMissingDockerRegistry = errors.New(diego_errors.MISSING_DOCKER_REGISTRY)
var ErrMissingDockerCredentials = errors.New(diego_errors.MISSING_DOCKER_CREDENTIALS)
var ErrInvalidDockerRegistryAddress = errors.New(diego_errors.INVALID_DOCKER_REGISTRY_ADDRESS)

type dockerBackend struct {
	config Config
	logger lager.Logger
}

type consulServiceInfo struct {
	Address string
}

func NewDockerBackend(config Config, logger lager.Logger) Backend {
	return &dockerBackend{
		config: config,
		logger: logger.Session("docker"),
	}
}

func (backend *dockerBackend) BuildRecipe(stagingGuid string, request cc_messages.StagingRequestFromCC) (*models.TaskDefinition, string, string, error) {
	logger := backend.logger.Session("build-recipe", lager.Data{"app-id": request.AppId, "staging-guid": stagingGuid})
	logger.Info("staging-request")

	var lifecycleData cc_messages.DockerStagingData
	err := json.Unmarshal(*request.LifecycleData, &lifecycleData)
	if err != nil {
		return &models.TaskDefinition{}, "", "", err
	}

	err = backend.validateRequest(request, lifecycleData)
	if err != nil {
		return &models.TaskDefinition{}, "", "", err
	}

	compilerURL, err := backend.compilerDownloadURL()
	if err != nil {
		return &models.TaskDefinition{}, "", "", err
	}

	cachedDependencies := []*models.CachedDependency{
		&models.CachedDependency{
			From:     compilerURL.String(),
			To:       path.Dir(DockerBuilderExecutablePath),
			CacheKey: "docker-lifecycle",
		},
	}

	runActionArguments := []string{
		"-outputMetadataJSONFilename", DockerBuilderOutputPath,
		"-dockerRef", lifecycleData.DockerImageUrl,
	}

	if len(backend.config.InsecureDockerRegistries) > 0 {
		insecureDockerRegistries := strings.Join(backend.config.InsecureDockerRegistries, ",")
		runActionArguments = append(runActionArguments, "-insecureDockerRegistries", insecureDockerRegistries)
	}

	fileDescriptorLimit := uint64(request.FileDescriptors)
	runAs := "vcap"

	actions := []models.ActionInterface{}

	if cacheDockerImage(request.Environment) {
		runAs = "root"

		additionalEgressRules, additionalArgs, err := cachingEgressRulesAndArgs(
			logger,
			backend.config.DockerRegistryAddress,
			backend.config.ConsulCluster,
			lifecycleData,
		)
		if err != nil {
			return &models.TaskDefinition{}, "", "", err
		}

		runActionArguments = append(runActionArguments, additionalArgs...)
		request.EgressRules = append(request.EgressRules, additionalEgressRules...)

		actions = append(
			actions,
			models.EmitProgressFor(
				&models.RunAction{
					Path: MountCgroupsPath,
					ResourceLimits: &models.ResourceLimits{
						Nofile: &fileDescriptorLimit,
					},
					User: runAs,
				},
				"Preparing docker daemon...",
				"",
				"Failed to set up docker environment",
			),
		)
	}

	actions = append(
		actions,
		models.EmitProgressFor(
			&models.RunAction{
				Path: DockerBuilderExecutablePath,
				Args: runActionArguments,
				Env:  request.Environment,
				ResourceLimits: &models.ResourceLimits{
					Nofile: &fileDescriptorLimit,
				},
				User: runAs,
			},
			"Staging...",
			"Staging Complete",
			"Staging Failed",
		),
	)

	annotationJson, _ := json.Marshal(cc_messages.StagingTaskAnnotation{
		Lifecycle:          DockerLifecycleName,
		CompletionCallback: request.CompletionCallback,
	})

	taskDefinition := &models.TaskDefinition{
		RootFs:                        models.PreloadedRootFS(backend.config.DockerStagingStack),
		ResultFile:                    DockerBuilderOutputPath,
		Privileged:                    backend.config.PrivilegedContainers,
		MemoryMb:                      int32(request.MemoryMB),
		LogSource:                     TaskLogSource,
		LogGuid:                       request.LogGuid,
		EgressRules:                   request.EgressRules,
		DiskMb:                        int32(request.DiskMB),
		CompletionCallbackUrl:         backend.config.CallbackURL(stagingGuid),
		Annotation:                    string(annotationJson),
		Action:                        models.WrapAction(models.Timeout(models.Serial(actions...), dockerTimeout(request, backend.logger))),
		CachedDependencies:            cachedDependencies,
		LegacyDownloadUser:            "vcap",
		TrustedSystemCertificatesPath: TrustedSystemCertificatesPath,
	}
	logger.Debug("staging-task-request")

	return taskDefinition, stagingGuid, backend.config.TaskDomain, nil
}

func (backend *dockerBackend) BuildStagingResponse(taskResponse *models.TaskCallbackResponse) (cc_messages.StagingResponseForCC, error) {
	var response cc_messages.StagingResponseForCC

	if taskResponse.Failed {
		response.Error = backend.config.Sanitizer(taskResponse.FailureReason)
	} else {
		result := json.RawMessage([]byte(taskResponse.Result))
		response.Result = &result
	}

	return response, nil
}

func (backend *dockerBackend) compilerDownloadURL() (*url.URL, error) {
	lifecycleFilename := backend.config.Lifecycles["docker"]
	if lifecycleFilename == "" {
		return nil, ErrNoCompilerDefined
	}

	parsed, err := url.Parse(lifecycleFilename)
	if err != nil {
		return nil, errors.New("couldn't parse compiler URL")
	}

	switch parsed.Scheme {
	case "http", "https":
		return parsed, nil
	case "":
		break
	default:
		return nil, fmt.Errorf("unknown scheme: '%s'", parsed.Scheme)
	}

	urlString := urljoiner.Join(backend.config.FileServerURL, "/v1/static", lifecycleFilename)

	url, err := url.ParseRequestURI(urlString)
	if err != nil {
		return nil, fmt.Errorf("failed to parse compiler download URL: %s", err)
	}

	return url, nil
}

func (backend *dockerBackend) validateRequest(stagingRequest cc_messages.StagingRequestFromCC, dockerData cc_messages.DockerStagingData) error {
	if len(stagingRequest.AppId) == 0 {
		return ErrMissingAppId
	}

	if len(dockerData.DockerImageUrl) == 0 {
		return ErrMissingDockerImageUrl
	}

	credentialsPresent := (len(dockerData.DockerUser) + len(dockerData.DockerPassword) + len(dockerData.DockerEmail)) > 0
	if credentialsPresent && (len(dockerData.DockerUser) == 0 || len(dockerData.DockerPassword) == 0 || len(dockerData.DockerEmail) == 0) {
		return ErrMissingDockerCredentials
	}

	return nil
}

func dockerTimeout(request cc_messages.StagingRequestFromCC, logger lager.Logger) time.Duration {
	if request.Timeout > 0 {
		return time.Duration(request.Timeout) * time.Second
	} else {
		logger.Info("overriding requested timeout", lager.Data{
			"requested-timeout": request.Timeout,
			"default-timeout":   DefaultStagingTimeout,
			"app-id":            request.AppId,
		})
		return DefaultStagingTimeout
	}
}

func getDockerRegistryServices(consulCluster string, backendLogger lager.Logger) ([]consulServiceInfo, error) {
	logger := backendLogger.Session("docker-registry-consul-services")

	response, err := http.Get(consulCluster + "/v1/catalog/service/docker-registry")
	if err != nil {
		return nil, err
	}

	defer response.Body.Close()
	body, err := ioutil.ReadAll(response.Body)
	if err != nil {
		return nil, err
	}

	var ips []consulServiceInfo
	err = json.Unmarshal(body, &ips)
	if err != nil {
		return nil, err
	}

	if len(ips) == 0 {
		return nil, ErrMissingDockerRegistry
	}

	logger.Debug("docker-registry-consul-services", lager.Data{"ips": ips})

	return ips, nil
}

func cachingEgressRulesAndArgs(
	logger lager.Logger,
	dockerRegistryAddress string,
	consulCluster string,
	stagingData cc_messages.DockerStagingData,
) ([]*models.SecurityGroupRule, []string, error) {
	host, port, err := net.SplitHostPort(dockerRegistryAddress)
	if err != nil {
		logger.Error("invalid-docker-registry-address", err, lager.Data{
			"registry-address": dockerRegistryAddress,
		})
		return []*models.SecurityGroupRule{}, []string{}, ErrInvalidDockerRegistryAddress
	}

	registryServices, err := getDockerRegistryServices(consulCluster, logger)
	if err != nil {
		logger.Error("failed-getting-docker-registry-services", err)
		return []*models.SecurityGroupRule{}, []string{}, err
	}

	egressRules := []*models.SecurityGroupRule{}
	registryIPs := make([]string, 0, len(registryServices))
	for _, registry := range registryServices {
		egressRules = append(egressRules, &models.SecurityGroupRule{
			Protocol:     models.TCPProtocol,
			Destinations: []string{registry.Address},
			Ports:        []uint32{8080},
		})

		registryIPs = append(registryIPs, registry.Address)
	}

	args := []string{
		"-cacheDockerImage",
		"-dockerRegistryHost",
		host, "-dockerRegistryPort",
		port, "-dockerRegistryIPs",
		strings.Join(registryIPs, ","),
	}

	if len(stagingData.DockerLoginServer) > 0 {
		args = append(args, "-dockerLoginServer", stagingData.DockerLoginServer)
	}

	if len(stagingData.DockerUser) > 0 {
		args = append(args, "-dockerUser", stagingData.DockerUser,
			"-dockerPassword", stagingData.DockerPassword,
			"-dockerEmail", stagingData.DockerEmail)
	}

	return egressRules, args, nil
}

func cacheDockerImage(env []*models.EnvironmentVariable) bool {
	for _, envVar := range env {
		if envVar.Name == "DIEGO_DOCKER_CACHE" && envVar.Value == "true" {
			return true
		}
	}

	return false
}
