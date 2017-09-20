package auctionmetricemitterdelegate

import (
	"time"

	"code.cloudfoundry.org/auction/auctiontypes"
	"code.cloudfoundry.org/auctioneer"
)

type auctionMetricEmitterDelegate struct{}

func New() auctionMetricEmitterDelegate {
	return auctionMetricEmitterDelegate{}
}

func (_ auctionMetricEmitterDelegate) FetchStatesCompleted(fetchStatesDuration time.Duration) error {
	return auctioneer.FetchStatesDuration.Send(fetchStatesDuration)
}

func (_ auctionMetricEmitterDelegate) FailedCellStateRequest() {
	auctioneer.FailedCellStateRequests.Increment()
}

func (_ auctionMetricEmitterDelegate) AuctionCompleted(results auctiontypes.AuctionResults) {
	auctioneer.LRPAuctionsStarted.Add(uint64(len(results.SuccessfulLRPs)))
	auctioneer.TaskAuctionsStarted.Add(uint64(len(results.SuccessfulTasks)))

	auctioneer.LRPAuctionsFailed.Add(uint64(len(results.FailedLRPs)))
	auctioneer.TaskAuctionsFailed.Add(uint64(len(results.FailedTasks)))
}
