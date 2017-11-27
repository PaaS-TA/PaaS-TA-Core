package config

import (
	"encoding/json"
	"path/filepath"
)

type Config struct {
	Node   ConfigNode
	Confab ConfigConfab
	Consul ConfigConsul
	Path   ConfigPath
}

type ConfigConfab struct {
	TimeoutInSeconds int `json:"timeout_in_seconds"`
}

type ConfigConsul struct {
	Agent       ConfigConsulAgent
	EncryptKeys []string `json:"encrypt_keys"`
}

type ConfigPath struct {
	AgentPath       string `json:"agent_path"`
	ConsulConfigDir string `json:"consul_config_dir"`
	PIDFile         string `json:"pid_file"`
	KeyringFile     string `json:"keyring_file"`
	DataDir         string `json:"data_dir"`
}

type ConfigNode struct {
	Name       string `json:"name"`
	Index      int    `json:"index"`
	ExternalIP string `json:"external_ip"`
	Zone       string `json:"zone"`
}

type ConfigConsulAgent struct {
	Servers         ConfigConsulAgentServers     `json:"servers"`
	Services        map[string]ServiceDefinition `json:"services"`
	Mode            string                       `json:"mode"`
	Domain          string                       `json:"domain"`
	Datacenter      string                       `json:"datacenter"`
	LogLevel        string                       `json:"log_level"`
	ProtocolVersion int                          `json:"protocol_version"`
	DnsConfig       ConfigConsulAgentDnsConfig   `json:"dns_config"`
	Telemetry       ConfigConsulTelemetry        `json:"telemetry"`
	Bootstrap       bool                         `json:"bootstrap"`
	NodeName        string                       `json:"node_name"`
	RequireSSL      bool                         `json:"require_ssl"`
	Ports           ConfigConsulAgentPorts       `json:"ports"`
}

type ConfigConsulAgentPorts struct {
	DNS int `json:"dns"`
}

type ConfigConsulAgentDnsConfig struct {
	AllowStale      bool   `json:"allow_stale"`
	MaxStale        string `json:"max_stale"`
	RecursorTimeout string `json:"recursor_timeout"`
}

type ConfigConsulTelemetry struct {
	StatsdAddress string `json:"statsd_address"`
}

type ConfigConsulAgentServers struct {
	LAN []string `json:"lan"`
	WAN []string `json:"wan"`
}

func defaultConfig() Config {
	return Config{
		Path: ConfigPath{
			AgentPath:       "/var/vcap/packages/consul/bin/consul",
			ConsulConfigDir: "/var/vcap/jobs/consul_agent/config",
			PIDFile:         "/var/vcap/sys/run/consul_agent/consul_agent.pid",
		},
		Consul: ConfigConsul{
			Agent: ConfigConsulAgent{
				DnsConfig: ConfigConsulAgentDnsConfig{
					AllowStale:      true,
					MaxStale:        "30s",
					RecursorTimeout: "5s",
				},
				Servers: ConfigConsulAgentServers{
					LAN: []string{},
					WAN: []string{},
				},
			},
		},
		Confab: ConfigConfab{
			TimeoutInSeconds: 55,
		},
	}
}

func ConfigFromJSON(configData, configConsulLinkData []byte) (Config, error) {
	config := defaultConfig()

	if err := json.Unmarshal(configData, &config); err != nil {
		return Config{}, err
	}

	configConsul, err := mergedConsulConfig(config.Consul, configConsulLinkData)
	if err != nil {
		return Config{}, err
	}
	config.Consul = configConsul

	if config.Path.DataDir == "" {
		if config.Consul.Agent.Mode == "server" {
			config.Path.DataDir = "/var/vcap/store/consul_agent"
		} else {
			config.Path.DataDir = "/var/vcap/data/consul_agent"
		}
	}

	if config.Path.KeyringFile == "" {
		config.Path.KeyringFile = filepath.Join(config.Path.DataDir, "serf", "local.keyring")
	}

	return config, nil
}

func mergedConsulConfig(configConsul ConfigConsul, configConsulLinkData []byte) (ConfigConsul, error) {
	if len(configConsulLinkData) > 0 {
		if err := json.Unmarshal(configConsulLinkData, &configConsul); err != nil {
			return ConfigConsul{}, err
		}
	}

	return configConsul, nil
}
