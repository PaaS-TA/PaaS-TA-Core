package consul_test

import (
	"github.com/pivotal-cf-experimental/destiny/consul"
	"github.com/pivotal-cf-experimental/destiny/core"
	"github.com/pivotal-cf-experimental/destiny/iaas"
	"github.com/pivotal-cf-experimental/destiny/turbulence"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/pivotal-cf-experimental/gomegamatchers"
)

var _ = Describe("Manifest", func() {
	Describe("NewManifestWithTurbulenceAgent", func() {
		It("generates a valid Consul BOSH-lite manifest with additional turbulence agent on test consumer", func() {
			manifest, err := consul.NewManifestWithTurbulenceAgent(consul.ConfigV2{
				DirectorUUID:   "some-director-uuid",
				Name:           "consul-some-random-guid",
				TurbulenceHost: "10.244.4.32",
				AZs: []consul.ConfigAZ{
					{
						IPRange: "10.244.4.0/24",
						Nodes:   2,
						Name:    "z1",
					},
					{
						IPRange: "10.244.5.0/24",
						Nodes:   1,
						Name:    "z2",
					},
				},
			}, iaas.NewWardenConfig())
			Expect(err).NotTo(HaveOccurred())

			Expect(manifest.DirectorUUID).To(Equal("some-director-uuid"))
			Expect(manifest.Name).To(Equal("consul-some-random-guid"))

			Expect(manifest.Properties.TurbulenceAgent.API).To(Equal(core.PropertiesTurbulenceAgentAPI{
				Host:     "10.244.4.32",
				Password: turbulence.DefaultPassword,
				CACert:   turbulence.APICACert,
			}))
			Expect(manifest.InstanceGroups[2].VMType).To(Equal("default"))
			Expect(manifest.InstanceGroups[2].Networks[0]).To(Equal(core.InstanceGroupNetwork{
				Name:      "private",
				StaticIPs: []string{"10.244.4.13"},
			}))
			Expect(manifest.InstanceGroups[2].Name).To(Equal("fake-dns-server"))
			Expect(manifest.InstanceGroups[2].Instances).To(Equal(1))
			Expect(manifest.InstanceGroups[2].VMType).To(Equal("default"))
			Expect(manifest.InstanceGroups[2].Stemcell).To(Equal("default"))
			Expect(manifest.InstanceGroups[2].PersistentDiskType).To(Equal("default"))
			Expect(manifest.InstanceGroups[2].Jobs).To(gomegamatchers.ContainSequence([]core.InstanceGroupJob{
				{
					Name:    "turbulence_agent",
					Release: "turbulence",
				},
				{
					Name:    "fake-dns-server",
					Release: "consul",
				},
			}))

			Expect(manifest.Releases).To(ContainElement(core.Release{
				Name:    "turbulence",
				Version: "latest",
			}))
			Expect(manifest.Releases).To(ContainElement(core.Release{
				Name:    "consul",
				Version: "latest",
			}))

			Expect(manifest.Properties.TurbulenceAgent.API).To(Equal(core.PropertiesTurbulenceAgentAPI{
				Host:     "10.244.4.32",
				Password: turbulence.DefaultPassword,
				CACert:   turbulence.APICACert,
			}))
			Expect(manifest.Properties.ConsulTestConsumer.NameServer).To(Equal("10.244.4.13"))
		})

		It("generates a valid Consul BOSH-lite manifest with additional turbulence agent on test consumer", func() {
			manifest, err := consul.NewManifestWithTurbulenceAgent(consul.ConfigV2{
				DirectorUUID:   "some-director-uuid",
				Name:           "consul-some-random-guid",
				TurbulenceHost: "10.244.4.32",
				AZs: []consul.ConfigAZ{
					{
						IPRange: "10.244.4.0/24",
						Nodes:   2,
						Name:    "z1",
					},
					{
						IPRange: "10.244.5.0/24",
						Nodes:   1,
						Name:    "z2",
					},
				},
				PersistentDiskType: "1GB",
				VMType:             "m3.medium",
			}, iaas.NewWardenConfig())
			Expect(err).NotTo(HaveOccurred())

			Expect(manifest.InstanceGroups[2].PersistentDiskType).To(Equal("1GB"))
			Expect(manifest.InstanceGroups[2].VMType).To(Equal("m3.medium"))
		})
	})

	Context("failure cases", func() {
		It("returns an error when the manifest creation fails", func() {
			_, err := consul.NewManifestWithTurbulenceAgent(consul.ConfigV2{
				DirectorUUID: "some-director-uuid",
				Name:         "consul-some-random-guid",
				AZs: []consul.ConfigAZ{
					{
						IPRange: "fake-cidr-block",
						Nodes:   1,
						Name:    "z1",
					},
				},
			}, iaas.NewWardenConfig())
			Expect(err).To(MatchError(`"fake-cidr-block" cannot parse CIDR block`))
		})
	})
})
