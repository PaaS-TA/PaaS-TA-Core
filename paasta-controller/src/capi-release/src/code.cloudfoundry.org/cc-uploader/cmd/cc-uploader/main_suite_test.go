package main_test

import (
	"fmt"
	"os"

	"code.cloudfoundry.org/cc-uploader/ccclient/fake_cc"
	"code.cloudfoundry.org/consuladapter/consulrunner"
	. "github.com/onsi/ginkgo"
	"github.com/onsi/ginkgo/config"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"
	"github.com/tedsuo/ifrit"

	"testing"
)

var ccUploaderBinary string
var fakeCC *fake_cc.FakeCC
var fakeCCProcess ifrit.Process
var consulRunner *consulrunner.ClusterRunner

func TestCCUploader(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "CC Uploader Suite")
}

var _ = SynchronizedBeforeSuite(func() []byte {
	ccUploaderPath, err := gexec.Build("code.cloudfoundry.org/cc-uploader/cmd/cc-uploader")
	Expect(err).NotTo(HaveOccurred())
	return []byte(ccUploaderPath)
}, func(ccUploaderPath []byte) {
	fakeCCAddress := fmt.Sprintf("127.0.0.1:%d", 6767+GinkgoParallelNode())
	fakeCC = fake_cc.New(fakeCCAddress)

	ccUploaderBinary = string(ccUploaderPath)

	consulRunner = consulrunner.NewClusterRunner(
		9001+config.GinkgoConfig.ParallelNode*consulrunner.PortOffsetLength,
		1,
		"http",
	)

	consulRunner.Start()
	consulRunner.WaitUntilReady()
})

var _ = SynchronizedAfterSuite(func() {
	consulRunner.Stop()
}, func() {
	gexec.CleanupBuildArtifacts()
})

var _ = BeforeEach(func() {
	consulRunner.Reset()
	fakeCCProcess = ifrit.Envoke(fakeCC)
})

var _ = AfterEach(func() {
	fakeCCProcess.Signal(os.Kill)
	Eventually(fakeCCProcess.Wait()).Should(Receive(BeNil()))
})
