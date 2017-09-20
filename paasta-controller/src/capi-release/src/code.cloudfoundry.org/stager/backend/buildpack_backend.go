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
	"code.cloudfoundry.org/buildpackapplifecycle"
	"code.cloudfoundry.org/cc-uploader"
	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/runtimeschema/cc_messages"
	"code.cloudfoundry.org/urljoiner"
	"github.com/tedsuo/rata"
)

const (
	TraditionalLifecycleName = "buildpack"
	StagingTaskCpuWeight     = uint32(50)

	DefaultLANG = "en_US.UTF-8"
)

type traditionalBackend struct {
	config Config
	logger lager.Logger
}

func NewTraditionalBackend(config Config, logger lager.Logger) Backend {
	return &traditionalBackend{
		config: config,
		logger: logger.Session("traditional"),
	}
}

func (backend *traditionalBackend) BuildRecipe(stagingGuid string, request cc_messages.StagingRequestFromCC) (*models.TaskDefinition, string, string, error) {
	logger := backend.logger.Session("build-recipe", lager.Data{"app-id": request.AppId, "staging-guid": stagingGuid})
	logger.Info("staging-request")

	if request.LifecycleData == nil {
		return &models.TaskDefinition{}, "", "", ErrMissingLifecycleData
	}

	var lifecycleData cc_messages.BuildpackStagingData
	err := json.Unmarshal(*request.LifecycleData, &lifecycleData)
	if err != nil {
		return &models.TaskDefinition{}, "", "", err
	}

	err = backend.validateRequest(request, lifecycleData)
	if err != nil {
		return &models.TaskDefinition{}, "", "", err
	}

	compilerURL, err := backend.compilerDownloadURL(request, lifecycleData)
	if err != nil {
		return &models.TaskDefinition{}, "", "", err
	}

	buildpacksOrder := []string{}
	for _, buildpack := range lifecycleData.Buildpacks {
		buildpacksOrder = append(buildpacksOrder, buildpack.Key)
	}

	skipDetect := len(lifecycleData.Buildpacks) == 1 && lifecycleData.Buildpacks[0].SkipDetect
	builderConfig := buildpackapplifecycle.NewLifecycleBuilderConfig(buildpacksOrder, skipDetect, backend.config.SkipCertVerify)

	timeout := traditionalTimeout(request, backend.logger)

	actions := []models.ActionInterface{}

	//Download app package
	appDownloadAction := &models.DownloadAction{
		Artifact: "app package",
		From:     lifecycleData.AppBitsDownloadUri,
		To:       builderConfig.BuildDir(),
		User:     "vcap",
	}

	actions = append(actions, appDownloadAction)

	cachedDependencies := []*models.CachedDependency{}
	//Download builder
	cachedDependencies = append(
		cachedDependencies,
		&models.CachedDependency{
			From:     compilerURL.String(),
			To:       path.Dir(builderConfig.ExecutablePath),
			CacheKey: fmt.Sprintf("buildpack-%s-lifecycle", lifecycleData.Stack),
		},
	)

	//Download buildpacks
	for _, buildpack := range lifecycleData.Buildpacks {
		if buildpack.Name != cc_messages.CUSTOM_BUILDPACK {
			cachedDependencies = append(
				cachedDependencies,
				&models.CachedDependency{
					Name:     buildpack.Name,
					From:     buildpack.Url,
					To:       builderConfig.BuildpackPath(buildpack.Key),
					CacheKey: buildpack.Key,
				},
			)
		}
	}

	//Download buildpack artifacts cache
	downloadURL, err := backend.buildArtifactsDownloadURL(lifecycleData)
	if err != nil {
		return &models.TaskDefinition{}, "", "", err
	}

	if downloadURL != nil {
		downloadAction := models.Try(
			&models.DownloadAction{
				Artifact: "build artifacts cache",
				From:     downloadURL.String(),
				To:       builderConfig.BuildArtifactsCacheDir(),
				User:     "vcap",
			},
		)
		actions = append(actions, downloadAction)
	}

	fileDescriptorLimit := uint64(request.FileDescriptors)

	//Run Builder
	runEnv := append(request.Environment, &models.EnvironmentVariable{"CF_STACK", lifecycleData.Stack})
	actions = append(
		actions,
		models.EmitProgressFor(
			&models.RunAction{
				User: "vcap",
				Path: builderConfig.Path(),
				Args: builderConfig.Args(),
				Env:  runEnv,
				ResourceLimits: &models.ResourceLimits{
					Nofile: &fileDescriptorLimit,
				},
			},
			"Staging...",
			"Staging complete",
			"Staging failed",
		),
	)

	//Upload Droplet
	uploadActions := []models.ActionInterface{}
	uploadNames := []string{}
	uploadURL, err := backend.dropletUploadURL(request, lifecycleData)
	if err != nil {
		return &models.TaskDefinition{}, "", "", err
	}

	uploadActions = append(
		uploadActions,
		&models.UploadAction{
			Artifact: "droplet",
			From:     builderConfig.OutputDroplet(), // get the droplet
			To:       addTimeoutParamToURL(*uploadURL, timeout).String(),
			User:     "vcap",
		},
	)
	uploadNames = append(uploadNames, "droplet")

	//Upload Buildpack Artifacts Cache
	uploadURL, err = backend.buildArtifactsUploadURL(request, lifecycleData)
	if err != nil {
		return &models.TaskDefinition{}, "", "", err
	}

	uploadActions = append(uploadActions,
		models.Try(
			&models.UploadAction{
				Artifact: "build artifacts cache",
				From:     builderConfig.OutputBuildArtifactsCache(), // get the compressed build artifacts cache
				To:       addTimeoutParamToURL(*uploadURL, timeout).String(),
				User:     "vcap",
			},
		),
	)
	uploadNames = append(uploadNames, "build artifacts cache")

	uploadMsg := fmt.Sprintf("Uploading %s...", strings.Join(uploadNames, ", "))
	actions = append(actions, models.EmitProgressFor(models.Parallel(uploadActions...), uploadMsg, "Uploading complete", "Uploading failed"))

	annotationJson, _ := json.Marshal(cc_messages.StagingTaskAnnotation{
		Lifecycle:          TraditionalLifecycleName,
		CompletionCallback: request.CompletionCallback,
	})

	taskDefinition := &models.TaskDefinition{
		RootFs:                        models.PreloadedRootFS(lifecycleData.Stack),
		ResultFile:                    builderConfig.OutputMetadata(),
		MemoryMb:                      int32(request.MemoryMB),
		DiskMb:                        int32(request.DiskMB),
		CpuWeight:                     uint32(StagingTaskCpuWeight),
		CachedDependencies:            cachedDependencies,
		Action:                        models.WrapAction(models.Timeout(models.Serial(actions...), timeout)),
		LogGuid:                       request.LogGuid,
		LogSource:                     TaskLogSource,
		CompletionCallbackUrl:         backend.config.CallbackURL(stagingGuid),
		EgressRules:                   request.EgressRules,
		Annotation:                    string(annotationJson),
		Privileged:                    backend.config.PrivilegedContainers,
		EnvironmentVariables:          []*models.EnvironmentVariable{{"LANG", DefaultLANG}},
		LegacyDownloadUser:            "vcap",
		TrustedSystemCertificatesPath: TrustedSystemCertificatesPath,
	}

	logger.Debug("staging-task-request")

	return taskDefinition, stagingGuid, backend.config.TaskDomain, nil
}

func (backend *traditionalBackend) BuildStagingResponse(taskResponse *models.TaskCallbackResponse) (cc_messages.StagingResponseForCC, error) {
	var response cc_messages.StagingResponseForCC

	if taskResponse.Failed {
		response.Error = backend.config.Sanitizer(taskResponse.FailureReason)
	} else {
		result := json.RawMessage([]byte(taskResponse.Result))
		response.Result = &result
	}

	return response, nil
}

func (backend *traditionalBackend) compilerDownloadURL(request cc_messages.StagingRequestFromCC, buildpackData cc_messages.BuildpackStagingData) (*url.URL, error) {
	compilerPath, ok := backend.config.Lifecycles[request.Lifecycle+"/"+buildpackData.Stack]
	if !ok {
		return nil, ErrNoCompilerDefined
	}

	parsed, err := url.Parse(compilerPath)
	if err != nil {
		return nil, errors.New("couldn't parse compiler URL")
	}

	switch parsed.Scheme {
	case "http", "https":
		return parsed, nil
	case "":
		break
	default:
		return nil, errors.New("Unknown Scheme")
	}

	urlString := urljoiner.Join(backend.config.FileServerURL, "/v1/static/", compilerPath)

	url, err := url.ParseRequestURI(urlString)
	if err != nil {
		return nil, fmt.Errorf("failed to parse compiler download URL: %s", err)
	}

	return url, nil
}

func (backend *traditionalBackend) dropletUploadURL(request cc_messages.StagingRequestFromCC, buildpackData cc_messages.BuildpackStagingData) (*url.URL, error) {
	path, err := ccuploader.Routes.CreatePathForRoute(ccuploader.UploadDropletRoute, rata.Params{
		"guid": request.AppId,
	})
	if err != nil {
		return nil, fmt.Errorf("couldn't generate droplet upload URL: %s", err)
	}

	urlString := urljoiner.Join(backend.config.CCUploaderURL, path)

	u, err := url.ParseRequestURI(urlString)
	if err != nil {
		return nil, fmt.Errorf("failed to parse droplet upload URL: %s", err)
	}

	values := make(url.Values, 1)
	values.Add(cc_messages.CcDropletUploadUriKey, buildpackData.DropletUploadUri)
	u.RawQuery = values.Encode()

	return u, nil
}

func (backend *traditionalBackend) buildArtifactsUploadURL(request cc_messages.StagingRequestFromCC, buildpackData cc_messages.BuildpackStagingData) (*url.URL, error) {
	path, err := ccuploader.Routes.CreatePathForRoute(ccuploader.UploadBuildArtifactsRoute, rata.Params{
		"app_guid": request.AppId,
	})
	if err != nil {
		return nil, fmt.Errorf("couldn't generate build artifacts cache upload URL: %s", err)
	}

	urlString := urljoiner.Join(backend.config.CCUploaderURL, path)

	u, err := url.ParseRequestURI(urlString)
	if err != nil {
		return nil, fmt.Errorf("failed to parse build artifacts cache upload URL: %s", err)
	}

	values := make(url.Values, 1)
	values.Add(cc_messages.CcBuildArtifactsUploadUriKey, buildpackData.BuildArtifactsCacheUploadUri)
	u.RawQuery = values.Encode()

	return u, nil
}

func (backend *traditionalBackend) buildArtifactsDownloadURL(buildpackData cc_messages.BuildpackStagingData) (*url.URL, error) {
	urlString := buildpackData.BuildArtifactsCacheDownloadUri
	if urlString == "" {
		return nil, nil
	}

	url, err := url.ParseRequestURI(urlString)
	if err != nil {
		return nil, fmt.Errorf("failed to parse build artifacts cache download URL: %s", err)
	}

	return url, nil
}

func (backend *traditionalBackend) validateRequest(stagingRequest cc_messages.StagingRequestFromCC, buildpackData cc_messages.BuildpackStagingData) error {
	if len(stagingRequest.AppId) == 0 {
		return ErrMissingAppId
	}

	if len(buildpackData.AppBitsDownloadUri) == 0 {
		return ErrMissingAppBitsDownloadUri
	}

	return nil
}

func traditionalTimeout(request cc_messages.StagingRequestFromCC, logger lager.Logger) time.Duration {
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
