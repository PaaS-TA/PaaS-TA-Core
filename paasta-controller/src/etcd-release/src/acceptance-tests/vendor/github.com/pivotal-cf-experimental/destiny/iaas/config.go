package iaas

import "github.com/pivotal-cf-experimental/destiny/core"

type Config interface {
	NetworkSubnet(ipRange string) core.NetworkSubnetCloudProperties
	Compilation(availabilityZone string) core.CompilationCloudProperties
	ResourcePool(ipRange string) core.ResourcePoolCloudProperties
	CPI() CPI
	Properties(staticIP string) Properties
	Stemcell() string
	WindowsStemcell() string
}
