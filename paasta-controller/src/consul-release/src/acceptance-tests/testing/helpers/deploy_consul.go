package helpers

import (
	"errors"
	"fmt"

	"github.com/pivotal-cf-experimental/bosh-test/bosh"
	"github.com/pivotal-cf-experimental/destiny/consul"
	"github.com/pivotal-cf-experimental/destiny/ops"
)

func NewConsulManifestWithInstanceCountAndReleaseVersion(deploymentPrefix string, instanceCount int, windowsClients bool, boshClient bosh.Client, releaseVersion string) (string, error) {
	manifestName := fmt.Sprintf("consul-%s", deploymentPrefix)

	//TODO: AZs should be pulled from integration_config
	var (
		manifest string
		err      error
	)
	if windowsClients {
		manifest, err = consul.NewManifestV2Windows(consul.ConfigV2{
			Name: manifestName,
			AZs:  []string{"z1", "z2"},
		})
	} else {
		manifest, err = consul.NewManifestV2(consul.ConfigV2{
			Name: manifestName,
			AZs:  []string{"z1", "z2"},
		})
	}

	if err != nil {
		return "", err
	}

	manifest, err = ops.ApplyOp(manifest, ops.Op{
		Type:  "replace",
		Path:  "/releases/name=consul/version",
		Value: releaseVersion,
	})
	if err != nil {
		return "", err
	}

	manifest, err = ops.ApplyOp(manifest, ops.Op{
		Type:  "replace",
		Path:  "/instance_groups/name=consul/instances",
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

func NewConsulManifestWithInstanceCount(deploymentPrefix string, instanceCount int, windowsClients bool, boshClient bosh.Client) (string, error) {
	return NewConsulManifestWithInstanceCountAndReleaseVersion(deploymentPrefix, instanceCount, windowsClients, boshClient, ConsulReleaseVersion())
}

func DeployConsulWithInstanceCountAndReleaseVersion(deploymentPrefix string, instanceCount int, windowsClients bool, boshClient bosh.Client, releaseVersion string) (string, error) {
	manifest, err := NewConsulManifestWithInstanceCountAndReleaseVersion(deploymentPrefix, instanceCount, windowsClients, boshClient, releaseVersion)
	if err != nil {
		return "", err
	}

	_, err = boshClient.Deploy([]byte(manifest))
	if err != nil {
		return "", err
	}

	return manifest, nil
}

func DeployConsulWithInstanceCount(deploymentPrefix string, instanceCount int, windowsClients bool, boshClient bosh.Client) (string, error) {
	return DeployConsulWithInstanceCountAndReleaseVersion(deploymentPrefix, instanceCount, windowsClients, boshClient, ConsulReleaseVersion())
}

func VerifyDeploymentRelease(client bosh.Client, deploymentName string, releaseVersion string) error {
	deployments, err := client.Deployments()
	if err != nil {
		return err
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

	return err
}
