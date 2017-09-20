package etcd

import (
	"github.com/pivotal-cf-experimental/destiny/consul"
	"github.com/pivotal-cf-experimental/destiny/core"
	"github.com/pivotal-cf-experimental/destiny/iaas"
	"github.com/pivotal-cf-experimental/destiny/turbulence"
	"gopkg.in/yaml.v2"
)

type Manifest struct {
	DirectorUUID  string              `yaml:"director_uuid"`
	Name          string              `yaml:"name"`
	Jobs          []core.Job          `yaml:"jobs"`
	Properties    Properties          `yaml:"properties"`
	Update        core.Update         `yaml:"update"`
	Compilation   core.Compilation    `yaml:"compilation"`
	Networks      []core.Network      `yaml:"networks"`
	Releases      []core.Release      `yaml:"releases"`
	ResourcePools []core.ResourcePool `yaml:"resource_pools"`
}

type EtcdMember struct {
	Address string
}

func NewTLSManifest(config Config, iaasConfig iaas.Config) (Manifest, error) {
	config = NewConfigWithDefaults(config)

	manifest, err := NewManifest(config, iaasConfig)
	if err != nil {
		return Manifest{}, err
	}

	consulStaticIP, err := manifest.Networks[0].StaticIPsFromRange(24)
	if err != nil {
		return Manifest{}, err
	}

	manifest.Jobs = append([]core.Job{
		{
			Name:      "consul_z1",
			Instances: 1,
			Networks: []core.JobNetwork{{
				Name:      manifest.Networks[0].Name,
				StaticIPs: []string{consulStaticIP[5]},
			}},
			PersistentDisk: 1024,
			ResourcePool:   manifest.ResourcePools[0].Name,
			Properties: &core.JobProperties{
				Consul: &core.JobPropertiesConsul{
					Agent: core.JobPropertiesConsulAgent{
						Mode: "server",
					},
				},
			},
			Templates: []core.JobTemplate{
				{
					Name:    "consul_agent",
					Release: "consul",
				},
			},
		},
	}, manifest.Jobs...)

	for i, job := range manifest.Jobs {
		switch job.Name {
		case "etcd_z1":
			job.Templates = append([]core.JobTemplate{
				{
					Name:    "consul_agent",
					Release: "consul",
				}}, job.Templates...)
			job.Properties = &core.JobProperties{
				Consul: &core.JobPropertiesConsul{
					Agent: core.JobPropertiesConsulAgent{
						Services: core.JobPropertiesConsulAgentServices{
							"etcd": core.JobPropertiesConsulAgentService{},
						},
					},
				},
			}

		case "testconsumer_z1":
			job.Templates = append([]core.JobTemplate{
				{
					Name:    "consul_agent",
					Release: "consul",
				}}, job.Templates...)
		}

		manifest.Jobs[i] = job
	}

	manifest.Properties.EtcdTestConsumer.Etcd.RequireSSL = true
	manifest.Properties.EtcdTestConsumer.Etcd.Machines = []string{"etcd.service.cf.internal"}
	manifest.Properties.EtcdTestConsumer.Etcd.CACert = config.Secrets.Etcd.CACert
	manifest.Properties.EtcdTestConsumer.Etcd.ClientCert = config.Secrets.Etcd.ClientCert
	manifest.Properties.EtcdTestConsumer.Etcd.ClientKey = config.Secrets.Etcd.ClientKey

	manifest.Properties.Etcd.Machines = []string{"etcd.service.cf.internal"}
	manifest.Properties.Etcd.PeerRequireSSL = true
	manifest.Properties.Etcd.RequireSSL = true
	manifest.Properties.Etcd.AdvertiseURLsDNSSuffix = "etcd.service.cf.internal"
	manifest.Properties.Etcd.CACert = config.Secrets.Etcd.CACert
	manifest.Properties.Etcd.ClientCert = config.Secrets.Etcd.ClientCert
	manifest.Properties.Etcd.ClientKey = config.Secrets.Etcd.ClientKey
	manifest.Properties.Etcd.PeerCACert = config.Secrets.Etcd.PeerCACert
	manifest.Properties.Etcd.PeerCert = config.Secrets.Etcd.PeerCert
	manifest.Properties.Etcd.PeerKey = config.Secrets.Etcd.PeerKey
	manifest.Properties.Etcd.ServerCert = config.Secrets.Etcd.ServerCert
	manifest.Properties.Etcd.ServerKey = config.Secrets.Etcd.ServerKey

	manifest.Properties.Consul = &consul.PropertiesConsul{
		Agent: consul.PropertiesConsulAgent{
			Domain: "cf.internal",
			Servers: consul.PropertiesConsulAgentServers{
				Lan: []string{consulStaticIP[5]},
			},
		},
		CACert:      config.Secrets.Consul.CACert,
		AgentCert:   config.Secrets.Consul.AgentCert,
		AgentKey:    config.Secrets.Consul.AgentKey,
		ServerCert:  config.Secrets.Consul.ServerCert,
		ServerKey:   config.Secrets.Consul.ServerKey,
		EncryptKeys: []string{config.Secrets.Consul.EncryptKey},
	}

	manifest.Releases = append(manifest.Releases, core.Release{
		Name:    "consul",
		Version: "latest",
	})

	return manifest, nil
}

func NewManifest(config Config, iaasConfig iaas.Config) (Manifest, error) {
	config = NewConfigWithDefaults(config)

	releases := []core.Release{
		{
			Name:    "etcd",
			Version: "latest",
		},
	}

	cidr, err := core.ParseCIDRBlock(config.IPRange)
	if err != nil {
		return Manifest{}, err
	}

	etcdNetwork1 := core.Network{
		Name: "etcd1",
		Subnets: []core.NetworkSubnet{{
			CloudProperties: iaasConfig.NetworkSubnet(cidr.String()),
			Gateway:         cidr.GetFirstIP().Add(1).String(),
			Range:           cidr.String(),
			Reserved:        []string{cidr.Range(2, 3), cidr.GetLastIP().String()},
			Static:          []string{cidr.Range(4, cidr.CIDRSize-5)},
		}},
		Type: "manual",
	}

	compilation := core.Compilation{
		Network:             etcdNetwork1.Name,
		ReuseCompilationVMs: true,
		Workers:             3,
		CloudProperties:     iaasConfig.Compilation("us-east-1a"),
	}

	update := core.Update{
		Canaries:        1,
		CanaryWatchTime: "1000-180000",
		MaxInFlight:     1,
		Serial:          true,
		UpdateWatchTime: "1000-180000",
	}

	stemcell := core.ResourcePoolStemcell{
		Name:    iaasConfig.Stemcell(),
		Version: "latest",
	}

	z1ResourcePool := core.ResourcePool{
		Name:            "etcd_z1",
		Network:         etcdNetwork1.Name,
		Stemcell:        stemcell,
		CloudProperties: iaasConfig.ResourcePool(etcdNetwork1.Subnets[0].Range),
	}

	staticIPs, err := etcdNetwork1.StaticIPsFromRange(24)
	if err != nil {
		return Manifest{}, err
	}

	etcdZ1JobTemplates := []core.JobTemplate{
		{
			Name:    "etcd",
			Release: "etcd",
		},
	}

	etcdZ1Job := core.Job{
		Name:      "etcd_z1",
		Instances: 1,
		Networks: []core.JobNetwork{{
			Name:      etcdNetwork1.Name,
			StaticIPs: []string{staticIPs[0]},
		}},
		PersistentDisk: 1024,
		ResourcePool:   z1ResourcePool.Name,
		Templates:      etcdZ1JobTemplates,
	}

	if config.IPTablesAgent {
		etcdZ1Job.Templates = append(etcdZ1Job.Templates, core.JobTemplate{
			Name:    "iptables_agent",
			Release: "etcd",
		})
	}

	testconsumerZ1Job := core.Job{
		Name:      "testconsumer_z1",
		Instances: 1,
		Networks: []core.JobNetwork{{
			Name:      etcdNetwork1.Name,
			StaticIPs: []string{staticIPs[8]},
		}},
		PersistentDisk: 1024,
		ResourcePool:   z1ResourcePool.Name,
		Templates: []core.JobTemplate{
			{
				Name:    "etcd_testconsumer",
				Release: "etcd",
			},
		},
	}

	globalProperties := Properties{
		Etcd: &PropertiesEtcd{
			Cluster: []PropertiesEtcdCluster{{
				Instances: 1,
				Name:      "etcd_z1",
			}},
			Machines:                        etcdZ1Job.Networks[0].StaticIPs,
			PeerRequireSSL:                  false,
			RequireSSL:                      false,
			HeartbeatIntervalInMilliseconds: 50,
		},
		EtcdTestConsumer: &PropertiesEtcdTestConsumer{
			Etcd: PropertiesEtcdTestConsumerEtcd{
				Machines: etcdZ1Job.Networks[0].StaticIPs,
			},
		},
	}

	if config.TurbulenceHost != "" {
		globalProperties.TurbulenceAgent = &core.PropertiesTurbulenceAgent{
			API: core.PropertiesTurbulenceAgentAPI{
				Host:     config.TurbulenceHost,
				Password: turbulence.DefaultPassword,
				CACert:   turbulence.APICACert,
			},
		}

		etcdZ1Job.Templates = append(etcdZ1Job.Templates, core.JobTemplate{
			Name:    "turbulence_agent",
			Release: "turbulence",
		})

		releases = append(releases, core.Release{
			Name:    "turbulence",
			Version: "latest",
		})
	}

	return Manifest{
		DirectorUUID: config.DirectorUUID,
		Name:         config.Name,
		Compilation:  compilation,
		Jobs: []core.Job{
			etcdZ1Job,
			testconsumerZ1Job,
		},
		Networks: []core.Network{
			etcdNetwork1,
		},
		Properties: globalProperties,
		Releases:   releases,
		ResourcePools: []core.ResourcePool{
			z1ResourcePool,
		},
		Update: update,
	}, nil
}

func (m Manifest) EtcdMembers() []EtcdMember {
	members := []EtcdMember{}
	for _, job := range m.Jobs {
		if len(job.Networks) == 0 {
			continue
		}

		if job.HasTemplate("etcd", "etcd") {
			for i := 0; i < job.Instances; i++ {
				if len(job.Networks[0].StaticIPs) > i {
					members = append(members, EtcdMember{
						Address: job.Networks[0].StaticIPs[i],
					})
				}
			}
		}
	}

	return members
}

func (m Manifest) ToYAML() ([]byte, error) {
	return yaml.Marshal(m)
}

func FromYAML(manifestYAML []byte) (Manifest, error) {
	var m Manifest
	if err := yaml.Unmarshal(manifestYAML, &m); err != nil {
		return m, err
	}
	return m, nil
}
