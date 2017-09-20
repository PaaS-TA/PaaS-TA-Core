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

var _ = Describe("Manifest", func() {
	Describe("NewManifestWithJobLevelProperties", func() {
		It("generates a valid Consul BOSH-Lite manifest with proper job level consul properties", func() {
			manifest, err := consul.NewManifestWithJobLevelProperties(consul.ConfigV2{
				DirectorUUID: "some-director-uuid",
				Name:         "consul-some-random-guid",
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

			Expect(manifest.InstanceGroups[0].Properties.Consul).To(Equal(&core.JobPropertiesConsul{
				Agent: core.JobPropertiesConsulAgent{
					Domain:     "cf.internal",
					Datacenter: "dc1",
					Servers: core.JobPropertiesConsulAgentServers{
						Lan: []string{"10.244.4.4", "10.244.4.5", "10.244.5.4"},
					},
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
				CACert:      consul.CACert,
				ServerCert:  consul.DC1ServerCert,
				ServerKey:   consul.DC1ServerKey,
				AgentCert:   consul.DC1AgentCert,
				AgentKey:    consul.DC1AgentKey,
				EncryptKeys: []string{consul.EncryptKey},
			}))

			Expect(manifest.InstanceGroups[1].Properties.Consul).To(Equal(&core.JobPropertiesConsul{
				Agent: core.JobPropertiesConsulAgent{
					Domain:     "cf.internal",
					Datacenter: "dc1",
					Servers: core.JobPropertiesConsulAgentServers{
						Lan: []string{"10.244.4.4", "10.244.4.5", "10.244.5.4"},
					},
				},
				CACert:      consul.CACert,
				AgentCert:   consul.DC1AgentCert,
				AgentKey:    consul.DC1AgentKey,
				EncryptKeys: []string{consul.EncryptKey},
			}))

			Expect(manifest.Properties.Consul).To(BeNil())
		})

		Context("failure cases", func() {
			It("returns an error when it fails to build base manifest", func() {
				_, err := consul.NewManifestWithJobLevelProperties(consul.ConfigV2{
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

	Describe("ToYAML", func() {
		It("returns a YAML representation of the consul manifest with job level consul properties", func() {
			consulManifest, err := ioutil.ReadFile("fixtures/consul_manifest_with_job_level_properties.yml")
			Expect(err).NotTo(HaveOccurred())

			manifest, err := consul.NewManifestWithJobLevelProperties(consul.ConfigV2{
				DirectorUUID: "some-director-uuid",
				Name:         "consul",
				AZs: []consul.ConfigAZ{
					{
						IPRange: "10.244.4.0/24",
						Nodes:   1,
						Name:    "z1",
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
