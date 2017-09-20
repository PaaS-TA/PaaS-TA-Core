package main_test

import (
	"time"

	"code.cloudfoundry.org/auctioneer"
	"code.cloudfoundry.org/bbs"
	"code.cloudfoundry.org/bbs/models"
	"code.cloudfoundry.org/bbs/models/test/model_helpers"
	"code.cloudfoundry.org/clock"
	"code.cloudfoundry.org/locket"
	"code.cloudfoundry.org/rep"
	"github.com/hashicorp/consul/api"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gexec"
	"github.com/tedsuo/ifrit"
	"github.com/tedsuo/ifrit/ginkgomon"
)

var dummyAction = &models.RunAction{
	User: "me",
	Path: "cat",
	Args: []string{"/tmp/file"},
}

var exampleDesiredLRP = models.DesiredLRP{
	ProcessGuid: "process-guid",
	DiskMb:      1,
	MemoryMb:    1,
	RootFs:      linuxRootFSURL,
	Action:      models.WrapAction(dummyAction),
	Domain:      "test",
	Instances:   2,
}

func exampleTaskDefinition() *models.TaskDefinition {
	taskDef := model_helpers.NewValidTaskDefinition()
	taskDef.RootFs = linuxRootFSURL
	taskDef.Action = models.WrapAction(dummyAction)
	taskDef.PlacementTags = nil
	return taskDef
}

var _ = Describe("Auctioneer", func() {
	Context("when etcd is down", func() {
		BeforeEach(func() {
			etcdRunner.Stop()
			auctioneerProcess = ginkgomon.Invoke(runner)
		})

		AfterEach(func() {
			etcdRunner.Start()
		})

		It("starts", func() {
			Consistently(runner).ShouldNot(Exit())
		})
	})

	Context("when the auctioneer starts up", func() {
		BeforeEach(func() {
			auctioneerProcess = ginkgomon.Invoke(runner)
		})

		It("registers itself as a service", func() {
			client := consulRunner.NewClient()
			services, err := client.Agent().Services()
			Expect(err).NotTo(HaveOccurred())

			Expect(services).To(HaveKeyWithValue("auctioneer", &api.AgentService{
				ID:      "auctioneer",
				Service: "auctioneer",
				Port:    auctioneerServerPort,
				Address: "",
			}))
		})

		It("registers a TTL healthcheck", func() {
			client := consulRunner.NewClient()
			checks, err := client.Agent().Checks()
			Expect(err).NotTo(HaveOccurred())

			Expect(checks).To(HaveKeyWithValue("service:auctioneer", &api.AgentCheck{
				Node:        "0",
				CheckID:     "service:auctioneer",
				Name:        "Service 'auctioneer' check",
				Status:      "passing",
				Notes:       "",
				Output:      "",
				ServiceID:   "auctioneer",
				ServiceName: "auctioneer",
			}))
		})
	})

	Context("when a start auction message arrives", func() {
		BeforeEach(func() {
			auctioneerProcess = ginkgomon.Invoke(runner)

			err := auctioneerClient.RequestLRPAuctions([]*auctioneer.LRPStartRequest{{
				ProcessGuid: exampleDesiredLRP.ProcessGuid,
				Domain:      exampleDesiredLRP.Domain,
				Indices:     []int{0},
				Resource: rep.Resource{
					MemoryMB: 5,
					DiskMB:   5,
				},
				PlacementConstraint: rep.PlacementConstraint{
					RootFs: exampleDesiredLRP.RootFs,
				},
			}})
			Expect(err).NotTo(HaveOccurred())

			err = auctioneerClient.RequestLRPAuctions([]*auctioneer.LRPStartRequest{{
				ProcessGuid: exampleDesiredLRP.ProcessGuid,
				Domain:      exampleDesiredLRP.Domain,
				Indices:     []int{1},
				Resource: rep.Resource{
					MemoryMB: 5,
					DiskMB:   5,
				},
				PlacementConstraint: rep.PlacementConstraint{
					RootFs: exampleDesiredLRP.RootFs,
				},
			}})
			Expect(err).NotTo(HaveOccurred())
		})

		It("should start the process running on reps of the appropriate stack", func() {
			Eventually(linuxCell.LRPs).Should(HaveLen(2))
			Expect(dotNetCell.LRPs()).To(BeEmpty())
		})
	})

	Context("when a task message arrives", func() {
		BeforeEach(func() {
			auctioneerProcess = ginkgomon.Invoke(runner)
		})

		Context("when there are sufficient resources to start the task", func() {
			BeforeEach(func() {
				taskDef := exampleTaskDefinition()
				taskDef.DiskMb = 1
				taskDef.MemoryMb = 1
				err := bbsClient.DesireTask(logger, "guid", "domain", taskDef)
				Expect(err).NotTo(HaveOccurred())
			})

			It("should start the task running on reps of the appropriate stack", func() {
				Eventually(linuxCell.Tasks).Should(HaveLen(1))
				Expect(dotNetCell.Tasks()).To(BeEmpty())
			})
		})

		Context("when there are insufficient resources to start the task", func() {
			BeforeEach(func() {
				taskDef := exampleTaskDefinition()
				taskDef.DiskMb = 1000
				taskDef.MemoryMb = 1000

				err := bbsClient.DesireTask(logger, "task-guid", "domain", taskDef)
				Expect(err).NotTo(HaveOccurred())
			})

			It("should not start the task on any rep", func() {
				Consistently(linuxCell.Tasks).Should(BeEmpty())
				Consistently(dotNetCell.Tasks).Should(BeEmpty())
			})

			It("should mark the task as failed in the BBS", func() {
				Eventually(func() []*models.Task {
					return getTasksByState(bbsClient, models.Task_Completed)
				}).Should(HaveLen(1))

				completedTasks := getTasksByState(bbsClient, models.Task_Completed)
				completedTask := completedTasks[0]
				Expect(completedTask.TaskGuid).To(Equal("task-guid"))
				Expect(completedTask.Failed).To(BeTrue())
				Expect(completedTask.FailureReason).To(Equal("insufficient resources: disk, memory"))
			})
		})
	})

	Context("when the auctioneer loses the lock", func() {
		BeforeEach(func() {
			auctioneerProcess = ginkgomon.Invoke(runner)
			consulRunner.Reset()
		})

		It("exits with an error", func() {
			Eventually(runner.ExitCode, 3).Should(Equal(1))
		})
	})

	Context("when the auctioneer cannot acquire the lock on startup", func() {
		var task *rep.Task
		var competingAuctioneerProcess ifrit.Process

		BeforeEach(func() {
			task = &rep.Task{
				TaskGuid: "task-guid",
				Domain:   "test",
				Resource: rep.Resource{
					MemoryMB: 124,
					DiskMB:   456,
				},
				PlacementConstraint: rep.PlacementConstraint{
					RootFs: "some-rootfs",
				},
			}

			competingAuctioneerLock := locket.NewLock(logger, consulClient, locket.LockSchemaPath("auctioneer_lock"), []byte{}, clock.NewClock(), 500*time.Millisecond, 10*time.Second)
			competingAuctioneerProcess = ifrit.Invoke(competingAuctioneerLock)

			runner.StartCheck = "auctioneer.lock-bbs.lock.acquiring-lock"

			auctioneerProcess = ifrit.Background(runner)
		})

		AfterEach(func() {
			ginkgomon.Kill(competingAuctioneerProcess)
		})

		It("should not advertise its presence, and should not be reachable", func() {
			Eventually(func() error {
				return auctioneerClient.RequestTaskAuctions([]*auctioneer.TaskStartRequest{
					&auctioneer.TaskStartRequest{*task},
				})
			}).Should(HaveOccurred())
		})

		It("should eventually come up in the event that the lock is released", func() {
			ginkgomon.Kill(competingAuctioneerProcess)

			Eventually(func() error {
				return auctioneerClient.RequestTaskAuctions([]*auctioneer.TaskStartRequest{
					&auctioneer.TaskStartRequest{*task},
				})
			}).ShouldNot(HaveOccurred())
		})
	})
})

func getTasksByState(client bbs.InternalClient, state models.Task_State) []*models.Task {
	tasks, err := client.Tasks(logger)
	Expect(err).NotTo(HaveOccurred())

	filteredTasks := make([]*models.Task, 0)
	for _, task := range tasks {
		if task.State == state {
			filteredTasks = append(filteredTasks, task)
		}
	}
	return filteredTasks
}
