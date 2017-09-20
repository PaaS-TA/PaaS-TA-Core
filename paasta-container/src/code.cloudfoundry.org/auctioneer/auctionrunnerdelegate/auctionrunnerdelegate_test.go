package auctionrunnerdelegate_test

import (
	"errors"

	"code.cloudfoundry.org/auction/auctiontypes"
	"code.cloudfoundry.org/bbs/fake_bbs"
	"code.cloudfoundry.org/bbs/models"
	"code.cloudfoundry.org/rep"
	"code.cloudfoundry.org/rep/repfakes"
	"github.com/cloudfoundry/dropsonde/metric_sender/fake"
	"github.com/cloudfoundry/dropsonde/metrics"

	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/lager/lagertest"

	"code.cloudfoundry.org/auctioneer/auctionrunnerdelegate"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
)

var _ = Describe("Auction Runner Delegate", func() {
	var (
		delegate         *auctionrunnerdelegate.AuctionRunnerDelegate
		bbsClient        *fake_bbs.FakeInternalClient
		metricSender     *fake.FakeMetricSender
		repClientFactory *repfakes.FakeClientFactory
		repClient        *repfakes.FakeClient
		logger           lager.Logger
	)

	BeforeEach(func() {
		metricSender = fake.NewFakeMetricSender()
		metrics.Initialize(metricSender, nil)

		bbsClient = &fake_bbs.FakeInternalClient{}
		repClientFactory = &repfakes.FakeClientFactory{}
		repClient = &repfakes.FakeClient{}
		repClientFactory.CreateClientReturns(repClient, nil)
		logger = lagertest.NewTestLogger("delegate")

		delegate = auctionrunnerdelegate.New(repClientFactory, bbsClient, logger)
	})

	Describe("fetching cell reps", func() {
		Context("when the BSS succeeds", func() {
			BeforeEach(func() {
				cellPresence1 := models.NewCellPresence("cell-A", "cell-a.url", "", "zone-1", models.NewCellCapacity(123, 456, 789), []string{}, []string{}, []string{}, []string{})
				cellPresence2 := models.NewCellPresence("cell-B", "cell-b.url", "", "zone-1", models.NewCellCapacity(123, 456, 789), []string{}, []string{}, []string{}, []string{})
				cells := []*models.CellPresence{&cellPresence1, &cellPresence2}

				bbsClient.CellsReturns(cells, nil)
			})

			It("creates rep clients with the correct addresses", func() {
				_, err := delegate.FetchCellReps()
				Expect(err).NotTo(HaveOccurred())
				Expect(repClientFactory.CreateClientCallCount()).To(Equal(2))
				repAddr1, _ := repClientFactory.CreateClientArgsForCall(0)
				repAddr2, _ := repClientFactory.CreateClientArgsForCall(1)

				urls := []string{
					repAddr1,
					repAddr2,
				}
				Expect(urls).To(ConsistOf("cell-a.url", "cell-b.url"))
			})

			Context("when the rep has a url", func() {
				BeforeEach(func() {
					cellPresence := models.NewCellPresence("cell-A",
						"cell-a.url",
						"http://cell-a.url",
						"zone-1",
						models.NewCellCapacity(123,
							456,
							789),
						[]string{},
						[]string{},
						[]string{},
						[]string{},
					)
					cells := []*models.CellPresence{&cellPresence}
					bbsClient.CellsReturns(cells, nil)
				})

				It("creates rep clients with the correct addresses", func() {
					_, err := delegate.FetchCellReps()
					Expect(err).NotTo(HaveOccurred())
					Expect(repClientFactory.CreateClientCallCount()).To(Equal(1))
					repAddr, repURL := repClientFactory.CreateClientArgsForCall(0)

					urls := []string{
						repAddr,
						repURL,
					}
					Expect(urls).To(ConsistOf("cell-a.url", "http://cell-a.url"))
				})
			})

			It("returns correctly configured auction_http_clients", func() {
				reps, err := delegate.FetchCellReps()
				Expect(err).NotTo(HaveOccurred())
				Expect(reps).To(HaveLen(2))
				Expect(reps).To(HaveKey("cell-A"))
				Expect(reps).To(HaveKey("cell-B"))

				Expect(reps["cell-A"]).To(Equal(repClient))
				Expect(reps["cell-B"]).To(Equal(repClient))
			})

			Context("when creating a rep client fails", func() {
				var (
					reps map[string]rep.Client
					err  error
				)

				BeforeEach(func() {
					err = errors.New("BOOM!!!")
					cellPresence := models.NewCellPresence("cell-B",
						"cell-b.url",
						"",
						"zone-1",
						models.NewCellCapacity(123,
							456,
							789),
						[]string{},
						[]string{},
						[]string{},
						[]string{},
					)
					cells := []*models.CellPresence{&cellPresence}
					bbsClient.CellsReturns(cells, nil)
					repClientFactory.CreateClientReturns(nil, err)
					reps, err = delegate.FetchCellReps()
				})

				It("should log the error", func() {
					Expect(logger.(*lagertest.TestLogger).Buffer()).To(gbytes.Say("BOOM!!!"))
				})

				It("not return the client", func() {
					Expect(err).NotTo(HaveOccurred())
					Expect(reps).To(HaveLen(0))
				})
			})
		})

		Context("when the BBS errors", func() {
			BeforeEach(func() {
				bbsClient.CellsReturns(nil, errors.New("boom"))
			})

			It("should error", func() {
				cells, err := delegate.FetchCellReps()
				Expect(err).To(MatchError(errors.New("boom")))
				Expect(cells).To(BeEmpty())
			})
		})
	})

	Describe("when batches are distributed", func() {
		var results auctiontypes.AuctionResults

		BeforeEach(func() {
			resource := rep.NewResource(10, 10)
			pc := rep.NewPlacementConstraint("linux", []string{}, []string{})

			results = auctiontypes.AuctionResults{
				SuccessfulLRPs: []auctiontypes.LRPAuction{
					{
						LRP: rep.NewLRP(models.NewActualLRPKey("successful-start", 0, "domain"), resource, pc),
					},
				},
				SuccessfulTasks: []auctiontypes.TaskAuction{
					{
						Task: rep.NewTask("successful-task", "domain", resource, pc),
					},
				},
				FailedLRPs: []auctiontypes.LRPAuction{
					{
						LRP:           rep.NewLRP(models.NewActualLRPKey("insufficient-capacity", 0, "domain"), resource, pc),
						AuctionRecord: auctiontypes.AuctionRecord{PlacementError: "insufficient resources"},
					},
					{
						LRP:           rep.NewLRP(models.NewActualLRPKey("incompatible-stacks", 0, "domain"), resource, pc),
						AuctionRecord: auctiontypes.AuctionRecord{PlacementError: auctiontypes.ErrorCellMismatch.Error()},
					},
				},
				FailedTasks: []auctiontypes.TaskAuction{
					{
						Task:          rep.NewTask("failed-task", "domain", resource, pc),
						AuctionRecord: auctiontypes.AuctionRecord{PlacementError: "insufficient resources"},
					},
				},
			}

			delegate.AuctionCompleted(results)
		})

		It("should mark all failed tasks as COMPLETE with the appropriate failure reason", func() {
			Expect(bbsClient.FailTaskCallCount()).To(Equal(1))
			_, taskGuid, failureReason := bbsClient.FailTaskArgsForCall(0)
			Expect(taskGuid).To(Equal("failed-task"))
			Expect(failureReason).To(Equal("insufficient resources"))
		})

		It("should mark all failed LRPs as UNCLAIMED with the appropriate placement error", func() {
			Expect(bbsClient.FailActualLRPCallCount()).To(Equal(2))
			_, lrpKey, errorMessage := bbsClient.FailActualLRPArgsForCall(0)
			Expect(*lrpKey).To(Equal(models.NewActualLRPKey("insufficient-capacity", 0, "domain")))
			Expect(errorMessage).To(Equal("insufficient resources"))

			_, lrpKey1, errorMessage1 := bbsClient.FailActualLRPArgsForCall(1)
			Expect(*lrpKey1).To(Equal(models.NewActualLRPKey("incompatible-stacks", 0, "domain")))
			Expect(errorMessage1).To(Equal(auctiontypes.ErrorCellMismatch.Error()))
		})
	})
})
