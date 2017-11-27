package vizzini_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Cells", func() {
	It("should return all cells", func() {
		cells, err := bbsClient.Cells(logger)
		Expect(err).NotTo(HaveOccurred())
		Expect(len(cells)).To(BeNumerically(">=", 1))

		cell0 := cells[0]

		Expect(cell0).NotTo(BeNil())
		Expect(cell0.Capacity.MemoryMb).To(BeNumerically(">", 0))
		Expect(cell0.Capacity.DiskMb).To(BeNumerically(">", 0))
		Expect(cell0.Capacity.Containers).To(BeNumerically(">", 0))
		Expect(len(cell0.RootfsProviders)).To(BeNumerically(">", 0))
	})
})
