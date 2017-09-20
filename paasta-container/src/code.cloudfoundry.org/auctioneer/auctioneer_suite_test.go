package auctioneer_test

import (
	"code.cloudfoundry.org/consuladapter/consulrunner"

	. "github.com/onsi/ginkgo"
	"github.com/onsi/ginkgo/config"
	. "github.com/onsi/gomega"

	"testing"
)

var consulRunner *consulrunner.ClusterRunner

func TestAuctioneer(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Auctioneer Suite")
}

var _ = BeforeSuite(func() {
	consulRunner = consulrunner.NewClusterRunner(
		9001+config.GinkgoConfig.ParallelNode*consulrunner.PortOffsetLength,
		1,
		"http",
	)

	consulRunner.Start()
	consulRunner.WaitUntilReady()
})

var _ = AfterSuite(func() {
	consulRunner.Stop()
})

var _ = BeforeEach(func() {
	consulRunner.Reset()
})
