package core_test

import (
	"github.com/pivotal-cf-experimental/destiny/core"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("AZs", func() {
	It("returns a slice of AZs given a number of AZs desired", func() {
		azs := core.AZs(2)
		Expect(azs).To(Equal([]string{"z1", "z2"}))
	})
})
