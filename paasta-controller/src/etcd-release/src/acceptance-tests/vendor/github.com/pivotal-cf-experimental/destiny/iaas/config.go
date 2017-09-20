package iaas

import "github.com/pivotal-cf-experimental/destiny/core"

type Config interface {
	NetworkSubnet(ipRange string) core.NetworkSubnetCloudProperties
	Compilation() core.CompilationCloudProperties
	ResourcePool(ipRange string) core.ResourcePoolCloudProperties
	CPI() CPI
	Properties(staticIP string) Properties
	Stemcell() string
}
