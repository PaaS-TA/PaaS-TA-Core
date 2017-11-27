package cell_test

import (
	"net/http"
	"os"
	"path/filepath"

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

var _ = Describe("Privileges", func() {
	var runtime ifrit.Process

	BeforeEach(func() {
		fileServer, fileServerStaticDir := componentMaker.FileServer()
		runtime = ginkgomon.Invoke(grouper.NewParallel(os.Kill, grouper.Members{
			{"router", componentMaker.Router()},
			{"file-server", fileServer},
			{"rep", componentMaker.Rep()},
			{"auctioneer", componentMaker.Auctioneer()},
			{"route-emitter", componentMaker.RouteEmitter()},
		}))

		test_helper.CreateZipArchive(
			filepath.Join(fileServerStaticDir, "lrp.zip"),
			fixtures.GoServerApp(),
		)
	})

	AfterEach(func() {
		helpers.StopProcesses(runtime)
	})

	Context("when a task that tries to do privileged things is requested", func() {
		var taskToDesire *models.Task

		BeforeEach(func() {
			taskToDesire = helpers.TaskCreateRequest(
				helpers.GenerateGuid(),
				&models.RunAction{
					Path: "sh",
					// always run as root; tests change task-level privileged
					User: "root",
					Args: []string{
						"-c",
						// writing to /proc/sysrq-trigger requires full privileges;
						// h is a safe thing to write
						"echo h > /proc/sysrq-trigger",
					},
				},
			)
		})

		JustBeforeEach(func() {
			err := bbsClient.DesireTask(logger, taskToDesire.TaskGuid, taskToDesire.Domain, taskToDesire.TaskDefinition)
			Expect(err).NotTo(HaveOccurred())
		})

		Context("when the task is privileged", func() {
			BeforeEach(func() {
				taskToDesire.Privileged = true
			})

			It("succeeds", func() {
				var task models.Task
				Eventually(helpers.TaskStatePoller(logger, bbsClient, taskToDesire.TaskGuid, &task)).Should(Equal(models.Task_Completed))
				Expect(task.Failed).To(BeFalse())
			})
		})

		Context("when the task is not privileged", func() {
			BeforeEach(func() {
				taskToDesire.Privileged = false
			})

			It("fails", func() {
				var task models.Task
				Eventually(helpers.TaskStatePoller(logger, bbsClient, taskToDesire.TaskGuid, &task)).Should(Equal(models.Task_Completed))
				Expect(task.Failed).To(BeTrue())
			})
		})
	})

	Context("when a LRP that tries to do privileged things is requested", func() {
		var lrpRequest *models.DesiredLRP

		BeforeEach(func() {
			lrpRequest = helpers.DefaultLRPCreateRequest(helpers.GenerateGuid(), "log-guid", 1)
			lrpRequest.Action = models.WrapAction(&models.RunAction{
				User: "root",
				Path: "/tmp/diego/go-server",
				Env:  []*models.EnvironmentVariable{{"PORT", "8080"}},
			})
		})

		JustBeforeEach(func() {
			err := bbsClient.DesireLRP(logger, lrpRequest)
			Expect(err).NotTo(HaveOccurred())
			Eventually(helpers.LRPStatePoller(logger, bbsClient, lrpRequest.ProcessGuid, nil)).Should(Equal(models.ActualLRPStateRunning))
		})

		Context("when the LRP is privileged", func() {
			BeforeEach(func() {
				lrpRequest.Privileged = true
			})

			It("succeeds", func() {
				Eventually(helpers.ResponseCodeFromHostPoller(componentMaker.Addresses.Router, helpers.DefaultHost, "privileged")).Should(Equal(http.StatusOK))
			})
		})

		Context("when the LRP is not privileged", func() {
			BeforeEach(func() {
				lrpRequest.Privileged = false
			})

			It("fails", func() {
				Expect(helpers.ResponseCodeFromHostPoller(componentMaker.Addresses.Router, helpers.DefaultHost, "privileged")()).To(Equal(http.StatusInternalServerError))
			})
		})
	})
})
