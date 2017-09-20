package cell_test

import (
	"os"

	"github.com/tedsuo/ifrit"
	"github.com/tedsuo/ifrit/ginkgomon"
	"github.com/tedsuo/ifrit/grouper"

	"code.cloudfoundry.org/bbs/models"
	"code.cloudfoundry.org/inigo/helpers"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Tasks as specific user", func() {
	var cellProcess ifrit.Process

	BeforeEach(func() {
		var fileServerRunner ifrit.Runner

		fileServerRunner, _ = componentMaker.FileServer()

		cellGroup := grouper.Members{
			{"file-server", fileServerRunner},
			{"rep", componentMaker.Rep("-memoryMB", "1024")},
			{"auctioneer", componentMaker.Auctioneer()},
		}
		cellProcess = ginkgomon.Invoke(grouper.NewParallel(os.Interrupt, cellGroup))

		Eventually(func() (models.CellSet, error) { return bbsServiceClient.Cells(logger) }).Should(HaveLen(1))
	})

	AfterEach(func() {
		helpers.StopProcesses(cellProcess)
	})

	Describe("Running a task", func() {
		var guid string

		BeforeEach(func() {
			guid = helpers.GenerateGuid()
		})

		It("runs the command as a specific user", func() {
			expectedTask := helpers.TaskCreateRequest(
				guid,
				&models.RunAction{
					User: "testuser",
					Path: "sh",
					Args: []string{"-c", `[ $(whoami) = testuser ]`},
				},
			)
			expectedTask.Privileged = true
			err := bbsClient.DesireTask(logger, expectedTask.TaskGuid, expectedTask.Domain, expectedTask.TaskDefinition)
			Expect(err).NotTo(HaveOccurred())

			var task *models.Task

			Eventually(func() interface{} {
				var err error

				task, err = bbsClient.TaskByGuid(logger, guid)
				Expect(err).NotTo(HaveOccurred())

				return task.State
			}).Should(Equal(models.Task_Completed))
			Expect(task.Failed).To(BeFalse())
		})
	})
})
