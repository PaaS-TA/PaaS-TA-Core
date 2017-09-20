package etcd_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/pivotal-cf-experimental/destiny/core"
	"github.com/pivotal-cf-experimental/destiny/etcd"

	"testing"
)

func TestEtcd(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "etcd")
}

func findJob(manifest etcd.Manifest, name string) *core.Job {
	for _, job := range manifest.Jobs {
		if job.Name == name {
			return &job
		}
	}
	return &core.Job{}
}
