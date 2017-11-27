package vizzini_test

import (
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Freshness", func() {
	Describe("Creating a fresh domain", func() {
		Context("with no TTL", func() {
			It("should create a fresh domain that never disappears", func() {
				Expect(bbsClient.UpsertDomain(logger, domain, 0)).To(Succeed())
				Consistently(func() ([]string, error) {
					return bbsClient.Domains(logger)
				}, 3).Should(ContainElement(domain))
				bbsClient.UpsertDomain(logger, domain, 1*time.Second) //to clear it out
			})
		})

		Context("with a TTL", func() {
			It("should create a fresh domain that eventually disappears", func() {
				Expect(bbsClient.UpsertDomain(logger, domain, 2*time.Second)).To(Succeed())

				Expect(bbsClient.Domains(logger)).To(ContainElement(domain))
				Eventually(func() ([]string, error) {
					return bbsClient.Domains(logger)
				}, 5).ShouldNot(ContainElement(domain))
			})
		})

		Context("with no domain", func() {
			It("should error", func() {
				Expect(bbsClient.UpsertDomain(logger, "", 0)).NotTo(Succeed())
			})
		})
	})
})
