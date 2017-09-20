package helpers

import (
	"errors"
	"fmt"
	"time"

	"github.com/pivotal-cf-experimental/bosh-test/bosh"
	"github.com/pivotal-cf-experimental/destiny/iaas"
	"github.com/pivotal-cf-experimental/destiny/turbulence"

	turbulenceclient "github.com/pivotal-cf-experimental/bosh-test/turbulence"
)

func GetTurbulenceVMsFromManifest(manifest turbulence.Manifest) []bosh.VM {
	var vms []bosh.VM

	for _, job := range manifest.InstanceGroups {
		for i := 0; i < job.Instances; i++ {
			vms = append(vms, bosh.VM{JobName: job.Name, Index: i, State: "running"})
		}
	}

	return vms
}

func NewTurbulenceClient(manifest turbulence.Manifest) turbulenceclient.Client {
	turbulenceUrl := fmt.Sprintf("https://turbulence:%s@%s:8080",
		manifest.Properties.TurbulenceAPI.Password,
		manifest.InstanceGroups[0].Networks[0].StaticIPs[0])

	return turbulenceclient.NewClient(turbulenceUrl, 5*time.Minute, 2*time.Second)
}

func DeployTurbulence(client bosh.Client, config Config) (turbulence.Manifest, error) {
	info, err := client.Info()
	if err != nil {
		return turbulence.Manifest{}, err
	}

	guid, err := NewGUID()
	if err != nil {
		return turbulence.Manifest{}, err
	}

	manifestConfig := turbulence.Config{
		DirectorUUID: info.UUID,
		Name:         "turbulence-consul-" + guid,
		BOSH: turbulence.ConfigBOSH{
			Target:         config.BOSH.Target,
			Username:       config.BOSH.Username,
			Password:       config.BOSH.Password,
			DirectorCACert: config.BOSH.DirectorCACert,
		},
	}

	var iaasConfig iaas.Config
	switch info.CPI {
	case "aws_cpi":
		awsConfig := iaas.AWSConfig{
			AccessKeyID:           config.AWS.AccessKeyID,
			SecretAccessKey:       config.AWS.SecretAccessKey,
			DefaultKeyName:        config.AWS.DefaultKeyName,
			DefaultSecurityGroups: config.AWS.DefaultSecurityGroups,
			Region:                config.AWS.Region,
			RegistryHost:          config.Registry.Host,
			RegistryPassword:      config.Registry.Password,
			RegistryPort:          config.Registry.Port,
			RegistryUsername:      config.Registry.Username,
		}

		manifestConfig.BOSH.PersistentDiskType = config.BOSH.Errand.DefaultPersistentDiskType
		manifestConfig.BOSH.VMType = config.BOSH.Errand.DefaultVMType

		if len(config.AWS.Subnets) > 0 {
			subnet := config.AWS.Subnets[0]

			var cidrBlock string
			cidrPool := NewCIDRPool(subnet.Range, 24, 27)
			cidrBlock, err = cidrPool.Last()
			if err != nil {
				return turbulence.Manifest{}, err
			}

			awsConfig.Subnets = append(awsConfig.Subnets, iaas.AWSConfigSubnet{ID: subnet.ID, Range: cidrBlock, AZ: subnet.AZ, SecurityGroup: subnet.SecurityGroup})
			manifestConfig.IPRange = cidrBlock
		} else {
			return turbulence.Manifest{}, errors.New("aws.subnet is required for AWS IAAS deployment")
		}

		iaasConfig = awsConfig
	case "warden_cpi":
		var cidrBlock string
		cidrPool := NewCIDRPool("10.244.4.0", 24, 27)
		cidrBlock, err = cidrPool.Last()
		if err != nil {
			return turbulence.Manifest{}, err
		}

		manifestConfig.IPRange = cidrBlock
		iaasConfig = iaas.NewWardenConfig()
	default:
		return turbulence.Manifest{}, errors.New("unknown infrastructure type")
	}

	turbulenceManifest, err := turbulence.NewManifest(manifestConfig, iaasConfig)
	if err != nil {
		return turbulence.Manifest{}, err
	}

	yaml, err := turbulenceManifest.ToYAML()
	if err != nil {
		return turbulence.Manifest{}, err
	}

	yaml, err = client.ResolveManifestVersions(yaml)
	if err != nil {
		return turbulence.Manifest{}, err
	}

	turbulenceManifest, err = turbulence.FromYAML(yaml)
	if err != nil {
		return turbulence.Manifest{}, err
	}

	_, err = client.Deploy(yaml)
	if err != nil {
		return turbulence.Manifest{}, err
	}

	return turbulenceManifest, nil
}
