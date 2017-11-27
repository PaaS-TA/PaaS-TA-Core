package cell_test

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"code.cloudfoundry.org/archiver/extractor/test_helper"
	"code.cloudfoundry.org/bbs/models"
	"code.cloudfoundry.org/durationjson"
	"code.cloudfoundry.org/inigo/fixtures"
	"code.cloudfoundry.org/inigo/helpers"
	repconfig "code.cloudfoundry.org/rep/cmd/rep/config"
	"github.com/tedsuo/ifrit"
	"github.com/tedsuo/ifrit/ginkgomon"
	"github.com/tedsuo/ifrit/grouper"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Evacuation", func() {
	var (
		runtime ifrit.Process

		cellAID            string
		cellARepAddr       string
		cellARepSecureAddr string

		cellBID            string
		cellBRepAddr       string
		cellBRepSecureAddr string

		cellARepRunner *ginkgomon.Runner
		cellBRepRunner *ginkgomon.Runner

		cellA ifrit.Process
		cellB ifrit.Process

		processGuid string
	)

	BeforeEach(func() {
		processGuid = helpers.GenerateGuid()

		fileServer, fileServerStaticDir := componentMaker.FileServer()

		By("restarting the bbs with smaller convergeRepeatInterval")
		ginkgomon.Interrupt(bbsProcess)
		bbsProcess = ginkgomon.Invoke(componentMaker.BBS(
			overrideConvergenceRepeatInterval,
		))

		runtime = ginkgomon.Invoke(grouper.NewParallel(os.Kill, grouper.Members{
			{"router", componentMaker.Router()},
			{"file-server", fileServer},
			{"auctioneer", componentMaker.Auctioneer()},
			{"route-emitter", componentMaker.RouteEmitter()},
		}))

		cellAID = "cell-a"
		cellBID = "cell-b"

		cellARepAddr = fmt.Sprintf("0.0.0.0:%d", 14200+GinkgoParallelNode())
		cellARepSecureAddr = fmt.Sprintf("0.0.0.0:%d", 14300+GinkgoParallelNode())
		cellBRepAddr = fmt.Sprintf("0.0.0.0:%d", 14400+GinkgoParallelNode())
		cellBRepSecureAddr = fmt.Sprintf("0.0.0.0:%d", 14500+GinkgoParallelNode())

		cellARepRunner = componentMaker.RepN(0,
			func(config *repconfig.RepConfig) {
				config.CellID = cellAID
				config.ListenAddr = cellARepAddr
				config.ListenAddrSecurable = cellARepSecureAddr
				config.EvacuationTimeout = durationjson.Duration(30 * time.Second)
			},
		)

		cellBRepRunner = componentMaker.RepN(1,
			func(config *repconfig.RepConfig) {
				config.CellID = cellBID
				config.ListenAddr = cellBRepAddr
				config.ListenAddrSecurable = cellBRepSecureAddr
				config.EvacuationTimeout = durationjson.Duration(30 * time.Second)
			})

		cellA = ginkgomon.Invoke(cellARepRunner)
		cellB = ginkgomon.Invoke(cellBRepRunner)

		test_helper.CreateZipArchive(
			filepath.Join(fileServerStaticDir, "lrp.zip"),
			fixtures.GoServerApp(),
		)
	})

	AfterEach(func() {
		helpers.StopProcesses(runtime, cellA, cellB)
	})

	It("handles evacuation", func() {
		By("desiring an LRP")
		lrp := helpers.DefaultLRPCreateRequest(processGuid, "log-guid", 1)
		lrp.Setup = models.WrapAction(&models.DownloadAction{
			From: fmt.Sprintf("http://%s/v1/static/%s", componentMaker.Addresses.FileServer, "lrp.zip"),
			To:   "/tmp/diego",
			User: "vcap",
		})
		lrp.Action = models.WrapAction(&models.RunAction{
			User: "vcap",
			Path: "/tmp/diego/go-server",
			Env:  []*models.EnvironmentVariable{{"PORT", "8080"}},
		})

		lrp.Monitor = models.WrapAction(
			models.EmitProgressFor(
				models.Timeout(
					&models.RunAction{
						User: "vcap",
						Path: "sh",
						Args: []string{
							"-c",
							`curl -I http://0.0.0.0:8080 --connect-timeout 2`,
						},
					},
					5*time.Second,
				),
				"start",
				"success",
				"fail",
			),
		)

		err := bbsClient.DesireLRP(logger, lrp)
		Expect(err).NotTo(HaveOccurred())

		By("running an actual LRP instance")
		Eventually(helpers.LRPStatePoller(logger, bbsClient, processGuid, nil)).Should(Equal(models.ActualLRPStateRunning))
		Eventually(helpers.ResponseCodeFromHostPoller(componentMaker.Addresses.Router, helpers.DefaultHost)).Should(Equal(http.StatusOK))

		actualLRPGroup, err := bbsClient.ActualLRPGroupByProcessGuidAndIndex(logger, processGuid, 0)
		Expect(err).NotTo(HaveOccurred())

		actualLRP, isEvacuating := actualLRPGroup.Resolve()
		Expect(isEvacuating).To(BeFalse())

		var evacuatingRepAddr string
		var evacuatingRepRunner *ginkgomon.Runner

		switch actualLRP.CellId {
		case cellAID:
			evacuatingRepAddr = cellARepAddr
			evacuatingRepRunner = cellARepRunner
		case cellBID:
			evacuatingRepAddr = cellBRepAddr
			evacuatingRepRunner = cellBRepRunner
		default:
			panic("what? who?")
		}

		By("posting the evacuation endpoint")
		resp, err := http.Post(fmt.Sprintf("http://%s/evacuate", evacuatingRepAddr), "text/html", nil)
		Expect(err).NotTo(HaveOccurred())
		resp.Body.Close()
		Expect(resp.StatusCode).To(Equal(http.StatusAccepted))

		By("staying routable so long as its rep is alive")
		Eventually(func() int {
			Expect(helpers.ResponseCodeFromHostPoller(componentMaker.Addresses.Router, helpers.DefaultHost)()).To(Equal(http.StatusOK))
			return evacuatingRepRunner.ExitCode()
		}).Should(Equal(0))

		By("running immediately after the rep exits and is routable")
		Expect(helpers.LRPStatePoller(logger, bbsClient, processGuid, nil)()).To(Equal(models.ActualLRPStateRunning))
		Consistently(helpers.ResponseCodeFromHostPoller(componentMaker.Addresses.Router, helpers.DefaultHost)).Should(Equal(http.StatusOK))
	})
})
