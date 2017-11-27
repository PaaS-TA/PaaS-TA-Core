package consul

import (
	"github.com/pivotal-cf-experimental/destiny/core"
	"github.com/pivotal-cf-experimental/destiny/iaas"
)

type Properties struct {
	Consul             *PropertiesConsul               `yaml:"consul,omitempty"`
	WardenCPI          *iaas.PropertiesWardenCPI       `yaml:"warden_cpi,omitempty"`
	AWS                *iaas.PropertiesAWS             `yaml:"aws,omitempty"`
	Registry           *core.PropertiesRegistry        `yaml:"registry,omitempty"`
	Blobstore          *core.PropertiesBlobstore       `yaml:"blobstore,omitempty"`
	Agent              *core.PropertiesAgent           `yaml:"agent,omitempty"`
	TurbulenceAgent    *core.PropertiesTurbulenceAgent `yaml:"turbulence_agent,omitempty"`
	ConsulTestConsumer *core.ConsulTestConsumer        `yaml:"consul-test-consumer,omitempty"`
}

type PropertiesConsul struct {
	Agent       PropertiesConsulAgent `yaml:"agent"`
	CACert      string                `yaml:"ca_cert"`
	AgentCert   string                `yaml:"agent_cert"`
	AgentKey    string                `yaml:"agent_key"`
	ServerCert  string                `yaml:"server_cert"`
	ServerKey   string                `yaml:"server_key"`
	EncryptKeys []string              `yaml:"encrypt_keys"`
	RequireSSL  bool                  `yaml:"require_ssl,omitempty"`
}

type PropertiesConsulAgent struct {
	Domain     string                         `yaml:"domain"`
	LogLevel   string                         `yaml:"log_level,omitempty"`
	Servers    PropertiesConsulAgentServers   `yaml:"servers"`
	Mode       string                         `yaml:"mode,omitempty"`
	Datacenter string                         `yaml:"datacenter,omitempty"`
	DNSConfig  PropertiesConsulAgentDNSConfig `yaml:"dns_config,omitempty"`
	RequireSSL bool                           `yaml:"require_ssl,omitempty"`
}

type PropertiesConsulAgentServers struct {
	Lan []string `yaml:"lan"`
}

type PropertiesConsulAgentDNSConfig struct {
	RecursorTimeout string `yaml:"recursor_timeout"`
}

func newConsulProperties(staticIPs []string) *PropertiesConsul {
	return &PropertiesConsul{
		Agent: PropertiesConsulAgent{
			Domain:     "cf.internal",
			Datacenter: "dc1",
			Servers: PropertiesConsulAgentServers{
				Lan: staticIPs,
			},
		},
		AgentCert: DC1AgentCert,
		AgentKey:  DC1AgentKey,
		CACert:    CACert,
		EncryptKeys: []string{
			EncryptKey,
		},
		ServerCert: DC1ServerCert,
		ServerKey:  DC1ServerKey,
	}
}
