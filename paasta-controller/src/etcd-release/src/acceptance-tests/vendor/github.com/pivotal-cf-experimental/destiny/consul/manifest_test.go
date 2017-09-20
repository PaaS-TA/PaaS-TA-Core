package consul_test

import (
	"io/ioutil"

	"github.com/pivotal-cf-experimental/destiny/consul"
	"github.com/pivotal-cf-experimental/destiny/core"
	"github.com/pivotal-cf-experimental/destiny/iaas"
	"github.com/pivotal-cf-experimental/gomegamatchers"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
)

var _ = Describe("Manifest", func() {
	Describe("NewManifest", func() {
		It("generates a valid Consul BOSH-Lite manifest", func() {
			manifest, err := consul.NewManifest(consul.Config{
				DirectorUUID: "some-director-uuid",
				Name:         "consul-some-random-guid",
				Networks: []consul.ConfigNetwork{
					{
						IPRange: "10.244.4.0/26",
						Nodes:   2,
					},
					{
						IPRange: "10.244.5.0/26",
						Nodes:   1,
					},
				},
			}, iaas.NewWardenConfig())
			Expect(err).NotTo(HaveOccurred())

			Expect(manifest.DirectorUUID).To(Equal("some-director-uuid"))
			Expect(manifest.Name).To(Equal("consul-some-random-guid"))
			Expect(manifest.Releases).To(Equal([]core.Release{{
				Name:    "consul",
				Version: "latest",
			}}))

			Expect(manifest.Compilation).To(Equal(core.Compilation{
				Network:             "consul1",
				ReuseCompilationVMs: true,
				Workers:             3,
			}))

			Expect(manifest.Update).To(Equal(core.Update{
				Canaries:        1,
				CanaryWatchTime: "1000-180000",
				MaxInFlight:     1,
				Serial:          true,
				UpdateWatchTime: "1000-180000",
			}))

			Expect(manifest.ResourcePools).To(Equal([]core.ResourcePool{
				{
					Name:    "consul_z1",
					Network: "consul1",
					Stemcell: core.ResourcePoolStemcell{
						Name:    "bosh-warden-boshlite-ubuntu-trusty-go_agent",
						Version: "latest",
					},
				},
				{
					Name:    "consul_z2",
					Network: "consul2",
					Stemcell: core.ResourcePoolStemcell{
						Name:    "bosh-warden-boshlite-ubuntu-trusty-go_agent",
						Version: "latest",
					},
				},
			}))

			Expect(manifest.Jobs).To(HaveLen(3))
			Expect(manifest.Jobs[0]).To(Equal(core.Job{
				Name:      "consul_z1",
				Instances: 2,
				Networks: []core.JobNetwork{{
					Name:      "consul1",
					StaticIPs: []string{"10.244.4.4", "10.244.4.5"},
				}},
				PersistentDisk: 1024,
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
				ResourcePool: "consul_z1",
				Templates: []core.JobTemplate{{
					Name:    "consul_agent",
					Release: "consul",
				}},
				Update: &core.JobUpdate{
					MaxInFlight: 1,
				},
			}))

			Expect(manifest.Jobs[1]).To(Equal(core.Job{
				Name:      "consul_z2",
				Instances: 1,
				Networks: []core.JobNetwork{{
					Name:      "consul2",
					StaticIPs: []string{"10.244.5.4"},
				}},
				PersistentDisk: 1024,
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
				ResourcePool: "consul_z2",
				Templates: []core.JobTemplate{{
					Name:    "consul_agent",
					Release: "consul",
				}},
				Update: &core.JobUpdate{
					MaxInFlight: 1,
				},
			}))

			Expect(manifest.Jobs[2]).To(Equal(core.Job{
				Name:      "consul_test_consumer",
				Instances: 1,
				Networks: []core.JobNetwork{{
					Name: "consul1",
					StaticIPs: []string{
						"10.244.4.10",
					},
				}},
				Properties: &core.JobProperties{
					Consul: &core.JobPropertiesConsul{
						Agent: core.JobPropertiesConsulAgent{
							Mode:     "client",
							LogLevel: "info",
						},
					},
				},
				ResourcePool: "consul_z1",
				Templates: []core.JobTemplate{
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

			Expect(manifest.Networks).To(HaveLen(2))
			Expect(manifest.Networks[0]).To(Equal(core.Network{
				Name: "consul1",
				Subnets: []core.NetworkSubnet{
					{
						CloudProperties: core.NetworkSubnetCloudProperties{Name: "random"},
						Gateway:         "10.244.4.1",
						Range:           "10.244.4.0/26",
						Reserved: []string{
							"10.244.4.2-10.244.4.3",
							"10.244.4.63",
						},
						Static: []string{"10.244.4.4-10.244.4.59"},
					},
				},
				Type: "manual",
			}))
			Expect(manifest.Networks[1]).To(Equal(core.Network{
				Name: "consul2",
				Subnets: []core.NetworkSubnet{
					{
						CloudProperties: core.NetworkSubnetCloudProperties{Name: "random"},
						Gateway:         "10.244.5.1",
						Range:           "10.244.5.0/26",
						Reserved: []string{
							"10.244.5.2-10.244.5.3",
							"10.244.5.63",
						},
						Static: []string{"10.244.5.4-10.244.5.59"},
					},
				},
				Type: "manual",
			}))

			Expect(manifest.Properties).To(Equal(consul.Properties{
				Consul: &consul.PropertiesConsul{
					Agent: consul.PropertiesConsulAgent{
						Domain:     "cf.internal",
						Datacenter: "dc1",
						Servers: consul.PropertiesConsulAgentServers{
							Lan: []string{"10.244.4.4", "10.244.4.5", "10.244.5.4"},
						},
						DNSConfig: consul.PropertiesConsulAgentDNSConfig{
							RecursorTimeout: "5s",
						},
					},
					CACert:      consul.CACert,
					AgentCert:   consul.DC1AgentCert,
					AgentKey:    consul.DC1AgentKey,
					ServerCert:  consul.DC1ServerCert,
					ServerKey:   consul.DC1ServerKey,
					EncryptKeys: []string{consul.EncryptKey},
				},
			}))
		})

		It("generates a valid Consul AWS manifest", func() {
			manifest, err := consul.NewManifest(consul.Config{
				DirectorUUID: "some-director-uuid",
				Name:         "consul-some-random-guid",
				Networks: []consul.ConfigNetwork{
					{
						IPRange: "10.0.4.0/24",
						Nodes:   1,
					},
					{
						IPRange: "10.0.5.0/24",
						Nodes:   1,
					},
				},
			}, iaas.AWSConfig{
				Subnets: []iaas.AWSConfigSubnet{
					{ID: "subnet-1", Range: "10.0.4.0/24", AZ: "some-az-1a", SecurityGroup: "some-security-group-1"},
					{ID: "subnet-2", Range: "10.0.5.0/24", AZ: "some-az-1d", SecurityGroup: "some-security-group-2"},
				},
			})
			Expect(err).NotTo(HaveOccurred())

			Expect(manifest).To(Equal(consul.Manifest{
				DirectorUUID: "some-director-uuid",
				Name:         "consul-some-random-guid",
				Releases: []core.Release{{
					Name:    "consul",
					Version: "latest",
				}},
				Compilation: core.Compilation{
					Network:             "consul1",
					ReuseCompilationVMs: true,
					Workers:             3,
					CloudProperties: core.CompilationCloudProperties{
						InstanceType:     "c3.large",
						AvailabilityZone: "us-east-1a",
						EphemeralDisk: &core.CompilationCloudPropertiesEphemeralDisk{
							Size: 2048,
							Type: "gp2",
						},
					},
				},
				Update: core.Update{
					Canaries:        1,
					CanaryWatchTime: "1000-180000",
					MaxInFlight:     1,
					Serial:          true,
					UpdateWatchTime: "1000-180000",
				},
				ResourcePools: []core.ResourcePool{
					{
						Name:    "consul_z1",
						Network: "consul1",
						Stemcell: core.ResourcePoolStemcell{
							Name:    "bosh-aws-xen-hvm-ubuntu-trusty-go_agent",
							Version: "latest",
						},
						CloudProperties: core.ResourcePoolCloudProperties{
							InstanceType:     "m3.medium",
							AvailabilityZone: "some-az-1a",
							EphemeralDisk: &core.ResourcePoolCloudPropertiesEphemeralDisk{
								Size: 10240,
								Type: "gp2",
							},
						},
					},
					{
						Name:    "consul_z2",
						Network: "consul2",
						Stemcell: core.ResourcePoolStemcell{
							Name:    "bosh-aws-xen-hvm-ubuntu-trusty-go_agent",
							Version: "latest",
						},
						CloudProperties: core.ResourcePoolCloudProperties{
							InstanceType:     "m3.medium",
							AvailabilityZone: "some-az-1d",
							EphemeralDisk: &core.ResourcePoolCloudPropertiesEphemeralDisk{
								Size: 10240,
								Type: "gp2",
							},
						},
					},
				},
				Jobs: []core.Job{
					{
						Name:      "consul_z1",
						Instances: 1,
						Networks: []core.JobNetwork{{
							Name:      "consul1",
							StaticIPs: []string{"10.0.4.4"},
						}},
						PersistentDisk: 1024,
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
						ResourcePool: "consul_z1",
						Templates: []core.JobTemplate{{
							Name:    "consul_agent",
							Release: "consul",
						}},
						Update: &core.JobUpdate{
							MaxInFlight: 1,
						},
					},
					{
						Name:      "consul_z2",
						Instances: 1,
						Networks: []core.JobNetwork{{
							Name:      "consul2",
							StaticIPs: []string{"10.0.5.4"},
						}},
						PersistentDisk: 1024,
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
						ResourcePool: "consul_z2",
						Templates: []core.JobTemplate{{
							Name:    "consul_agent",
							Release: "consul",
						}},
						Update: &core.JobUpdate{
							MaxInFlight: 1,
						},
					},
					{
						Name:      "consul_test_consumer",
						Instances: 1,
						Networks: []core.JobNetwork{{
							Name: "consul1",
							StaticIPs: []string{
								"10.0.4.10",
							},
						}},
						Properties: &core.JobProperties{
							Consul: &core.JobPropertiesConsul{
								Agent: core.JobPropertiesConsulAgent{
									Mode:     "client",
									LogLevel: "info",
								},
							},
						},
						ResourcePool: "consul_z1",
						Templates: []core.JobTemplate{
							{
								Name:    "consul_agent",
								Release: "consul",
							},
							{
								Name:    "consul-test-consumer",
								Release: "consul",
							},
						},
					},
				},
				Networks: []core.Network{
					{
						Name: "consul1",
						Subnets: []core.NetworkSubnet{
							{
								CloudProperties: core.NetworkSubnetCloudProperties{
									Subnet:         "subnet-1",
									SecurityGroups: []string{"some-security-group-1"},
								},
								Gateway: "10.0.4.1",
								Range:   "10.0.4.0/24",
								Reserved: []string{
									"10.0.4.2-10.0.4.3",
									"10.0.4.255",
								},
								Static: []string{"10.0.4.4-10.0.4.251"},
							},
						},
						Type: "manual",
					},
					{
						Name: "consul2",
						Subnets: []core.NetworkSubnet{
							{
								CloudProperties: core.NetworkSubnetCloudProperties{
									Subnet:         "subnet-2",
									SecurityGroups: []string{"some-security-group-2"},
								},
								Gateway: "10.0.5.1",
								Range:   "10.0.5.0/24",
								Reserved: []string{
									"10.0.5.2-10.0.5.3",
									"10.0.5.255",
								},
								Static: []string{"10.0.5.4-10.0.5.251"},
							},
						},
						Type: "manual",
					},
				},
				Properties: consul.Properties{
					Consul: &consul.PropertiesConsul{
						Agent: consul.PropertiesConsulAgent{
							Domain:     "cf.internal",
							Datacenter: "dc1",
							LogLevel:   "",
							Servers: consul.PropertiesConsulAgentServers{
								Lan: []string{"10.0.4.4", "10.0.5.4"},
							},
							DNSConfig: consul.PropertiesConsulAgentDNSConfig{
								RecursorTimeout: "5s",
							},
						},
						CACert:      consul.CACert,
						AgentCert:   consul.DC1AgentCert,
						AgentKey:    consul.DC1AgentKey,
						ServerCert:  consul.DC1ServerCert,
						ServerKey:   consul.DC1ServerKey,
						EncryptKeys: []string{consul.EncryptKey},
					},
				},
			}))
		})

		Context("when config nodes is not specified", func() {
			It("sets job instances to 1 and assigns a static IP", func() {
				manifest, err := consul.NewManifest(consul.Config{
					DirectorUUID: "some-director-uuid",
					Name:         "consul-some-random-guid",
					Networks: []consul.ConfigNetwork{
						{
							IPRange: "10.0.4.0/24",
						},
					},
				}, iaas.AWSConfig{
					Subnets: []iaas.AWSConfigSubnet{
						{ID: "subnet-1234", Range: "10.0.4.0/24", AZ: "some-az-1"},
					},
				})
				Expect(err).NotTo(HaveOccurred())

				Expect(manifest.Jobs[0].Instances).To(Equal(1))
			})
		})

		DescribeTable("TLS configuration",
			func(dcName, agentCert, agentKey, serverCert, serverKey string) {
				manifest, err := consul.NewManifest(consul.Config{
					DirectorUUID: "some-director-uuid",
					Name:         "consul-some-random-guid",
					Networks: []consul.ConfigNetwork{
						{
							IPRange: "10.244.4.0/24",
							Nodes:   1,
						},
					},
					DC: dcName,
				}, iaas.NewWardenConfig())
				Expect(err).NotTo(HaveOccurred())

				Expect(manifest.Properties.Consul.Agent.Datacenter).To(Equal(dcName))
				Expect(manifest.Properties.Consul.AgentCert).To(Equal(agentCert))
				Expect(manifest.Properties.Consul.AgentKey).To(Equal(agentKey))
				Expect(manifest.Properties.Consul.ServerCert).To(Equal(serverCert))
				Expect(manifest.Properties.Consul.ServerKey).To(Equal(serverKey))
				Expect(manifest.Properties.Consul.CACert).To(Equal(consul.CACert))
			},
			Entry("generates a manifest with dc1.cf.internal signed certs", "dc1", consul.DC1AgentCert, consul.DC1AgentKey, consul.DC1ServerCert, consul.DC1ServerKey),
			Entry("generates a manifest with dc2.cf.internal signed certs", "dc2", consul.DC2AgentCert, consul.DC2AgentKey, consul.DC2ServerCert, consul.DC2ServerKey),
			Entry("generates a manifest with dc3.cf.internal signed certs", "dc3", consul.DC3AgentCert, consul.DC3AgentKey, consul.DC3ServerCert, consul.DC3ServerKey),
		)

		Context("failure cases", func() {
			It("returns an error when it cannot parse the cidr block provided in config", func() {
				_, err := consul.NewManifest(consul.Config{
					DirectorUUID: "some-director-uuid",
					Name:         "consul-some-random-guid",
					Networks: []consul.ConfigNetwork{
						{
							IPRange: "fake-cidr-block",
							Nodes:   1,
						},
					},
				}, iaas.NewWardenConfig())
				Expect(err).To(MatchError(`"fake-cidr-block" cannot parse CIDR block`))
			})

			It("returns an error when nodes is less than zero ", func() {
				_, err := consul.NewManifest(consul.Config{
					DirectorUUID: "some-director-uuid",
					Name:         "consul-some-random-guid",
					Networks: []consul.ConfigNetwork{
						{
							IPRange: "10.244.4.0/24",
							Nodes:   -1,
						},
					},
				}, iaas.NewWardenConfig())
				Expect(err).To(MatchError("count must be greater than or equal to zero"))
			})

			It("returns an error when not enough ips for test consumers", func() {
				_, err := consul.NewManifest(consul.Config{
					DirectorUUID: "some-director-uuid",
					Name:         "consul-some-random-guid",
					Networks: []consul.ConfigNetwork{
						{
							IPRange: "10.244.4.0/28",
							Nodes:   1,
						},
					},
				}, iaas.NewWardenConfig())
				Expect(err).To(MatchError("can't allocate 9 ips from 8 available ips"))
			})
		})
	})

	Describe("ConsulMembers", func() {
		Context("when there is a single job with a single instance", func() {
			It("returns a list of members in the cluster", func() {
				manifest := consul.Manifest{
					Jobs: []core.Job{
						{
							Instances: 1,
							Networks: []core.JobNetwork{{
								StaticIPs: []string{"10.244.4.2"},
							}},
						},
					},
				}

				members := manifest.ConsulMembers()
				Expect(members).To(Equal([]consul.ConsulMember{{
					Address: "10.244.4.2",
				}}))
			})
		})

		Context("when there are multiple jobs with multiple instances", func() {
			It("returns a list of members in the cluster", func() {
				manifest := consul.Manifest{
					Jobs: []core.Job{
						{
							Instances: 0,
						},
						{
							Instances: 1,
							Networks: []core.JobNetwork{{
								StaticIPs: []string{"10.244.4.2"},
							}},
						},
						{
							Instances: 2,
							Networks: []core.JobNetwork{{
								StaticIPs: []string{"10.244.5.2", "10.244.5.6"},
							}},
						},
					},
				}

				members := manifest.ConsulMembers()
				Expect(members).To(Equal([]consul.ConsulMember{
					{
						Address: "10.244.4.2",
					},
					{
						Address: "10.244.5.2",
					},
					{
						Address: "10.244.5.6",
					},
				}))
			})
		})

		Context("when the job does not have a network", func() {
			It("returns an empty list", func() {
				manifest := consul.Manifest{
					Jobs: []core.Job{
						{
							Instances: 1,
							Networks:  []core.JobNetwork{},
						},
					},
				}

				members := manifest.ConsulMembers()
				Expect(members).To(BeEmpty())
			})
		})

		Context("when the job network does not have enough static IPs", func() {
			It("returns as much about the list as possible", func() {
				manifest := consul.Manifest{
					Jobs: []core.Job{
						{
							Instances: 2,
							Networks: []core.JobNetwork{{
								StaticIPs: []string{"10.244.5.2"},
							}},
						},
					},
				}

				members := manifest.ConsulMembers()
				Expect(members).To(Equal([]consul.ConsulMember{
					{
						Address: "10.244.5.2",
					},
				}))
			})
		})
	})

	Describe("FromYAML", func() {
		It("returns a Manifest matching the given YAML", func() {
			consulManifest, err := ioutil.ReadFile("fixtures/consul_manifest.yml")
			Expect(err).NotTo(HaveOccurred())

			var manifest consul.Manifest
			err = consul.FromYAML(consulManifest, &manifest)
			Expect(err).NotTo(HaveOccurred())

			Expect(manifest.DirectorUUID).To(Equal("some-director-uuid"))
			Expect(manifest.Name).To(Equal("consul"))
			Expect(manifest.Releases).To(HaveLen(1))
			Expect(manifest.Releases).To(ContainElement(core.Release{
				Name:    "consul",
				Version: "latest",
			}))
			Expect(manifest.Compilation).To(Equal(core.Compilation{
				Network:             "consul1",
				ReuseCompilationVMs: true,
				Workers:             3,
			}))
			Expect(manifest.Update).To(Equal(core.Update{
				Canaries:        1,
				CanaryWatchTime: "1000-180000",
				MaxInFlight:     1,
				Serial:          true,
				UpdateWatchTime: "1000-180000",
			}))
			Expect(manifest.ResourcePools).To(HaveLen(1))
			Expect(manifest.ResourcePools).To(ContainElement(core.ResourcePool{
				Name:    "consul_z1",
				Network: "consul1",
				Stemcell: core.ResourcePoolStemcell{
					Name:    "bosh-warden-boshlite-ubuntu-trusty-go_agent",
					Version: "latest",
				},
			}))
			Expect(manifest.Jobs).To(HaveLen(2))
			Expect(manifest.Jobs[0]).To(Equal(core.Job{
				Name:      "consul_z1",
				Instances: 1,
				Networks: []core.JobNetwork{{
					Name:      "consul1",
					StaticIPs: []string{"10.244.4.4"},
				}},
				PersistentDisk: 1024,
				ResourcePool:   "consul_z1",
				Templates: []core.JobTemplate{
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
				Update: &core.JobUpdate{
					MaxInFlight: 1,
				},
			}))
			Expect(manifest.Jobs[1]).To(Equal(core.Job{
				Name:      "consul_test_consumer",
				Instances: 1,
				Networks: []core.JobNetwork{{
					Name: "consul1",
					StaticIPs: []string{
						"10.244.4.10",
					},
				}},
				Properties: &core.JobProperties{
					Consul: &core.JobPropertiesConsul{
						Agent: core.JobPropertiesConsulAgent{
							Mode:     "client",
							LogLevel: "info",
						},
					},
				},
				ResourcePool: "consul_z1",
				Templates: []core.JobTemplate{
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

			Expect(manifest.Networks).To(HaveLen(1))
			Expect(manifest.Networks).To(ContainElement(core.Network{
				Name: "consul1",
				Subnets: []core.NetworkSubnet{
					{
						CloudProperties: core.NetworkSubnetCloudProperties{Name: "random"},
						Gateway:         "10.244.4.1",
						Range:           "10.244.4.0/24",
						Reserved: []string{
							"10.244.4.2-10.244.4.3",
							"10.244.4.255",
						},
						Static: []string{"10.244.4.4-10.244.4.251"},
					},
				},
				Type: "manual",
			}))
			Expect(manifest.Properties).To(Equal(consul.Properties{
				Consul: &consul.PropertiesConsul{
					Agent: consul.PropertiesConsulAgent{
						Domain:     "cf.internal",
						Datacenter: "dc1",
						Servers: consul.PropertiesConsulAgentServers{
							Lan: []string{"10.244.4.4"},
						},
						DNSConfig: consul.PropertiesConsulAgentDNSConfig{
							RecursorTimeout: "5s",
						},
					},
					CACert:      consul.CACert,
					AgentCert:   consul.DC1AgentCert,
					AgentKey:    consul.DC1AgentKey,
					ServerCert:  consul.DC1ServerCert,
					ServerKey:   consul.DC1ServerKey,
					EncryptKeys: []string{consul.EncryptKey},
				},
			}))
		})

		It("returns a ManifestV2 matching the given YAML", func() {
			consulManifest, err := ioutil.ReadFile("fixtures/consul_manifest_v2.yml")
			Expect(err).NotTo(HaveOccurred())

			var manifest consul.ManifestV2
			err = consul.FromYAML(consulManifest, &manifest)
			Expect(err).NotTo(HaveOccurred())

			Expect(manifest.DirectorUUID).To(Equal("some-director-uuid"))
			Expect(manifest.Name).To(Equal("consul-some-random-guid"))
			Expect(manifest.Releases).To(HaveLen(1))
			Expect(manifest.Releases).To(ContainElement(core.Release{
				Name:    "consul",
				Version: "latest",
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
				Name:      "consul",
				Instances: 3,
				AZs:       []string{"z1", "z2"},
				Networks: []core.InstanceGroupNetwork{{
					Name: "private",
					StaticIPs: []string{
						"10.244.4.4",
						"10.244.4.5",
						"10.244.5.4",
					},
				}},
				VMType:             "default",
				Stemcell:           "default",
				PersistentDiskType: "default",
				Jobs: []core.InstanceGroupJob{
					{
						Name:    "consul_agent",
						Release: "consul",
					},
				},
				MigratedFrom: []core.InstanceGroupMigratedFrom{
					{Name: "consul_z1", AZ: "z1"},
					{Name: "consul_z2", AZ: "z2"},
				},
				Properties: core.InstanceGroupProperties{
					Consul: core.InstanceGroupPropertiesConsul{
						Agent: core.InstanceGroupPropertiesConsulAgent{
							Mode:     "server",
							LogLevel: "info",
							Services: map[string]core.InstanceGroupPropertiesConsulAgentService{
								"router": core.InstanceGroupPropertiesConsulAgentService{
									Name: "gorouter",
									Check: core.InstanceGroupPropertiesConsulAgentServiceCheck{
										Name:     "router-check",
										Script:   "/var/vcap/jobs/router/bin/script",
										Interval: "1m",
									},
									Tags: []string{"routing"},
								},
								"cloud_controller": core.InstanceGroupPropertiesConsulAgentService{},
							},
						},
					},
				},
			}))
			Expect(manifest.InstanceGroups[1]).To(Equal(core.InstanceGroup{
				Name:      "test_consumer",
				AZs:       []string{"z1"},
				Instances: 3,
				Networks: []core.InstanceGroupNetwork{{
					Name: "private",
					StaticIPs: []string{
						"10.244.4.10",
						"10.244.4.11",
						"10.244.4.12",
					},
				}},
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
				MigratedFrom: []core.InstanceGroupMigratedFrom{
					{
						Name: "consul_test_consumer",
						AZ:   "z1",
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
					CACert:      consul.CACert,
					AgentCert:   consul.DC1AgentCert,
					AgentKey:    consul.DC1AgentKey,
					ServerCert:  consul.DC1ServerCert,
					ServerKey:   consul.DC1ServerKey,
					EncryptKeys: []string{consul.EncryptKey},
				},
			}))
		})
	})

	Describe("ToYAML", func() {
		It("returns a YAML representation of the consul manifest", func() {
			consulManifest, err := ioutil.ReadFile("fixtures/consul_manifest.yml")
			Expect(err).NotTo(HaveOccurred())

			manifest, err := consul.NewManifest(consul.Config{
				DirectorUUID: "some-director-uuid",
				Name:         "consul",
				Networks: []consul.ConfigNetwork{
					{
						IPRange: "10.244.4.0/24",
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
