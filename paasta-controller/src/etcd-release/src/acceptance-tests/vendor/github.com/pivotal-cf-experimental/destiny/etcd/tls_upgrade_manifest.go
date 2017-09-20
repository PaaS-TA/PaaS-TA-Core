package etcd

import (
	"github.com/pivotal-cf-experimental/destiny/consul"
	"github.com/pivotal-cf-experimental/destiny/core"
	"github.com/pivotal-cf-experimental/destiny/iaas"
)

func NewTLSUpgradeManifest(config Config, iaasConfig iaas.Config) Manifest {
	config = NewConfigWithDefaults(config)

	releases := []core.Release{
		{
			Name:    "etcd",
			Version: "latest",
		},
		{
			Name:    "consul",
			Version: "latest",
		},
	}

	compilation := core.Compilation{
		Network:             "etcd1",
		ReuseCompilationVMs: true,
		Workers:             3,
		CloudProperties:     iaasConfig.Compilation(),
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

	resourcePools := []core.ResourcePool{
		{
			Name:            "etcd_z1",
			Network:         "etcd1",
			CloudProperties: iaasConfig.ResourcePool(config.IPRange),
			Stemcell:        stemcell,
		},
	}

	cidr, err := core.ParseCIDRBlock(config.IPRange)
	if err != nil {
		return Manifest{}
	}

	networks := []core.Network{
		{
			Name: "etcd1",
			Subnets: []core.NetworkSubnet{{
				CloudProperties: iaasConfig.NetworkSubnet(cidr.String()),
				Gateway:         cidr.GetFirstIP().Add(1).String(),
				Range:           cidr.String(),
				Reserved:        []string{cidr.Range(2, 3), cidr.GetLastIP().String()},
				Static:          []string{cidr.Range(4, cidr.CIDRSize-5)},
			}},
			Type: "manual",
		},
	}

	staticIPs, err := networks[0].StaticIPsFromRange(24)
	if err != nil {
		return Manifest{}
	}

	consulJob := core.Job{
		Name:      "consul_z1",
		Instances: 3,
		Networks: []core.JobNetwork{{
			Name: "etcd1",
			StaticIPs: []string{
				staticIPs[5],
				staticIPs[6],
				staticIPs[7],
			},
		}},
		PersistentDisk: 1024,
		ResourcePool:   "etcd_z1",
		Templates: []core.JobTemplate{
			{
				Name:    "consul_agent",
				Release: "consul",
			},
		},
		Properties: &core.JobProperties{
			Consul: &core.JobPropertiesConsul{
				Agent: core.JobPropertiesConsulAgent{
					Mode: "server",
				},
			},
		},
	}

	etcdTLS := core.Job{
		Name:      "etcd_tls_z1",
		Instances: 3,
		Networks: []core.JobNetwork{{
			Name: "etcd1",
			StaticIPs: []string{
				staticIPs[13],
				staticIPs[14],
				staticIPs[15],
			},
		}},
		PersistentDisk: 1024,
		ResourcePool:   "etcd_z1",
		Templates: []core.JobTemplate{
			{
				Name:    "consul_agent",
				Release: "consul",
			},
			{
				Name:    "etcd",
				Release: "etcd",
			},
		},
		Properties: &core.JobProperties{
			Consul: &core.JobPropertiesConsul{
				Agent: core.JobPropertiesConsulAgent{
					Services: core.JobPropertiesConsulAgentServices{
						"etcd": core.JobPropertiesConsulAgentService{},
					},
				},
			},
			Etcd: &core.JobPropertiesEtcd{
				Cluster: []core.JobPropertiesEtcdCluster{{
					Instances: 3,
					Name:      "etcd_tls_z1",
				}},
				Machines: []string{
					"etcd.service.cf.internal",
				},
				PeerRequireSSL:                  true,
				RequireSSL:                      true,
				HeartbeatIntervalInMilliseconds: 50,
				AdvertiseURLsDNSSuffix:          "etcd.service.cf.internal",
				CACert:                          config.Secrets.Etcd.CACert,
				ClientCert:                      config.Secrets.Etcd.ClientCert,
				ClientKey:                       config.Secrets.Etcd.ClientKey,
				PeerCACert:                      config.Secrets.Etcd.PeerCACert,
				PeerCert:                        config.Secrets.Etcd.PeerCert,
				PeerKey:                         config.Secrets.Etcd.PeerKey,
				ServerCert:                      config.Secrets.Etcd.ServerCert,
				ServerKey:                       config.Secrets.Etcd.ServerKey,
			},
		},
	}

	etcdNoTLS := core.Job{
		Name:      "etcd_z1",
		Instances: 1,
		Networks: []core.JobNetwork{{
			Name: "etcd1",
			StaticIPs: []string{
				staticIPs[0],
			},
		}},
		PersistentDisk: 1024,
		ResourcePool:   "etcd_z1",
		Templates: []core.JobTemplate{
			{
				Name:    "consul_agent",
				Release: "consul",
			},
			{
				Name:    "etcd_proxy",
				Release: "etcd",
			},
		},
	}

	testconsumerJob := core.Job{
		Name:      "testconsumer_z1",
		Instances: 5,
		Networks: []core.JobNetwork{{
			Name: "etcd1",
			StaticIPs: []string{
				staticIPs[8],
				staticIPs[9],
				staticIPs[10],
				staticIPs[11],
				staticIPs[12],
			},
		}},
		PersistentDisk: 1024,
		ResourcePool:   "etcd_z1",
		Templates: []core.JobTemplate{
			{
				Name:    "consul_agent",
				Release: "consul",
			},
			{
				Name:    "etcd_testconsumer",
				Release: "etcd",
			},
		},
		Properties: &core.JobProperties{
			EtcdTestConsumer: &core.JobPropertiesEtcdTestConsumer{
				Etcd: core.JobPropertiesEtcdTestConsumerEtcd{
					Machines: []string{
						"etcd.service.cf.internal",
					},
					RequireSSL: true,
					CACert:     config.Secrets.Etcd.CACert,
					ClientCert: config.Secrets.Etcd.ClientCert,
					ClientKey:  config.Secrets.Etcd.ClientKey,
				},
			},
		},
	}

	properties := Properties{
		Consul: &consul.PropertiesConsul{
			Agent: consul.PropertiesConsulAgent{
				Domain: "cf.internal",
				Servers: consul.PropertiesConsulAgentServers{
					Lan: []string{
						staticIPs[5],
						staticIPs[6],
						staticIPs[7],
					},
				},
			},
			CACert:      config.Secrets.Consul.CACert,
			AgentCert:   config.Secrets.Consul.AgentCert,
			AgentKey:    config.Secrets.Consul.AgentKey,
			ServerCert:  config.Secrets.Consul.ServerCert,
			ServerKey:   config.Secrets.Consul.ServerKey,
			EncryptKeys: []string{config.Secrets.Consul.EncryptKey},
		},
		EtcdProxy: &PropertiesEtcdProxy{
			Port: 4001,
			Etcd: PropertiesEtcdProxyEtcd{
				DNSSuffix:  "etcd.service.cf.internal",
				Port:       4001,
				CACert:     config.Secrets.Etcd.CACert,
				ClientCert: config.Secrets.Etcd.ClientCert,
				ClientKey:  config.Secrets.Etcd.ClientKey,
			},
		},
	}

	return Manifest{
		DirectorUUID:  config.DirectorUUID,
		Name:          config.Name,
		Releases:      releases,
		Compilation:   compilation,
		Update:        update,
		ResourcePools: resourcePools,
		Networks:      networks,
		Jobs: []core.Job{
			consulJob,
			etcdTLS,
			etcdNoTLS,
			testconsumerJob,
		},
		Properties: properties,
	}
}
