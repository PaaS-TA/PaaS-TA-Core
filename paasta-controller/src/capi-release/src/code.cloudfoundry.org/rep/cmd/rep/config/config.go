package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"code.cloudfoundry.org/debugserver"
	"code.cloudfoundry.org/durationjson"
	executorinit "code.cloudfoundry.org/executor/initializer"
	"code.cloudfoundry.org/lager/lagerflags"
	"code.cloudfoundry.org/locket"
	"code.cloudfoundry.org/loggregator_v2"
)

type StackMap map[string]string

func (m *StackMap) UnmarshalJSON(data []byte) error {
	*m = make(map[string]string)
	arr := []string{}
	err := json.Unmarshal(data, &arr)
	if err != nil {
		return err
	}

	for _, s := range arr {
		parts := strings.SplitN(s, ":", 2)
		if len(parts) != 2 {
			return errors.New("Invalid preloaded RootFS value: not of the form 'stack-name:path'")
		}

		if parts[0] == "" {
			return errors.New("Invalid preloaded RootFS value: blank stack")
		}

		if parts[1] == "" {
			return errors.New("Invalid preloaded RootFS value: blank path")
		}

		(*m)[parts[0]] = parts[1]
	}

	return nil
}

func (m StackMap) MarshalJSON() (b []byte, err error) {
	arr := []string{}
	for k, v := range m {
		arr = append(arr, fmt.Sprintf("%s:%s", k, v))
	}
	data, err := json.Marshal(arr)
	if err != nil {
		return nil, err
	}
	return data, nil
}

type RepConfig struct {
	loggregator_v2.MetronConfig
	AdvertiseDomain           string                `json:"advertise_domain,omitempty"`
	BBSAddress                string                `json:"bbs_address"`
	BBSCACertFile             string                `json:"bbs_ca_cert_file"`
	BBSClientCertFile         string                `json:"bbs_client_cert_file"`
	BBSClientKeyFile          string                `json:"bbs_client_key_file"`
	BBSClientSessionCacheSize int                   `json:"bbs_client_session_cache_size,omitempty"`
	BBSMaxIdleConnsPerHost    int                   `json:"bbs_max_idle_conns_per_host,omitempty"`
	CaCertFile                string                `json:"ca_cert_file"`
	CellID                    string                `json:"cell_id"`
	CommunicationTimeout      durationjson.Duration `json:"communication_timeout,omitempty"`
	ConsulCACert              string                `json:"consul_ca_cert"`
	ConsulClientCert          string                `json:"consul_client_cert"`
	ConsulClientKey           string                `json:"consul_client_key"`
	ConsulCluster             string                `json:"consul_cluster"`
	DropsondePort             int                   `json:"dropsonde_port,omitempty"`
	EnableLegacyAPIServer     bool                  `json:"enable_legacy_api_endpoints"`
	EvacuationPollingInterval durationjson.Duration `json:"evacuation_polling_interval,omitempty"`
	EvacuationTimeout         durationjson.Duration `json:"evacuation_timeout,omitempty"`
	ListenAddr                string                `json:"listen_addr,omitempty"`
	ListenAddrAdmin           string                `json:"listen_addr_admin"`
	ListenAddrSecurable       string                `json:"listen_addr_securable,omitempty"`
	LockRetryInterval         durationjson.Duration `json:"lock_retry_interval,omitempty"`
	LockTTL                   durationjson.Duration `json:"lock_ttl,omitempty"`
	OptionalPlacementTags     []string              `json:"optional_placement_tags"`
	PlacementTags             []string              `json:"placement_tags"`
	PollingInterval           durationjson.Duration `json:"polling_interval,omitempty"`
	PreloadedRootFS           StackMap              `json:"preloaded_root_fs"`
	RequireTLS                bool                  `json:"require_tls"`
	ServerCertFile            string                `json:"server_cert_file"`
	ServerKeyFile             string                `json:"server_key_file"`
	SessionName               string                `json:"session_name,omitempty"`
	SupportedProviders        []string              `json:"supported_providers"`
	Zone                      string                `json:"zone"`
	debugserver.DebugServerConfig
	executorinit.ExecutorConfig
	lagerflags.LagerConfig
	locket.ClientLocketConfig
}

func defaultConfig() RepConfig {
	return RepConfig{
		AdvertiseDomain:           "cell.service.cf.internal",
		BBSClientSessionCacheSize: 0,
		BBSMaxIdleConnsPerHost:    0,
		CommunicationTimeout:      durationjson.Duration(10 * time.Second),
		DropsondePort:             3457,
		EnableLegacyAPIServer:     true,
		EvacuationPollingInterval: durationjson.Duration(10 * time.Second),
		EvacuationTimeout:         durationjson.Duration(10 * time.Minute),
		ExecutorConfig:            executorinit.DefaultConfiguration,
		LagerConfig:               lagerflags.DefaultLagerConfig(),
		ListenAddr:                "0.0.0.0:1800",
		ListenAddrSecurable:       "0.0.0.0:1801",
		LockRetryInterval:         durationjson.Duration(locket.RetryInterval),
		LockTTL:                   durationjson.Duration(locket.DefaultSessionTTL),
		PollingInterval:           durationjson.Duration(30 * time.Second),
		RequireTLS:                true,
		SessionName:               "rep",
	}
}

func NewRepConfig(configPath string) (RepConfig, error) {
	repConfig := defaultConfig()
	configFile, err := os.Open(configPath)
	if err != nil {
		return RepConfig{}, err
	}
	decoder := json.NewDecoder(configFile)

	err = decoder.Decode(&repConfig)
	if err != nil {
		return RepConfig{}, err
	}

	return repConfig, nil
}
