package consul_test

import (
	"io/ioutil"

	"github.com/pivotal-cf-experimental/destiny/consul"
	"github.com/pivotal-cf-experimental/destiny/core"
	"github.com/pivotal-cf-experimental/destiny/iaas"
	"github.com/pivotal-cf-experimental/gomegamatchers"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("ManifestV2", func() {
	Describe("NewManifestV2", func() {
		It("returns a BOSH 2.0 manifest for bosh-lite", func() {
			manifest, err := consul.NewManifestV2(consul.ConfigV2{
				DirectorUUID: "some-director-uuid",
				Name:         "consul-some-random-guid",
				AZs: []consul.ConfigAZ{
					{
						Name:    "z1",
						IPRange: "10.244.4.0/24",
						Nodes:   2,
					},
					{
						Name:    "z2",
						IPRange: "10.244.5.0/24",
						Nodes:   1,
					},
				},
			}, iaas.NewWardenConfig())

			Expect(err).NotTo(HaveOccurred())
			Expect(manifest.DirectorUUID).To(Equal("some-director-uuid"))
			Expect(manifest.Name).To(Equal("consul-some-random-guid"))

			Expect(manifest.Releases).To(Equal([]core.Release{
				{
					Name:    "consul",
					Version: "latest",
				},
			}))

			Expect(manifest.Stemcells).To(Equal([]core.Stemcell{
				{
					Alias:   "default",
					Name:    "bosh-warden-boshlite-ubuntu-trusty-go_agent",
					Version: "latest",
				},
			}))

			Expect(manifest.Update).To(Equal(core.Update{
				Canaries:        1,
				CanaryWatchTime: "1000-180000",
				MaxInFlight:     1,
				Serial:          true,
				UpdateWatchTime: "1000-180000",
			}))

			Expect(manifest.InstanceGroups).To(HaveLen(2))
			Expect(manifest.InstanceGroups[0]).To(Equal(core.InstanceGroup{
				Instances: 3,
				Name:      "consul",
				AZs:       []string{"z1", "z2"},
				Networks: []core.InstanceGroupNetwork{
					{
						Name: "private",
						StaticIPs: []string{
							"10.244.4.4",
							"10.244.4.5",
							"10.244.5.4",
						},
					},
				},
				VMType:             "default",
				Stemcell:           "default",
				PersistentDiskType: "default",
				Jobs: []core.InstanceGroupJob{
					{
						Name:    "consul_agent",
						Release: "consul",
					},
				},
				Properties: &core.JobProperties{
					Consul: &core.JobPropertiesConsul{
						Agent: core.JobPropertiesConsulAgent{
							Mode:     "server",
							LogLevel: "info",
							Services: core.JobPropertiesConsulAgentServices{
								"router": core.JobPropertiesConsulAgentService{
									Name: "gorouter",
									Check: &core.JobPropertiesConsulAgentServiceCheck{
										Name:     "router-check",
										Script:   "/var/vcap/jobs/router/bin/script",
										Interval: "1m",
									},
									Tags: []string{"routing"},
								},
								"cloud_controller": core.JobPropertiesConsulAgentService{},
							},
						},
					},
				},
			}))

			Expect(manifest.InstanceGroups[1]).To(Equal(core.InstanceGroup{
				Instances: 1,
				Name:      "test_consumer",
				AZs:       []string{"z1"},
				Networks: []core.InstanceGroupNetwork{
					{
						Name: "private",
						StaticIPs: []string{
							"10.244.4.10",
						},
					},
				},
				VMType:   "default",
				Stemcell: "default",
				Jobs: []core.InstanceGroupJob{
					{
						Name:    "consul_agent",
						Release: "consul",
					},
					{
						Name:    "consul-test-consumer",
						Release: "consul",
					},
				},
			}))

			Expect(manifest.Properties).To(Equal(consul.Properties{
				Consul: &consul.PropertiesConsul{
					Agent: consul.PropertiesConsulAgent{
						Domain:     "cf.internal",
						Datacenter: "dc1",
						Servers: consul.PropertiesConsulAgentServers{
							Lan: []string{
								"10.244.4.4",
								"10.244.4.5",
								"10.244.5.4",
							},
						},
					},
					AgentCert: consul.DC1AgentCert,
					AgentKey:  consul.DC1AgentKey,
					CACert:    consul.CACert,
					EncryptKeys: []string{
						consul.EncryptKey,
					},
					ServerCert: consul.DC1ServerCert,
					ServerKey:  consul.DC1ServerKey,
				},
			}))
		})

		It("returns a bosh 2.0 consul manifest for aws", func() {
			manifest, err := consul.NewManifestV2(consul.ConfigV2{
				DirectorUUID:       "some-director-uuid",
				Name:               "consul-some-random-guid",
				PersistentDiskType: "1GB",
				VMType:             "m3.medium",
				AZs: []consul.ConfigAZ{
					{
						Name:    "us-west-2",
						IPRange: "10.0.4.192/27",
						Nodes:   3,
					},
					{
						Name:    "us-west-2",
						IPRange: "10.0.5.192/27",
						Nodes:   3,
					},
				},
			}, iaas.AWSConfig{})

			Expect(err).NotTo(HaveOccurred())
			Expect(manifest.Stemcells).To(Equal([]core.Stemcell{
				{
					Alias:   "default",
					Name:    "bosh-aws-xen-hvm-ubuntu-trusty-go_agent",
					Version: "latest",
				},
			}))

			Expect(manifest.InstanceGroups[0].Networks).To(Equal([]core.InstanceGroupNetwork{
				{
					Name: "private",
					StaticIPs: []string{
						"10.0.4.196",
						"10.0.4.197",
						"10.0.4.198",
						"10.0.5.196",
						"10.0.5.197",
						"10.0.5.198",
					},
				},
			}))

			Expect(manifest.InstanceGroups[1].Networks).To(Equal([]core.InstanceGroupNetwork{
				{
					Name: "private",
					StaticIPs: []string{
						"10.0.4.202",
					},
				},
			}))

			Expect(manifest.InstanceGroups[0].PersistentDiskType).To(Equal("1GB"))
			Expect(manifest.InstanceGroups[0].VMType).To(Equal("m3.medium"))

			Expect(manifest.InstanceGroups[1].VMType).To(Equal("m3.medium"))
		})

		It("returns a bosh 2.0 consul manifest with windows clients", func() {
			manifest, err := consul.NewManifestV2(consul.ConfigV2{
				AZs: []consul.ConfigAZ{
					{
						Name:    "us-west-2",
						IPRange: "10.0.4.192/27",
						Nodes:   3,
					},
				},
				WindowsClients: true,
			}, iaas.AWSConfig{})

			Expect(err).NotTo(HaveOccurred())
			Expect(manifest.Stemcells).To(HaveLen(2))
			Expect(manifest.Stemcells[0]).To(Equal(core.Stemcell{
				Alias:   "linux",
				Name:    "bosh-aws-xen-hvm-ubuntu-trusty-go_agent",
				Version: "latest",
			}))
			Expect(manifest.Stemcells[1]).To(Equal(core.Stemcell{
				Alias:   "windows",
				Name:    "bosh-aws-xen-hvm-windows-stemcell-go_agent",
				Version: "latest",
			}))

			Expect(manifest.InstanceGroups).To(HaveLen(2))
			Expect(manifest.InstanceGroups[0].Stemcell).To(Equal("linux"))
			Expect(manifest.InstanceGroups[1].Stemcell).To(Equal("windows"))

			Expect(manifest.InstanceGroups[1].Jobs).To(HaveLen(2))
			Expect(manifest.InstanceGroups[1].Jobs[0]).To(Equal(core.InstanceGroupJob{
				Name:    "consul_agent_windows",
				Release: "consul",
			}))
			Expect(manifest.InstanceGroups[1].Jobs[1]).To(Equal(core.InstanceGroupJob{
				Name:    "consul-test-consumer-windows",
				Release: "consul",
			}))
		})

		Context("failure cases", func() {
			It("returns an error when the az ip range is not a valid cidrblock", func() {
				_, err := consul.NewManifestV2(consul.ConfigV2{
					DirectorUUID: "some-director-uuid",
					Name:         "consul-some-random-guid",
					AZs: []consul.ConfigAZ{
						{
							Name:    "z1",
							IPRange: "%%%%%%%%",
							Nodes:   2,
						},
						{
							Name:    "z2",
							IPRange: "%%%%%%%%",
							Nodes:   2,
						},
					},
				}, iaas.NewWardenConfig())

				Expect(err).To(MatchError("\"%%%%%%%%\" cannot parse CIDR block"))
			})
		})
	})

	Describe("ToYAML", func() {
		It("returns a YAML representation of the consul manifest", func() {
			consulManifest, err := ioutil.ReadFile("fixtures/consul_manifest_v2.yml")
			Expect(err).NotTo(HaveOccurred())

			manifest, err := consul.NewManifestV2(consul.ConfigV2{
				DirectorUUID: "some-director-uuid",
				Name:         "consul-some-random-guid",
				AZs: []consul.ConfigAZ{
					{
						Name:    "z1",
						IPRange: "10.244.4.0/24",
						Nodes:   2,
					},
					{
						Name:    "z2",
						IPRange: "10.244.5.0/24",
						Nodes:   1,
					},
				},
			}, iaas.NewWardenConfig())
			Expect(err).NotTo(HaveOccurred())

			yaml, err := manifest.ToYAML()
			Expect(err).NotTo(HaveOccurred())
			Expect(yaml).To(gomegamatchers.MatchYAML(consulManifest))
		})
	})
})
