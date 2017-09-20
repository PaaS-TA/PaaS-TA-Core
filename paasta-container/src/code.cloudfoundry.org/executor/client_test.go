package executor_test

import (
	. "code.cloudfoundry.org/executor"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Allocation Request", func() {
	It("is invalid when the guid is empty", func() {
		allocationInfo := NewResource(20, 30, "rootfs")
		allocRequest := NewAllocationRequest("", &allocationInfo, nil)
		err := allocRequest.Validate()
		Expect(err).To(HaveOccurred())
		Expect(err).To(MatchError(ErrGuidNotSpecified))
	})
})
