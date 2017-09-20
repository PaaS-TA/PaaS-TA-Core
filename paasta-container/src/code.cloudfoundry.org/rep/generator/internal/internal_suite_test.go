package internal_test

import (
	"fmt"

	"code.cloudfoundry.org/bbs"
	bbsrunner "code.cloudfoundry.org/bbs/cmd/bbs/testrunner"
	"code.cloudfoundry.org/consuladapter/consulrunner"
	"github.com/cloudfoundry/storeadapter/storerunner/etcdstorerunner"
	. "github.com/onsi/ginkgo"
	"github.com/onsi/ginkgo/config"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"
	"github.com/tedsuo/ifrit"
	"github.com/tedsuo/ifrit/ginkgomon"

	"testing"
)

var etcdRunner *etcdstorerunner.ETCDClusterRunner
var consulRunner *consulrunner.ClusterRunner
var bbsArgs bbsrunner.Args
var bbsBinPath string
var bbsRunner *ginkgomon.Runner
var bbsProcess ifrit.Process
var bbsClient bbs.InternalClient

func TestInternal(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Internal Suite")
}

var _ = SynchronizedBeforeSuite(func() []byte {
	bbsBinPath, err := gexec.Build("code.cloudfoundry.org/bbs/cmd/bbs")
	Expect(err).NotTo(HaveOccurred())
	return []byte(bbsBinPath)
}, func(payload []byte) {
	etcdRunner = etcdstorerunner.NewETCDClusterRunner(5001+config.GinkgoConfig.ParallelNode, 1, nil)

	consulRunner = consulrunner.NewClusterRunner(
		9001+config.GinkgoConfig.ParallelNode*consulrunner.PortOffsetLength,
		1,
		"http",
	)

	bbsPort := 13000 + GinkgoParallelNode()*2
	healthPort := bbsPort + 1
	bbsAddress := fmt.Sprintf("127.0.0.1:%d", bbsPort)
	healthAddress := fmt.Sprintf("127.0.0.1:%d", healthPort)

	etcdRunner.Start()
	bbsBinPath = string(payload)
	bbsArgs = bbsrunner.Args{
		Address:           bbsAddress,
		AdvertiseURL:      "http://" + bbsAddress,
		AuctioneerAddress: "some-address",
		EtcdCluster:       etcdRunner.NodeURLS()[0],
		ConsulCluster:     consulRunner.ConsulCluster(),
		HealthAddress:     healthAddress,

		EncryptionKeys: []string{"label:key"},
		ActiveKeyLabel: "label",
	}
	bbsClient = bbs.NewClient("http://" + bbsAddress)

	consulRunner.Start()
	consulRunner.WaitUntilReady()
})

var _ = SynchronizedAfterSuite(func() {
	consulRunner.Stop()
	etcdRunner.KillWithFire()
}, func() {
	gexec.CleanupBuildArtifacts()
})

var _ = BeforeEach(func() {
	consulRunner.Reset()
	bbsRunner = bbsrunner.New(bbsBinPath, bbsArgs)
	bbsProcess = ginkgomon.Invoke(bbsRunner)
})

var _ = AfterEach(func() {
	ginkgomon.Kill(bbsProcess)
})
