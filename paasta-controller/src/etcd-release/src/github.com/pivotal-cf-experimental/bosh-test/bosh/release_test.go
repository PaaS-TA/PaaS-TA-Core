package bosh_test

import (
	"github.com/pivotal-cf-experimental/bosh-test/bosh"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("release", func() {
	Context("Latest", func() {
		It("should return the latest release available", func() {
			release := bosh.NewRelease()

			release.Versions = []string{
				"21+dev.2",
				"21+dev.3",
				"21+dev.4",
				"21+dev.5",
				"21+dev.6",
				"21+dev.7",
				"21+dev.8",
				"21+dev.9",
				"21+dev.10",
				"21+dev.11",
				"21+dev.12",
				"21+dev.13",
				"21+dev.14",
				"21+dev.15",
				"21+dev.16",
				"21+dev.17",
				"21+dev.18",
				"21+dev.19",
				"21+dev.20",
				"21+dev.21",
				"21+dev.22",
				"21+dev.23",
				"21+dev.24",
				"21+dev.25",
				"21+dev.26",
				"21+dev.27",
				"21+dev.28",
			}

			Expect(release.Latest()).To(Equal("21+dev.28"))
		})
	})
})
