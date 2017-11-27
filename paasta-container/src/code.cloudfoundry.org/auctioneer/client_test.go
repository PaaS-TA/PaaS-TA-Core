package auctioneer_test

import (
	. "code.cloudfoundry.org/auctioneer"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Auctioneer Client", func() {
	Describe("NewSecureClient", func() {
		var caFile, certFile, keyFile, auctioneerURL string

		BeforeEach(func() {
			auctioneerURL = "http://jim.jim.jim"
			caFile = "cmd/auctioneer/fixtures/blue-certs/ca.crt"
			certFile = "cmd/auctioneer/fixtures/blue-certs/client.crt"
			keyFile = "cmd/auctioneer/fixtures/blue-certs/client.key"
		})

		It("works", func() {
			_, err := NewSecureClient(auctioneerURL, caFile, certFile, keyFile, false)
			Expect(err).NotTo(HaveOccurred())
		})

		Context("when the tls config is invalid", func() {
			BeforeEach(func() {
				certFile = "cmd/auctioneer/fixtures/green-certs/client.crt"
			})

			It("returns an error", func() {
				_, err := NewSecureClient(auctioneerURL, caFile, certFile, keyFile, false)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(MatchRegexp("failed to load keypair.*"))
			})
		})
	})
})
