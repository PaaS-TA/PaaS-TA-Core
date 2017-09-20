package simulation_test

import (
	"time"

	"code.cloudfoundry.org/auction/auctiontypes"
)

type auctionMetricEmitterDelegate struct{}

func NewAuctionMetricEmitterDelegate() auctionMetricEmitterDelegate {
	return auctionMetricEmitterDelegate{}
}

func (_ auctionMetricEmitterDelegate) FetchStatesCompleted(_ time.Duration) error {
	return nil
}

func (_ auctionMetricEmitterDelegate) FailedCellStateRequest() {}

func (_ auctionMetricEmitterDelegate) AuctionCompleted(_ auctiontypes.AuctionResults) {}
