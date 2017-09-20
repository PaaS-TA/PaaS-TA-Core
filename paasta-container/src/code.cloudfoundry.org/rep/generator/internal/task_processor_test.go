package internal_test

import (
	"errors"

	etcddb "code.cloudfoundry.org/bbs/db/etcd"
	"code.cloudfoundry.org/bbs/models"
	"code.cloudfoundry.org/bbs/models/test/model_helpers"
	"code.cloudfoundry.org/executor"
	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/lager/lagertest"
	"code.cloudfoundry.org/rep"
	"code.cloudfoundry.org/rep/generator/internal"
	"code.cloudfoundry.org/rep/generator/internal/fake_internal"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var processor internal.TaskProcessor

var _ = Describe("Task <-> Container table", func() {
	var containerDelegate *fake_internal.FakeContainerDelegate

	const (
		taskGuid      = "my-guid"
		localCellID   = "a"
		otherCellID   = "w"
		sessionPrefix = "task-table-test"
	)

	BeforeEach(func() {
		etcdRunner.ResetAllBut(etcddb.VersionKey)
		containerDelegate = new(fake_internal.FakeContainerDelegate)
		processor = internal.NewTaskProcessor(bbsClient, containerDelegate, localCellID)

		containerDelegate.DeleteContainerReturns(true)
		containerDelegate.StopContainerReturns(true)
		containerDelegate.RunContainerReturns(true)
	})

	itDeletesTheContainer := func(logger *lagertest.TestLogger) {
		It("deletes the container", func() {
			Expect(containerDelegate.DeleteContainerCallCount()).To(Equal(1))
			_, containerGuid := containerDelegate.DeleteContainerArgsForCall(0)
			Expect(containerGuid).To(Equal(taskGuid))
		})
	}

	itCompletesTheTaskWithFailure := func(reason string) func(*lagertest.TestLogger) {
		return func(logger *lagertest.TestLogger) {
			It("completes the task with failure", func() {
				task, err := bbsClient.TaskByGuid(logger, taskGuid)
				Expect(err).NotTo(HaveOccurred())

				Expect(task.State).To(Equal(models.Task_Completed))
				Expect(task.Failed).To(BeTrue())
				Expect(task.FailureReason).To(Equal(reason))
			})
		}
	}

	successfulRunResult := executor.ContainerRunResult{
		Failed: false,
	}

	itCompletesTheSuccessfulTaskAndDeletesTheContainer := func(logger *lagertest.TestLogger) {
		Context("when fetching the result succeeds", func() {
			BeforeEach(func() {
				containerDelegate.FetchContainerResultFileReturns("some-result", nil)

				containerDelegate.DeleteContainerStub = func(logger lager.Logger, guid string) bool {
					task, err := bbsClient.TaskByGuid(logger, taskGuid)
					Expect(err).NotTo(HaveOccurred())

					Expect(task.State).To(Equal(models.Task_Completed))

					return true
				}
			})

			It("completes the task with the result", func() {
				task, err := bbsClient.TaskByGuid(logger, taskGuid)
				Expect(err).NotTo(HaveOccurred())

				Expect(task.Failed).To(BeFalse())

				_, guid, filename := containerDelegate.FetchContainerResultFileArgsForCall(0)
				Expect(guid).To(Equal(taskGuid))
				Expect(filename).To(Equal("some-result-filename"))
				Expect(task.Result).To(Equal("some-result"))
			})

			itDeletesTheContainer(logger)
		})

		Context("when fetching the result fails", func() {
			disaster := errors.New("nope")

			BeforeEach(func() {
				containerDelegate.FetchContainerResultFileReturns("", disaster)
			})

			itCompletesTheTaskWithFailure("failed to fetch result")(logger)

			itDeletesTheContainer(logger)
		})
	}

	failedRunResult := executor.ContainerRunResult{
		Failed:        true,
		FailureReason: "because",
	}

	itCompletesTheFailedTaskAndDeletesTheContainer := func(logger *lagertest.TestLogger) {
		It("does not attempt to fetch the result", func() {
			Expect(containerDelegate.FetchContainerResultFileCallCount()).To(BeZero())
		})

		itCompletesTheTaskWithFailure("because")(logger)

		itDeletesTheContainer(logger)
	}

	itSetsTheTaskToRunning := func(logger *lagertest.TestLogger) {
		It("transitions the task to the running state", func() {
			task, err := bbsClient.TaskByGuid(logger, taskGuid)
			Expect(err).NotTo(HaveOccurred())

			Expect(task.State).To(Equal(models.Task_Running))
		})
	}

	itRunsTheContainer := func(logger *lagertest.TestLogger) {
		itSetsTheTaskToRunning(logger)

		It("runs the container", func() {
			Expect(containerDelegate.RunContainerCallCount()).To(Equal(1))

			task, err := bbsClient.TaskByGuid(logger, taskGuid)
			Expect(err).NotTo(HaveOccurred())

			expectedRunRequest, err := rep.NewRunRequestFromTask(task)
			Expect(err).NotTo(HaveOccurred())

			_, runRequest := containerDelegate.RunContainerArgsForCall(0)
			Expect(*runRequest).To(Equal(expectedRunRequest))
		})

		Context("when running the container fails", func() {
			BeforeEach(func() {
				containerDelegate.RunContainerReturns(false)
			})

			itCompletesTheTaskWithFailure("failed to run container")(logger)
		})
	}

	itDoesNothing := func(logger *lagertest.TestLogger) {
		It("does not run the container", func() {
			Expect(containerDelegate.RunContainerCallCount()).To(Equal(0))
		})

		It("does not stop the container", func() {
			Expect(containerDelegate.StopContainerCallCount()).To(Equal(0))
		})

		It("does not delete the container", func() {
			Expect(containerDelegate.DeleteContainerCallCount()).To(Equal(0))
		})
	}

	table := TaskTable{
		LocalCellID: localCellID,
		Logger:      lagertest.NewTestLogger(sessionPrefix),
		Rows: []Row{
			// container reserved
			ConceivableTaskScenario( // task deleted? (operator/etcd?)
				NewContainer(taskGuid, executor.StateReserved),
				nil,
				itDeletesTheContainer,
			),
			ExpectedTaskScenario( // container is reserved for a pending container
				NewContainer(taskGuid, executor.StateReserved),
				NewTask(taskGuid, "", models.Task_Pending),
				itRunsTheContainer,
			),
			ExpectedTaskScenario( // task is started before we run the container. it should eventually transition to initializing or be reaped if things really go wrong.
				NewContainer(taskGuid, executor.StateReserved),
				NewTask(taskGuid, "a", models.Task_Running),
				itDoesNothing,
			),
			ConceivableTaskScenario( // maybe the rep reserved the container and failed to report success back to the auctioneer
				NewContainer(taskGuid, executor.StateReserved),
				NewTask(taskGuid, "w", models.Task_Running),
				itDeletesTheContainer,
			),
			ConceivableTaskScenario( // if the Run call to the executor fails we complete the task with failure, and try to remove the reservation, but there's a time window.
				NewContainer(taskGuid, executor.StateReserved),
				NewTask(taskGuid, "a", models.Task_Completed),
				itDeletesTheContainer,
			),
			ConceivableTaskScenario( // maybe the rep reserved the container and failed to report success back to the auctioneer
				NewContainer(taskGuid, executor.StateReserved),
				NewTask(taskGuid, "w", models.Task_Completed),
				itDeletesTheContainer,
			),
			ConceivableTaskScenario( // caller is processing failure from Run call
				NewContainer(taskGuid, executor.StateReserved),
				NewTask(taskGuid, "a", models.Task_Resolving),
				itDeletesTheContainer,
			),
			ConceivableTaskScenario( // maybe the rep reserved the container and failed to report success back to the auctioneer
				NewContainer(taskGuid, executor.StateReserved),
				NewTask(taskGuid, "w", models.Task_Resolving),
				itDeletesTheContainer,
			),

			// container initializing
			ConceivableTaskScenario( // task deleted? (operator/etcd?)
				NewContainer(taskGuid, executor.StateInitializing),
				nil,
				itDeletesTheContainer,
			),
			InconceivableTaskScenario( // task should be started before anyone tries to run
				NewContainer(taskGuid, executor.StateInitializing),
				NewTask(taskGuid, "", models.Task_Pending),
				itRunsTheContainer,
			),
			ExpectedTaskScenario( // task is running throughout initializing, completed, and running
				NewContainer(taskGuid, executor.StateInitializing),
				NewTask(taskGuid, "a", models.Task_Running),
				itDoesNothing,
			),
			InconceivableTaskScenario( // state machine borked? no other cell should get this far.
				NewContainer(taskGuid, executor.StateInitializing),
				NewTask(taskGuid, "w", models.Task_Running),
				itDeletesTheContainer,
			),
			ConceivableTaskScenario( // task was cancelled
				NewContainer(taskGuid, executor.StateInitializing),
				NewTask(taskGuid, "a", models.Task_Completed),
				itDeletesTheContainer,
			),
			InconceivableTaskScenario( // state machine borked? no other cell should get this far.
				NewContainer(taskGuid, executor.StateInitializing),
				NewTask(taskGuid, "w", models.Task_Completed),
				itDeletesTheContainer,
			),
			ConceivableTaskScenario( // task was cancelled
				NewContainer(taskGuid, executor.StateInitializing),
				NewTask(taskGuid, "a", models.Task_Resolving),
				itDeletesTheContainer,
			),
			InconceivableTaskScenario( // state machine borked? no other cell should get this far.
				NewContainer(taskGuid, executor.StateInitializing),
				NewTask(taskGuid, "w", models.Task_Resolving),
				itDeletesTheContainer,
			),

			// container created
			ConceivableTaskScenario( // task deleted? (operator/etcd?)
				NewContainer(taskGuid, executor.StateCreated),
				nil,
				itDeletesTheContainer,
			),
			InconceivableTaskScenario( // task should be started before anyone tries to run
				NewContainer(taskGuid, executor.StateCreated),
				NewTask(taskGuid, "", models.Task_Pending),
				itSetsTheTaskToRunning,
			),
			ExpectedTaskScenario( // task is running throughout initializing, completed, and running
				NewContainer(taskGuid, executor.StateCreated),
				NewTask(taskGuid, "a", models.Task_Running),
				itDoesNothing,
			),
			InconceivableTaskScenario( // state machine borked? no other cell should get this far.
				NewContainer(taskGuid, executor.StateCreated),
				NewTask(taskGuid, "w", models.Task_Running),
				itDeletesTheContainer,
			),
			ConceivableTaskScenario( // task was cancelled
				NewContainer(taskGuid, executor.StateCreated),
				NewTask(taskGuid, "a", models.Task_Completed),
				itDeletesTheContainer,
			),
			InconceivableTaskScenario( // state machine borked? no other cell should get this far.
				NewContainer(taskGuid, executor.StateCreated),
				NewTask(taskGuid, "w", models.Task_Completed),
				itDeletesTheContainer,
			),
			ConceivableTaskScenario( // task was cancelled
				NewContainer(taskGuid, executor.StateCreated),
				NewTask(taskGuid, "a", models.Task_Resolving),
				itDeletesTheContainer,
			),
			InconceivableTaskScenario( // state machine borked? no other cell should get this far.
				NewContainer(taskGuid, executor.StateCreated),
				NewTask(taskGuid, "w", models.Task_Resolving),
				itDeletesTheContainer,
			),

			// container running
			ConceivableTaskScenario( // task deleted? (operator/etcd?)
				NewContainer(taskGuid, executor.StateRunning),
				nil,
				itDeletesTheContainer,
			),
			InconceivableTaskScenario( // task should be started before anyone tries to run
				NewContainer(taskGuid, executor.StateRunning),
				NewTask(taskGuid, "", models.Task_Pending),
				itSetsTheTaskToRunning,
			),
			ExpectedTaskScenario( // task is running throughout initializing, completed, and running
				NewContainer(taskGuid, executor.StateRunning),
				NewTask(taskGuid, "a", models.Task_Running),
				itDoesNothing,
			),
			InconceivableTaskScenario( // state machine borked? no other cell should get this far.
				NewContainer(taskGuid, executor.StateRunning),
				NewTask(taskGuid, "w", models.Task_Running),
				itDeletesTheContainer,
			),
			ConceivableTaskScenario( // task was cancelled
				NewContainer(taskGuid, executor.StateRunning),
				NewTask(taskGuid, "a", models.Task_Completed),
				itDeletesTheContainer,
			),
			InconceivableTaskScenario( // state machine borked? no other cell should get this far.
				NewContainer(taskGuid, executor.StateRunning),
				NewTask(taskGuid, "w", models.Task_Completed),
				itDeletesTheContainer,
			),
			ConceivableTaskScenario( // task was cancelled
				NewContainer(taskGuid, executor.StateRunning),
				NewTask(taskGuid, "a", models.Task_Resolving),
				itDeletesTheContainer,
			),
			InconceivableTaskScenario( // state machine borked? no other cell should get this far.
				NewContainer(taskGuid, executor.StateRunning),
				NewTask(taskGuid, "w", models.Task_Resolving),
				itDeletesTheContainer,
			),

			// container completed
			ConceivableTaskScenario( // task deleted? (operator/etcd?)
				NewCompletedContainer(taskGuid, failedRunResult),
				nil,
				itDeletesTheContainer,
			),
			InconceivableTaskScenario( // task should be walked through lifecycle by the time we get here
				NewCompletedContainer(taskGuid, failedRunResult),
				NewTask(taskGuid, "", models.Task_Pending),
				itCompletesTheTaskWithFailure("invalid state transition"),
			),
			ExpectedTaskScenario( // container completed and failed; complete the task with its failure reason
				NewCompletedContainer(taskGuid, failedRunResult),
				NewTask(taskGuid, "a", models.Task_Running),
				itCompletesTheFailedTaskAndDeletesTheContainer,
			),
			ExpectedTaskScenario( // container completed and succeeded; complete the task with its result
				NewCompletedContainer(taskGuid, successfulRunResult),
				NewTask(taskGuid, "a", models.Task_Running),
				itCompletesTheSuccessfulTaskAndDeletesTheContainer,
			),
			InconceivableTaskScenario( // state machine borked? no other cell should get this far.
				NewCompletedContainer(taskGuid, failedRunResult),
				NewTask(taskGuid, "w", models.Task_Running),
				itDeletesTheContainer,
			),
			ConceivableTaskScenario( // may have completed the task and then failed to delete the container
				NewCompletedContainer(taskGuid, failedRunResult),
				NewTask(taskGuid, "a", models.Task_Completed),
				itDeletesTheContainer,
			),
			InconceivableTaskScenario( // state machine borked? no other cell should get this far.
				NewCompletedContainer(taskGuid, failedRunResult),
				NewTask(taskGuid, "w", models.Task_Completed),
				itDeletesTheContainer,
			),
			ConceivableTaskScenario( // may have completed the task and then failed to delete the container, and someone started processing the completion
				NewCompletedContainer(taskGuid, failedRunResult),
				NewTask(taskGuid, "a", models.Task_Resolving),
				itDeletesTheContainer,
			),
			InconceivableTaskScenario( // state machine borked? no other cell should get this far.
				NewCompletedContainer(taskGuid, failedRunResult),
				NewTask(taskGuid, "w", models.Task_Resolving),
				itDeletesTheContainer,
			),
		},
	}

	table.Test()
})

type TaskTable struct {
	LocalCellID string
	Logger      *lagertest.TestLogger
	Rows        []Row
}

func (t *TaskTable) Test() {
	for _, row := range t.Rows {
		row := row

		Context(row.ContextDescription(), func() {
			row.Test(t.Logger)
		})
	}
}

type Row interface {
	ContextDescription() string
	Test(*lagertest.TestLogger)
}

type TaskTest func(*lagertest.TestLogger)

type TaskRow struct {
	Container executor.Container
	Task      *models.Task
	TestFunc  TaskTest
}

func (t TaskRow) Test(logger *lagertest.TestLogger) {
	BeforeEach(func() {
		if t.Task != nil {
			walkToState(logger, t.Task)
		}
	})

	JustBeforeEach(func() {
		processor.Process(logger, t.Container)
	})

	t.TestFunc(logger)
}

func (t TaskRow) ContextDescription() string {
	return "when the container is " + t.containerDescription() + " and the task is " + t.taskDescription()
}

func (t TaskRow) containerDescription() string {
	return string(t.Container.State)
}

func (t TaskRow) taskDescription() string {
	if t.Task == nil {
		return "missing"
	}

	msg := t.Task.State.String()
	if t.Task.CellId != "" {
		msg += " on '" + t.Task.CellId + "'"
	}

	return msg
}

func ExpectedTaskScenario(container executor.Container, task *models.Task, test TaskTest) Row {
	expectedTest := func(logger *lagertest.TestLogger) {
		test(logger)
	}

	return TaskRow{container, task, TaskTest(expectedTest)}
}

func ConceivableTaskScenario(container executor.Container, task *models.Task, test TaskTest) Row {
	conceivableTest := func(logger *lagertest.TestLogger) {
		test(logger)
	}

	return TaskRow{container, task, TaskTest(conceivableTest)}
}

func InconceivableTaskScenario(container executor.Container, task *models.Task, test TaskTest) Row {
	inconceivableTest := func(logger *lagertest.TestLogger) {
		test(logger)
	}

	return TaskRow{container, task, TaskTest(inconceivableTest)}
}

func NewContainer(taskGuid string, containerState executor.State) executor.Container {
	return executor.Container{
		Guid:  taskGuid,
		State: containerState,
		Tags: executor.Tags{
			rep.ResultFileTag: "some-result-filename",
		},
	}
}

func NewCompletedContainer(taskGuid string, runResult executor.ContainerRunResult) executor.Container {
	container := NewContainer(taskGuid, executor.StateCompleted)
	container.RunResult = runResult
	return container
}

func NewTask(taskGuid, cellID string, taskState models.Task_State) *models.Task {
	task := model_helpers.NewValidTask(taskGuid)
	task.CellId = cellID
	task.State = taskState
	return task
}

func walkToState(logger lager.Logger, task *models.Task) {
	var currentState models.Task_State
	desiredState := task.State
	for desiredState != currentState {
		currentState = advanceState(logger, task, currentState)
	}
}

func advanceState(logger lager.Logger, task *models.Task, currentState models.Task_State) models.Task_State {
	switch currentState {
	case models.Task_Invalid:
		err := bbsClient.DesireTask(logger, task.TaskGuid, task.Domain, task.TaskDefinition)
		Expect(err).NotTo(HaveOccurred())
		return models.Task_Pending

	case models.Task_Pending:
		changed, err := bbsClient.StartTask(logger, task.TaskGuid, task.CellId)
		Expect(err).NotTo(HaveOccurred())
		Expect(changed).To(BeTrue())
		return models.Task_Running

	case models.Task_Running:
		err := bbsClient.CompleteTask(logger, task.TaskGuid, task.CellId, true, "reason", "result")
		Expect(err).NotTo(HaveOccurred())
		return models.Task_Completed

	case models.Task_Completed:
		err := bbsClient.ResolvingTask(logger, task.TaskGuid)
		Expect(err).NotTo(HaveOccurred())
		return models.Task_Resolving

	default:
		panic("not a thing.")
	}
}
