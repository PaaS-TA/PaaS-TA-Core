package helpers

import (
	"errors"
	"fmt"

	"github.com/cloudfoundry-incubator/consul-release/src/acceptance-tests/testing/consulclient"
	"github.com/pivotal-cf-experimental/bosh-test/bosh"
	"github.com/pivotal-cf-experimental/destiny/consul"
	"github.com/pivotal-cf-experimental/destiny/core"
	"github.com/pivotal-cf-experimental/destiny/iaas"

	ginkgoConfig "github.com/onsi/ginkgo/config"
)

type ManifestGenerator func(consul.ConfigV2, iaas.Config) (consul.ManifestV2, error)

func DeployConsulWithTurbulence(deploymentPrefix string, count int, client bosh.Client, config Config) (manifest consul.ManifestV2, kv consulclient.HTTPKV, err error) {
	return deployConsul(deploymentPrefix, count, client, config, ConsulReleaseVersion(), consul.NewManifestWithTurbulenceAgent)
}

func DeployConsulWithInstanceCount(deploymentPrefix string, count int, client bosh.Client, config Config) (manifest consul.ManifestV2, kv consulclient.HTTPKV, err error) {
	return DeployConsulWithInstanceCountAndReleaseVersion(deploymentPrefix, count, client, config, ConsulReleaseVersion())
}

func DeployConsulWithJobLevelConsulProperties(deploymentPrefix string, client bosh.Client, config Config) (manifest consul.ManifestV2, err error) {
	manifest, _, err = deployConsul(deploymentPrefix, 1, client, config, ConsulReleaseVersion(), consul.NewManifestWithJobLevelProperties)
	return
}

func DeployConsulWithInstanceCountAndReleaseVersion(deploymentPrefix string, count int, client bosh.Client, config Config, releaseVersion string) (manifest consul.ManifestV2, kv consulclient.HTTPKV, err error) {
	return deployConsul(deploymentPrefix, count, client, config, releaseVersion, consul.NewManifestV2)
}

func deployConsul(deploymentPrefix string, count int, client bosh.Client, config Config, releaseVersion string, manifestGenerator ManifestGenerator) (manifest consul.ManifestV2, kv consulclient.HTTPKV, err error) {
	guid, err := NewGUID()
	if err != nil {
		return
	}

	info, err := client.Info()
	if err != nil {
		return
	}

	manifestConfig := consul.ConfigV2{
		DirectorUUID:   info.UUID,
		Name:           fmt.Sprintf("consul-%s-%s", deploymentPrefix, guid),
		TurbulenceHost: config.TurbulenceHost,
		WindowsClients: config.WindowsClients,
	}

	var iaasConfig iaas.Config
	switch info.CPI {
	case "aws_cpi":
		manifestConfig.PersistentDiskType = config.BOSH.Errand.DefaultPersistentDiskType
		manifestConfig.VMType = config.BOSH.Errand.DefaultVMType

		awsConfig := buildAWSConfig(config)
		if len(config.AWS.Subnets) > 0 {
			subnet := config.AWS.Subnets[0]

			var cidrBlock string
			cidrPool := NewCIDRPool(subnet.Range, 24, 27)
			cidrBlock, err = cidrPool.Get(ginkgoConfig.GinkgoConfig.ParallelNode)
			if err != nil {
				return
			}

			awsConfig.Subnets = append(awsConfig.Subnets, iaas.AWSConfigSubnet{ID: subnet.ID, Range: cidrBlock, AZ: subnet.AZ, SecurityGroup: subnet.SecurityGroup})
			manifestConfig.AZs = append(manifestConfig.AZs, consul.ConfigAZ{IPRange: cidrBlock, Nodes: count, Name: "z1"})
		} else {
			err = errors.New("AWSSubnet is required for AWS IAAS deployment")
			return
		}

		iaasConfig = awsConfig
	case "warden_cpi":
		iaasConfig = iaas.NewWardenConfig()

		var cidrBlock string
		cidrPool := NewCIDRPool("10.244.4.0", 24, 27)
		cidrBlock, err = cidrPool.Get(ginkgoConfig.GinkgoConfig.ParallelNode)
		if err != nil {
			return
		}

		manifestConfig.AZs = []consul.ConfigAZ{
			{
				IPRange: cidrBlock,
				Nodes:   count,
				Name:    "z1",
			},
		}
	default:
		err = errors.New("unknown infrastructure type")
		return
	}

	manifest, err = manifestGenerator(manifestConfig, iaasConfig)
	if err != nil {
		return
	}

	for i := range manifest.Releases {
		if manifest.Releases[i].Name == "consul" {
			manifest.Releases[i].Version = releaseVersion
		}
	}

	yaml, err := manifest.ToYAML()
	if err != nil {
		return
	}

	yaml, err = client.ResolveManifestVersions(yaml)
	if err != nil {
		return
	}

	err = consul.FromYAML(yaml, &manifest)
	if err != nil {
		return
	}

	_, err = client.Deploy(yaml)
	if err != nil {
		return
	}

	err = VerifyDeploymentRelease(client, manifestConfig.Name, releaseVersion)
	if err != nil {
		return
	}

	kv = consulclient.NewHTTPKV(fmt.Sprintf("http://%s:6769", manifest.InstanceGroups[1].Networks[0].StaticIPs[0]))
	return
}

func DeployMultiAZConsul(deploymentPrefix string, client bosh.Client, config Config) (manifest consul.Manifest, err error) {
	guid, err := NewGUID()
	if err != nil {
		return
	}

	info, err := client.Info()
	if err != nil {
		return
	}

	manifestConfig := consul.Config{
		DirectorUUID: info.UUID,
		Name:         fmt.Sprintf("consul-%s-%s", deploymentPrefix, guid),
	}

	var iaasConfig iaas.Config
	switch info.CPI {
	case "aws_cpi":
		awsConfig := buildAWSConfig(config)
		if len(config.AWS.Subnets) >= 2 {
			subnet := config.AWS.CloudConfigSubnets[0]

			awsConfig.Subnets = append(awsConfig.Subnets, iaas.AWSConfigSubnet{ID: subnet.ID, Range: subnet.Range, AZ: subnet.AZ, SecurityGroup: subnet.SecurityGroup})
			manifestConfig.Networks = append(manifestConfig.Networks, consul.ConfigNetwork{IPRange: subnet.Range, Nodes: 2})

			subnet = config.AWS.CloudConfigSubnets[1]

			awsConfig.Subnets = append(awsConfig.Subnets, iaas.AWSConfigSubnet{ID: subnet.ID, Range: subnet.Range, AZ: subnet.AZ, SecurityGroup: subnet.SecurityGroup})
			manifestConfig.Networks = append(manifestConfig.Networks, consul.ConfigNetwork{IPRange: subnet.Range, Nodes: 1})
		} else {
			err = errors.New("AWSSubnet is required for AWS IAAS deployment")
			return
		}

		iaasConfig = awsConfig
	case "warden_cpi":
		iaasConfig = iaas.NewWardenConfig()

		manifestConfig.Networks = []consul.ConfigNetwork{
			{IPRange: "10.244.4.0/24", Nodes: 2},
			{IPRange: "10.244.5.0/24", Nodes: 1},
		}
	default:
		err = errors.New("unknown infrastructure type")
		return
	}

	manifest, err = consul.NewManifest(manifestConfig, iaasConfig)
	if err != nil {
		return
	}

	for i := range manifest.Releases {
		if manifest.Releases[i].Name == "consul" {
			manifest.Releases[i].Version = ConsulReleaseVersion()
		}
	}

	yaml, err := manifest.ToYAML()
	if err != nil {
		return
	}

	yaml, err = client.ResolveManifestVersions(yaml)
	if err != nil {
		return
	}

	err = consul.FromYAML(yaml, &manifest)
	if err != nil {
		return
	}

	_, err = client.Deploy(yaml)
	if err != nil {
		return
	}

	err = VerifyDeploymentRelease(client, manifestConfig.Name, ConsulReleaseVersion())
	if err != nil {
		return
	}

	return
}

func DeployMultiAZConsulMigration(client bosh.Client, config Config, deploymentName string) (consul.ManifestV2, error) {
	info, err := client.Info()
	if err != nil {
		return consul.ManifestV2{}, err
	}

	manifestConfig := consul.ConfigV2{
		DirectorUUID:   info.UUID,
		Name:           deploymentName,
		WindowsClients: config.WindowsClients,
	}

	var iaasConfig iaas.Config
	switch info.CPI {
	case "aws_cpi":
		manifestConfig.PersistentDiskType = config.BOSH.Errand.DefaultPersistentDiskType
		manifestConfig.VMType = config.BOSH.Errand.DefaultVMType

		awsConfig := buildAWSConfig(config)
		if len(config.AWS.CloudConfigSubnets) >= 2 {
			subnet := config.AWS.CloudConfigSubnets[0]

			awsConfig.Subnets = append(awsConfig.Subnets, iaas.AWSConfigSubnet{ID: subnet.ID, Range: subnet.Range, AZ: subnet.AZ, SecurityGroup: subnet.SecurityGroup})
			manifestConfig.AZs = append(manifestConfig.AZs, consul.ConfigAZ{Name: "z1", IPRange: subnet.Range, Nodes: 2})

			subnet = config.AWS.CloudConfigSubnets[1]

			awsConfig.Subnets = append(awsConfig.Subnets, iaas.AWSConfigSubnet{ID: subnet.ID, Range: subnet.Range, AZ: subnet.AZ, SecurityGroup: subnet.SecurityGroup})
			manifestConfig.AZs = append(manifestConfig.AZs, consul.ConfigAZ{Name: "z2", IPRange: subnet.Range, Nodes: 1})
		} else {
			return consul.ManifestV2{}, errors.New("AWSSubnet is required for AWS IAAS deployment")
		}

		iaasConfig = awsConfig
	case "warden_cpi":
		iaasConfig = iaas.NewWardenConfig()

		var cidrBlock string
		cidrPool := NewCIDRPool("10.244.4.0", 24, 27)
		cidrBlock, err = cidrPool.Get(0)
		if err != nil {
			return consul.ManifestV2{}, err
		}

		var cidrBlock2 string
		cidrPool2 := NewCIDRPool("10.244.5.0", 24, 27)
		cidrBlock2, err = cidrPool2.Get(0)
		if err != nil {
			return consul.ManifestV2{}, err
		}

		manifestConfig.AZs = []consul.ConfigAZ{
			{
				Name:    "z1",
				IPRange: cidrBlock,
				Nodes:   2,
			},
			{
				Name:    "z2",
				IPRange: cidrBlock2,
				Nodes:   1,
			},
		}
	default:
		return consul.ManifestV2{}, errors.New("unknown infrastructure type")
	}

	manifest, err := consul.NewManifestV2(manifestConfig, iaasConfig)
	if err != nil {
		return consul.ManifestV2{}, err
	}

	consulInstanceGroup, err := manifest.GetInstanceGroup("consul")
	if err != nil {
		return consul.ManifestV2{}, err
	}
	consulInstanceGroup.MigratedFrom = []core.InstanceGroupMigratedFrom{
		{
			Name: "consul_z1",
			AZ:   "z1",
		},
		{
			Name: "consul_z2",
			AZ:   "z2",
		},
	}

	consulTestConsumerInstanceGroup, err := manifest.GetInstanceGroup("test_consumer")
	if err != nil {
		return consul.ManifestV2{}, err
	}

	consulTestConsumerInstanceGroup.MigratedFrom = []core.InstanceGroupMigratedFrom{
		{
			Name: "consul_test_consumer",
			AZ:   "z1",
		},
	}

	for i := range manifest.Releases {
		if manifest.Releases[i].Name == "consul" {
			manifest.Releases[i].Version = ConsulReleaseVersion()
		}
	}

	manifestYAML, err := manifest.ToYAML()
	if err != nil {
		return consul.ManifestV2{}, err
	}

	_, err = client.Deploy(manifestYAML)
	if err != nil {
		return consul.ManifestV2{}, err
	}

	if err := VerifyDeploymentRelease(client, manifestConfig.Name, ConsulReleaseVersion()); err != nil {
		return consul.ManifestV2{}, err
	}

	return manifest, nil
}

func VerifyDeploymentRelease(client bosh.Client, deploymentName string, releaseVersion string) (err error) {
	deployments, err := client.Deployments()
	if err != nil {
		return
	}

	for _, deployment := range deployments {
		if deployment.Name == deploymentName {
			for _, release := range deployment.Releases {
				if release.Name == "consul" {
					switch {
					case len(release.Versions) > 1:
						err = errors.New("too many releases")
					case len(release.Versions) == 1 && release.Versions[0] != releaseVersion:
						err = fmt.Errorf("expected consul-release version %q but got %q", releaseVersion, release.Versions[0])
					}
				}
			}
		}
	}

	return
}

func buildAWSConfig(config Config) iaas.AWSConfig {
	return iaas.AWSConfig{
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
}
