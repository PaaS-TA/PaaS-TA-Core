package main_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"

	"testing"
)

var metricsServerPath string

func TestEtcdMetricsServer(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Etcd Metrics Server Suite")
}

var _ = BeforeSuite(func() {
	var err error
	metricsServerPath, err = gexec.Build("github.com/cloudfoundry-incubator/etcd-metrics-server/cmd/etcd-metrics-server")
	Ω(err).ShouldNot(HaveOccurred())
})

var _ = AfterSuite(func() {
	gexec.CleanupBuildArtifacts()
})
