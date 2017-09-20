package consul

import (
	"fmt"

	"github.com/pivotal-cf-experimental/destiny/core"
	"github.com/pivotal-cf-experimental/destiny/iaas"
)

func NewManifestWithJobLevelProperties(config ConfigV2, iaasConfig iaas.Config) (ManifestV2, error) {
	manifest, err := NewManifestV2(config, iaasConfig)
	if err != nil {
		return ManifestV2{}, err
	}

	consulInstanceGroup, err := manifest.GetInstanceGroup("consul")
	if err != nil {
		// not tested
		return ManifestV2{}, err
	}
	consulInstanceGroup.Properties.Consul = &core.JobPropertiesConsul{
		Agent: core.JobPropertiesConsulAgent{
			Domain:     manifest.Properties.Consul.Agent.Domain,
			Datacenter: manifest.Properties.Consul.Agent.Datacenter,
			Servers: core.JobPropertiesConsulAgentServers{
				Lan: manifest.Properties.Consul.Agent.Servers.Lan,
			},
			Mode:     consulInstanceGroup.Properties.Consul.Agent.Mode,
			LogLevel: consulInstanceGroup.Properties.Consul.Agent.LogLevel,
			Services: consulInstanceGroup.Properties.Consul.Agent.Services,
		},
		CACert:      manifest.Properties.Consul.CACert,
		ServerCert:  manifest.Properties.Consul.ServerCert,
		ServerKey:   manifest.Properties.Consul.ServerKey,
		EncryptKeys: manifest.Properties.Consul.EncryptKeys,
		AgentCert:   manifest.Properties.Consul.AgentCert,
		AgentKey:    manifest.Properties.Consul.AgentKey,
	}

	consulTestConsumerJob, err := manifest.GetInstanceGroup("test_consumer")
	if err != nil {
		// not tested
		return ManifestV2{}, err
	}
	consulTestConsumerJob.Properties = &core.JobProperties{
		Consul: &core.JobPropertiesConsul{
			Agent: core.JobPropertiesConsulAgent{
				Domain:     manifest.Properties.Consul.Agent.Domain,
				Datacenter: manifest.Properties.Consul.Agent.Datacenter,
				Servers: core.JobPropertiesConsulAgentServers{
					Lan: manifest.Properties.Consul.Agent.Servers.Lan,
				},
			},
			CACert:      manifest.Properties.Consul.CACert,
			AgentCert:   manifest.Properties.Consul.AgentCert,
			AgentKey:    manifest.Properties.Consul.AgentKey,
			EncryptKeys: manifest.Properties.Consul.EncryptKeys,
		},
	}

	manifest.Properties.Consul = nil

	return manifest, nil
}

func findJob(manifest Manifest, name string) (*core.Job, error) {
	for index := range manifest.Jobs {
		if manifest.Jobs[index].Name == name {
			return &manifest.Jobs[index], nil
		}
	}
	return &core.Job{}, fmt.Errorf("%q job does not exist", name)
}
