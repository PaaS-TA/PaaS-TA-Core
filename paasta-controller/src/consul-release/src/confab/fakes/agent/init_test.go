package main_test

import (
	"testing"

	"github.com/onsi/gomega/gexec"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var pathToFakeAgent string

func TestFakeAgent(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "agent")
}

var _ = BeforeSuite(func() {
	var err error
	pathToFakeAgent, err = gexec.Build("github.com/cloudfoundry-incubator/consul-release/src/confab/fakes/agent")
	Expect(err).NotTo(HaveOccurred())
})

var _ = AfterSuite(func() {
	gexec.CleanupBuildArtifacts()
})
