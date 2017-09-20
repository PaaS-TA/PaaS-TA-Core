package etcd

import (
	"github.com/pivotal-cf-experimental/destiny/consul"
)

type Config struct {
	DirectorUUID   string
	Name           string
	IPRange        string
	Secrets        ConfigSecrets
	TurbulenceHost string
	IPTablesAgent  bool
}

type ConfigSecrets struct {
	Etcd   ConfigSecretsEtcd
	Consul consul.ConfigSecretsConsul
}

type ConfigSecretsEtcd struct {
	CACert     string
	ClientCert string
	ClientKey  string
	PeerCACert string
	PeerCert   string
	PeerKey    string
	ServerCert string
	ServerKey  string
}

func NewConfigWithDefaults(config Config) Config {
	if config.Secrets.Consul.CACert == "" {
		config.Secrets.Consul.CACert = consul.CACert
	}

	if config.Secrets.Consul.EncryptKey == "" {
		config.Secrets.Consul.EncryptKey = consul.EncryptKey
	}

	if config.Secrets.Consul.AgentKey == "" {
		config.Secrets.Consul.AgentKey = consul.DC1AgentKey
	}

	if config.Secrets.Consul.AgentCert == "" {
		config.Secrets.Consul.AgentCert = consul.DC1AgentCert
	}

	if config.Secrets.Consul.ServerKey == "" {
		config.Secrets.Consul.ServerKey = consul.DC1ServerKey
	}

	if config.Secrets.Consul.ServerCert == "" {
		config.Secrets.Consul.ServerCert = consul.DC1ServerCert
	}

	if config.Secrets.Etcd.CACert == "" {
		config.Secrets.Etcd.CACert = CACert
	}

	if config.Secrets.Etcd.ClientCert == "" {
		config.Secrets.Etcd.ClientCert = ClientCert
	}

	if config.Secrets.Etcd.ClientKey == "" {
		config.Secrets.Etcd.ClientKey = ClientKey
	}

	if config.Secrets.Etcd.PeerCACert == "" {
		config.Secrets.Etcd.PeerCACert = PeerCACert
	}

	if config.Secrets.Etcd.PeerCert == "" {
		config.Secrets.Etcd.PeerCert = PeerCert
	}

	if config.Secrets.Etcd.PeerKey == "" {
		config.Secrets.Etcd.PeerKey = PeerKey
	}

	if config.Secrets.Etcd.ServerCert == "" {
		config.Secrets.Etcd.ServerCert = ServerCert
	}

	if config.Secrets.Etcd.ServerKey == "" {
		config.Secrets.Etcd.ServerKey = ServerKey
	}

	return config
}
