package main_test

import (
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"testing"
	"time"

	"google.golang.org/grpc/grpclog"

	"code.cloudfoundry.org/bbs"
	bbsconfig "code.cloudfoundry.org/bbs/cmd/bbs/config"
	bbstestrunner "code.cloudfoundry.org/bbs/cmd/bbs/testrunner"
	"code.cloudfoundry.org/bbs/encryption"
	"code.cloudfoundry.org/bbs/test_helpers"
	"code.cloudfoundry.org/bbs/test_helpers/sqlrunner"
	"code.cloudfoundry.org/consuladapter/consulrunner"
	. "github.com/onsi/ginkgo"
	"github.com/onsi/ginkgo/config"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"
	"github.com/onsi/gomega/ghttp"
	"github.com/tedsuo/ifrit"
	"github.com/tedsuo/ifrit/ginkgomon"
)

var (
	cellID              string
	representativePath  string
	natsPort            int
	serverPort          int
	serverPortSecurable int
	consulRunner        *consulrunner.ClusterRunner

	bbsConfig        bbsconfig.BBSConfig
	bbsBinPath       string
	bbsURL           *url.URL
	bbsRunner        *ginkgomon.Runner
	bbsProcess       ifrit.Process
	bbsClient        bbs.InternalClient
	auctioneerServer *ghttp.Server
	locketBinPath    string

	sqlProcess ifrit.Process
	sqlRunner  sqlrunner.SQLRunner
)

func TestRep(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Rep Integration Suite")
}

var _ = SynchronizedBeforeSuite(func() []byte {
	bbsConfig, err := gexec.Build("code.cloudfoundry.org/bbs/cmd/bbs", "-race")
	Expect(err).NotTo(HaveOccurred())

	locketPath, err := gexec.Build("code.cloudfoundry.org/locket/cmd/locket", "-race")
	Expect(err).NotTo(HaveOccurred())

	representative, err := gexec.Build("code.cloudfoundry.org/rep/cmd/rep", "-race")
	Expect(err).NotTo(HaveOccurred())

	return []byte(strings.Join([]string{representative, locketPath, bbsConfig}, ","))
}, func(pathsByte []byte) {
	grpclog.SetLogger(log.New(ioutil.Discard, "", 0))

	// tests here are fairly Eventually driven which tends to flake out under
	// load (for insignificant reasons); bump the default a bit higher than the
	// default (1 second)
	SetDefaultEventuallyTimeout(5 * time.Second)

	path := string(pathsByte)
	representativePath = strings.Split(path, ",")[0]
	locketBinPath = strings.Split(path, ",")[1]
	bbsBinPath = strings.Split(path, ",")[2]

	cellID = "the_rep_id-" + strconv.Itoa(GinkgoParallelNode())

	serverPort = 1800 + GinkgoParallelNode()
	serverPortSecurable = 1901 + GinkgoParallelNode()

	dbName := fmt.Sprintf("diego_%d", GinkgoParallelNode())

	sqlRunner = test_helpers.NewSQLRunner(dbName)
	sqlProcess = ginkgomon.Invoke(sqlRunner)

	consulRunner = consulrunner.NewClusterRunner(
		consulrunner.ClusterRunnerConfig{
			StartingPort: 9001 + config.GinkgoConfig.ParallelNode*consulrunner.PortOffsetLength,
			NumNodes:     1,
			Scheme:       "http",
		},
	)

	consulRunner.Start()

	bbsPort := 13000 + GinkgoParallelNode()*2
	healthPort := bbsPort + 1
	bbsAddress := fmt.Sprintf("127.0.0.1:%d", bbsPort)
	healthAddress := fmt.Sprintf("127.0.0.1:%d", healthPort)

	bbsURL = &url.URL{
		Scheme: "http",
		Host:   bbsAddress,
	}

	bbsClient = bbs.NewClient(bbsURL.String())

	auctioneerServer = ghttp.NewServer()
	auctioneerServer.UnhandledRequestStatusCode = http.StatusAccepted
	auctioneerServer.AllowUnhandledRequests = true

	bbsConfig = bbsconfig.BBSConfig{
		ListenAddress:            bbsAddress,
		AdvertiseURL:             bbsURL.String(),
		AuctioneerAddress:        auctioneerServer.URL(),
		DatabaseDriver:           sqlRunner.DriverName(),
		DatabaseConnectionString: sqlRunner.ConnectionString(),
		ConsulCluster:            consulRunner.ConsulCluster(),
		HealthAddress:            healthAddress,

		EncryptionConfig: encryption.EncryptionConfig{
			EncryptionKeys: map[string]string{"label": "key"},
			ActiveKeyLabel: "label",
		},
	}
})

var _ = BeforeEach(func() {
	consulRunner.WaitUntilReady()
	consulRunner.Reset()

	bbsRunner = bbstestrunner.New(bbsBinPath, bbsConfig)
	bbsProcess = ginkgomon.Invoke(bbsRunner)
})

var _ = AfterEach(func() {
	sqlRunner.Reset()

	ginkgomon.Kill(bbsProcess)
})

var _ = SynchronizedAfterSuite(func() {
	ginkgomon.Kill(sqlProcess)
	if consulRunner != nil {
		consulRunner.Stop()
	}
	if runner != nil {
		runner.KillWithFire()
	}
	if auctioneerServer != nil {
		auctioneerServer.Close()
	}
}, func() {
	gexec.CleanupBuildArtifacts()
})
