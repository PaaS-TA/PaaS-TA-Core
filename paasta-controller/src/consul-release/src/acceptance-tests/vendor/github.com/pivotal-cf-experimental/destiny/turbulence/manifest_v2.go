package turbulence

import "github.com/pivotal-cf-experimental/destiny/ops"

type ConfigV2 struct {
	Name             string
	AZs              []string
	DirectorHost     string
	DirectorUsername string
	DirectorPassword string
	DirectorCACert   string
}

func NewManifestV2(config ConfigV2) (string, error) {
	return ops.ApplyOps(manifestV2, []ops.Op{
		{"replace", "/name", config.Name},
		{"replace", "/instance_groups/name=api/azs", config.AZs},
		{"replace", "/instance_groups/name=api/properties/director/host", config.DirectorHost},
		{"replace", "/instance_groups/name=api/properties/director/client", config.DirectorUsername},
		{"replace", "/instance_groups/name=api/properties/director/client_secret", config.DirectorPassword},
		{"replace", "/instance_groups/name=api/properties/director/ca_cert", config.DirectorCACert},
	})
}
