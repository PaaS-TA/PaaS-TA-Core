package main_test

import (
	"fmt"
	"io/ioutil"
	"log"
	"time"

	"google.golang.org/grpc/grpclog"

	"code.cloudfoundry.org/bbs/test_helpers"
	"code.cloudfoundry.org/bbs/test_helpers/sqlrunner"
	"code.cloudfoundry.org/consuladapter/consulrunner"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"
	"github.com/tedsuo/ifrit"
	"github.com/tedsuo/ifrit/ginkgomon"

	"testing"
)

var (
	locketBinPath string

	sqlProcess   ifrit.Process
	sqlRunner    sqlrunner.SQLRunner
	consulRunner *consulrunner.ClusterRunner

	TruncateTableList = []string{"locks"}
)

func TestLocket(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Locket Suite")
}

var _ = SynchronizedBeforeSuite(
	func() []byte {
		locketBinPathData, err := gexec.Build("code.cloudfoundry.org/locket/cmd/locket", "-race")
		Expect(err).NotTo(HaveOccurred())
		return []byte(locketBinPathData)
	},
	func(locketBinPathData []byte) {
		grpclog.SetLogger(log.New(ioutil.Discard, "", 0))

		locketBinPath = string(locketBinPathData)
		SetDefaultEventuallyTimeout(15 * time.Second)

		dbName := fmt.Sprintf("diego_%d", GinkgoParallelNode())
		sqlRunner = test_helpers.NewSQLRunner(dbName)
		sqlProcess = ginkgomon.Invoke(sqlRunner)

		consulRunner = consulrunner.NewClusterRunner(
			consulrunner.ClusterRunnerConfig{
				StartingPort: 9001 + GinkgoParallelNode()*consulrunner.PortOffsetLength,
				NumNodes:     1,
				Scheme:       "http",
			},
		)
		consulRunner.Start()
	},
)

var _ = BeforeEach(func() {
	consulRunner.WaitUntilReady()
	consulRunner.Reset()
})

var _ = SynchronizedAfterSuite(func() {
	if consulRunner != nil {
		consulRunner.Stop()
	}
	ginkgomon.Kill(sqlProcess)
}, func() {
	gexec.CleanupBuildArtifacts()
})
