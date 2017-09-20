package main_test

import (
	"encoding/json"
	"fmt"

	"code.cloudfoundry.org/consuladapter/consulrunner"
	"code.cloudfoundry.org/lager/lagertest"
	"code.cloudfoundry.org/tps/cmd/tpsrunner"
	. "github.com/onsi/ginkgo"
	"github.com/onsi/ginkgo/config"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"
	"github.com/onsi/gomega/ghttp"
	"github.com/tedsuo/ifrit"
	"github.com/tedsuo/ifrit/ginkgomon"

	"testing"
)

var (
	consulRunner *consulrunner.ClusterRunner

	listenerPort int
	listenerAddr string
	listener     ifrit.Process
	runner       *ginkgomon.Runner

	listenerPath string

	fakeCC                *ghttp.Server
	fakeBBS               *ghttp.Server
	fakeTrafficController *ghttp.Server

	logger *lagertest.TestLogger
)

func TestTPS(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "TPS-Listener Suite")
}

var _ = SynchronizedBeforeSuite(func() []byte {
	tps, err := gexec.Build("code.cloudfoundry.org/tps/cmd/tps-listener", "-race")
	Expect(err).NotTo(HaveOccurred())

	payload, err := json.Marshal(map[string]string{
		"listener": tps,
	})
	Expect(err).NotTo(HaveOccurred())

	return payload
}, func(payload []byte) {
	binaries := map[string]string{}

	err := json.Unmarshal(payload, &binaries)
	Expect(err).NotTo(HaveOccurred())

	listenerPort = 1518 + GinkgoParallelNode()

	listenerPath = string(binaries["listener"])

	consulRunner = consulrunner.NewClusterRunner(
		9001+config.GinkgoConfig.ParallelNode*consulrunner.PortOffsetLength,
		1,
		"http",
	)

	logger = lagertest.NewTestLogger("test")

	consulRunner.Start()
	consulRunner.WaitUntilReady()
})

var _ = BeforeEach(func() {
	consulRunner.Reset()

	fakeBBS = ghttp.NewServer()
	fakeCC = ghttp.NewServer()
	fakeTrafficController = ghttp.NewTLSServer()

	listenerAddr = fmt.Sprintf("127.0.0.1:%d", uint16(listenerPort))

	runner = tpsrunner.NewListener(
		string(listenerPath),
		listenerAddr,
		fakeBBS.URL(),
		fakeTrafficController.URL(),
		consulRunner.URL(),
	)
})

var _ = AfterEach(func() {
	fakeBBS.Close()
	fakeCC.Close()
	fakeTrafficController.Close()
})

var _ = SynchronizedAfterSuite(func() {
	consulRunner.Stop()
}, func() {
	gexec.CleanupBuildArtifacts()
})
