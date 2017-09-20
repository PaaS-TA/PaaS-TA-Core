package configuration_test

import (
	"errors"

	"code.cloudfoundry.org/executor"
	"code.cloudfoundry.org/executor/fakes"
	"code.cloudfoundry.org/executor/initializer/configuration"
	"code.cloudfoundry.org/garden"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("configuration", func() {
	var gardenClient *fakes.FakeGardenClient

	BeforeEach(func() {
		gardenClient = fakes.NewGardenClient()
	})

	Describe("ConfigureCapacity", func() {
		var capacity executor.ExecutorResources
		var err error
		var memLimit string
		var diskLimit string

		JustBeforeEach(func() {
			capacity, err = configuration.ConfigureCapacity(gardenClient, memLimit, diskLimit)
		})

		Context("when getting the capacity fails", func() {
			BeforeEach(func() {
				gardenClient.Connection.CapacityReturns(garden.Capacity{}, errors.New("uh oh"))
			})

			It("returns an error", func() {
				Expect(err).To(Equal(errors.New("uh oh")))
			})
		})

		Context("when getting the capacity succeeds", func() {
			BeforeEach(func() {
				memLimit = "99"
				diskLimit = "99"
				gardenClient.Connection.CapacityReturns(
					garden.Capacity{
						MemoryInBytes: 1024 * 1024 * 3,
						DiskInBytes:   1024 * 1024 * 4,
						MaxContainers: 5,
					},
					nil,
				)
			})

			Describe("Memory Limit", func() {
				Context("when the memory limit flag is 'auto'", func() {
					BeforeEach(func() {
						memLimit = "auto"
					})

					It("does not return an error", func() {
						Expect(err).NotTo(HaveOccurred())
					})

					It("uses the garden server's memory capacity", func() {
						Expect(capacity.MemoryMB).To(Equal(3))
					})
				})

				Context("when the memory limit flag is a positive number", func() {
					BeforeEach(func() {
						memLimit = "2"
					})

					It("does not return an error", func() {
						Expect(err).NotTo(HaveOccurred())
					})

					It("uses that number", func() {
						Expect(capacity.MemoryMB).To(Equal(2))
					})
				})

				Context("when the memory limit flag is not a number", func() {
					BeforeEach(func() {
						memLimit = "stuff"
					})

					It("returns an error", func() {
						Expect(err).To(Equal(configuration.ErrMemoryFlagInvalid))
					})
				})

				Context("when the memory limit flag is not positive", func() {
					BeforeEach(func() {
						memLimit = "0"
					})

					It("returns an error", func() {
						Expect(err).To(Equal(configuration.ErrMemoryFlagInvalid))
					})
				})
			})

			Describe("Disk Limit", func() {
				Context("when the disk limit flag is 'auto'", func() {
					BeforeEach(func() {
						diskLimit = "auto"
					})

					It("does not return an error", func() {
						Expect(err).NotTo(HaveOccurred())
					})

					It("uses the garden server's memory capacity", func() {
						Expect(capacity.DiskMB).To(Equal(4))
					})
				})

				Context("when the disk limit flag is a positive number", func() {
					BeforeEach(func() {
						diskLimit = "2"
					})

					It("does not return an error", func() {
						Expect(err).NotTo(HaveOccurred())
					})

					It("uses that number", func() {
						Expect(capacity.DiskMB).To(Equal(2))
					})
				})

				Context("when the disk limit flag is not a number", func() {
					BeforeEach(func() {
						diskLimit = "stuff"
					})

					It("returns an error", func() {
						Expect(err).To(Equal(configuration.ErrDiskFlagInvalid))
					})
				})

				Context("when the disk limit flag is not positive", func() {
					BeforeEach(func() {
						diskLimit = "0"
					})

					It("returns an error", func() {
						Expect(err).To(Equal(configuration.ErrDiskFlagInvalid))
					})
				})
			})

			Describe("Containers Limit", func() {
				It("uses the garden server's max containers", func() {
					Expect(capacity.Containers).To(Equal(4))
				})
			})
		})
	})
})
