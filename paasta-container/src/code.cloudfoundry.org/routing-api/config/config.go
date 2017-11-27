package config

import (
	"errors"
	"io/ioutil"
	"time"

	"gopkg.in/yaml.v2"

	"code.cloudfoundry.org/locket"
	"code.cloudfoundry.org/routing-api/models"
)

type MetronConfig struct {
	Address string
	Port    string
}

type OAuthConfig struct {
	TokenEndpoint     string `yaml:"token_endpoint"`
	Port              int    `yaml:"port"`
	SkipSSLValidation bool   `yaml:"skip_ssl_validation"`
	ClientName        string `yaml:"client_name"`
	ClientSecret      string `yaml:"client_secret"`
	CACerts           string `yaml:"ca_certs"`
}

type SqlDB struct {
	Host     string `yaml:"host"`
	Port     int    `yaml:"port"`
	Schema   string `yaml:"schema"`
	Type     string `yaml:"type"`
	Username string `yaml:"username"`
	Password string `yaml:"password"`
}

type Etcd struct {
	CertFile   string   `yaml:"cert_file"`
	KeyFile    string   `yaml:"key_file"`
	CAFile     string   `yaml:"ca_file"`
	RequireSSL bool     `yaml:"require_ssl"`
	NodeURLS   []string `yaml:"node_urls"`
}

type ConsulCluster struct {
	Servers       string        `yaml:"servers"`
	LockTTL       time.Duration `yaml:"lock_ttl"`
	RetryInterval time.Duration `yaml:"retry_interval"`
}

type Config struct {
	DebugAddress                    string                    `yaml:"debug_address"`
	LogGuid                         string                    `yaml:"log_guid"`
	MetronConfig                    MetronConfig              `yaml:"metron_config"`
	MaxTTL                          time.Duration             `yaml:"max_ttl"`
	SystemDomain                    string                    `yaml:"system_domain"`
	MetricsReportingIntervalString  string                    `yaml:"metrics_reporting_interval"`
	MetricsReportingInterval        time.Duration             `yaml:"-"`
	StatsdEndpoint                  string                    `yaml:"statsd_endpoint"`
	StatsdClientFlushIntervalString string                    `yaml:"statsd_client_flush_interval"`
	StatsdClientFlushInterval       time.Duration             `yaml:"-"`
	OAuth                           OAuthConfig               `yaml:"oauth"`
	RouterGroups                    models.RouterGroups       `yaml:"router_groups"`
	Etcd                            Etcd                      `yaml:"etcd"`
	SqlDB                           SqlDB                     `yaml:"sqldb"`
	ConsulCluster                   ConsulCluster             `yaml:"consul_cluster"`
	SkipConsulLock                  bool                      `yaml:"skip_consul_lock"`
	Locket                          locket.ClientLocketConfig `yaml:"locket"`
	UUID                            string                    `yaml:"uuid"`
}

func NewConfigFromFile(configFile string, authDisabled bool) (Config, error) {
	c, err := ioutil.ReadFile(configFile)
	if err != nil {
		return Config{}, err
	}

	// Init things
	config := Config{}
	if err = config.Initialize(c, authDisabled); err != nil {
		return config, err
	}

	return config, nil
}

func (cfg *Config) Initialize(file []byte, authDisabled bool) error {
	err := yaml.Unmarshal(file, &cfg)
	if err != nil {
		return err
	}

	if cfg.SystemDomain == "" {
		return errors.New("No system_domain specified")
	}

	if cfg.LogGuid == "" {
		return errors.New("No log_guid specified")
	}

	if !authDisabled && cfg.OAuth.TokenEndpoint == "" {
		return errors.New("No token endpoint specified")
	}

	if !authDisabled && cfg.OAuth.TokenEndpoint != "" && cfg.OAuth.Port == -1 {
		return errors.New("Routing API requires TLS enabled to get OAuth token")
	}
	if cfg.ConsulCluster.LockTTL == 0 {
		cfg.ConsulCluster.LockTTL = locket.DefaultSessionTTL
	}
	if cfg.ConsulCluster.RetryInterval == 0 {
		cfg.ConsulCluster.RetryInterval = locket.RetryInterval
	}
	if cfg.UUID == "" {
		return errors.New("No UUID is specified")
	}

	err = cfg.process()
	if err != nil {
		return err
	}

	return nil
}

func (cfg *Config) process() error {
	metricsReportingInterval, err := time.ParseDuration(cfg.MetricsReportingIntervalString)
	if err != nil {
		return err
	}
	cfg.MetricsReportingInterval = metricsReportingInterval

	statsdClientFlushInterval, err := time.ParseDuration(cfg.StatsdClientFlushIntervalString)
	if err != nil {
		return err
	}
	cfg.StatsdClientFlushInterval = statsdClientFlushInterval

	if cfg.MaxTTL == 0 {
		cfg.MaxTTL = 2 * time.Minute
	}

	if err := cfg.RouterGroups.Validate(); err != nil {
		return err
	}

	return nil
}
