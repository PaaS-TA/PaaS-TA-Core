package vizzini_test

import (
	"code.cloudfoundry.org/bbs/models"
	. "code.cloudfoundry.org/vizzini/matchers"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Targetting different RootFSes", func() {
	var task *models.TaskDefinition
	var rootFS string

	JustBeforeEach(func() {
		task = Task()
		task.RootFs = rootFS
		task.Action = models.WrapAction(&models.RunAction{
			Path: "bash",
			Args: []string{"-c", "bash --version > /tmp/bar"},
			User: "vcap",
		})
		Expect(bbsClient.DesireTask(logger, guid, domain, task)).To(Succeed())
		Eventually(TaskGetter(logger, guid)).Should(HaveTaskState(models.Task_Completed))
	})

	AfterEach(func() {
		Expect(bbsClient.ResolvingTask(logger, guid)).To(Succeed())
		Expect(bbsClient.DeleteTask(logger, guid)).To(Succeed())
	})

	Describe("cflinuxfs2", func() {
		BeforeEach(func() {
			rootFS = models.PreloadedRootFS("cflinuxfs2")
		})

		It("should run the cflinuxfs2 rootfs", func() {
			completedTask, err := bbsClient.TaskByGuid(logger, guid)
			Expect(err).NotTo(HaveOccurred())
			Expect(completedTask.Result).To(ContainSubstring(`bash, version 4.3.11`))
		})
	})
})
