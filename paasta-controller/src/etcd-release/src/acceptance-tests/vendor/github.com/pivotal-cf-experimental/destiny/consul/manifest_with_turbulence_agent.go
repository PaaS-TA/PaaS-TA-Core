package consul

import (
	"github.com/pivotal-cf-experimental/destiny/core"
	"github.com/pivotal-cf-experimental/destiny/iaas"
	"github.com/pivotal-cf-experimental/destiny/turbulence"
)

func NewManifestWithTurbulenceAgent(config Config, iaasConfig iaas.Config) (Manifest, error) {
	manifest, err := NewManifest(config, iaasConfig)
	if err != nil {
		return Manifest{}, err
	}
	consulTestConsumerJob, err := findJob(manifest, "consul_test_consumer")
	if err != nil {
		// not tested
		return Manifest{}, err
	}

	consulTestConsumerJob.Templates = append(consulTestConsumerJob.Templates, core.JobTemplate{
		Name:    "turbulence_agent",
		Release: "turbulence",
	})

	manifest.Releases = append(manifest.Releases, core.Release{
		Name:    "turbulence",
		Version: "latest",
	})

	staticIpForAddTestHost, err := manifest.Networks[0].StaticIPsFromRange(10)
	if err != nil {
		// not tested
		return Manifest{}, err
	}

	manifest.Jobs = append(manifest.Jobs, core.Job{
		Name:         "fake-dns-server",
		Instances:    1,
		ResourcePool: manifest.Jobs[0].ResourcePool,
		Networks: []core.JobNetwork{
			{
				Name: manifest.Networks[0].Name,
				StaticIPs: []string{
					staticIpForAddTestHost[9],
				},
			},
		},
		Templates: []core.JobTemplate{
			{
				Name:    "turbulence_agent",
				Release: "turbulence",
			},
			{
				Name:    "fake-dns-server",
				Release: "consul",
			},
		},
	})

	manifest.Properties.ConsulTestConsumer = &core.ConsulTestConsumer{
		NameServer: staticIpForAddTestHost[9],
	}

	manifest.Properties.TurbulenceAgent = &core.PropertiesTurbulenceAgent{
		API: core.PropertiesTurbulenceAgentAPI{
			Host:     config.TurbulenceHost,
			Password: turbulence.DefaultPassword,
			CACert:   turbulence.APICACert,
		},
	}

	return manifest, nil
}
