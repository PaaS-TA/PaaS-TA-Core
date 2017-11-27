package executor_test

import (
	"code.cloudfoundry.org/executor"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Container", func() {
	Describe("HasTags", func() {
		var container executor.Container

		Context("when tags are nil", func() {
			BeforeEach(func() {
				container = executor.Container{
					Tags: nil,
				}
			})

			It("returns true if requested tags are nil", func() {
				Expect(container.HasTags(nil)).To(BeTrue())
			})

			It("returns false if requested tags are not nil", func() {
				Expect(container.HasTags(executor.Tags{"a": "b"})).To(BeFalse())
			})
		})

		Context("when tags are not nil", func() {
			BeforeEach(func() {
				container = executor.Container{
					Tags: executor.Tags{"a": "b"},
				}
			})

			It("returns true when found", func() {
				Expect(container.HasTags(executor.Tags{"a": "b"})).To(BeTrue())
			})

			It("returns false when nil", func() {
				Expect(container.HasTags(nil)).To(BeFalse())
			})

			It("returns false when not found", func() {
				Expect(container.HasTags(executor.Tags{"a": "c"})).To(BeFalse())
			})
		})
	})

	Describe("Subtract", func() {
		const (
			defaultDiskMB     = 20
			defaultMemoryMB   = 30
			defaultContainers = 3
		)

		It("returns false when the number of containers is less than 1", func() {
			resources := executor.NewExecutorResources(defaultMemoryMB, defaultDiskMB, 0)
			resourceToSubtract := executor.NewResource(defaultMemoryMB-1, defaultDiskMB-1, -1, "rootfs")
			Expect(resources.Subtract(&resourceToSubtract)).To(BeFalse())
		})

		It("returns false when disk size exceeds total available disk size", func() {
			resources := executor.NewExecutorResources(defaultMemoryMB, 10, defaultContainers)
			resourceToSubtract := executor.NewResource(defaultMemoryMB-1, 20, -1, "rootfs")
			Expect(resources.Subtract(&resourceToSubtract)).To(BeFalse())
		})

		It("returns false when memory exceeds total available memory", func() {
			resources := executor.NewExecutorResources(10, defaultDiskMB, defaultContainers)
			resourceToSubtract := executor.NewResource(20, defaultDiskMB-1, -1, "rootfs")
			Expect(resources.Subtract(&resourceToSubtract)).To(BeFalse())
		})
	})
})
