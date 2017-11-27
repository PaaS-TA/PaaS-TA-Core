package helpers

import (
	"github.com/pivotal-cf-experimental/bosh-test/bosh"
	"github.com/pivotal-cf-experimental/destiny/ops"
)

func DeploymentVMs(boshClient bosh.Client, deploymentName string) ([]bosh.VM, error) {
	vms, err := boshClient.DeploymentVMs(deploymentName)
	if err != nil {
		return nil, err
	}

	for index := range vms {
		vms[index].IPs = nil
		vms[index].ID = ""
	}

	return vms, nil
}

func GetVMsFromManifest(manifest string) []bosh.VM {
	var vms []bosh.VM

	instanceGroups, err := ops.InstanceGroups(manifest)
	if err != nil {
		panic(err)
	}

	for _, ig := range instanceGroups {
		for i := 0; i < ig.Instances; i++ {
			vms = append(vms, bosh.VM{JobName: ig.Name, Index: i, State: "running"})
		}
	}

	return vms
}

func GetVMIPs(boshClient bosh.Client, deploymentName, jobName string) ([]string, error) {
	vms, err := boshClient.DeploymentVMs(deploymentName)
	if err != nil {
		return []string{}, err
	}

	ips := []string{}
	for _, vm := range vms {
		if vm.JobName == jobName {
			ips = append(ips, vm.IPs...)
		}
	}

	return ips, nil
}

func GetVMIDByIndices(boshClient bosh.Client, deploymentName, jobName string, indices []int) ([]string, error) {
	vms, err := boshClient.DeploymentVMs(deploymentName)
	if err != nil {
		return []string{}, err
	}

	var vmIDs []string
	for _, vm := range vms {
		if vm.JobName == jobName {
			for _, index := range indices {
				if index == vm.Index {
					vmIDs = append(vmIDs, vm.ID)
				}
			}
		}
	}

	return vmIDs, nil
}
