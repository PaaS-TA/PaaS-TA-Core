package volman_test

import (
	"encoding/json"
	"os"
	"time"

	"code.cloudfoundry.org/auction/auctiontypes"
	"code.cloudfoundry.org/bbs"
	"code.cloudfoundry.org/bbs/models"
	"code.cloudfoundry.org/inigo/helpers"
	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/lager/lagertest"
	repconfig "code.cloudfoundry.org/rep/cmd/rep/config"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/tedsuo/ifrit"
	"github.com/tedsuo/ifrit/ginkgomon"
	"github.com/tedsuo/ifrit/grouper"
)

var _ = Describe("Tasks", func() {
	var (
		cellProcess, plumbing ifrit.Process
		logger                lager.Logger
		bbsClient             bbs.InternalClient
	)

	BeforeEach(func() {
		logger = lagertest.NewTestLogger("volman-tasks")
		var fileServerRunner ifrit.Runner
		fileServerRunner, _ = componentMaker.FileServer()

		plumbing = ginkgomon.Invoke(grouper.NewOrdered(os.Kill, grouper.Members{
			{"initial-services", grouper.NewParallel(os.Kill, grouper.Members{
				{"sql", componentMaker.SQL()},
				{"consul", componentMaker.Consul()},
			})},
			{"locket", componentMaker.Locket()},
			{"bbs", componentMaker.BBS()},
		}))

		helpers.ConsulWaitUntilReady()

		cellProcess = ginkgomon.Invoke(grouper.NewParallel(os.Interrupt, grouper.Members{
			{"file-server", fileServerRunner},
			{"rep", componentMaker.Rep(func(config *repconfig.RepConfig) { config.MemoryMB = "1024" })},
			{"auctioneer", componentMaker.Auctioneer()},
		}))

		bbsServiceClient := componentMaker.BBSServiceClient(logger)
		bbsClient = componentMaker.BBSClient()

		Eventually(func() (models.CellSet, error) { return bbsServiceClient.Cells(logger) }).Should(HaveLen(1))
	})

	AfterEach(func() {
		helpers.StopProcesses(plumbing, cellProcess)
	})

	Describe("running a task with volume mount", func() {
		var (
			fileName, guid string
		)

		Context("when driver required is available on a cell", func() {
			BeforeEach(func() {
				guid = helpers.GenerateGuid()

				fileName = "testfile-" + string(time.Now().UnixNano()) + ".txt"
				expectedTask := helpers.TaskCreateRequest(
					guid,
					&models.RunAction{
						Path: "/bin/touch",
						User: "root",
						Args: []string{"/testmount/" + fileName},
					},
				)
				expectedTask.VolumeMounts = []*models.VolumeMount{
					generateVolumeObject("localdriver"),
				}
				expectedTask.Privileged = true

				err := bbsClient.DesireTask(logger, expectedTask.TaskGuid, expectedTask.Domain, expectedTask.TaskDefinition)
				Expect(err).NotTo(HaveOccurred())
			})

			It("can write files to the mounted volume", func() {
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

		Context("when driver required not available on any cell", func() {
			var (
				expectedTask *models.Task
			)

			BeforeEach(func() {
				guid = helpers.GenerateGuid()
				expectedTask = helpers.TaskCreateRequest(
					guid,
					&models.RunAction{
						User: "vcap",
						Path: "sh",
						Args: []string{"-c", "echo 'volman!'"},
					},
				)
				expectedTask.VolumeMounts = []*models.VolumeMount{
					generateVolumeObject("non-existent-driver"),
				}
				expectedTask.Privileged = true
			})

			It("should error placing the task", func() {
				err := bbsClient.DesireTask(logger, expectedTask.TaskGuid, expectedTask.Domain, expectedTask.TaskDefinition)
				Expect(err).NotTo(HaveOccurred())

				var task *models.Task
				Eventually(func() interface{} {
					var err error

					task, err = bbsClient.TaskByGuid(logger, expectedTask.TaskGuid)
					Expect(err).NotTo(HaveOccurred())

					return task.State
				}).Should(Equal(models.Task_Completed))

				Expect(task.Failed).To(BeTrue())
				Expect(task.FailureReason).To(Equal(auctiontypes.ErrorVolumeDriverMismatch.Error()))
			})
		})

		Context("when one of the drivers required is not available on any cell", func() {
			var (
				expectedTask *models.Task
			)

			BeforeEach(func() {
				guid = helpers.GenerateGuid()
				expectedTask = helpers.TaskCreateRequest(
					guid,
					&models.RunAction{
						User: "vcap",
						Path: "sh",
						Args: []string{"-c", "echo 'volman!'"},
					},
				)

				expectedTask.VolumeMounts = []*models.VolumeMount{
					generateVolumeObject("non-existent-driver"),
					generateVolumeObject("localdriver"),
				}

				expectedTask.Privileged = true
			})

			It("should error placing the task", func() {
				err := bbsClient.DesireTask(logger, expectedTask.TaskGuid, expectedTask.Domain, expectedTask.TaskDefinition)
				Expect(err).NotTo(HaveOccurred())

				var task *models.Task
				Eventually(func() interface{} {
					var err error

					task, err = bbsClient.TaskByGuid(logger, expectedTask.TaskGuid)
					Expect(err).NotTo(HaveOccurred())

					return task.State
				}).Should(Equal(models.Task_Completed))

				Expect(task.Failed).To(BeTrue())
				Expect(task.FailureReason).To(Equal(auctiontypes.ErrorVolumeDriverMismatch.Error()))
			})
		})
	})
})

func generateVolumeObject(driver string) *models.VolumeMount {
	volumeId := "some-volumeID-" + string(time.Now().UnixNano())
	someConfig := map[string]interface{}{"volume_id": volumeId}
	jsonSomeConfig, err := json.Marshal(someConfig)
	Expect(err).NotTo(HaveOccurred())

	return &models.VolumeMount{
		Driver:       driver,
		ContainerDir: "/testmount",
		Mode:         "rw",
		Shared: &models.SharedDevice{
			VolumeId:    volumeId,
			MountConfig: string(jsonSomeConfig),
		},
	}
}
