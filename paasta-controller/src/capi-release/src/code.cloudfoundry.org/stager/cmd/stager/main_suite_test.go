package main_test

import (
	"testing"

	. "github.com/onsi/ginkgo"
	"github.com/onsi/ginkgo/config"
	. "github.com/onsi/gomega"

	"code.cloudfoundry.org/consuladapter/consulrunner"
	"code.cloudfoundry.org/stager/cmd/stager/testrunner"
	"github.com/onsi/gomega/gexec"
)

func TestStager(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Stager Suite")
}

var stagerPath string
var runner *testrunner.StagerRunner
var consulRunner *consulrunner.ClusterRunner

var _ = SynchronizedBeforeSuite(func() []byte {
	stager, err := gexec.Build("code.cloudfoundry.org/stager/cmd/stager", "-race")
	Expect(err).NotTo(HaveOccurred())
	return []byte(stager)
}, func(stager []byte) {
	stagerPath = string(stager)

	consulRunner = consulrunner.NewClusterRunner(
		9001+config.GinkgoConfig.ParallelNode*consulrunner.PortOffsetLength,
		1,
		"http",
	)

	consulRunner.Start()
	consulRunner.WaitUntilReady()
})

var _ = SynchronizedAfterSuite(func() {
	if runner != nil {
		runner.Stop()
	}

	consulRunner.Stop()
}, func() {
	gexec.CleanupBuildArtifacts()
})

var _ = BeforeEach(func() {
	consulRunner.Reset()
})
