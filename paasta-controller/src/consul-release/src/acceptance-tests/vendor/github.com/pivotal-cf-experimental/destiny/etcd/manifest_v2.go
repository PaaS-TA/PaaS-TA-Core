package etcd

import "github.com/pivotal-cf-experimental/destiny/ops"

type ConfigV2 struct {
	Name      string
	AZs       []string
	EnableSSL bool
}

func NewManifestV2(config ConfigV2) (string, error) {
	if config.EnableSSL {
		return ops.ApplyOps(manifestV2TLS, []ops.Op{
			{"replace", "/name", config.Name},
			{"replace", "/instance_groups/name=consul/azs", config.AZs},
			{"replace", "/instance_groups/name=etcd/azs", config.AZs},
			{"replace", "/instance_groups/name=testconsumer/azs", config.AZs},
		})
	}

	return ops.ApplyOps(manifestV2NonTLS, []ops.Op{
		{"replace", "/name", config.Name},
		{"replace", "/instance_groups/name=etcd/azs", config.AZs},
		{"replace", "/instance_groups/name=testconsumer/azs", config.AZs},
	})
}
