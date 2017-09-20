package main_test

import (
	"encoding/json"
	"fmt"
	"net"
	"net/url"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"testing"
	"time"

	"code.cloudfoundry.org/lager/lagertest"
	"code.cloudfoundry.org/route-emitter/diegonats"
	"code.cloudfoundry.org/route-emitter/diegonats/gnatsdrunner"
	"github.com/cloudfoundry/sonde-go/events"
	"github.com/cloudfoundry/storeadapter/storerunner/etcdstorerunner"
	"github.com/gogo/protobuf/proto"
	. "github.com/onsi/ginkgo"
	"github.com/onsi/ginkgo/config"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"
	"github.com/tedsuo/ifrit"
	"github.com/tedsuo/ifrit/ginkgomon"

	"code.cloudfoundry.org/bbs"
	bbstestrunner "code.cloudfoundry.org/bbs/cmd/bbs/testrunner"
	"code.cloudfoundry.org/bbs/test_helpers"
	"code.cloudfoundry.org/bbs/test_helpers/sqlrunner"
	"code.cloudfoundry.org/consuladapter/consulrunner"
)

const heartbeatInterval = 1 * time.Second

var (
	emitterPath   string
	etcdPort      int
	natsPort      int
	dropsondePort int

	bbsPath    string
	bbsURL     *url.URL
	bbsArgs    bbstestrunner.Args
	bbsRunner  *ginkgomon.Runner
	bbsProcess ifrit.Process

	etcdRunner           *etcdstorerunner.ETCDClusterRunner
	consulRunner         *consulrunner.ClusterRunner
	gnatsdRunner         ifrit.Process
	natsClient           diegonats.NATSClient
	bbsClient            bbs.InternalClient
	logger               *lagertest.TestLogger
	syncInterval         time.Duration
	consulClusterAddress string
	testMetricsListener  net.PacketConn
	testMetricsChan      chan *events.Envelope

	sqlProcess ifrit.Process
	sqlRunner  sqlrunner.SQLRunner
)

func TestRouteEmitter(t *testing.T) {
	RegisterFailHandler(Fail)
	SetDefaultEventuallyTimeout(2 * time.Second)
	RunSpecs(t, "Route Emitter Suite")
}

func createEmitterRunner(sessionName string) *ginkgomon.Runner {
	return ginkgomon.New(ginkgomon.Config{
		Command: exec.Command(
			string(emitterPath),
			"-sessionName", sessionName,
			"-dropsondePort", strconv.Itoa(dropsondePort),
			"-natsAddresses", fmt.Sprintf("127.0.0.1:%d", natsPort),
			"-bbsAddress", bbsURL.String(),
			"-communicationTimeout", "100ms",
			"-syncInterval", syncInterval.String(),
			"-lockRetryInterval", "1s",
			"-lockTTL", "5s",
			"-consulCluster", consulClusterAddress,
		),

		StartCheck: "route-emitter.watcher.sync.complete",

		AnsiColorCode: "97m",
	})
}

var _ = SynchronizedBeforeSuite(func() []byte {
	emitter, err := gexec.Build("code.cloudfoundry.org/route-emitter/cmd/route-emitter", "-race")
	Expect(err).NotTo(HaveOccurred())

	bbs, err := gexec.Build("code.cloudfoundry.org/bbs/cmd/bbs", "-race")
	Expect(err).NotTo(HaveOccurred())

	payload, err := json.Marshal(map[string]string{
		"emitter": emitter,
		"bbs":     bbs,
	})

	Expect(err).NotTo(HaveOccurred())

	return payload
}, func(payload []byte) {
	binaries := map[string]string{}

	err := json.Unmarshal(payload, &binaries)
	Expect(err).NotTo(HaveOccurred())

	etcdPort = 5001 + GinkgoParallelNode()
	natsPort = 4001 + GinkgoParallelNode()

	etcdRunner = etcdstorerunner.NewETCDClusterRunner(etcdPort, 1, nil)

	emitterPath = string(binaries["emitter"])

	if test_helpers.UseSQL() {
		dbName := fmt.Sprintf("diego_%d", GinkgoParallelNode())
		sqlRunner = test_helpers.NewSQLRunner(dbName)
		sqlProcess = ginkgomon.Invoke(sqlRunner)
	}

	consulRunner = consulrunner.NewClusterRunner(
		9001+config.GinkgoConfig.ParallelNode*consulrunner.PortOffsetLength,
		1,
		"http",
	)

	logger = lagertest.NewTestLogger("test")

	syncInterval = 200 * time.Millisecond

	bbsPath = string(binaries["bbs"])
	bbsPort := 13000 + GinkgoParallelNode()*2
	healthPort := bbsPort + 1
	bbsAddress := fmt.Sprintf("127.0.0.1:%d", bbsPort)
	healthAddress := fmt.Sprintf("127.0.0.1:%d", healthPort)

	bbsURL = &url.URL{
		Scheme: "http",
		Host:   bbsAddress,
	}

	bbsClient = bbs.NewClient(bbsURL.String())

	bbsArgs = bbstestrunner.Args{
		Address:           bbsAddress,
		AdvertiseURL:      bbsURL.String(),
		AuctioneerAddress: "some-address",
		EtcdCluster:       strings.Join(etcdRunner.NodeURLS(), ","),
		ConsulCluster:     consulRunner.ConsulCluster(),
		HealthAddress:     healthAddress,

		EncryptionKeys: []string{"label:key"},
		ActiveKeyLabel: "label",
	}

	if test_helpers.UseSQL() {
		bbsArgs.DatabaseDriver = sqlRunner.DriverName()
		bbsArgs.DatabaseConnectionString = sqlRunner.ConnectionString()
	}
})

var _ = BeforeEach(func() {
	etcdRunner.Start()
	consulRunner.Start()
	consulRunner.WaitUntilReady()
	consulClusterAddress = consulRunner.ConsulCluster()

	bbsRunner = bbstestrunner.New(bbsPath, bbsArgs)
	bbsProcess = ginkgomon.Invoke(bbsRunner)

	gnatsdRunner, natsClient = gnatsdrunner.StartGnatsd(natsPort)

	testMetricsListener, _ = net.ListenPacket("udp", "127.0.0.1:0")
	testMetricsChan = make(chan *events.Envelope, 1)
	go func() {
		defer GinkgoRecover()
		for {
			buffer := make([]byte, 1024)
			n, _, err := testMetricsListener.ReadFrom(buffer)
			if err != nil {
				close(testMetricsChan)
				return
			}

			var envelope events.Envelope
			err = proto.Unmarshal(buffer[:n], &envelope)
			Expect(err).NotTo(HaveOccurred())
			testMetricsChan <- &envelope
		}
	}()

	var err error
	dropsondePort, err = strconv.Atoi(strings.TrimPrefix(testMetricsListener.LocalAddr().String(), "127.0.0.1:"))
	Expect(err).NotTo(HaveOccurred())
})

var _ = AfterEach(func() {
	ginkgomon.Kill(bbsProcess)
	etcdRunner.Stop()
	consulRunner.Stop()
	gnatsdRunner.Signal(os.Interrupt)
	Eventually(gnatsdRunner.Wait(), 5).Should(Receive())

	testMetricsListener.Close()
	Eventually(testMetricsChan).Should(BeClosed())

	if test_helpers.UseSQL() {
		sqlRunner.Reset()
	}
})

var _ = SynchronizedAfterSuite(func() {
	ginkgomon.Kill(sqlProcess)

	if etcdRunner != nil {
		etcdRunner.Stop()
	}
}, func() {
	gexec.CleanupBuildArtifacts()
})
