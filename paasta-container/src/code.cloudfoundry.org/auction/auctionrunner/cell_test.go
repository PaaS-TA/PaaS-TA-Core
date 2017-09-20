package auctionrunner_test

import (
	"errors"

	"code.cloudfoundry.org/auction/auctionrunner"
	"code.cloudfoundry.org/bbs/models"
	"code.cloudfoundry.org/rep"
	"code.cloudfoundry.org/rep/repfakes"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Cell", func() {
	var (
		client          *repfakes.FakeSimClient
		emptyCell, cell *auctionrunner.Cell
	)

	BeforeEach(func() {
		client = &repfakes.FakeSimClient{}
		emptyState := BuildCellState("the-zone", 100, 200, 50, false, 0, linuxOnlyRootFSProviders, nil, []string{}, []string{}, []string{})
		emptyCell = auctionrunner.NewCell(logger, "empty-cell", client, emptyState)

		state := BuildCellState("the-zone", 100, 200, 50, false, 10, linuxOnlyRootFSProviders, []rep.LRP{
			*BuildLRP("pg-1", "domain", 0, linuxRootFSURL, 10, 20, []string{}),
			*BuildLRP("pg-1", "domain", 1, linuxRootFSURL, 10, 20, []string{}),
			*BuildLRP("pg-2", "domain", 0, linuxRootFSURL, 10, 20, []string{}),
			*BuildLRP("pg-3", "domain", 0, linuxRootFSURL, 10, 20, []string{}),
			*BuildLRP("pg-4", "domain", 0, linuxRootFSURL, 10, 20, []string{}),
		},
			[]string{},
			[]string{},
			[]string{},
		)
		cell = auctionrunner.NewCell(logger, "the-cell", client, state)
	})

	Describe("ScoreForLRP", func() {
		It("factors in memory usage", func() {
			bigInstance := BuildLRP("pg-big", "domain", 0, linuxRootFSURL, 20, 10, []string{})
			smallInstance := BuildLRP("pg-small", "domain", 0, linuxRootFSURL, 10, 10, []string{})

			By("factoring in the amount of memory taken up by the instance")
			bigScore, err := emptyCell.ScoreForLRP(bigInstance, 0.0)
			Expect(err).NotTo(HaveOccurred())
			smallScore, err := emptyCell.ScoreForLRP(smallInstance, 0.0)
			Expect(err).NotTo(HaveOccurred())

			Expect(smallScore).To(BeNumerically("<", bigScore))

			By("factoring in the relative emptiness of Cells")
			emptyScore, err := emptyCell.ScoreForLRP(smallInstance, 0.0)
			Expect(err).NotTo(HaveOccurred())
			score, err := cell.ScoreForLRP(smallInstance, 0.0)
			Expect(err).NotTo(HaveOccurred())
			Expect(emptyScore).To(BeNumerically("<", score))
		})

		It("factors in disk usage", func() {
			bigInstance := BuildLRP("pg-big", "domain", 0, linuxRootFSURL, 10, 20, []string{})
			smallInstance := BuildLRP("pg-small", "domain", 0, linuxRootFSURL, 10, 10, []string{})

			By("factoring in the amount of memory taken up by the instance")
			bigScore, err := emptyCell.ScoreForLRP(bigInstance, 0.0)
			Expect(err).NotTo(HaveOccurred())
			smallScore, err := emptyCell.ScoreForLRP(smallInstance, 0.0)
			Expect(err).NotTo(HaveOccurred())

			Expect(smallScore).To(BeNumerically("<", bigScore))

			By("factoring in the relative emptiness of Cells")
			emptyScore, err := emptyCell.ScoreForLRP(smallInstance, 0.0)
			Expect(err).NotTo(HaveOccurred())
			score, err := cell.ScoreForLRP(smallInstance, 0.0)
			Expect(err).NotTo(HaveOccurred())
			Expect(emptyScore).To(BeNumerically("<", score))
		})

		It("factors in container usage", func() {
			instance := BuildLRP("pg-big", "domain", 0, linuxRootFSURL, 20, 20, []string{})

			bigState := BuildCellState("the-zone", 100, 200, 50, false, 0, linuxOnlyRootFSProviders, nil, []string{}, []string{}, []string{})
			bigCell := auctionrunner.NewCell(logger, "big-cell", client, bigState)

			smallState := BuildCellState("the-zone", 100, 200, 20, false, 0, linuxOnlyRootFSProviders, nil, []string{}, []string{}, []string{})
			smallCell := auctionrunner.NewCell(logger, "small-cell", client, smallState)

			bigScore, err := bigCell.ScoreForLRP(instance, 0.0)
			Expect(err).NotTo(HaveOccurred())
			smallScore, err := smallCell.ScoreForLRP(instance, 0.0)
			Expect(err).NotTo(HaveOccurred())
			Expect(bigScore).To(BeNumerically("<", smallScore), "prefer Cells with more resources")
		})

		Context("Starting Containers", func() {
			var instance *rep.LRP
			var busyState, boredState rep.CellState
			var busyCell, boredCell *auctionrunner.Cell

			BeforeEach(func() {
				instance = BuildLRP("pg-busy", "domain", 0, linuxRootFSURL, 20, 20, []string{})

				busyState = BuildCellState(
					"the-zone",
					100,
					200,
					50,
					false,
					10,
					linuxOnlyRootFSProviders,
					[]rep.LRP{{ActualLRPKey: models.ActualLRPKey{ProcessGuid: "not-HA"}}},
					[]string{},
					[]string{},
					[]string{},
				)
				busyCell = auctionrunner.NewCell(logger, "busy-cell", client, busyState)

				boredState = BuildCellState(
					"the-zone",
					100,
					200,
					50,
					false,
					0,
					linuxOnlyRootFSProviders,
					[]rep.LRP{{ActualLRPKey: models.ActualLRPKey{ProcessGuid: "HA"}}},
					[]string{},
					[]string{},
					[]string{},
				)
				boredCell = auctionrunner.NewCell(logger, "bored-cell", client, boredState)
			})

			It("factors in starting containers when a weight is provided", func() {
				startingContainerWeight := 0.25

				busyScore, err := busyCell.ScoreForLRP(instance, startingContainerWeight)
				Expect(err).NotTo(HaveOccurred())
				boredScore, err := boredCell.ScoreForLRP(instance, startingContainerWeight)
				Expect(err).NotTo(HaveOccurred())

				Expect(busyScore).To(BeNumerically(">", boredScore), "prefer Cells that have less starting containers")

				smallerWeightState := BuildCellState(
					"the-zone",
					100,
					200,
					50,
					false,
					10,
					linuxOnlyRootFSProviders,
					nil,
					[]string{},
					[]string{},
					[]string{},
				)
				smallerWeightCell := auctionrunner.NewCell(logger, "busy-cell", client, smallerWeightState)
				smallerWeightScore, err := smallerWeightCell.ScoreForLRP(instance, startingContainerWeight-0.1)
				Expect(err).NotTo(HaveOccurred())

				Expect(busyScore).To(BeNumerically(">", smallerWeightScore), "the number of starting containers is weighted")
			})

			It("privileges spreading LRPs across cells over starting containers", func() {
				instance = BuildLRP("HA", "domain", 1, linuxRootFSURL, 20, 20, []string{})
				startingContainerWeight := 0.25

				busyScore, err := busyCell.ScoreForLRP(instance, startingContainerWeight)
				Expect(err).NotTo(HaveOccurred())
				boredScore, err := boredCell.ScoreForLRP(instance, startingContainerWeight)
				Expect(err).NotTo(HaveOccurred())

				Expect(busyScore).To(BeNumerically("<", boredScore), "prefer Cells that do not have an instance of self already running")
			})

			It("ignores starting containers when a weight is not provided", func() {
				startingContainerWeight := 0.0

				busyScore, err := busyCell.ScoreForLRP(instance, startingContainerWeight)
				Expect(err).NotTo(HaveOccurred())
				boredScore, err := boredCell.ScoreForLRP(instance, startingContainerWeight)
				Expect(err).NotTo(HaveOccurred())

				Expect(busyScore).To(BeNumerically("==", boredScore), "ignore how many starting Containers a cell has")
			})
		})

		It("factors in process-guids that are already present", func() {
			instanceWithTwoMatches := BuildLRP("pg-1", "domain", 2, linuxRootFSURL, 10, 10, []string{})
			instanceWithOneMatch := BuildLRP("pg-2", "domain", 1, linuxRootFSURL, 10, 10, []string{})
			instanceWithNoMatches := BuildLRP("pg-new", "domain", 0, linuxRootFSURL, 10, 10, []string{})

			twoMatchesScore, err := cell.ScoreForLRP(instanceWithTwoMatches, 0.0)
			Expect(err).NotTo(HaveOccurred())
			oneMatchesScore, err := cell.ScoreForLRP(instanceWithOneMatch, 0.0)
			Expect(err).NotTo(HaveOccurred())
			noMatchesScore, err := cell.ScoreForLRP(instanceWithNoMatches, 0.0)
			Expect(err).NotTo(HaveOccurred())

			Expect(noMatchesScore).To(BeNumerically("<", oneMatchesScore))
			Expect(oneMatchesScore).To(BeNumerically("<", twoMatchesScore))
		})

		Context("when the LRP does not fit", func() {
			Context("because of memory constraints", func() {
				It("should error", func() {
					massiveMemoryInstance := BuildLRP("pg-new", "domain", 0, linuxRootFSURL, 10000, 10, []string{})
					score, err := cell.ScoreForLRP(massiveMemoryInstance, 0.0)
					Expect(score).To(BeZero())
					Expect(err).To(MatchError("insufficient resources: memory"))
				})
			})

			Context("because of disk constraints", func() {
				It("should error", func() {
					massiveDiskInstance := BuildLRP("pg-new", "domain", 0, linuxRootFSURL, 10, 10000, []string{})
					score, err := cell.ScoreForLRP(massiveDiskInstance, 0.0)
					Expect(score).To(BeZero())
					Expect(err).To(MatchError("insufficient resources: disk"))
				})
			})

			Context("because of container constraints", func() {
				It("should error", func() {
					instance := BuildLRP("pg-new", "domain", 0, linuxRootFSURL, 10, 10, []string{})
					zeroState := BuildCellState("the-zone", 100, 100, 0, false, 0, linuxOnlyRootFSProviders, nil, []string{}, []string{}, []string{})
					zeroCell := auctionrunner.NewCell(logger, "zero-cell", client, zeroState)
					score, err := zeroCell.ScoreForLRP(instance, 0.0)
					Expect(score).To(BeZero())
					Expect(err).To(MatchError("insufficient resources: containers"))
				})
			})
		})
	})

	Describe("ScoreForTask", func() {
		It("factors in number of tasks currently running", func() {
			bigTask := BuildTask("tg-big", "domain", linuxRootFSURL, 20, 10, []string{}, []string{})
			smallTask := BuildTask("tg-small", "domain", linuxRootFSURL, 10, 10, []string{}, []string{})

			By("factoring in the amount of memory taken up by the task")
			bigScore, err := emptyCell.ScoreForTask(bigTask, 0.0)
			Expect(err).NotTo(HaveOccurred())
			smallScore, err := emptyCell.ScoreForTask(smallTask, 0.0)
			Expect(err).NotTo(HaveOccurred())

			Expect(smallScore).To(BeNumerically("<", bigScore))

			By("factoring in the relative emptiness of Cells")
			emptyScore, err := emptyCell.ScoreForTask(smallTask, 0.0)
			Expect(err).NotTo(HaveOccurred())
			score, err := cell.ScoreForTask(smallTask, 0.0)
			Expect(err).NotTo(HaveOccurred())
			Expect(emptyScore).To(BeNumerically("<", score))
		})

		It("factors in memory usage", func() {
			bigTask := BuildTask("tg-big", "domain", linuxRootFSURL, 20, 10, []string{}, []string{})
			smallTask := BuildTask("tg-small", "domain", linuxRootFSURL, 10, 10, []string{}, []string{})

			By("factoring in the amount of memory taken up by the task")
			bigScore, err := emptyCell.ScoreForTask(bigTask, 0.0)
			Expect(err).NotTo(HaveOccurred())
			smallScore, err := emptyCell.ScoreForTask(smallTask, 0.0)
			Expect(err).NotTo(HaveOccurred())

			Expect(smallScore).To(BeNumerically("<", bigScore))

			By("factoring in the relative emptiness of Cells")
			emptyScore, err := emptyCell.ScoreForTask(smallTask, 0.0)
			Expect(err).NotTo(HaveOccurred())
			score, err := cell.ScoreForTask(smallTask, 0.0)
			Expect(err).NotTo(HaveOccurred())
			Expect(emptyScore).To(BeNumerically("<", score))
		})

		It("factors in disk usage", func() {
			bigTask := BuildTask("tg-big", "domain", linuxRootFSURL, 10, 20, []string{}, []string{})
			smallTask := BuildTask("tg-small", "domain", linuxRootFSURL, 10, 10, []string{}, []string{})

			By("factoring in the amount of memory taken up by the task")
			bigScore, err := emptyCell.ScoreForTask(bigTask, 0.0)
			Expect(err).NotTo(HaveOccurred())
			smallScore, err := emptyCell.ScoreForTask(smallTask, 0.0)
			Expect(err).NotTo(HaveOccurred())

			Expect(smallScore).To(BeNumerically("<", bigScore))

			By("factoring in the relative emptiness of Cells")
			emptyScore, err := emptyCell.ScoreForTask(smallTask, 0.0)
			Expect(err).NotTo(HaveOccurred())
			score, err := cell.ScoreForTask(smallTask, 0.0)
			Expect(err).NotTo(HaveOccurred())
			Expect(emptyScore).To(BeNumerically("<", score))
		})

		It("factors in container usage", func() {
			task := BuildTask("tg-big", "domain", linuxRootFSURL, 20, 20, []string{}, []string{})

			bigState := BuildCellState("the-zone", 100, 200, 50, false, 0, linuxOnlyRootFSProviders, nil, []string{}, []string{}, []string{})
			bigCell := auctionrunner.NewCell(logger, "big-cell", client, bigState)

			smallState := BuildCellState("the-zone", 100, 200, 20, false, 0, linuxOnlyRootFSProviders, nil, []string{}, []string{}, []string{})
			smallCell := auctionrunner.NewCell(logger, "small-cell", client, smallState)

			bigScore, err := bigCell.ScoreForTask(task, 0.0)
			Expect(err).NotTo(HaveOccurred())
			smallScore, err := smallCell.ScoreForTask(task, 0.0)
			Expect(err).NotTo(HaveOccurred())
			Expect(bigScore).To(BeNumerically("<", smallScore), "prefer Cells with more resources")
		})

		Context("Starting Containers", func() {
			var task *rep.Task
			var busyState, boredState rep.CellState
			var busyCell, boredCell *auctionrunner.Cell

			BeforeEach(func() {
				task = BuildTask("tg-big", "domain", linuxRootFSURL, 20, 20, []string{}, []string{})

				busyState = BuildCellState("the-zone", 100, 200, 50, false, 10, linuxOnlyRootFSProviders, nil, []string{}, []string{}, []string{})
				busyCell = auctionrunner.NewCell(logger, "busy-cell", client, busyState)

				boredState = BuildCellState("the-zone", 100, 200, 50, false, 0, linuxOnlyRootFSProviders, nil, []string{}, []string{}, []string{})
				boredCell = auctionrunner.NewCell(logger, "bored-cell", client, boredState)
			})

			It("factors in starting containers when a weight is provided", func() {
				startingContainerWeight := 0.25
				busyScore, err := busyCell.ScoreForTask(task, startingContainerWeight)
				Expect(err).NotTo(HaveOccurred())
				boredScore, err := boredCell.ScoreForTask(task, startingContainerWeight)
				Expect(err).NotTo(HaveOccurred())
				Expect(busyScore).To(BeNumerically(">", boredScore), "prefer Cells that have less starting containers")
			})

			It("ignores starting containers when a weight is not provided", func() {
				startingContainerWeight := 0.0
				busyScore, err := busyCell.ScoreForTask(task, startingContainerWeight)
				Expect(err).NotTo(HaveOccurred())
				boredScore, err := boredCell.ScoreForTask(task, startingContainerWeight)
				Expect(err).NotTo(HaveOccurred())
				Expect(busyScore).To(BeNumerically("==", boredScore), "ignore how many starting Containers a cell has")
			})
		})

		Context("when the task does not fit", func() {
			Context("because of memory constraints", func() {
				It("should error", func() {
					massiveMemoryTask := BuildTask("pg-new", "domain", linuxRootFSURL, 10000, 10, []string{}, []string{})
					score, err := cell.ScoreForTask(massiveMemoryTask, 0.0)
					Expect(score).To(BeZero())
					Expect(err).To(MatchError("insufficient resources: memory"))
				})
			})

			Context("because of disk constraints", func() {
				It("should error", func() {
					massiveDiskTask := BuildTask("pg-new", "domain", linuxRootFSURL, 10, 10000, []string{}, []string{})
					score, err := cell.ScoreForTask(massiveDiskTask, 0.0)
					Expect(score).To(BeZero())
					Expect(err).To(MatchError("insufficient resources: disk"))
				})
			})

			Context("because of container constraints", func() {
				It("should error", func() {
					task := BuildTask("pg-new", "domain", linuxRootFSURL, 10, 10, []string{}, []string{})
					zeroState := BuildCellState("the-zone", 100, 100, 0, false, 0, linuxOnlyRootFSProviders, nil, []string{}, []string{}, []string{})
					zeroCell := auctionrunner.NewCell(logger, "zero-cell", client, zeroState)
					score, err := zeroCell.ScoreForTask(task, 0.0)
					Expect(score).To(BeZero())
					Expect(err).To(MatchError("insufficient resources: containers"))
				})
			})
		})
	})

	Describe("ReserveLRP", func() {
		Context("when there is room for the LRP", func() {
			It("should register its resources usage and keep it in mind when handling future requests", func() {
				instance := BuildLRP("pg-test", "domain", 0, linuxRootFSURL, 10, 10, []string{})
				instanceToAdd := BuildLRP("pg-new", "domain", 0, linuxRootFSURL, 10, 10, []string{})

				initialScore, err := cell.ScoreForLRP(instance, 0.0)
				Expect(err).NotTo(HaveOccurred())

				Expect(cell.ReserveLRP(instanceToAdd)).To(Succeed())

				subsequentScore, err := cell.ScoreForLRP(instance, 0.0)
				Expect(err).NotTo(HaveOccurred())
				Expect(initialScore).To(BeNumerically("<", subsequentScore), "the score should have gotten worse")
			})

			It("should register the LRP and keep it in mind when handling future requests", func() {
				instance := BuildLRP("pg-test", "domain", 0, linuxRootFSURL, 10, 10, []string{})
				instanceWithMatchingProcessGuid := BuildLRP("pg-new", "domain", 1, linuxRootFSURL, 10, 10, []string{})
				instanceToAdd := BuildLRP("pg-new", "domain", 0, linuxRootFSURL, 10, 10, []string{})

				initialScore, err := cell.ScoreForLRP(instance, 0.0)
				Expect(err).NotTo(HaveOccurred())

				initialScoreForInstanceWithMatchingProcessGuid, err := cell.ScoreForLRP(instanceWithMatchingProcessGuid, 0.0)
				Expect(err).NotTo(HaveOccurred())

				Expect(initialScore).To(BeNumerically("==", initialScoreForInstanceWithMatchingProcessGuid))

				Expect(cell.ReserveLRP(instanceToAdd)).To(Succeed())

				subsequentScore, err := cell.ScoreForLRP(instance, 0.0)
				Expect(err).NotTo(HaveOccurred())

				subsequentScoreForInstanceWithMatchingProcessGuid, err := cell.ScoreForLRP(instanceWithMatchingProcessGuid, 0.0)
				Expect(err).NotTo(HaveOccurred())

				Expect(initialScore).To(BeNumerically("<", subsequentScore), "the score should have gotten worse")
				Expect(initialScoreForInstanceWithMatchingProcessGuid).To(BeNumerically("<", subsequentScoreForInstanceWithMatchingProcessGuid), "the score should have gotten worse")

				Expect(subsequentScore).To(BeNumerically("<", subsequentScoreForInstanceWithMatchingProcessGuid), "the score should be substantially worse for the instance with the matching process guid")
			})
		})

		Context("when there is no room for the LRP", func() {
			It("should error", func() {
				instance := BuildLRP("pg-test", "domain", 0, linuxRootFSURL, 10000, 10, []string{})
				err := cell.ReserveLRP(instance)
				Expect(err).To(MatchError("insufficient resources: memory"))
			})
		})
	})

	Describe("ReserveTask", func() {
		Context("when there is room for the task", func() {
			It("should register its resources usage and keep it in mind when handling future requests", func() {
				task := BuildTask("tg-test", "domain", linuxRootFSURL, 10, 10, []string{}, []string{})
				taskToAdd := BuildTask("tg-new", "domain", linuxRootFSURL, 10, 10, []string{}, []string{})

				initialScore, err := cell.ScoreForTask(task, 0.0)
				Expect(err).NotTo(HaveOccurred())

				Expect(cell.ReserveTask(taskToAdd)).To(Succeed())

				subsequentScore, err := cell.ScoreForTask(task, 0.0)
				Expect(err).NotTo(HaveOccurred())
				Expect(initialScore).To(BeNumerically("<", subsequentScore), "the score should have gotten worse")
			})

			It("should register the Task and keep it in mind when handling future requests", func() {
				task := BuildTask("tg-test", "domain", linuxRootFSURL, 10, 10, []string{}, []string{})
				taskToAdd := BuildTask("tg-new", "domain", linuxRootFSURL, 10, 10, []string{}, []string{})

				initialScore, err := cell.ScoreForTask(task, 0.25)
				Expect(err).NotTo(HaveOccurred())

				initialScoreForTaskToAdd, err := cell.ScoreForTask(taskToAdd, 0.25)
				Expect(err).NotTo(HaveOccurred())

				Expect(initialScore).To(BeNumerically("==", initialScoreForTaskToAdd))

				Expect(cell.ReserveTask(taskToAdd)).To(Succeed())

				subsequentScore, err := cell.ScoreForTask(task, 0.25)
				Expect(err).NotTo(HaveOccurred())

				Expect(subsequentScore).To(BeNumerically(">", initialScore+auctionrunner.LocalityOffset), "the score should have gotten worse by at least 1")
			})
		})

		Context("when there is no room for the Task", func() {
			It("should error", func() {
				task := BuildTask("tg-test", "domain", linuxRootFSURL, 10000, 10, []string{}, []string{})
				err := cell.ReserveTask(task)
				Expect(err).To(MatchError("insufficient resources: memory"))
			})
		})
	})

	Describe("Commit", func() {
		Context("with nothing to commit", func() {
			It("does nothing and returns empty", func() {
				failedWork := cell.Commit()
				Expect(failedWork).To(BeZero())
				Expect(client.PerformCallCount()).To(Equal(0))
			})
		})

		Context("with work to commit", func() {
			var lrp rep.LRP

			BeforeEach(func() {
				lrp = *BuildLRP("pg-new", "domain", 0, linuxRootFSURL, 20, 10, []string{})
				Expect(cell.ReserveLRP(&lrp)).To(Succeed())
			})

			It("asks the client to perform", func() {
				cell.Commit()
				Expect(client.PerformCallCount()).To(Equal(1))
				_, work := client.PerformArgsForCall(0)
				Expect(work).To(Equal(rep.Work{LRPs: []rep.LRP{lrp}}))
			})

			Context("when the client returns some failed work", func() {
				It("forwards the failed work", func() {
					failedWork := rep.Work{
						LRPs: []rep.LRP{lrp},
					}
					client.PerformReturns(failedWork, nil)
					Expect(cell.Commit()).To(Equal(failedWork))
				})
			})

			Context("when the client returns an error", func() {
				It("does not return any failed work", func() {
					client.PerformReturns(rep.Work{}, errors.New("boom"))
					Expect(cell.Commit()).To(BeZero())
				})
			})
		})
	})
})
