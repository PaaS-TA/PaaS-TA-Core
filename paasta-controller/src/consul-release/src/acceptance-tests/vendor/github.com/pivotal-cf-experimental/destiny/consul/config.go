package consul

import "github.com/pivotal-cf-experimental/destiny/core"

type Config struct {
	DirectorUUID   string
	Name           string
	ConsulSecrets  ConfigSecretsConsul
	DC             string
	Networks       []ConfigNetwork
	TurbulenceHost string
}

type ConfigNetwork struct {
	IPRange string
	Nodes   int
}

type ConfigSecretsConsul struct {
	EncryptKey string
	AgentKey   string
	AgentCert  string
	ServerKey  string
	ServerCert string
	CACert     string
}

func (c *Config) PopulateDefaultConfigNodes() {
	for i, _ := range c.Networks {
		if c.Networks[i].Nodes == 0 {
			c.Networks[i].Nodes = 1
		}
	}
}

func (config Config) GetCIDRBlocks() ([]core.CIDRBlock, error) {
	cidrBlocks := []core.CIDRBlock{}
	for _, cfgNetwork := range config.Networks {
		cidr, err := core.ParseCIDRBlock(cfgNetwork.IPRange)
		if err != nil {
			return nil, err
		}
		cidrBlocks = append(cidrBlocks, cidr)
	}
	return cidrBlocks, nil
}
