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

	if count == 0 {
		job.Networks[0].StaticIPs = []string{}
		return manifest, nil
	}

	network := job.Networks[0]
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

func (manifest ManifestV2) SetInstanceCount(instance string, count int) (ManifestV2, error) {
	instanceGroup, err := manifest.GetInstanceGroup(instance)
	if err != nil {
		return ManifestV2{}, err
	}
	instanceGroup.Instances = count
	if count == 0 {
		instanceGroup.Networks[0].StaticIPs = []string{}
		return manifest, nil
	}

	network := instanceGroup.Networks[0]
	firstIP, err := core.ParseIP(network.StaticIPs[0])
	if err != nil {
		return ManifestV2{}, err
	}
	lastIP := firstIP.Add(count - 1)
	instanceGroup.Networks[0].StaticIPs = firstIP.To(lastIP)

	return manifest, nil
}

func (manifest ManifestV2) SetConsulJobInstanceCount(count int) (ManifestV2, error) {
	manifest, err := manifest.SetInstanceCount("consul", count)
	if err != nil {
		return ManifestV2{}, err
	}

	consulInstanceGroup, err := manifest.GetInstanceGroup("consul")
	if err != nil {
		return ManifestV2{}, err
	}

	manifest.Properties.Consul.Agent.Servers.Lan = consulInstanceGroup.Networks[0].StaticIPs
	return manifest, nil
}

func (manifest ManifestV2) GetInstanceGroup(name string) (*core.InstanceGroup, error) {
	for index := range manifest.InstanceGroups {
		if manifest.InstanceGroups[index].Name == name {
			return &manifest.InstanceGroups[index], nil
		}
	}
	return &core.InstanceGroup{}, fmt.Errorf("instance group %q does not exist", name)
}
