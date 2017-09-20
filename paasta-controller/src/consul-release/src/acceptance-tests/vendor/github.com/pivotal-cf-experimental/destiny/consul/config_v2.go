package consul

import "github.com/pivotal-cf-experimental/destiny/core"

type ConfigV2 struct {
	DirectorUUID       string
	Name               string
	AZs                []ConfigAZ
	PersistentDiskType string
	VMType             string
	TurbulenceHost     string
	WindowsClients     bool
}

type ConfigAZ struct {
	Name    string
	IPRange string
	Nodes   int
}

func (c *ConfigV2) PopulateDefaultConfigNodes() {
	for i, _ := range c.AZs {
		if c.AZs[i].Nodes == 0 {
			c.AZs[i].Nodes = 1
		}
	}
}

func (cfgAZ ConfigAZ) StaticIPs() ([]string, error) {
	staticIPs := []string{}
	cidr, err := core.ParseCIDRBlock(cfgAZ.IPRange)
	if err != nil {
		return []string{}, err
	}
	for n := 0; n < cfgAZ.Nodes; n++ {
		staticIPs = append(staticIPs, cidr.GetFirstIP().Add(4+n).String())
	}
	return staticIPs, nil
}
