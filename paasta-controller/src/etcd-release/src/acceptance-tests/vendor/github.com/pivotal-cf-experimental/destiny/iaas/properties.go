package iaas

import "github.com/pivotal-cf-experimental/destiny/core"

type Properties struct {
	WardenCPI *PropertiesWardenCPI      `yaml:"warden_cpi,omitempty"`
	AWS       *PropertiesAWS            `yaml:"aws,omitempty"`
	Registry  *core.PropertiesRegistry  `yaml:"registry,omitempty"`
	Blobstore *core.PropertiesBlobstore `yaml:"blobstore,omitempty"`
	Agent     *core.PropertiesAgent     `yaml:"agent,omitempty"`
}

type PropertiesWardenCPI struct {
	Agent  PropertiesWardenCPIAgent  `yaml:"agent"`
	Warden PropertiesWardenCPIWarden `yaml:"warden"`
}

type PropertiesAWS struct {
	AccessKeyID           string   `yaml:"access_key_id"`
	SecretAccessKey       string   `yaml:"secret_access_key"`
	DefaultKeyName        string   `yaml:"default_key_name"`
	DefaultSecurityGroups []string `yaml:"default_security_groups"`
	Region                string   `yaml:"region"`
}

type PropertiesWardenCPIAgent struct {
	Blobstore PropertiesWardenCPIAgentBlobstore `yaml:"blobstore"`
	Mbus      string                            `yaml:"mbus"`
}

type PropertiesWardenCPIAgentBlobstore struct {
	Options  PropertiesWardenCPIAgentBlobstoreOptions `yaml:"options"`
	Provider string                                   `yaml:"provider"`
}

type PropertiesWardenCPIAgentBlobstoreOptions struct {
	Endpoint string `yaml:"endpoint"`
	Password string `yaml:"password"`
	User     string `yaml:"user"`
}

type PropertiesWardenCPIWarden struct {
	ConnectAddress string `yaml:"connect_address"`
	ConnectNetwork string `yaml:"connect_network"`
}
