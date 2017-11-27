// loggregator provides a top-level client for connecting to the loggregator v1
// and v2 API's.
//
// All members in the package here are deprecated and will be removed in the
// next major version of this library. Instead, see the v1 and v2 packages for
// the preferred way of connecting to the respective loggregator API.
package loggregator

import (
	"fmt"
	"time"

	v2 "code.cloudfoundry.org/go-loggregator"
	"code.cloudfoundry.org/go-loggregator/internal/v2shim"
	"code.cloudfoundry.org/go-loggregator/v1"

	"github.com/cloudfoundry/sonde-go/events"
)

// IngressClient is the shared contract between v1 and v2 clients.
//
// Deprecated: This interface will be removed in the next major version.
// Instead, use the v1 or v2 clients directly.
type IngressClient interface {
	SendDuration(name string, value time.Duration) error
	SendMebiBytes(name string, value int) error
	SendMetric(name string, value int) error
	SendBytesPerSecond(name string, value float64) error
	SendRequestsPerSecond(name string, value float64) error
	IncrementCounter(name string) error
	IncrementCounterWithDelta(name string, value uint64) error
	SendAppLog(appID, message, sourceType, sourceInstance string) error
	SendAppErrorLog(appID, message, sourceType, sourceInstance string) error
	SendAppMetrics(metrics *events.ContainerMetric) error
	SendComponentMetric(name string, value float64, unit string) error
}

// Config is the shared configuration between v1 and v2 clients.
//
// Deprecated: Config will be removed in the next major version.
// Instead, create a v1 or v2 client directly.
type Config struct {
	UseV2API      bool   `json:"loggregator_use_v2_api"`
	APIPort       int    `json:"loggregator_api_port"`
	CACertPath    string `json:"loggregator_ca_path"`
	CertPath      string `json:"loggregator_cert_path"`
	KeyPath       string `json:"loggregator_key_path"`
	JobDeployment string `json:"loggregator_job_deployment"`
	JobName       string `json:"loggregator_job_name"`
	JobIndex      string `json:"loggregator_job_index"`
	JobIP         string `json:"loggregator_job_ip"`
	JobOrigin     string `json:"loggregator_job_origin"`

	BatchMaxSize       uint
	BatchFlushInterval time.Duration
}

// NewIngressClient returns a v1 or v2 client depending on the value of `UseV2API`
// from the config
//
// Deprecated: NewIngressClient will be removed in the next major version.
// Instead, create a v1 or v2 client directly.
func NewIngressClient(config Config) (IngressClient, error) {
	if config.UseV2API {
		return NewV2IngressClient(config)
	}

	return NewV1IngressClient(config)
}

// NewV1IngressClient creates a V1 connection to the Loggregator API.
//
// Deprecated: NewV1IngressClient will be removed in the next major version.
// Instead, use v1.NewIngressClient.
func NewV1IngressClient(config Config) (IngressClient, error) {
	return v1.NewClient()
}

// NewV2IngressClient creates a V2 connection to the Loggregator API.
//
// Deprecated: NewV2IngressClient will be removed in the next major version.
// Instead, use v2.NewIngressClient.
func NewV2IngressClient(config Config) (IngressClient, error) {
	tlsConfig, err := v2.NewIngressTLSConfig(
		config.CACertPath,
		config.CertPath,
		config.KeyPath,
	)
	if err != nil {
		return nil, err
	}

	opts := []v2.IngressOption{
		// Whereas Metron will add tags for deployment, name, index, and ip,
		// it does not add job origin and so we must add it manually here.
		v2.WithStringTag("origin", config.JobOrigin),
	}

	if config.BatchMaxSize != 0 {
		opts = append(opts, v2.WithBatchMaxSize(config.BatchMaxSize))
	}

	if config.BatchFlushInterval != time.Duration(0) {
		opts = append(opts, v2.WithBatchFlushInterval(config.BatchFlushInterval))
	}

	if config.APIPort != 0 {
		opts = append(opts, v2.WithAddr(fmt.Sprintf("localhost:%d", config.APIPort)))
	}

	c, err := v2.NewIngressClient(tlsConfig, opts...)
	if err != nil {
		return nil, err
	}

	return v2shim.NewIngressClient(c), nil
}
