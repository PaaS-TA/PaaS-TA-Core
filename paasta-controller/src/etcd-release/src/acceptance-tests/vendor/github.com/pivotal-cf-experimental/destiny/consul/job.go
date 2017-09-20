package consul

import (
	"fmt"

	"github.com/pivotal-cf-experimental/destiny/core"
)

func (manifest Manifest) SetConsulJobInstanceCount(count int) (Manifest, error) {
	manifest, err := manifest.SetJobInstanceCount("consul_z1", count)
	if err != nil {
		return Manifest{}, err
	}

	job, err := findJob(manifest, "consul_z1")
	if err != nil {
		return Manifest{}, err
	}

	manifest.Properties.Consul.Agent.Servers.Lan = job.Networks[0].StaticIPs

	return manifest, nil
}

func (manifest Manifest) SetJobInstanceCount(jobName string, count int) (Manifest, error) {
	job, err := findJob(manifest, jobName)
	if err != nil {
		return Manifest{}, err
	}

	job.Instances = count

	if len(job.Networks) == 0 {
		return Manifest{}, fmt.Errorf("%q job must have an existing network to modify", jobName)
	}
	network := job.Networks[0]

	if count == 0 {
		job.Networks[0].StaticIPs = []string{}
		return manifest, nil
	}

	if len(network.StaticIPs) == 0 {
		return Manifest{}, fmt.Errorf("%q job must have at least one static ip in its network", jobName)
	}
	firstIP, err := core.ParseIP(network.StaticIPs[0])
	if err != nil {
		return Manifest{}, err
	}
	lastIP := firstIP.Add(count - 1)
	job.Networks[0].StaticIPs = firstIP.To(lastIP)

	return manifest, nil
}
