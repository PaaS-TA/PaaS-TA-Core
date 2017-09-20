package cell_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"code.cloudfoundry.org/archiver/extractor/test_helper"
	"code.cloudfoundry.org/bbs/models"
	"code.cloudfoundry.org/inigo/fixtures"
	"code.cloudfoundry.org/inigo/helpers"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/tedsuo/ifrit"
	"github.com/tedsuo/ifrit/ginkgomon"
	"github.com/tedsuo/ifrit/grouper"
)

var _ = Describe("Network Environment Variables", func() {
	var (
		guid                string
		repFlags            []string
		fileServerStaticDir string
		fileServer          ifrit.Runner
		runtime             ifrit.Process
	)

	BeforeEach(func() {
		fileServer, fileServerStaticDir = componentMaker.FileServer()
		repFlags = []string{}
		guid = helpers.GenerateGuid()
	})

	JustBeforeEach(func() {
		runtime = ginkgomon.Invoke(grouper.NewParallel(os.Kill, grouper.Members{
			{"rep", componentMaker.Rep(repFlags...)},
			{"auctioneer", componentMaker.Auctioneer()},
			{"router", componentMaker.Router()},
			{"route-emitter", componentMaker.RouteEmitter()},
			{"file-server", fileServer},
		}))
	})

	AfterEach(func() {
		helpers.StopProcesses(runtime)
	})

	Describe("tasks", func() {
		var task *models.Task

		JustBeforeEach(func() {
			taskToDesire := helpers.TaskCreateRequest(
				guid,
				&models.RunAction{
					User: "vcap",
					Path: "sh",
					Args: []string{"-c", "/usr/bin/env | grep 'CF_INSTANCE' > /home/vcap/env"},
				},
			)
			taskToDesire.ResultFile = "/home/vcap/env"

			err := bbsClient.DesireTask(logger, taskToDesire.TaskGuid, taskToDesire.Domain, taskToDesire.TaskDefinition)
			Expect(err).NotTo(HaveOccurred())

			Eventually(func() interface{} {
				var err error
				task, err = bbsClient.TaskByGuid(logger, guid)
				Expect(err).ShouldNot(HaveOccurred())

				return task.State
			}).Should(Equal(models.Task_Completed))
		})

		Context("when -exportNetworkEnvVars=false is set", func() {
			BeforeEach(func() {
				repFlags = []string{"-exportNetworkEnvVars=false"}
			})

			It("does not set the networking environment variables", func() {
				Expect(task.Result).To(Equal(""))
			})
		})

		Context("when -exportNetworkEnvVars=true is set", func() {
			BeforeEach(func() {
				repFlags = []string{"-exportNetworkEnvVars=true"}
			})

			It("sets the networking environment variables", func() {
				Expect(task.Result).To(ContainSubstring("CF_INSTANCE_ADDR=\n"))
				Expect(task.Result).To(ContainSubstring("CF_INSTANCE_PORT=\n"))
				Expect(task.Result).To(ContainSubstring("CF_INSTANCE_PORTS=[]\n"))
				Expect(task.Result).To(ContainSubstring(fmt.Sprintf("CF_INSTANCE_IP=%s\n", componentMaker.ExternalAddress)))
			})
		})
	})

	Describe("LRPs", func() {
		var response []byte
		var actualLRP *models.ActualLRP

		BeforeEach(func() {
			test_helper.CreateZipArchive(
				filepath.Join(fileServerStaticDir, "lrp.zip"),
				fixtures.GoServerApp(),
			)
		})

		JustBeforeEach(func() {
			lrp := helpers.DefaultLRPCreateRequest(guid, guid, 1)
			lrp.Setup = models.WrapAction(&models.DownloadAction{
				User: "vcap",
				From: fmt.Sprintf("http://%s/v1/static/%s", componentMaker.Addresses.FileServer, "lrp.zip"),
				To:   "/tmp",
			})
			lrp.Action = models.WrapAction(&models.RunAction{
				User: "vcap",
				Path: "/tmp/go-server",
				Env:  []*models.EnvironmentVariable{{"PORT", "8080"}},
			})

			err := bbsClient.DesireLRP(logger, lrp)
			Expect(err).NotTo(HaveOccurred())

			Eventually(helpers.LRPStatePoller(logger, bbsClient, guid, nil)).Should(Equal(models.ActualLRPStateRunning))
			Eventually(helpers.ResponseCodeFromHostPoller(componentMaker.Addresses.Router, helpers.DefaultHost)).Should(Equal(http.StatusOK))

			lrps, err := bbsClient.ActualLRPGroupsByProcessGuid(logger, guid)
			Expect(err).NotTo(HaveOccurred())

			Expect(lrps).To(HaveLen(1))
			actualLRP = lrps[0].Instance

			var status int
			response, status, err = helpers.ResponseBodyAndStatusCodeFromHost(componentMaker.Addresses.Router, helpers.DefaultHost, "env")
			Expect(status).To(Equal(http.StatusOK))
			Expect(err).NotTo(HaveOccurred())
		})

		Context("when -exportNetworkEnvVars=false is set", func() {
			BeforeEach(func() {
				repFlags = []string{"-exportNetworkEnvVars=false"}
			})

			It("does not set the networking environment variables", func() {
				Expect(response).NotTo(ContainSubstring("CF_INSTANCE_IP="))
				Expect(response).NotTo(ContainSubstring("CF_INSTANCE_ADDR="))
				Expect(response).NotTo(ContainSubstring("CF_INSTANCE_PORT="))
				Expect(response).NotTo(ContainSubstring("CF_INSTANCE_PORTS="))
			})
		})

		Context("when -exportNetworkEnvVars=true is set", func() {
			BeforeEach(func() {
				repFlags = []string{"-exportNetworkEnvVars=true"}
			})

			It("sets the networking environment variables", func() {
				netInfo := actualLRP.ActualLRPNetInfo
				Expect(response).To(ContainSubstring(fmt.Sprintf("CF_INSTANCE_ADDR=%s:%d\n", netInfo.Address, netInfo.Ports[0].HostPort)))
				Expect(response).To(ContainSubstring(fmt.Sprintf("CF_INSTANCE_IP=%s\n", componentMaker.ExternalAddress)))
				Expect(response).To(ContainSubstring(fmt.Sprintf("CF_INSTANCE_PORT=%d\n", netInfo.Ports[0].HostPort)))

				type portMapping struct {
					External uint32 `json:"external"`
					Internal uint32 `json:"internal"`
				}
				ports := []portMapping{}

				buf := bytes.NewBuffer(response)
				for {
					line, err := buf.ReadString('\n')
					if err != nil {
						break
					}
					if strings.HasPrefix(line, "CF_INSTANCE_PORTS=") {
						err := json.Unmarshal([]byte(strings.TrimPrefix(line, "CF_INSTANCE_PORTS=")), &ports)
						Expect(err).NotTo(HaveOccurred())
						break
					}
				}

				Expect(ports).To(Equal([]portMapping{
					{External: netInfo.Ports[0].HostPort, Internal: netInfo.Ports[0].ContainerPort},
				}))
			})
		})
	})
})
