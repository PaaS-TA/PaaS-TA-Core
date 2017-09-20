package consul

import (
	"github.com/pivotal-cf-experimental/destiny/core"
	"github.com/pivotal-cf-experimental/destiny/iaas"
	"gopkg.in/yaml.v2"
)

type ManifestV2 struct {
	DirectorUUID   string               `yaml:"director_uuid"`
	Name           string               `yaml:"name"`
	Releases       []core.Release       `yaml:"releases"`
	Stemcells      []Stemcell           `yaml:"stemcells"`
	Update         core.Update          `yaml:"update"`
	InstanceGroups []core.InstanceGroup `yaml:"instance_groups"`
	Properties     Properties           `yaml:"properties"`
}

type Stemcell struct {
	Alias   string
	Name    string
	Version string
}

func NewManifestV2(config ConfigV2, iaasConfig iaas.Config) (ManifestV2, error) {
	consulInstanceGroup, err := consulInstanceGroup(config.AZs, config.PersistentDiskType, config.VMType)
	if err != nil {
		return ManifestV2{}, err
	}

	consulTestConsumerInstanceGroup, err := consulTestConsumerInstanceGroup(config.AZs, config.VMType)
	if err != nil {
		return ManifestV2{}, err
	}

	properties, err := properties(config.AZs)
	if err != nil {
		return ManifestV2{}, err
	}

	return ManifestV2{
		DirectorUUID: config.DirectorUUID,
		Name:         config.Name,
		Releases:     releases(),
		Stemcells: []Stemcell{
			{
				Alias:   "default",
				Version: "latest",
				Name:    iaasConfig.Stemcell(),
			},
		},
		Update: update(),
		InstanceGroups: []core.InstanceGroup{
			consulInstanceGroup,
			consulTestConsumerInstanceGroup,
		},
		Properties: properties,
	}, nil
}

func consulInstanceGroup(azs []ConfigAZ, persistentDiskType string, vmType string) (core.InstanceGroup, error) {
	totalNodes := 0
	for _, az := range azs {
		totalNodes += az.Nodes
	}

	if persistentDiskType == "" {
		persistentDiskType = "default"
	}

	if vmType == "" {
		vmType = "default"
	}

	consulInstanceGroupStaticIPs, err := consulInstanceGroupStaticIPs(azs)
	if err != nil {
		return core.InstanceGroup{}, err
	}

	return core.InstanceGroup{
		Instances: totalNodes,
		Name:      "consul",
		AZs:       core.AZs(len(azs)),
		Networks: []core.InstanceGroupNetwork{
			{
				Name:      "private",
				StaticIPs: consulInstanceGroupStaticIPs,
			},
		},
		VMType:             vmType,
		Stemcell:           "default",
		PersistentDiskType: persistentDiskType,
		Jobs: []core.InstanceGroupJob{
			{
				Name:    "consul_agent",
				Release: "consul",
			},
		},
		MigratedFrom: []core.InstanceGroupMigratedFrom{
			{
				Name: "consul_z1",
				AZ:   "z1",
			},
			{
				Name: "consul_z2",
				AZ:   "z2",
			},
		},
		Properties: core.InstanceGroupProperties{
			Consul: core.InstanceGroupPropertiesConsul{
				Agent: core.InstanceGroupPropertiesConsulAgent{
					Mode:     "server",
					LogLevel: "info",
					Services: map[string]core.InstanceGroupPropertiesConsulAgentService{
						"router": core.InstanceGroupPropertiesConsulAgentService{
							Name: "gorouter",
							Check: core.InstanceGroupPropertiesConsulAgentServiceCheck{
								Name:     "router-check",
								Script:   "/var/vcap/jobs/router/bin/script",
								Interval: "1m",
							},
							Tags: []string{"routing"},
						},
						"cloud_controller": core.InstanceGroupPropertiesConsulAgentService{},
					},
				},
			},
		},
	}, nil
}

func consulTestConsumerInstanceGroup(azs []ConfigAZ, vmType string) (core.InstanceGroup, error) {
	cidr, err := core.ParseCIDRBlock(azs[0].IPRange)
	if err != nil {
		return core.InstanceGroup{}, err
	}

	if vmType == "" {
		vmType = "default"
	}

	return core.InstanceGroup{
		Instances: 3,
		Name:      "test_consumer",
		AZs:       []string{azs[0].Name},
		Networks: []core.InstanceGroupNetwork{
			{
				Name: "private",
				StaticIPs: []string{
					cidr.GetFirstIP().Add(10).String(),
					cidr.GetFirstIP().Add(11).String(),
					cidr.GetFirstIP().Add(12).String(),
				},
			},
		},
		VMType:   vmType,
		Stemcell: "default",
		Jobs: []core.InstanceGroupJob{
			{
				Name:    "consul_agent",
				Release: "consul",
			},
			{
				Name:    "consul-test-consumer",
				Release: "consul",
			},
		},
		MigratedFrom: []core.InstanceGroupMigratedFrom{
			{
				Name: "consul_test_consumer",
				AZ:   "z1",
			},
		},
	}, nil
}

func properties(azs []ConfigAZ) (Properties, error) {
	consulInstanceGroupStaticIPs, err := consulInstanceGroupStaticIPs(azs)
	if err != nil {
		return Properties{}, err
	}

	return Properties{
		Consul: &PropertiesConsul{
			Agent: PropertiesConsulAgent{
				Domain:     "cf.internal",
				Datacenter: "dc1",
				Servers: PropertiesConsulAgentServers{
					Lan: consulInstanceGroupStaticIPs,
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
		},
	}, nil
}

func consulInstanceGroupStaticIPs(azs []ConfigAZ) ([]string, error) {
	staticIPs := []string{}
	for _, cfgAZs := range azs {
		cidr, err := core.ParseCIDRBlock(cfgAZs.IPRange)
		if err != nil {
			return []string{}, err
		}
		for n := 0; n < cfgAZs.Nodes; n++ {
			staticIPs = append(staticIPs, cidr.GetFirstIP().Add(4+n).String())
		}
	}

	return staticIPs, nil
}

func (m ManifestV2) ToYAML() ([]byte, error) {
	return yaml.Marshal(m)
}
