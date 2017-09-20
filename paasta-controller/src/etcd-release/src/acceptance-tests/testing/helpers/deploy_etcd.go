package helpers

import (
	"errors"
	"fmt"

	"github.com/pivotal-cf-experimental/bosh-test/bosh"
	"github.com/pivotal-cf-experimental/destiny/core"
	"github.com/pivotal-cf-experimental/destiny/etcd"
	"github.com/pivotal-cf-experimental/destiny/iaas"

	ginkgoConfig "github.com/onsi/ginkgo/config"
)

func ResolveVersionsAndDeploy(manifest etcd.Manifest, client bosh.Client) (err error) {
	yaml, err := manifest.ToYAML()
	if err != nil {
		return
	}

	yaml, err = client.ResolveManifestVersions(yaml)
	if err != nil {
		return
	}

	manifest, err = etcd.FromYAML(yaml)
	if err != nil {
		return
	}

	_, err = client.Deploy(yaml)
	if err != nil {
		return
	}

	return
}

func buildManifestInputs(deploymentPrefix string, config Config, client bosh.Client) (manifestConfig etcd.Config, iaasConfig iaas.Config, err error) {
	guid, err := NewGUID()
	if err != nil {
		return
	}

	info, err := client.Info()
	if err != nil {
		return
	}

	manifestConfig = etcd.Config{
		DirectorUUID:  info.UUID,
		Name:          fmt.Sprintf("etcd-%s-%s", deploymentPrefix, guid),
		IPTablesAgent: config.IPTablesAgent,
	}

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
			return
		}
		var cidrBlock string
		cidrPool := core.NewCIDRPool("10.0.16.0", 24, 27)
		cidrBlock, err = cidrPool.Get(ginkgoConfig.GinkgoConfig.ParallelNode)
		if err != nil {
			return
		}

		manifestConfig.IPRange = cidrBlock
		awsConfig.Subnets = []iaas.AWSConfigSubnet{{ID: config.AWS.Subnet, Range: cidrBlock, AZ: "us-east-1a"}}

		iaasConfig = awsConfig
	case "warden_cpi":
		iaasConfig = iaas.NewWardenConfig()

		var cidrBlock string
		cidrPool := core.NewCIDRPool("10.244.16.0", 24, 27)
		cidrBlock, err = cidrPool.Get(ginkgoConfig.GinkgoConfig.ParallelNode)
		if err != nil {
			return
		}
		manifestConfig.IPRange = cidrBlock
	default:
		err = errors.New("unknown infrastructure type")
	}

	return
}

func DeployEtcdWithInstanceCountAndReleaseVersion(deploymentPrefix string, count int, client bosh.Client, config Config, enableSSL bool, releaseVersion string) (manifest etcd.Manifest, err error) {
	manifest, err = NewEtcdWithInstanceCount(deploymentPrefix, count, client, config, enableSSL)
	if err != nil {
		return
	}

	for i := range manifest.Releases {
		if manifest.Releases[i].Name == "etcd" {
			manifest.Releases[i].Version = releaseVersion
		}
	}

	err = ResolveVersionsAndDeploy(manifest, client)
	return
}

func DeployEtcdWithInstanceCount(deploymentPrefix string, count int, client bosh.Client, config Config, enableSSL bool) (manifest etcd.Manifest, err error) {
	manifest, err = DeployEtcdWithInstanceCountAndReleaseVersion(deploymentPrefix, count, client, config, enableSSL, EtcdDevReleaseVersion())
	if err != nil {
		return
	}

	err = ResolveVersionsAndDeploy(manifest, client)
	return
}

func NewEtcdWithInstanceCount(deploymentPrefix string, count int, client bosh.Client, config Config, enableSSL bool) (manifest etcd.Manifest, err error) {
	manifestConfig, iaasConfig, err := buildManifestInputs(deploymentPrefix, config, client)
	if err != nil {
		return
	}

	if enableSSL {
		manifest, err = etcd.NewTLSManifest(manifestConfig, iaasConfig)
		if err != nil {
			return
		}
	} else {
		manifest, err = etcd.NewManifest(manifestConfig, iaasConfig)
		if err != nil {
			return
		}
	}

	manifest, err = SetEtcdInstanceCount(count, manifest)

	return
}

func SetEtcdInstanceCount(count int, manifest etcd.Manifest) (etcd.Manifest, error) {
	manifest, err := manifest.SetJobInstanceCount("etcd_z1", count)
	if err != nil {
		return manifest, err
	}
	jobIndex, err := FindJobIndexByName(manifest, "etcd_z1")
	if err != nil {
		return manifest, err
	}

	manifest.Properties = etcd.SetEtcdProperties(manifest.Jobs[jobIndex], manifest.Properties)

	return manifest, nil
}

func SetTestConsumerInstanceCount(count int, manifest etcd.Manifest) (etcd.Manifest, error) {
	manifest, err := manifest.SetJobInstanceCount("testconsumer_z1", count)
	if err != nil {
		return manifest, err
	}

	return manifest, nil
}

func NewEtcdManifestWithTLSUpgrade(manifestName string, client bosh.Client, config Config) (manifest etcd.Manifest, err error) {
	manifestConfig, iaasConfig, err := buildManifestInputs(manifestName, config, client)
	if err != nil {
		return
	}

	manifest = etcd.NewTLSUpgradeManifest(manifestConfig, iaasConfig)
	if manifestName != "" {
		manifest.Name = manifestName
	}

	return
}

func FindJobIndexByName(manifest etcd.Manifest, jobName string) (int, error) {
	for i, job := range manifest.Jobs {
		if job.Name == jobName {
			return i, nil
		}
	}
	return -1, errors.New("job not found")
}

func VerifyDeploymentRelease(client bosh.Client, deploymentName string, releaseVersion string) (err error) {
	deployments, err := client.Deployments()
	if err != nil {
		return
	}

	for _, deployment := range deployments {
		if deployment.Name == deploymentName {
			for _, release := range deployment.Releases {
				if release.Name == "etcd" {
					switch {
					case len(release.Versions) > 1:
						err = errors.New("too many releases")
					case len(release.Versions) == 1 && release.Versions[0] != releaseVersion:
						err = fmt.Errorf("expected etcd-release version %q but got %q", releaseVersion, release.Versions[0])
					}
				}
			}
		}
	}

	return
}
