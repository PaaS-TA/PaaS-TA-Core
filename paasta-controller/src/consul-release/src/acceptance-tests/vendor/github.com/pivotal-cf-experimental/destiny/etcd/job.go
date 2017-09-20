package etcd

import (
	"fmt"

	"github.com/pivotal-cf-experimental/destiny/core"
)

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

func SetEtcdProperties(job core.Job, properties Properties) Properties {
	if !properties.Etcd.RequireSSL {
		properties.EtcdTestConsumer.Etcd.Machines = job.Networks[0].StaticIPs
		properties.Etcd.Machines = job.Networks[0].StaticIPs
	}

	properties.Etcd.Cluster[0].Instances = job.Instances

	return properties
}

func (m Manifest) RemoveJob(jobName string) Manifest {
	for i, job := range m.Jobs {
		if job.Name == jobName {
			m.Jobs = append(m.Jobs[:i], m.Jobs[i+1:]...)
		}
	}
	return m
}

func findJob(manifest Manifest, name string) (*core.Job, error) {
	for index := range manifest.Jobs {
		if manifest.Jobs[index].Name == name {
			return &manifest.Jobs[index], nil
		}
	}
	return &core.Job{}, fmt.Errorf("%q job does not exist", name)
}
