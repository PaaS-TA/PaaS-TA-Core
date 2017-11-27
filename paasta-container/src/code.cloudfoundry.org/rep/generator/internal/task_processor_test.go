package internal_test

import (
	"errors"

	"code.cloudfoundry.org/bbs/fake_bbs"
	"code.cloudfoundry.org/bbs/models"
	"code.cloudfoundry.org/bbs/models/test/model_helpers"
	"code.cloudfoundry.org/executor"
	"code.cloudfoundry.org/lager/lagertest"
	"code.cloudfoundry.org/rep"
	"code.cloudfoundry.org/rep/generator/internal"
	"code.cloudfoundry.org/rep/generator/internal/fake_internal"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var processor internal.TaskProcessor

var _ = Describe("TaskProcessor", func() {
	var (
		bbsClient                *fake_bbs.FakeInternalClient
		expectedCellID, taskGuid string
		containerDelegate        *fake_internal.FakeContainerDelegate
		logger                   *lagertest.TestLogger
		task                     *models.Task
		expectedRunRequest       executor.RunRequest
		container                executor.Container
	)

	BeforeEach(func() {
		var err error

		bbsClient = &fake_bbs.FakeInternalClient{}
		containerDelegate = &fake_internal.FakeContainerDelegate{}
		logger = lagertest.NewTestLogger("task-processor")

		expectedCellID = "the-cell"
		taskGuid = "the-guid"

		processor = internal.NewTaskProcessor(bbsClient, containerDelegate, expectedCellID)

		task = model_helpers.NewValidTask(taskGuid)
		expectedRunRequest, err = rep.NewRunRequestFromTask(task)
		Expect(err).NotTo(HaveOccurred())

		bbsClient.TaskByGuidReturns(task, nil)
	})

	itProcessesAnActiveContainer := func() {
		BeforeEach(func() {
			bbsClient.StartTaskReturns(true, nil)
		})

		JustBeforeEach(func() {
			processor.Process(logger, container)
		})

		It("starts the task", func() {
			Expect(bbsClient.StartTaskCallCount()).To(Equal(1))
			_, guid, cellID := bbsClient.StartTaskArgsForCall(0)
			Expect(guid).To(Equal(taskGuid))
			Expect(cellID).To(Equal(expectedCellID))
		})

		It("runs the container", func() {
			Expect(containerDelegate.RunContainerCallCount()).To(Equal(1))
			_, runReq := containerDelegate.RunContainerArgsForCall(0)
			Expect(runReq).To(Equal(&expectedRunRequest))
		})

		Context("when the task hasn't changed", func() {
			BeforeEach(func() {
				bbsClient.StartTaskReturns(false, nil)
			})

			It("does not run the container", func() {
				Expect(containerDelegate.RunContainerCallCount()).To(Equal(0))
			})
		})

		Context("when fetching the task fails", func() {
			BeforeEach(func() {
				bbsClient.TaskByGuidReturns(nil, errors.New("boom"))
			})

			It("does not run the container", func() {
				Expect(bbsClient.StartTaskCallCount()).To(Equal(1))
				Expect(bbsClient.TaskByGuidCallCount()).To(Equal(1))
				Expect(containerDelegate.RunContainerCallCount()).To(Equal(0))
				Expect(containerDelegate.DeleteContainerCallCount()).To(Equal(0))
			})
		})

		Context("when creating the run request fails", func() {
			BeforeEach(func() {
				task.RootFs = "% s"
			})

			It("does not run the container", func() {
				Expect(bbsClient.StartTaskCallCount()).To(Equal(1))
				Expect(bbsClient.TaskByGuidCallCount()).To(Equal(1))
				Expect(containerDelegate.RunContainerCallCount()).To(Equal(0))
				Expect(containerDelegate.DeleteContainerCallCount()).To(Equal(0))
			})
		})

		Context("when running the container fails", func() {
			BeforeEach(func() {
				containerDelegate.RunContainerReturns(false)
			})

			It("fails the task", func() {
				Expect(bbsClient.FailTaskCallCount()).To(Equal(1))
				_, guid, reason := bbsClient.FailTaskArgsForCall(0)
				Expect(guid).To(Equal(taskGuid))
				Expect(reason).To(Equal(internal.TaskCompletionReasonFailedToRunContainer))
			})
		})

		Context("when starting the task fails", func() {
			Context("because of an invalid state transition", func() {
				BeforeEach(func() {
					bbsClient.StartTaskReturns(false, models.NewTaskTransitionError(
						models.Task_Pending,
						models.Task_Running,
					))
				})

				It("deletes the container", func() {
					Expect(containerDelegate.RunContainerCallCount()).To(Equal(0))
					Expect(containerDelegate.DeleteContainerCallCount()).To(Equal(1))
					_, guid := containerDelegate.DeleteContainerArgsForCall(0)
					Expect(guid).To(Equal(taskGuid))
				})
			})

			Context("because a resource was not found", func() {
				BeforeEach(func() {
					bbsClient.StartTaskReturns(false, models.ErrResourceNotFound)
				})

				It("deletes the container", func() {
					Expect(containerDelegate.RunContainerCallCount()).To(Equal(0))
					Expect(containerDelegate.DeleteContainerCallCount()).To(Equal(1))
					_, guid := containerDelegate.DeleteContainerArgsForCall(0)
					Expect(guid).To(Equal(taskGuid))
				})
			})

			Context("for another reason", func() {
				BeforeEach(func() {
					bbsClient.StartTaskReturns(false, errors.New("boom"))
				})

				It("does not delete the container", func() {
					Expect(containerDelegate.RunContainerCallCount()).To(Equal(0))
					Expect(containerDelegate.DeleteContainerCallCount()).To(Equal(0))
				})
			})
		})
	}

	Context("when the container is initializing", func() {
		BeforeEach(func() {
			container = executor.Container{
				State: executor.StateInitializing,
				Guid:  taskGuid,
			}
		})

		itProcessesAnActiveContainer()
	})

	Context("when the container is created", func() {
		BeforeEach(func() {
			container = executor.Container{
				State: executor.StateCreated,
				Guid:  taskGuid,
			}
		})

		itProcessesAnActiveContainer()
	})

	Context("when the container is running", func() {
		BeforeEach(func() {
			container = executor.Container{
				State: executor.StateRunning,
				Guid:  taskGuid,
			}
		})

		itProcessesAnActiveContainer()
	})

	Context("when the container is reserved", func() {
		BeforeEach(func() {
			container = executor.Container{
				State: executor.StateReserved,
				Guid:  taskGuid,
			}
		})

		itProcessesAnActiveContainer()
	})

	Context("when the container is completed", func() {
		BeforeEach(func() {
			container = executor.Container{
				State: executor.StateCompleted,
				Guid:  taskGuid,
				RunResult: executor.ContainerRunResult{
					Failed:        true,
					FailureReason: "oh nooooooooooooo mr bill",
				},
			}
		})

		JustBeforeEach(func() {
			processor.Process(logger, container)
		})

		It("deletes the container", func() {
			Expect(containerDelegate.DeleteContainerCallCount()).To(Equal(1))
			_, guid := containerDelegate.DeleteContainerArgsForCall(0)
			Expect(guid).To(Equal(taskGuid))
		})

		It("completes the task", func() {
			Expect(bbsClient.CompleteTaskCallCount()).To(Equal(1))
			_, guid, cellID, failed, failureReason, result := bbsClient.CompleteTaskArgsForCall(0)
			Expect(guid).To(Equal(taskGuid))
			Expect(cellID).To(Equal(expectedCellID))
			Expect(failed).To(Equal(true))
			Expect(failureReason).To(Equal("oh nooooooooooooo mr bill"))
			Expect(result).To(Equal(""))
		})

		Context("when completing the task fails", func() {
			Context("because of an invalid state transition", func() {
				BeforeEach(func() {
					bbsClient.CompleteTaskReturns(models.NewTaskTransitionError(models.Task_Running, models.Task_Completed))
				})

				It("fails the task", func() {
					Expect(bbsClient.CompleteTaskCallCount()).To(Equal(1))
					Expect(bbsClient.FailTaskCallCount()).To(Equal(1))
					_, guid, reason := bbsClient.FailTaskArgsForCall(0)
					Expect(guid).To(Equal(taskGuid))
					Expect(reason).To(Equal(internal.TaskCompletionReasonInvalidTransition))
				})
			})
		})

		Context("when the container run succeeds", func() {
			BeforeEach(func() {
				container.RunResult = executor.ContainerRunResult{
					Failed: false,
				}
				container.Tags = executor.Tags{
					rep.ResultFileTag: "foobar",
				}

				containerDelegate.FetchContainerResultFileReturns("i am a result yo", nil)
			})

			It("fetches the result file and completes the task", func() {
				Expect(containerDelegate.FetchContainerResultFileCallCount()).To(Equal(1))
				_, guid, tag := containerDelegate.FetchContainerResultFileArgsForCall(0)
				Expect(guid).To(Equal(taskGuid))
				Expect(tag).To(Equal(container.Tags[rep.ResultFileTag]))

				Expect(bbsClient.CompleteTaskCallCount()).To(Equal(1))
				_, guid, cellID, failed, failureReason, result := bbsClient.CompleteTaskArgsForCall(0)
				Expect(guid).To(Equal(taskGuid))
				Expect(cellID).To(Equal(expectedCellID))
				Expect(failed).To(Equal(false))
				Expect(failureReason).To(Equal(""))
				Expect(result).To(Equal("i am a result yo"))
			})

			Context("and there is no result file tag", func() {
				BeforeEach(func() {
					container.Tags = executor.Tags{}
				})

				It("does not attempt to fetch the result file and completes the task", func() {
					Expect(containerDelegate.FetchContainerResultFileCallCount()).To(Equal(0))

					Expect(bbsClient.CompleteTaskCallCount()).To(Equal(1))
					_, guid, cellID, failed, failureReason, result := bbsClient.CompleteTaskArgsForCall(0)
					Expect(guid).To(Equal(taskGuid))
					Expect(cellID).To(Equal(expectedCellID))
					Expect(failed).To(Equal(false))
					Expect(failureReason).To(Equal(""))
					Expect(result).To(Equal(""))
				})
			})

			Context("and fetching the container result fails", func() {
				BeforeEach(func() {
					containerDelegate.FetchContainerResultFileReturns("", errors.New("get outta here"))
				})

				It("fails the task", func() {
					Expect(containerDelegate.FetchContainerResultFileCallCount()).To(Equal(1))
					Expect(bbsClient.FailTaskCallCount()).To(Equal(1))
					_, guid, reason := bbsClient.FailTaskArgsForCall(0)
					Expect(guid).To(Equal(taskGuid))
					Expect(reason).To(Equal(internal.TaskCompletionReasonFailedToFetchResult))
				})
			})
		})
	})
})
