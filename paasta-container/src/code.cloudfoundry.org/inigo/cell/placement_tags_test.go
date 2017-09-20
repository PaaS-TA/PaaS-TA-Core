package cell_test

import (
	"os"
	"path/filepath"
	"time"

	archive_helper "code.cloudfoundry.org/archiver/extractor/test_helper"
	"code.cloudfoundry.org/bbs/models"
	"code.cloudfoundry.org/inigo/fixtures"
	"code.cloudfoundry.org/inigo/helpers"
	"code.cloudfoundry.org/lager"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/tedsuo/ifrit"
	"github.com/tedsuo/ifrit/ginkgomon"
	"github.com/tedsuo/ifrit/grouper"
)

var _ = Describe("Placement Tags", func() {
	var (
		processGuid string
		runtime     ifrit.Process
	)

	BeforeEach(func() {
		processGuid = helpers.GenerateGuid()

		var fileServer ifrit.Runner
		fileServer, fileServerStaticDir := componentMaker.FileServer()
		runtime = ginkgomon.Invoke(grouper.NewParallel(os.Kill, grouper.Members{
			{"file-server", fileServer},
			{"rep-with-tag", componentMaker.Rep("-placementTag=inigo-tag", "-optionalPlacementTag=inigo-optional-tag")},
			{"auctioneer", componentMaker.Auctioneer()},
		}))

		archive_helper.CreateZipArchive(
			filepath.Join(fileServerStaticDir, "lrp.zip"),
			fixtures.GoServerApp(),
		)
	})

	AfterEach(func() {
		helpers.StopProcesses(runtime)
	})

	It("advertises placement tags in the cell presence", func() {
		presences, err := bbsClient.Cells(logger)
		Expect(err).NotTo(HaveOccurred())

		Expect(presences).To(HaveLen(1))
		Expect(presences[0].PlacementTags).To(Equal([]string{"inigo-tag"}))
	})

	It("advertises optional placement tags in the cell presence", func() {
		presences, err := bbsClient.Cells(logger)
		Expect(err).NotTo(HaveOccurred())

		Expect(presences).To(HaveLen(1))
		Expect(presences[0].OptionalPlacementTags).To(Equal([]string{"inigo-optional-tag"}))
	})

	Describe("desired lrps", func() {
		var lrp *models.DesiredLRP

		JustBeforeEach(func() {
			err := bbsClient.DesireLRP(logger, lrp)
			Expect(err).NotTo(HaveOccurred())
		})

		Context("when the desired LRP matches the required tags", func() {
			BeforeEach(func() {
				lrp = helpers.LRPCreateRequestWithPlacementTag(processGuid, []string{"inigo-tag"})
			})

			It("succeeds and is running on correct cell", func() {
				lrpFunc := func() string {
					lrpGroups, err := bbsClient.ActualLRPGroupsByProcessGuid(logger, processGuid)
					Expect(err).NotTo(HaveOccurred())
					if len(lrpGroups) == 0 {
						return ""
					}
					lrp, _ := lrpGroups[0].Resolve()

					return lrp.CellId
				}
				Eventually(lrpFunc).Should(MatchRegexp("the-cell-id-.*-0"))
			})
		})

		Context("when the desired LRP matches the required  and optional tags", func() {
			BeforeEach(func() {
				lrp = helpers.LRPCreateRequestWithPlacementTag(processGuid, []string{"inigo-tag", "inigo-optional-tag"})
			})

			It("succeeds and is running on correct cell", func() {
				lrpFunc := func() string {
					lrpGroups, err := bbsClient.ActualLRPGroupsByProcessGuid(logger, processGuid)
					Expect(err).NotTo(HaveOccurred())
					if len(lrpGroups) == 0 {
						return ""
					}
					lrp, _ := lrpGroups[0].Resolve()

					return lrp.CellId
				}
				Eventually(lrpFunc).Should(MatchRegexp("the-cell-id-.*-0"))
			})
		})

		Context("when no cells are advertising the placement tags", func() {
			BeforeEach(func() {
				lrp = helpers.LRPCreateRequestWithPlacementTag(processGuid, []string{""})
			})

			It("fails and sets a placement error", func() {
				lrpFunc := func() string {
					lrpGroups, err := bbsClient.ActualLRPGroupsByProcessGuid(logger, processGuid)
					Expect(err).NotTo(HaveOccurred())
					if len(lrpGroups) == 0 {
						return ""
					}
					lrp, _ := lrpGroups[0].Resolve()
					logger.Info("lrp-cell-id", lager.Data{"cell-id": lrp.CellId})

					return lrp.PlacementError
				}

				Eventually(lrpFunc).Should(ContainSubstring("found no compatible cell with placement tag"))
			})
		})
	})

	Describe("tasks", func() {
		var (
			task   *models.Task
			action models.ActionInterface
		)

		BeforeEach(func() {
			action = models.Serial(
				models.Timeout(
					&models.RunAction{
						User: "vcap",
						Path: "sh",
						Args: []string{
							"-c",
							`
									kill_sleep() {
										kill -15 $child
										exit
									}

									trap kill_sleep 15 9

									sleep 1 &

									child=$!
									wait $child
									`,
						},
					},
					500*time.Millisecond,
				),
			)

		})

		JustBeforeEach(func() {
			err := bbsClient.DesireTask(logger, task.TaskGuid, task.Domain, task.TaskDefinition)
			Expect(err).NotTo(HaveOccurred())
		})

		Context("when the task matches the required tags", func() {
			BeforeEach(func() {
				task = helpers.TaskCreateRequestWithTags("task-guid", action, []string{"inigo-tag"})
			})

			It("succeeds and is running on correct cell", func() {
				taskFunc := func() string {
					t, err := bbsClient.TaskByGuid(logger, task.TaskGuid)
					Expect(err).NotTo(HaveOccurred())
					return t.CellId
				}
				Eventually(taskFunc).Should(MatchRegexp("the-cell-id-.*-0"))
			})
		})

		Context("when the task matches the required and optional tags", func() {
			BeforeEach(func() {
				task = helpers.TaskCreateRequestWithTags("task-guid", action, []string{"inigo-tag", "inigo-optional-tag"})
			})

			It("succeeds and is running on correct cell", func() {
				taskFunc := func() string {
					t, err := bbsClient.TaskByGuid(logger, task.TaskGuid)
					Expect(err).NotTo(HaveOccurred())
					return t.CellId
				}
				Eventually(taskFunc).Should(MatchRegexp("the-cell-id-.*-0"))
			})
		})

		Context("when no cells are advertising the placement tags", func() {
			BeforeEach(func() {
				task = helpers.TaskCreateRequestWithTags("task-guid", action, []string{""})
			})

			It("fails and sets a placement error", func() {
				taskFunc := func() string {
					t, err := bbsClient.TaskByGuid(logger, task.TaskGuid)
					Expect(err).NotTo(HaveOccurred())
					return t.FailureReason
				}

				Eventually(taskFunc).Should(ContainSubstring("found no compatible cell with placement tag"))
			})
		})
	})
})
