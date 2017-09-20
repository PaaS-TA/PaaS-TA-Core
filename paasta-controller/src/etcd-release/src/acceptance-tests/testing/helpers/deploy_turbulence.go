package helpers

import (
	"errors"
	"fmt"
	"time"

	"github.com/pivotal-cf-experimental/bosh-test/bosh"
	"github.com/pivotal-cf-experimental/destiny/core"
	"github.com/pivotal-cf-experimental/destiny/iaas"
	"github.com/pivotal-cf-experimental/destiny/turbulence"

	ginkgoConfig "github.com/onsi/ginkgo/config"
	turbulenceclient "github.com/pivotal-cf-experimental/bosh-test/turbulence"
)

func GetTurbulenceVMsFromManifest(manifest turbulence.Manifest) []bosh.VM {
	var vms []bosh.VM

	for _, job := range manifest.Jobs {
		for i := 0; i < job.Instances; i++ {
			vms = append(vms, bosh.VM{JobName: job.Name, Index: i, State: "running"})
		}
	}

	return vms
}

func NewTurbulenceClient(manifest turbulence.Manifest) turbulenceclient.Client {
	turbulenceUrl := fmt.Sprintf("https://turbulence:%s@%s:8080",
		manifest.Properties.TurbulenceAPI.Password,
		manifest.Jobs[0].Networks[0].StaticIPs[0])

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
		Name:         fmt.Sprintf("turbulence-etcd-%s", guid),
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

		if config.AWS.Subnet == "" {
			err = errors.New("AWSSubnet is required for AWS IAAS deployment")
			return turbulence.Manifest{}, err
		}
		var cidrBlock string
		cidrPool := core.NewCIDRPool("10.0.16.0", 24, 27)
		cidrBlock = cidrPool.Last()

		manifestConfig.IPRange = cidrBlock
		awsConfig.Subnets = []iaas.AWSConfigSubnet{{ID: config.AWS.Subnet, Range: cidrBlock, AZ: "us-east-1a"}}

		iaasConfig = awsConfig
	case "warden_cpi":
		iaasConfig = iaas.NewWardenConfig()

		var cidrBlock string
		cidrPool := core.NewCIDRPool("10.244.4.0", 24, 27)
		cidrBlock, err = cidrPool.Get(ginkgoConfig.GinkgoConfig.ParallelNode)
		if err != nil {
			return turbulence.Manifest{}, err
		}

		manifestConfig.IPRange = cidrBlock
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
