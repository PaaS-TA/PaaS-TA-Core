package turbulence_test

import (
	"io/ioutil"

	"github.com/pivotal-cf-experimental/destiny/core"
	"github.com/pivotal-cf-experimental/destiny/iaas"
	"github.com/pivotal-cf-experimental/destiny/turbulence"
	"github.com/pivotal-cf-experimental/gomegamatchers"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Manifest", func() {
	Describe("NewManifest", func() {
		It("generates a valid Turbulence AWS manifest", func() {
			manifest, err := turbulence.NewManifest(turbulence.Config{
				Name:         "turbulence",
				DirectorUUID: "some-director-uuid",
				IPRange:      "10.0.16.0/24",
				BOSH: turbulence.ConfigBOSH{
					Target:             "some-bosh-target",
					Username:           "some-bosh-username",
					Password:           "some-bosh-password",
					DirectorCACert:     "some-ca-cert",
					PersistentDiskType: "some-persistent-disk-type",
					VMType:             "some-vm-type",
				},
			}, iaas.AWSConfig{
				AccessKeyID:           "some-access-key-id",
				SecretAccessKey:       "some-secret-access-key",
				DefaultKeyName:        "some-default-key-name",
				DefaultSecurityGroups: []string{"some-default-security-group1"},
				Region:                "some-region",
				Subnets: []iaas.AWSConfigSubnet{
					{ID: "subnet-1234", Range: "10.0.16.0/24", AZ: "some-az-1a"},
				},
				RegistryHost:     "some-registry-host",
				RegistryPassword: "some-registry-password",
				RegistryPort:     25777,
				RegistryUsername: "some-registry-username",
			})
			Expect(err).NotTo(HaveOccurred())

			Expect(manifest.DirectorUUID).To(Equal("some-director-uuid"))
			Expect(manifest.Name).To(Equal("turbulence"))
			Expect(manifest.Stemcells).To(Equal([]core.Stemcell{
				{
					Alias:   "default",
					Name:    "bosh-aws-xen-hvm-ubuntu-trusty-go_agent",
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
			Expect(manifest.InstanceGroups).To(HaveLen(1))
			Expect(manifest.InstanceGroups[0]).To(Equal(core.InstanceGroup{
				Instances: 1,
				Name:      "api",
				AZs:       []string{"z1"},
				Networks: []core.InstanceGroupNetwork{
					{
						Name: "private",
						StaticIPs: []string{
							"10.0.16.20",
						},
					},
				},
				VMType:             "some-vm-type",
				Stemcell:           "default",
				PersistentDiskType: "some-persistent-disk-type",
				Jobs: []core.InstanceGroupJob{
					{
						Name:    "turbulence_api",
						Release: "turbulence",
					},
					{
						Name:    "aws_cpi",
						Release: "bosh-aws-cpi",
					},
				},
			}))
			Expect(manifest.Releases).To(Equal([]core.Release{
				{
					Name:    "turbulence",
					Version: "latest",
				},
				{
					Name:    "bosh-aws-cpi",
					Version: "latest",
				},
			}))

			Expect(manifest.Properties).To(Equal(turbulence.Properties{
				TurbulenceAPI: &turbulence.PropertiesTurbulenceAPI{
					Certificate: turbulence.APICertificate,
					CPIJobName:  "aws_cpi",
					Director: turbulence.PropertiesTurbulenceAPIDirector{
						CACert:   "some-ca-cert",
						Host:     "some-bosh-target",
						Password: "some-bosh-password",
						Username: "some-bosh-username",
					},
					Password:   "turbulence-password",
					PrivateKey: turbulence.APIPrivateKey,
				},
				AWS: &iaas.PropertiesAWS{
					AccessKeyID:           "some-access-key-id",
					DefaultKeyName:        "some-default-key-name",
					DefaultSecurityGroups: []string{"some-default-security-group1"},
					Region:                "some-region",
					SecretAccessKey:       "some-secret-access-key",
				},
				Registry: &core.PropertiesRegistry{
					Host:     "some-registry-host",
					Password: "some-registry-password",
					Port:     25777,
					Username: "some-registry-username",
				},
				Blobstore: &core.PropertiesBlobstore{
					Address: "10.0.16.20",
					Port:    2520,
					Agent: core.PropertiesBlobstoreAgent{
						User:     "agent",
						Password: "agent-password",
					},
				},
				Agent: &core.PropertiesAgent{
					Mbus: "nats://nats:password@10.0.16.20:4222",
				},
			}))
		})

		It("generates a valid Turbulence BOSH-Lite manifest", func() {
			manifest, err := turbulence.NewManifest(turbulence.Config{
				DirectorUUID: "some-director-uuid",
				IPRange:      "10.244.4.0/24",
				BOSH: turbulence.ConfigBOSH{
					Target:   "some-bosh-target",
					Username: "some-bosh-username",
					Password: "some-bosh-password",
				},
				Name: "turbulence",
			}, iaas.NewWardenConfig())
			Expect(err).NotTo(HaveOccurred())

			Expect(manifest.DirectorUUID).To(Equal("some-director-uuid"))
			Expect(manifest.Name).To(Equal("turbulence"))
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
			Expect(manifest.InstanceGroups).To(HaveLen(1))
			Expect(manifest.InstanceGroups[0]).To(Equal(core.InstanceGroup{
				Instances: 1,
				Name:      "api",
				AZs:       []string{"z1"},
				Networks: []core.InstanceGroupNetwork{
					{
						Name: "private",
						StaticIPs: []string{
							"10.244.4.20",
						},
					},
				},
				VMType:             "default",
				Stemcell:           "default",
				PersistentDiskType: "default",
				Jobs: []core.InstanceGroupJob{
					{
						Name:    "turbulence_api",
						Release: "turbulence",
					},
					{
						Name:    "warden_cpi",
						Release: "bosh-warden-cpi",
					},
				},
			}))
			Expect(manifest.Releases).To(Equal([]core.Release{
				{
					Name:    "turbulence",
					Version: "latest",
				},
				{
					Name:    "bosh-warden-cpi",
					Version: "latest",
				},
			}))

			Expect(manifest.Properties).To(Equal(turbulence.Properties{
				TurbulenceAPI: &turbulence.PropertiesTurbulenceAPI{
					Certificate: turbulence.APICertificate,
					CPIJobName:  "warden_cpi",
					Director: turbulence.PropertiesTurbulenceAPIDirector{
						CACert:   turbulence.BOSHDirectorCACert,
						Host:     "some-bosh-target",
						Password: "some-bosh-password",
						Username: "some-bosh-username",
					},
					Password:   turbulence.DefaultPassword,
					PrivateKey: turbulence.APIPrivateKey,
				},
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

		Context("failure cases", func() {
			It("returns an error when the codr block is invalid", func() {
				_, err := turbulence.NewManifest(turbulence.Config{
					DirectorUUID: "some-director-uuid",
					IPRange:      "%%%%%%",
				}, iaas.NewWardenConfig())
				Expect(err).To(MatchError(`"%%%%%%" cannot parse CIDR block`))
			})

			It("returns an error when the codr block is small", func() {
				_, err := turbulence.NewManifest(turbulence.Config{
					DirectorUUID: "some-director-uuid",
					IPRange:      "10.244.4.0/31",
				}, iaas.NewWardenConfig())
				Expect(err).To(MatchError("can't allocate 17 ips from 8 available ips"))
			})
		})
	})

	Describe("FromYAML", func() {
		It("returns a Manifest matching the given YAML", func() {
			turbulenceManifest, err := ioutil.ReadFile("fixtures/turbulence_manifest.yml")
			Expect(err).NotTo(HaveOccurred())

			manifest, err := turbulence.FromYAML(turbulenceManifest)
			Expect(err).NotTo(HaveOccurred())

			Expect(manifest.DirectorUUID).To(Equal("some-director-uuid"))
			Expect(manifest.Name).To(Equal("turbulence"))
			Expect(manifest.Releases).To(HaveLen(2))
			Expect(manifest.Releases).To(ContainElement(core.Release{
				Name:    "turbulence",
				Version: "latest",
			}))

			Expect(manifest.Releases).To(ContainElement(core.Release{
				Name:    "bosh-warden-cpi",
				Version: "latest",
			}))

			Expect(manifest.Update).To(Equal(core.Update{
				Canaries:        1,
				CanaryWatchTime: "1000-180000",
				MaxInFlight:     1,
				Serial:          true,
				UpdateWatchTime: "1000-180000",
			}))

			Expect(manifest.Stemcells).To(ContainElement(core.Stemcell{
				Alias:   "default",
				Name:    "bosh-warden-boshlite-ubuntu-trusty-go_agent",
				Version: "latest",
			}))

			Expect(manifest.InstanceGroups).To(HaveLen(1))
			Expect(manifest.InstanceGroups[0]).To(Equal(core.InstanceGroup{
				Name:      "api",
				Instances: 1,
				AZs:       []string{"z1"},
				Networks: []core.InstanceGroupNetwork{{
					Name:      "private",
					StaticIPs: []string{"10.244.4.20"},
				}},
				VMType:             "default",
				PersistentDiskType: "default",
				Stemcell:           "default",
				Jobs: []core.InstanceGroupJob{
					{
						Name:    "turbulence_api",
						Release: "turbulence",
					},
					{
						Name:    "warden_cpi",
						Release: "bosh-warden-cpi",
					},
				},
			}))

			Expect(manifest.Properties).To(Equal(turbulence.Properties{
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
				TurbulenceAPI: &turbulence.PropertiesTurbulenceAPI{
					Certificate: turbulence.APICertificate,
					CPIJobName:  "warden_cpi",
					Director: turbulence.PropertiesTurbulenceAPIDirector{
						CACert:   turbulence.BOSHDirectorCACert,
						Host:     "some-bosh-target",
						Password: "some-bosh-password",
						Username: "some-bosh-username",
					},
					Password:   turbulence.DefaultPassword,
					PrivateKey: turbulence.APIPrivateKey,
				},
			}))
		})
	})

	Describe("ToYAML", func() {
		It("returns a YAML representation of the turbulence manifest", func() {
			turbulenceManifest, err := ioutil.ReadFile("fixtures/turbulence_manifest.yml")
			Expect(err).NotTo(HaveOccurred())

			manifest, err := turbulence.NewManifest(turbulence.Config{
				DirectorUUID: "some-director-uuid",
				Name:         "turbulence",
				IPRange:      "10.244.4.0/24",
				BOSH: turbulence.ConfigBOSH{
					Target:   "some-bosh-target",
					Username: "some-bosh-username",
					Password: "some-bosh-password",
				},
			}, iaas.NewWardenConfig())
			Expect(err).NotTo(HaveOccurred())

			yaml, err := manifest.ToYAML()
			Expect(err).NotTo(HaveOccurred())
			Expect(yaml).To(gomegamatchers.MatchYAML(turbulenceManifest))
		})
	})
})
