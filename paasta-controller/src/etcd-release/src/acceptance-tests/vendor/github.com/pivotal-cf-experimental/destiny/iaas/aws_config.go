package iaas

import (
	"fmt"

	"github.com/pivotal-cf-experimental/destiny/core"
)

type AWSConfig struct {
	AccessKeyID           string
	SecretAccessKey       string
	DefaultKeyName        string
	DefaultSecurityGroups []string
	Region                string
	Subnets               []AWSConfigSubnet
	RegistryHost          string
	RegistryPassword      string
	RegistryPort          int
	RegistryUsername      string
	StaticIP              string
}

type AWSConfigSubnet struct {
	ID            string
	Range         string
	AZ            string
	SecurityGroup string
}

func (a AWSConfig) NetworkSubnet(ipRange string) core.NetworkSubnetCloudProperties {
	for _, subnet := range a.Subnets {
		if subnet.Range == ipRange {

			var securityGroups []string
			if subnet.SecurityGroup != "" {
				securityGroups = []string{
					subnet.SecurityGroup,
				}
			}

			return core.NetworkSubnetCloudProperties{
				Subnet:         subnet.ID,
				SecurityGroups: securityGroups,
			}
		}
	}

	return core.NetworkSubnetCloudProperties{}
}

func (a AWSConfig) Compilation(availabilityZone string) core.CompilationCloudProperties {
	return core.CompilationCloudProperties{
		InstanceType:     "c3.large",
		AvailabilityZone: availabilityZone,
		EphemeralDisk: &core.CompilationCloudPropertiesEphemeralDisk{
			Size: 2048,
			Type: "gp2",
		},
	}
}

func (a AWSConfig) ResourcePool(ipRange string) core.ResourcePoolCloudProperties {
	az := ""

	for _, subnet := range a.Subnets {
		if subnet.Range == ipRange {
			az = subnet.AZ
		}
	}

	return core.ResourcePoolCloudProperties{
		InstanceType:     "m3.medium",
		AvailabilityZone: az,
		EphemeralDisk: &core.ResourcePoolCloudPropertiesEphemeralDisk{
			Size: 10240,
			Type: "gp2",
		},
	}
}

func (a AWSConfig) CPI() CPI {
	return CPI{
		JobName:     "aws_cpi",
		ReleaseName: "bosh-aws-cpi",
	}
}

func (a AWSConfig) Properties(staticIP string) Properties {
	return Properties{
		AWS: &PropertiesAWS{
			AccessKeyID:           a.AccessKeyID,
			SecretAccessKey:       a.SecretAccessKey,
			DefaultKeyName:        a.DefaultKeyName,
			DefaultSecurityGroups: a.DefaultSecurityGroups,
			Region:                a.Region,
		},
		Registry: &core.PropertiesRegistry{
			Host:     a.RegistryHost,
			Password: a.RegistryPassword,
			Port:     a.RegistryPort,
			Username: a.RegistryUsername,
		},
		Blobstore: &core.PropertiesBlobstore{
			Address: staticIP,
			Port:    2520,
			Agent: core.PropertiesBlobstoreAgent{
				User:     "agent",
				Password: "agent-password",
			},
		},
		Agent: &core.PropertiesAgent{
			Mbus: fmt.Sprintf("nats://nats:password@%s:4222", staticIP),
		},
	}
}

func (AWSConfig) Stemcell() string {
	return AWSLinuxStemcell
}

func (AWSConfig) WindowsStemcell() string {
	return AWSWindowsStemcell
}
