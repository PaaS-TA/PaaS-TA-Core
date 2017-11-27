package config

import (
	"crypto/sha1"
	"encoding/base64"
	"path/filepath"

	"golang.org/x/crypto/pbkdf2"
)

type ConsulConfig struct {
	Server               bool                    `json:"server"`
	Domain               string                  `json:"domain"`
	Datacenter           string                  `json:"datacenter"`
	DataDir              string                  `json:"data_dir"`
	LogLevel             string                  `json:"log_level"`
	NodeName             string                  `json:"node_name"`
	Ports                ConsulConfigPorts       `json:"ports"`
	RejoinAfterLeave     bool                    `json:"rejoin_after_leave"`
	BindAddr             string                  `json:"bind_addr"`
	DisableRemoteExec    bool                    `json:"disable_remote_exec"`
	DisableUpdateCheck   bool                    `json:"disable_update_check"`
	Protocol             int                     `json:"protocol"`
	VerifyOutgoing       *bool                   `json:"verify_outgoing,omitempty"`
	VerifyIncoming       *bool                   `json:"verify_incoming,omitempty"`
	VerifyServerHostname *bool                   `json:"verify_server_hostname,omitempty"`
	CAFile               *string                 `json:"ca_file,omitempty"`
	KeyFile              *string                 `json:"key_file,omitempty"`
	CertFile             *string                 `json:"cert_file,omitempty"`
	Encrypt              *string                 `json:"encrypt,omitempty"`
	DnsConfig            ConsulConfigDnsConfig   `json:"dns_config"`
	Bootstrap            *bool                   `json:"bootstrap,omitempty"`
	Performance          ConsulConfigPerformance `json:"performance"`
	Telemetry            *ConsulConfigTelemetry  `json:"telemetry,omitempty"`
	TLSMinVersion        string                  `json:"tls_min_version"`
}

type ConsulConfigPorts struct {
	DNS   int `json:"dns,omitempty"`
	HTTP  int `json:"http,omitempty"`
	HTTPS int `json:"https,omitempty"`
}

type ConsulConfigDnsConfig struct {
	AllowStale      bool   `json:"allow_stale"`
	MaxStale        string `json:"max_stale"`
	RecursorTimeout string `json:"recursor_timeout"`
}

type ConsulConfigPerformance struct {
	RaftMultiplier int `json:"raft_multiplier"`
}

type ConsulConfigTelemetry struct {
	StatsdAddress string `json:"statsd_address,omitempty"`
}

func GenerateConfiguration(config Config, configDir, nodeName string) ConsulConfig {
	lan := config.Consul.Agent.Servers.LAN
	if lan == nil {
		lan = []string{}
	}

	wan := config.Consul.Agent.Servers.WAN
	if wan == nil {
		wan = []string{}
	}

	isServer := config.Consul.Agent.Mode == "server"

	dns := config.Consul.Agent.Ports.DNS
	if dns == 0 {
		dns = 53
	}

	consulConfig := ConsulConfig{
		Server:             isServer,
		Domain:             config.Consul.Agent.Domain,
		Datacenter:         config.Consul.Agent.Datacenter,
		DataDir:            config.Path.DataDir,
		LogLevel:           config.Consul.Agent.LogLevel,
		NodeName:           nodeName,
		RejoinAfterLeave:   true,
		BindAddr:           config.Node.ExternalIP,
		DisableRemoteExec:  true,
		DisableUpdateCheck: true,
		Protocol:           config.Consul.Agent.ProtocolVersion,
		Ports: ConsulConfigPorts{
			DNS: dns,
		},
		DnsConfig: ConsulConfigDnsConfig{
			AllowStale:      config.Consul.Agent.DnsConfig.AllowStale,
			MaxStale:        config.Consul.Agent.DnsConfig.MaxStale,
			RecursorTimeout: config.Consul.Agent.DnsConfig.RecursorTimeout,
		},
		Performance: ConsulConfigPerformance{
			RaftMultiplier: 1,
		},
		TLSMinVersion: "tls12",
	}

	if config.Consul.Agent.Telemetry.StatsdAddress != "" {
		consulConfig.Telemetry = &ConsulConfigTelemetry{
			StatsdAddress: config.Consul.Agent.Telemetry.StatsdAddress,
		}
	}

	if config.Consul.Agent.RequireSSL {
		consulConfig.Ports.HTTP = -1
		consulConfig.Ports.HTTPS = 8500
	}

	consulConfig.VerifyOutgoing = boolPtr(true)
	consulConfig.VerifyIncoming = boolPtr(true)
	consulConfig.VerifyServerHostname = boolPtr(true)
	certsDir := filepath.Join(configDir, "certs")
	consulConfig.CAFile = strPtr(filepath.Join(certsDir, "ca.crt"))

	if isServer {
		consulConfig.KeyFile = strPtr(filepath.Join(certsDir, "server.key"))
		consulConfig.CertFile = strPtr(filepath.Join(certsDir, "server.crt"))
	} else {
		consulConfig.KeyFile = strPtr(filepath.Join(certsDir, "agent.key"))
		consulConfig.CertFile = strPtr(filepath.Join(certsDir, "agent.crt"))
	}

	if len(config.Consul.EncryptKeys) > 0 {
		consulConfig.Encrypt = encryptKey(config.Consul.EncryptKeys[0])
	}

	if isServer {
		consulConfig.Bootstrap = boolPtr(config.Consul.Agent.Bootstrap)
	}

	return consulConfig
}

func encryptKey(key string) *string {
	decodedKey, err := base64.StdEncoding.DecodeString(key)

	if err != nil || len(decodedKey) != 16 {
		return strPtr(base64.StdEncoding.EncodeToString(pbkdf2.Key([]byte(key), []byte(""), 20000, 16, sha1.New)))
	} else {
		return strPtr(key)
	}
}

func intPtr(i int) *int {
	return &i
}

func boolPtr(b bool) *bool {
	return &b
}

func strPtr(s string) *string {
	return &s
}
