package maintain_test

import (
	"code.cloudfoundry.org/consuladapter"
	"code.cloudfoundry.org/consuladapter/consulrunner"
	. "github.com/onsi/ginkgo"
	"github.com/onsi/ginkgo/config"
	. "github.com/onsi/gomega"

	"testing"
)

var (
	consulRunner *consulrunner.ClusterRunner
	consulClient consuladapter.Client
)

var _ = BeforeSuite(func() {
	consulRunner = consulrunner.NewClusterRunner(
		consulrunner.ClusterRunnerConfig{
			StartingPort: 9001 + config.GinkgoConfig.ParallelNode*consulrunner.PortOffsetLength,
			NumNodes:     1,
			Scheme:       "http",
		},
	)

	consulRunner.Start()
	consulRunner.WaitUntilReady()
})

var _ = AfterSuite(func() {
	consulRunner.Stop()
})

var _ = BeforeEach(func() {
	_ = consulRunner.Reset()
	consulClient = consulRunner.NewClient()
})

func TestMaintain(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Maintain Suite")
}
