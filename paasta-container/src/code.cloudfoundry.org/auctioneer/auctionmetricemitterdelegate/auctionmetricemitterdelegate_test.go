package auctionmetricemitterdelegate_test

import (
	"time"

	"code.cloudfoundry.org/auction/auctiontypes"
	"code.cloudfoundry.org/auctioneer/auctionmetricemitterdelegate"
	"code.cloudfoundry.org/bbs/models"
	"code.cloudfoundry.org/rep"
	"github.com/cloudfoundry/dropsonde/metric_sender/fake"
	"github.com/cloudfoundry/dropsonde/metrics"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Auction Metric Emitter Delegate", func() {
	var delegate auctiontypes.AuctionMetricEmitterDelegate
	var metricSender *fake.FakeMetricSender

	BeforeEach(func() {
		metricSender = fake.NewFakeMetricSender()
		metrics.Initialize(metricSender, nil)

		delegate = auctionmetricemitterdelegate.New()
	})

	Describe("AuctionCompleted", func() {
		It("should adjust the metric counters", func() {
			resource := rep.NewResource(10, 10)
			pc := rep.NewPlacementConstraint("linux", []string{}, []string{})
			delegate.AuctionCompleted(auctiontypes.AuctionResults{
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
						AuctionRecord: auctiontypes.AuctionRecord{PlacementError: "insufficient resources"},
					},
				},
				FailedTasks: []auctiontypes.TaskAuction{
					{
						Task:          rep.NewTask("failed-task", "domain", resource, pc),
						AuctionRecord: auctiontypes.AuctionRecord{PlacementError: "insufficient resources"},
					},
				},
			})

			Expect(metricSender.GetCounter("AuctioneerLRPAuctionsStarted")).To(BeNumerically("==", 1))
			Expect(metricSender.GetCounter("AuctioneerTaskAuctionsStarted")).To(BeNumerically("==", 1))
			Expect(metricSender.GetCounter("AuctioneerLRPAuctionsFailed")).To(BeNumerically("==", 2))
			Expect(metricSender.GetCounter("AuctioneerTaskAuctionsFailed")).To(BeNumerically("==", 1))
		})
	})

	Describe("FetchStatesCompleted", func() {
		It("should adjust the metric counters", func() {
			err := delegate.FetchStatesCompleted(1 * time.Second)
			Expect(err).NotTo(HaveOccurred())

			sentMetric := metricSender.GetValue("AuctioneerFetchStatesDuration")
			Expect(sentMetric.Value).To(Equal(1e+09))
			Expect(sentMetric.Unit).To(Equal("nanos"))
		})
	})
})
