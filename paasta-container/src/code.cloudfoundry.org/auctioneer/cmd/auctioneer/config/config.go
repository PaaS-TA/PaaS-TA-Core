package config

import (
	"encoding/json"
	"os"
	"time"

	"code.cloudfoundry.org/debugserver"
	"code.cloudfoundry.org/durationjson"
	loggregator_v2 "code.cloudfoundry.org/go-loggregator/compatibility"
	"code.cloudfoundry.org/lager/lagerflags"
	"code.cloudfoundry.org/locket"
)

type AuctioneerConfig struct {
	AuctionRunnerWorkers          int                   `json:"auction_runner_workers,omitempty"`
	BBSAddress                    string                `json:"bbs_address,omitempty"`
	BBSCACertFile                 string                `json:"bbs_ca_cert_file,omitempty"`
	BBSClientCertFile             string                `json:"bbs_client_cert_file,omitempty"`
	BBSClientKeyFile              string                `json:"bbs_client_key_file,omitempty"`
	BBSClientSessionCacheSize     int                   `json:"bbs_client_session_cache_size,omitempty"`
	BBSMaxIdleConnsPerHost        int                   `json:"bbs_max_idle_conns_per_host,omitempty"`
	CACertFile                    string                `json:"ca_cert_file,omitempty"`
	CellStateTimeout              durationjson.Duration `json:"cell_state_timeout,omitempty"`
	CommunicationTimeout          durationjson.Duration `json:"communication_timeout,omitempty"`
	ConsulCluster                 string                `json:"consul_cluster,omitempty"`
	DropsondePort                 int                   `json:"dropsonde_port,omitempty"`
	ListenAddress                 string                `json:"listen_address,omitempty"`
	LockRetryInterval             durationjson.Duration `json:"lock_retry_interval,omitempty"`
	LockTTL                       durationjson.Duration `json:"lock_ttl,omitempty"`
	LoggregatorConfig             loggregator_v2.Config `json:"loggregator"`
	RepCACert                     string                `json:"rep_ca_cert,omitempty"`
	RepClientCert                 string                `json:"rep_client_cert,omitempty"`
	RepClientKey                  string                `json:"rep_client_key,omitempty"`
	RepClientSessionCacheSize     int                   `json:"rep_client_session_cache_size,omitempty"`
	RepRequireTLS                 bool                  `json:"rep_require_tls,omitempty"`
	ServerCertFile                string                `json:"server_cert_file,omitempty"`
	ServerKeyFile                 string                `json:"server_key_file,omitempty"`
	SkipConsulLock                bool                  `json:"skip_consul_lock"`
	StartingContainerCountMaximum int                   `json:"starting_container_count_maximum,omitempty"`
	StartingContainerWeight       float64               `json:"starting_container_weight,omitempty"`
	UUID                          string                `json:"uuid,omitempty"`
	debugserver.DebugServerConfig
	lagerflags.LagerConfig
	locket.ClientLocketConfig
}

func DefaultAuctioneerConfig() AuctioneerConfig {
	return AuctioneerConfig{
		AuctionRunnerWorkers: 1000,
		CellStateTimeout:     durationjson.Duration(1 * time.Second),
		CommunicationTimeout: durationjson.Duration(10 * time.Second),
		DropsondePort:        3457,
		LagerConfig:          lagerflags.DefaultLagerConfig(),
		ListenAddress:        "0.0.0.0:9016",
		LockRetryInterval:    durationjson.Duration(locket.RetryInterval),
		LockTTL:              durationjson.Duration(locket.DefaultSessionTTL),
		StartingContainerCountMaximum: 0,
		StartingContainerWeight:       .25,
	}
}

func NewAuctioneerConfig(configPath string) (AuctioneerConfig, error) {
	cfg := DefaultAuctioneerConfig()

	configFile, err := os.Open(configPath)
	if err != nil {
		return AuctioneerConfig{}, err
	}

	defer configFile.Close()

	decoder := json.NewDecoder(configFile)

	err = decoder.Decode(&cfg)
	if err != nil {
		return AuctioneerConfig{}, err
	}

	return cfg, nil
}
