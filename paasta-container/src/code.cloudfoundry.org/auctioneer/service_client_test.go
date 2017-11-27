package auctioneer_test

import (
	"time"

	"code.cloudfoundry.org/lager/lagertest"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/tedsuo/ifrit"
	"github.com/tedsuo/ifrit/ginkgomon"

	"code.cloudfoundry.org/auctioneer"
	"code.cloudfoundry.org/clock/fakeclock"
)

var _ = Describe("ServiceClient", func() {
	var serviceClient auctioneer.ServiceClient

	var clock *fakeclock.FakeClock
	var logger *lagertest.TestLogger

	BeforeEach(func() {
		clock = fakeclock.NewFakeClock(time.Now())
		logger = lagertest.NewTestLogger("test")

		consulClient := consulRunner.NewClient()
		serviceClient = auctioneer.NewServiceClient(consulClient, clock)
	})

	Describe("AuctioneerAddress", func() {
		Context("when able to get an auctioneer presence", func() {
			var heartbeater ifrit.Process
			var presence auctioneer.Presence

			BeforeEach(func() {
				presence = auctioneer.NewPresence("auctioneer-id", "auctioneer.example.com")

				auctioneerLock, err := serviceClient.NewAuctioneerLockRunner(logger, presence, 100*time.Millisecond, 10*time.Second)
				Expect(err).NotTo(HaveOccurred())
				heartbeater = ginkgomon.Invoke(auctioneerLock)
			})

			AfterEach(func() {
				ginkgomon.Interrupt(heartbeater)
			})

			It("returns the address", func() {
				address, err := serviceClient.CurrentAuctioneerAddress()
				Expect(err).NotTo(HaveOccurred())
				Expect(address).To(Equal(presence.AuctioneerAddress))
			})
		})

		Context("when unable to get any auctioneer presences", func() {
			It("returns ErrServiceUnavailable", func() {
				_, err := serviceClient.CurrentAuctioneerAddress()
				Expect(err).To(HaveOccurred())
			})
		})
	})
})
