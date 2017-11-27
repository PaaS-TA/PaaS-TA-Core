package backend

import (
	"encoding/json"
	"errors"
	"fmt"
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
var ErrMissingDockerCredentials = errors.New(diego_errors.MISSING_DOCKER_CREDENTIALS)

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

	if lifecycleData.DockerUser != "" {
		runActionArguments = append(runActionArguments,
			"-dockerUser", lifecycleData.DockerUser,
			"-dockerPassword", lifecycleData.DockerPassword)
	}

	fileDescriptorLimit := uint64(request.FileDescriptors)
	runAs := "vcap"

	actions := []models.ActionInterface{}

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

	if request.IsolationSegment != "" {
		taskDefinition.PlacementTags = []string{request.IsolationSegment}
	}

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

	if (dockerData.DockerUser != "" && dockerData.DockerPassword == "") ||
		(dockerData.DockerUser == "" && dockerData.DockerPassword != "") {
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
