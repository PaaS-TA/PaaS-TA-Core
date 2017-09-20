package backend

import (
	"errors"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"

	"code.cloudfoundry.org/bbs/models"
	"code.cloudfoundry.org/buildpackapplifecycle"
	"code.cloudfoundry.org/runtimeschema/cc_messages"
	"code.cloudfoundry.org/stager/diego_errors"
)

const (
	TaskLogSource                 = "STG"
	DefaultStagingTimeout         = 15 * time.Minute
	TrustedSystemCertificatesPath = "/etc/cf-system-certificates"
)

type FailureReasonSanitizer func(string) *cc_messages.StagingError

//go:generate counterfeiter -o fake_backend/fake_backend.go . Backend
type Backend interface {
	BuildRecipe(stagingGuid string, request cc_messages.StagingRequestFromCC) (*models.TaskDefinition, string, string, error)
	BuildStagingResponse(*models.TaskCallbackResponse) (cc_messages.StagingResponseForCC, error)
}

var ErrNoCompilerDefined = errors.New(diego_errors.NO_COMPILER_DEFINED_MESSAGE)
var ErrMissingAppId = errors.New(diego_errors.MISSING_APP_ID_MESSAGE)
var ErrMissingAppBitsDownloadUri = errors.New(diego_errors.MISSING_APP_BITS_DOWNLOAD_URI_MESSAGE)
var ErrMissingLifecycleData = errors.New(diego_errors.MISSING_LIFECYCLE_DATA_MESSAGE)

type Config struct {
	TaskDomain               string
	StagerURL                string
	FileServerURL            string
	CCUploaderURL            string
	Lifecycles               map[string]string
	DockerRegistryAddress    string
	InsecureDockerRegistries []string
	ConsulCluster            string
	SkipCertVerify           bool
	Sanitizer                FailureReasonSanitizer
	DockerStagingStack       string
	PrivilegedContainers     bool
}

func (c Config) CallbackURL(stagingGuid string) string {
	return fmt.Sprintf("%s/v1/staging/%s/completed", c.StagerURL, stagingGuid)
}

func max(x, y uint64) uint64 {
	if x > y {
		return x
	} else {
		return y
	}
}

func addTimeoutParamToURL(u url.URL, timeout time.Duration) *url.URL {
	query := u.Query()
	query.Set(cc_messages.CcTimeoutKey, fmt.Sprintf("%.0f", timeout.Seconds()))
	u.RawQuery = query.Encode()
	return &u
}

func SanitizeErrorMessage(message string) *cc_messages.StagingError {
	const staging_failed = "staging failed"
	id := cc_messages.STAGING_ERROR
	switch {
	case strings.HasSuffix(message, strconv.Itoa(buildpackapplifecycle.DETECT_FAIL_CODE)):
		id = cc_messages.BUILDPACK_DETECT_FAILED
		message = staging_failed
	case strings.HasSuffix(message, strconv.Itoa(buildpackapplifecycle.COMPILE_FAIL_CODE)):
		id = cc_messages.BUILDPACK_COMPILE_FAILED
		message = staging_failed
	case strings.HasSuffix(message, strconv.Itoa(buildpackapplifecycle.RELEASE_FAIL_CODE)):
		id = cc_messages.BUILDPACK_RELEASE_FAILED
		message = staging_failed
	case strings.HasPrefix(message, diego_errors.INSUFFICIENT_RESOURCES_MESSAGE):
		id = cc_messages.INSUFFICIENT_RESOURCES
	case strings.HasPrefix(message, diego_errors.CELL_MISMATCH_MESSAGE):
		id = cc_messages.NO_COMPATIBLE_CELL
	case message == diego_errors.CELL_COMMUNICATION_ERROR:
		id = cc_messages.CELL_COMMUNICATION_ERROR
	case message == diego_errors.MISSING_DOCKER_IMAGE_URL:
	case message == diego_errors.MISSING_DOCKER_REGISTRY:
	case message == diego_errors.MISSING_DOCKER_CREDENTIALS:
	case message == diego_errors.INVALID_DOCKER_REGISTRY_ADDRESS:
	default:
		message = "staging failed"
	}

	return &cc_messages.StagingError{
		Id:      id,
		Message: message,
	}
}
