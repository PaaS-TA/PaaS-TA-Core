package iaas_test

import (
	"fmt"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/pivotal-cf-experimental/destiny/core"
	"github.com/pivotal-cf-experimental/destiny/iaas"
)

var _ = Describe("AWS Config", func() {
	var (
		awsConfig iaas.AWSConfig
	)

	BeforeEach(func() {
		awsConfig = iaas.AWSConfig{
			AccessKeyID:           "some-access-key-id",
			SecretAccessKey:       "some-secret-access-key",
			DefaultKeyName:        "some-default-key-name",
			DefaultSecurityGroups: []string{"some-default-security-group"},
			Region:                "some-region",
			Subnets: []iaas.AWSConfigSubnet{
				{ID: "some-subnet-1", Range: "127.0.0.1/24", AZ: "some-az-1a", SecurityGroup: "some-security-group-1"},
				{ID: "some-subnet-2", Range: "127.0.0.2/24", AZ: "some-az-1c", SecurityGroup: "some-security-group-2"},
			},
			RegistryHost:     "some-host",
			RegistryPassword: "some-password",
			RegistryPort:     1234,
			RegistryUsername: "some-username",
		}
	})

	Describe("NetworkSubnet", func() {
		It("returns network subnet cloud properties given a range", func() {
			subnetCloudProperties := awsConfig.NetworkSubnet("127.0.0.2/24")
			Expect(subnetCloudProperties).To(Equal(core.NetworkSubnetCloudProperties{
				Subnet:         "some-subnet-2",
				SecurityGroups: []string{"some-security-group-2"},
			}))
		})

		Context("when security group is not provided", func() {
			It("returns network subnet cloud properties given a range", func() {
				awsConfig.Subnets[1].SecurityGroup = ""
				subnetCloudProperties := awsConfig.NetworkSubnet("127.0.0.2/24")
				Expect(subnetCloudProperties).To(Equal(core.NetworkSubnetCloudProperties{
					Subnet: "some-subnet-2",
				}))
			})
		})
	})

	Describe("Compilation", func() {
		It("returns a compilation specific to AWS", func() {
			compilationCloudProperties := awsConfig.Compilation()
			Expect(compilationCloudProperties).To(Equal(core.CompilationCloudProperties{
				InstanceType:     "c3.large",
				AvailabilityZone: "us-east-1a",
				EphemeralDisk: &core.CompilationCloudPropertiesEphemeralDisk{
					Size: 2048,
					Type: "gp2",
				},
			}))
		})
	})

	Describe("ResourcePool", func() {
		It("returns a resource pool given a range", func() {
			resourcePoolCloudProperties := awsConfig.ResourcePool("127.0.0.2/24")
			Expect(resourcePoolCloudProperties).To(Equal(core.ResourcePoolCloudProperties{
				InstanceType:     "m3.medium",
				AvailabilityZone: "some-az-1c",
				EphemeralDisk: &core.ResourcePoolCloudPropertiesEphemeralDisk{
					Size: 10240,
					Type: "gp2",
				},
			}))
		})
	})

	Describe("CPI", func() {
		It("returns the cpi specific to AWS", func() {
			cpi := awsConfig.CPI()
			Expect(cpi).To(Equal(iaas.CPI{
				JobName:     "aws_cpi",
				ReleaseName: "bosh-aws-cpi",
			}))
		})
	})

	Describe("Properties", func() {
		It("returns the properties specific to AWS", func() {
			properties := awsConfig.Properties("some-static-ip")

			Expect(properties).To(Equal(iaas.Properties{
				AWS: &iaas.PropertiesAWS{
					AccessKeyID:           "some-access-key-id",
					SecretAccessKey:       "some-secret-access-key",
					DefaultKeyName:        "some-default-key-name",
					DefaultSecurityGroups: []string{"some-default-security-group"},
					Region:                "some-region",
				},
				Registry: &core.PropertiesRegistry{
					Host:     "some-host",
					Password: "some-password",
					Port:     1234,
					Username: "some-username",
				},
				Blobstore: &core.PropertiesBlobstore{
					Address: "some-static-ip",
					Port:    2520,
					Agent: core.PropertiesBlobstoreAgent{
						User:     "agent",
						Password: "agent-password",
					},
				},
				Agent: &core.PropertiesAgent{
					Mbus: fmt.Sprintf("nats://nats:password@some-static-ip:4222"),
				},
			}))
		})
	})
})
