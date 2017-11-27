package helpers

import (
	"fmt"

	"github.com/pivotal-cf-experimental/bosh-test/bosh"
	"github.com/pivotal-cf-experimental/destiny/turbulence"
)

func DeployTurbulence(suffix string, client bosh.Client) (string, error) {
	boshConfig := client.GetConfig()

	manifest, err := turbulence.NewManifestV2(turbulence.ConfigV2{
		Name:             fmt.Sprintf("turbulence-%s", suffix),
		AZs:              []string{"z1"},
		DirectorHost:     boshConfig.Host,
		DirectorUsername: boshConfig.Username,
		DirectorPassword: boshConfig.Password,
		DirectorCACert:   boshConfig.DirectorCACert,
	})
	if err != nil {
		return "", err
	}

	yaml, err := client.ResolveManifestVersionsV2([]byte(manifest))
	if err != nil {
		return "", err
	}

	_, err = client.Deploy(yaml)
	if err != nil {
		return "", err
	}

	return string(yaml), nil
}
