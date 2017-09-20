package auctioneer

import "code.cloudfoundry.org/runtimeschema/metric"

const (
	LRPAuctionsStarted      = metric.Counter("AuctioneerLRPAuctionsStarted")
	LRPAuctionsFailed       = metric.Counter("AuctioneerLRPAuctionsFailed")
	TaskAuctionsStarted     = metric.Counter("AuctioneerTaskAuctionsStarted")
	TaskAuctionsFailed      = metric.Counter("AuctioneerTaskAuctionsFailed")
	FetchStatesDuration     = metric.Duration("AuctioneerFetchStatesDuration")
	FailedCellStateRequests = metric.Counter("AuctioneerFailedCellStateRequests")
)
