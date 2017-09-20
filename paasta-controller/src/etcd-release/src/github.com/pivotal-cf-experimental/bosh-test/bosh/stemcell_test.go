package bosh_test

import (
	"github.com/pivotal-cf-experimental/bosh-test/bosh"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Stemcell", func() {
	Context("Latest", func() {
		It("should return the latest stemcell available", func() {
			stemcell := bosh.NewStemcell()
			stemcell.Versions = []string{
				"2127",
				"3147",
				"389",
				"3126",
			}

			version, err := stemcell.Latest()
			Expect(err).NotTo(HaveOccurred())
			Expect(version).To(Equal("3147"))
		})

		It("should handle no installed stemcells", func() {
			stemcell := bosh.NewStemcell()
			stemcell.Versions = []string{}

			_, err := stemcell.Latest()
			Expect(err).To(MatchError("no stemcell versions found, cannot get latest"))
		})
	})
})
