package consul

import (
	"fmt"

	"github.com/pivotal-cf-experimental/destiny/core"
	"github.com/pivotal-cf-experimental/destiny/iaas"
	"gopkg.in/yaml.v2"
)

type Manifest struct {
	DirectorUUID  string              `yaml:"director_uuid"`
	Name          string              `yaml:"name"`
	Jobs          []core.Job          `yaml:"jobs"`
	Properties    Properties          `yaml:"properties"`
	Compilation   core.Compilation    `yaml:"compilation"`
	Update        core.Update         `yaml:"update"`
	Networks      []core.Network      `yaml:"networks"`
	Releases      []core.Release      `yaml:"releases"`
	ResourcePools []core.ResourcePool `yaml:"resource_pools"`
}

type ConsulMember struct {
	Address string
}

func NewManifest(config Config, iaasConfig iaas.Config) (Manifest, error) {
	config.PopulateDefaultConfigNodes()

	cidrBlocks, err := config.GetCIDRBlocks()
	if err != nil {
		return Manifest{}, err
	}

	consulNetworks := []core.Network{}
	for i, cidrBlock := range cidrBlocks {
		consulNetwork := core.Network{
			Name: fmt.Sprintf("consul%d", i+1),
			Subnets: []core.NetworkSubnet{{
				CloudProperties: iaasConfig.NetworkSubnet(cidrBlock.String()),
				Gateway:         cidrBlock.GetFirstIP().Add(1).String(),
				Range:           cidrBlock.String(),
				Reserved:        []string{cidrBlock.Range(2, 3), cidrBlock.GetLastIP().String()},
				Static:          []string{cidrBlock.Range(4, cidrBlock.CIDRSize-5)},
			}},
			Type: "manual",
		}
		consulNetworks = append(consulNetworks, consulNetwork)
	}

	compilation := core.Compilation{
		Network:             consulNetworks[0].Name,
		ReuseCompilationVMs: true,
		Workers:             3,
		CloudProperties:     iaasConfig.Compilation("us-west-2a"),
	}

	stemcell := core.ResourcePoolStemcell{
		Name:    iaasConfig.Stemcell(),
		Version: "latest",
	}

	resourcePools := []core.ResourcePool{}
	for i, network := range consulNetworks {
		resourcePool := core.ResourcePool{
			Name:            fmt.Sprintf("consul_z%d", i+1),
			Network:         network.Name,
			Stemcell:        stemcell,
			CloudProperties: iaasConfig.ResourcePool(network.Subnets[0].Range),
		}
		resourcePools = append(resourcePools, resourcePool)
	}

	jobs := []core.Job{}
	consulClusterStaticIPs := []string{}

	for i := range consulNetworks {
		instances := config.Networks[i].Nodes

		staticIps, err := consulNetworks[i].StaticIPsFromRange(instances)
		if err != nil {
			return Manifest{}, err
		}

		job := core.Job{
			Name:      fmt.Sprintf("consul_z%d", i+1),
			Instances: instances,
			Networks: []core.JobNetwork{{
				Name:      consulNetworks[i].Name,
				StaticIPs: staticIps,
			}},
			PersistentDisk: 1024,
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
			ResourcePool: resourcePools[i].Name,
			Templates: []core.JobTemplate{{
				Name:    "consul_agent",
				Release: "consul",
			}},
			Update: &core.JobUpdate{
				MaxInFlight: 1,
			},
		}

		jobs = append(jobs, job)
		consulClusterStaticIPs = append(consulClusterStaticIPs, staticIps...)
	}

	staticIps, err := consulNetworks[0].StaticIPsFromRange(9)
	if err != nil {
		return Manifest{}, err
	}

	jobs = append(jobs, core.Job{
		Name:      "consul_test_consumer",
		Instances: 1,
		Networks: []core.JobNetwork{{
			Name: consulNetworks[0].Name,
			StaticIPs: []string{
				staticIps[6],
			},
		}},
		Properties: &core.JobProperties{
			Consul: &core.JobPropertiesConsul{
				Agent: core.JobPropertiesConsulAgent{
					Mode:     "client",
					LogLevel: "info",
				},
			},
		},
		ResourcePool: resourcePools[0].Name,
		Templates: []core.JobTemplate{
			{
				Name:    "consul_agent",
				Release: "consul",
			},
			{
				Name:    "consul-test-consumer",
				Release: "consul",
			},
		},
	})

	properties := Properties{
		Consul: &PropertiesConsul{
			Agent: PropertiesConsulAgent{
				Domain: "cf.internal",
				Servers: PropertiesConsulAgentServers{
					Lan: consulClusterStaticIPs,
				},
				DNSConfig: PropertiesConsulAgentDNSConfig{
					RecursorTimeout: "5s",
				},
			},
			EncryptKeys: []string{EncryptKey},
		},
	}

	overrideTLS(properties.Consul, config.DC)

	return Manifest{
		DirectorUUID:  config.DirectorUUID,
		Name:          config.Name,
		Releases:      releases(),
		Update:        update(),
		Compilation:   compilation,
		ResourcePools: resourcePools,
		Jobs:          jobs,
		Networks:      consulNetworks,
		Properties:    properties,
	}, nil
}

func (m Manifest) ConsulMembers() []ConsulMember {
	members := []ConsulMember{}
	for _, job := range m.Jobs {
		if len(job.Networks) == 0 {
			continue
		}

		for i := 0; i < job.Instances; i++ {
			if len(job.Networks[0].StaticIPs) > i {
				members = append(members, ConsulMember{
					Address: job.Networks[0].StaticIPs[i],
				})
			}
		}
	}

	return members
}

func (m Manifest) ToYAML() ([]byte, error) {
	return yaml.Marshal(m)
}

func FromYAML(manifestYAML []byte, m interface{}) error {
	if err := yaml.Unmarshal(manifestYAML, m); err != nil {
		return err
	}

	return nil
}

func overrideTLS(properties *PropertiesConsul, dc string) {
	switch dc {
	case "dc1":
		properties.Agent.Datacenter = dc
		properties.AgentCert = DC1AgentCert
		properties.AgentKey = DC1AgentKey
		properties.ServerCert = DC1ServerCert
		properties.ServerKey = DC1ServerKey
	case "dc2":
		properties.Agent.Datacenter = dc
		properties.AgentCert = DC2AgentCert
		properties.AgentKey = DC2AgentKey
		properties.ServerCert = DC2ServerCert
		properties.ServerKey = DC2ServerKey
	case "dc3":
		properties.Agent.Datacenter = dc
		properties.AgentCert = DC3AgentCert
		properties.AgentKey = DC3AgentKey
		properties.ServerCert = DC3ServerCert
		properties.ServerKey = DC3ServerKey
	default:
		properties.Agent.Datacenter = "dc1"
		properties.AgentCert = DC1AgentCert
		properties.AgentKey = DC1AgentKey
		properties.ServerCert = DC1ServerCert
		properties.ServerKey = DC1ServerKey
	}

	properties.CACert = CACert
}
