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
		var (
			capacity            executor.ExecutorResources
			err                 error
			memLimit, diskLimit string
			maxCacheSizeInBytes uint64
			autoDiskMBOverhead  int
		)

		BeforeEach(func() {
			maxCacheSizeInBytes = 0
			autoDiskMBOverhead = 0
			memLimit = ""
			diskLimit = ""
		})

		JustBeforeEach(func() {
			capacity, err = configuration.ConfigureCapacity(gardenClient, memLimit, diskLimit, maxCacheSizeInBytes, autoDiskMBOverhead)
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

					It("uses the garden server's memory capacity", func() {
						Expect(err).NotTo(HaveOccurred())
						Expect(capacity.DiskMB).To(Equal(4))
					})

					Context("when the max cache size in bytes is non zero", func() {
						BeforeEach(func() {
							maxCacheSizeInBytes = 1024 * 1024 * 2
						})

						It("subtracts the cache size from the disk capacity", func() {
							Expect(capacity.DiskMB).To(Equal(2))
						})

						Context("when the max cache size in bytes is larger than the available disk capacity", func() {
							BeforeEach(func() {
								maxCacheSizeInBytes = 1024 * 1024 * 4
							})

							It("returns an error", func() {
								Expect(err).To(HaveOccurred())
							})
						})
					})

					Context("when the auto disk mb overhead property is set", func() {
						BeforeEach(func() {
							autoDiskMBOverhead = 1
						})

						It("subtracts the overhead to the disk capacity", func() {
							Expect(err).NotTo(HaveOccurred())
							Expect(capacity.DiskMB).To(Equal(3))
						})
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
