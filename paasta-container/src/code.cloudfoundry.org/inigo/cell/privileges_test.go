package cell_test

import (
	"net/http"
	"os"

	"code.cloudfoundry.org/bbs/models"
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
		runtime = ginkgomon.Invoke(grouper.NewParallel(os.Kill, grouper.Members{
			{"rep", componentMaker.Rep()},
			{"auctioneer", componentMaker.Auctioneer()},
			{"router", componentMaker.Router()},
			{"route-emitter", componentMaker.RouteEmitter()},
		}))
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
			lrpRequest = helpers.PrivilegedLRPCreateRequest(helpers.GenerateGuid())
		})

		JustBeforeEach(func() {
			err := bbsClient.DesireLRP(logger, lrpRequest)
			Expect(err).NotTo(HaveOccurred())
		})

		Context("when the LRP is privileged", func() {
			BeforeEach(func() {
				lrpRequest.Privileged = true
			})

			It("succeeds", func() {
				Eventually(helpers.ResponseCodeFromHostPoller(componentMaker.Addresses.Router, helpers.DefaultHost)).Should(Equal(http.StatusOK))
			})
		})

		Context("when the LRP is not privileged", func() {
			BeforeEach(func() {
				lrpRequest.Privileged = false
			})

			It("fails", func() {
				Eventually(helpers.ResponseCodeFromHostPoller(componentMaker.Addresses.Router, helpers.DefaultHost)).Should(Equal(http.StatusInternalServerError))
			})
		})
	})
})
