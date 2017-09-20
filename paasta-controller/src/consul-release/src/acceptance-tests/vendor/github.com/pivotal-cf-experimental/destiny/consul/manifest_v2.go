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
	Stemcells      []core.Stemcell      `yaml:"stemcells"`
	Update         core.Update          `yaml:"update"`
	InstanceGroups []core.InstanceGroup `yaml:"instance_groups"`
	Properties     Properties           `yaml:"properties"`
}

func NewManifestV2(config ConfigV2, iaasConfig iaas.Config) (ManifestV2, error) {
	manifest := ManifestV2{
		DirectorUUID:   config.DirectorUUID,
		Name:           config.Name,
		Releases:       releases(),
		Update:         update(),
		InstanceGroups: []core.InstanceGroup{},
		Properties:     Properties{},
	}

	if config.WindowsClients {
		manifest.Stemcells = []core.Stemcell{
			{
				Alias:   "linux",
				Version: "latest",
				Name:    iaasConfig.Stemcell(),
			},
			{
				Alias:   "windows",
				Version: "latest",
				Name:    iaasConfig.WindowsStemcell(),
			},
		}
	} else {
		manifest.Stemcells = []core.Stemcell{
			{
				Alias:   "default",
				Version: "latest",
				Name:    iaasConfig.Stemcell(),
			},
		}
	}

	consulInstanceGroup, err := consulInstanceGroup(config)
	if err != nil {
		return ManifestV2{}, err
	}
	manifest.InstanceGroups = append(manifest.InstanceGroups, consulInstanceGroup)

	consulTestConsumerInstanceGroup, err := consulTestConsumerInstanceGroup(config)
	if err != nil {
		return ManifestV2{}, err
	}
	manifest.InstanceGroups = append(manifest.InstanceGroups, consulTestConsumerInstanceGroup)

	manifest.Properties.Consul = newConsulProperties(consulInstanceGroup.Networks[0].StaticIPs)

	return manifest, nil
}

func consulInstanceGroup(config ConfigV2) (core.InstanceGroup, error) {
	persistentDiskType := config.PersistentDiskType
	azs := config.AZs
	vmType := config.VMType
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

	stemcell := "default"
	if config.WindowsClients {
		stemcell = "linux"
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
		Stemcell:           stemcell,
		PersistentDiskType: persistentDiskType,
		Jobs: []core.InstanceGroupJob{
			{
				Name:    "consul_agent",
				Release: "consul",
			},
		},
		Properties: &core.JobProperties{
			Consul: &core.JobPropertiesConsul{
				Agent: core.JobPropertiesConsulAgent{
					Mode:     "server",
					LogLevel: "info",
					Services: core.JobPropertiesConsulAgentServices{
						"router": core.JobPropertiesConsulAgentService{
							Name: "gorouter",
							Check: &core.JobPropertiesConsulAgentServiceCheck{
								Name:     "router-check",
								Script:   "/var/vcap/jobs/router/bin/script",
								Interval: "1m",
							},
							Tags: []string{"routing"},
						},
						"cloud_controller": core.JobPropertiesConsulAgentService{},
					},
				},
			},
		},
	}, nil
}

func consulTestConsumerInstanceGroup(config ConfigV2) (core.InstanceGroup, error) {
	cidr, err := core.ParseCIDRBlock(config.AZs[0].IPRange)
	if err != nil {
		return core.InstanceGroup{}, err
	}

	if config.VMType == "" {
		config.VMType = "default"
	}

	stemcell := "default"
	agentName := "consul_agent"
	testConsumerName := "consul-test-consumer"
	if config.WindowsClients {
		stemcell = "windows"
		agentName = "consul_agent_windows"
		testConsumerName = "consul-test-consumer-windows"
	}

	return core.InstanceGroup{
		Instances: 1,
		Name:      "test_consumer",
		AZs:       []string{config.AZs[0].Name},
		Networks: []core.InstanceGroupNetwork{
			{
				Name: "private",
				StaticIPs: []string{
					cidr.GetFirstIP().Add(10).String(),
				},
			},
		},
		VMType:   config.VMType,
		Stemcell: stemcell,
		Jobs: []core.InstanceGroupJob{
			{
				Name:    agentName,
				Release: "consul",
			},
			{
				Name:    testConsumerName,
				Release: "consul",
			},
		},
	}, nil
}

func consulInstanceGroupStaticIPs(azs []ConfigAZ) ([]string, error) {
	staticIPs := []string{}
	for _, cfgAZs := range azs {
		ips, err := cfgAZs.StaticIPs()
		if err != nil {
			return []string{}, err
		}
		staticIPs = append(staticIPs, ips...)
	}
	return staticIPs, nil
}

func (m ManifestV2) ToYAML() ([]byte, error) {
	return yaml.Marshal(m)
}
