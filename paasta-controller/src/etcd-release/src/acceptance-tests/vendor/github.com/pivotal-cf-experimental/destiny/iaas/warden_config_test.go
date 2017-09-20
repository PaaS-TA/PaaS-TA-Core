package iaas_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/pivotal-cf-experimental/destiny/core"
	"github.com/pivotal-cf-experimental/destiny/iaas"
)

var _ = Describe("Warden Config", func() {

	var (
		wardenConfig iaas.WardenConfig
	)

	BeforeEach(func() {
		wardenConfig = iaas.NewWardenConfig()
	})

	Describe("NetworkSubnet", func() {
		It("returns a network subnet specific to Warden", func() {
			subnetCloudProperties := wardenConfig.NetworkSubnet("")
			Expect(subnetCloudProperties).To(Equal(core.NetworkSubnetCloudProperties{
				Name: "random",
			}))
		})
	})

	Describe("Compilation", func() {
		It("returns an empty compilation cloud properties for Warden", func() {
			compilationCloudProperties := wardenConfig.Compilation()
			Expect(compilationCloudProperties).To(Equal(core.CompilationCloudProperties{}))
		})
	})

	Describe("ResourcePool", func() {
		It("returns an empty resource pool cloud properties for Warden", func() {
			resourcePoolCloudProperties := wardenConfig.ResourcePool("")
			Expect(resourcePoolCloudProperties).To(Equal(core.ResourcePoolCloudProperties{}))
		})
	})

	Describe("CPI", func() {
		It("returns the cpi specific to Warden", func() {
			cpi := wardenConfig.CPI()
			Expect(cpi).To(Equal(iaas.CPI{
				JobName:     "warden_cpi",
				ReleaseName: "bosh-warden-cpi",
			}))
		})
	})

	Describe("Properties", func() {
		It("returns the properties specific to Warden", func() {
			properties := wardenConfig.Properties("unused")
			Expect(properties).To(Equal(iaas.Properties{
				WardenCPI: &iaas.PropertiesWardenCPI{
					Agent: iaas.PropertiesWardenCPIAgent{
						Blobstore: iaas.PropertiesWardenCPIAgentBlobstore{
							Options: iaas.PropertiesWardenCPIAgentBlobstoreOptions{
								Endpoint: "http://10.254.50.4:25251",
								Password: "agent-password",
								User:     "agent",
							},
							Provider: "dav",
						},
						Mbus: "nats://nats:nats-password@10.254.50.4:4222",
					},
					Warden: iaas.PropertiesWardenCPIWarden{
						ConnectAddress: "10.254.50.4:7777",
						ConnectNetwork: "tcp",
					},
				},
			}))
		})
	})
})
