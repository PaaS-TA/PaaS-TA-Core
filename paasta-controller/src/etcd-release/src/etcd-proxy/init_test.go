package main_test

import (
	"testing"

	"github.com/onsi/gomega/gexec"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func TestEtcdProxy(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "etcd-proxy")
}

var pathToEtcdProxy string

var _ = BeforeSuite(func() {
	var err error
	pathToEtcdProxy, err = gexec.Build("github.com/cloudfoundry-incubator/etcd-release/src/etcd-proxy")
	Expect(err).NotTo(HaveOccurred())
})

var _ = AfterSuite(func() {
	gexec.CleanupBuildArtifacts()
})
