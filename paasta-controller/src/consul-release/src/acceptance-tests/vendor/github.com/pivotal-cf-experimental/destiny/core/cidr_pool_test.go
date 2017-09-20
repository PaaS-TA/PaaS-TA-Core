package core_test

import (
	"github.com/pivotal-cf-experimental/destiny/core"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("CIDRPool", func() {
	var (
		cidrPool core.CIDRPool
	)

	Describe("Get", func() {
		It("returns a valid cidr block from the cidr pool", func() {
			cidrPool = core.NewCIDRPool("10.244.4.0", 24, 27)

			cidr, err := cidrPool.Get(0)
			Expect(err).NotTo(HaveOccurred())
			Expect(cidr).To(Equal("10.244.4.0/27"))

			cidr, err = cidrPool.Get(1)
			Expect(err).NotTo(HaveOccurred())
			Expect(cidr).To(Equal("10.244.4.32/27"))

			cidr, err = cidrPool.Get(2)
			Expect(err).NotTo(HaveOccurred())
			Expect(cidr).To(Equal("10.244.4.64/27"))

			cidr, err = cidrPool.Get(3)
			Expect(err).NotTo(HaveOccurred())
			Expect(cidr).To(Equal("10.244.4.96/27"))
		})

		Context("failure cases", func() {
			It("returns an error if it attempts to get with an index higher than the pool size", func() {
				cidrPool = core.NewCIDRPool("10.244.4.0", 24, 27)

				_, err := cidrPool.Get(9)
				Expect(err).To(MatchError("cannot get cidr of index 9 when pool size is size of 8"))
			})
		})
	})

	Describe("Last", func() {
		It("returns the last cidr block", func() {
			cidrPool = core.NewCIDRPool("10.244.4.0", 24, 27)

			cidr := cidrPool.Last()
			Expect(cidr).To(Equal("10.244.4.224/27"))
		})
	})
})
