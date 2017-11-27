package helpers

import (
	"errors"
	"fmt"

	"github.com/pivotal-cf-experimental/bosh-test/bosh"
	"github.com/pivotal-cf-experimental/destiny/etcdwithops"
	"github.com/pivotal-cf-experimental/destiny/ops"
)

func NewEtcdManifestWithInstanceCountAndReleaseVersion(deploymentPrefix string, instanceCount int, enableSSL bool, boshClient bosh.Client, releaseVersion string) (string, error) {
	manifestName := fmt.Sprintf("etcd-%s", deploymentPrefix)

	//TODO: AZs should be pulled from integration_config
	var (
		manifest string
		err      error
	)
	manifest, err = etcdwithops.NewManifestV2(etcdwithops.ConfigV2{
		Name:      manifestName,
		AZs:       []string{"z1", "z2"},
		EnableSSL: enableSSL,
	})

	if err != nil {
		return "", err
	}

	manifest, err = ops.ApplyOp(manifest, ops.Op{
		Type:  "replace",
		Path:  "/releases/name=etcd/version",
		Value: releaseVersion,
	})
	if err != nil {
		return "", err
	}

	manifest, err = ops.ApplyOp(manifest, ops.Op{
		Type:  "replace",
		Path:  "/instance_groups/name=etcd/instances",
		Value: instanceCount,
	})
	if err != nil {
		return "", err
	}

	manifestYAML, err := boshClient.ResolveManifestVersionsV2([]byte(manifest))
	if err != nil {
		return "", err
	}

	return string(manifestYAML), nil
}

func NewEtcdManifestWithInstanceCount(deploymentPrefix string, instanceCount int, enableSSL bool, boshClient bosh.Client) (string, error) {
	return NewEtcdManifestWithInstanceCountAndReleaseVersion(deploymentPrefix, instanceCount, enableSSL, boshClient, EtcdDevReleaseVersion())
}

func DeployEtcdWithInstanceCountAndReleaseVersion(deploymentPrefix string, instanceCount int, enableSSL bool, boshClient bosh.Client, releaseVersion string) (string, error) {
	manifest, err := NewEtcdManifestWithInstanceCountAndReleaseVersion(deploymentPrefix, instanceCount, enableSSL, boshClient, releaseVersion)
	if err != nil {
		return "", err
	}

	_, err = boshClient.Deploy([]byte(manifest))
	if err != nil {
		return "", err
	}

	return manifest, nil
}

func DeployEtcdWithInstanceCount(deploymentPrefix string, instanceCount int, enableSSL bool, boshClient bosh.Client) (string, error) {
	return DeployEtcdWithInstanceCountAndReleaseVersion(deploymentPrefix, instanceCount, enableSSL, boshClient, EtcdDevReleaseVersion())
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
