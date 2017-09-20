package consul_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/pivotal-cf-experimental/destiny/consul"
	"github.com/pivotal-cf-experimental/destiny/core"

	"testing"
)

func TestConsul(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "consul")
}

func findJob(manifest consul.Manifest, name string) *core.Job {
	for _, job := range manifest.Jobs {
		if job.Name == name {
			return &job
		}
	}
	return &core.Job{}
}
