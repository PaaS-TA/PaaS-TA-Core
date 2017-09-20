package rep_test

import (
	"code.cloudfoundry.org/bbs/models"
	"code.cloudfoundry.org/rep"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Resources", func() {
	var (
		cellState      rep.CellState
		linuxRootFSURL string
	)

	BeforeEach(func() {
		linuxOnlyRootFSProviders := rep.RootFSProviders{models.PreloadedRootFSScheme: rep.NewFixedSetRootFSProvider("linux")}
		total := rep.NewResources(1000, 2000, 10)
		avail := rep.NewResources(950, 1900, 3)
		linuxRootFSURL = models.PreloadedRootFS("linux")

		lrps := []rep.LRP{
			*BuildLRP("pg-1", "domain", 0, linuxRootFSURL, 10, 20),
			*BuildLRP("pg-1", "domain", 1, linuxRootFSURL, 10, 20),
			*BuildLRP("pg-2", "domain", 0, linuxRootFSURL, 10, 20),
			*BuildLRP("pg-3", "domain", 0, linuxRootFSURL, 10, 20),
			*BuildLRP("pg-4", "domain", 0, linuxRootFSURL, 10, 20),
		}

		tasks := []rep.Task{
			*BuildTask("tg-big", "domain", linuxRootFSURL, 20, 10, []string{}),
			*BuildTask("tg-small", "domain", linuxRootFSURL, 10, 10, []string{}),
		}

		cellState = rep.NewCellState(linuxOnlyRootFSProviders,
			avail,
			total,
			lrps,
			tasks,
			"my-zone",
			7,
			false,
			nil,
			nil,
			nil,
		)
	})

	Describe("Resource Matching", func() {
		var requiredResource rep.Resource
		var err error
		BeforeEach(func() {
			requiredResource = rep.NewResource(10, 10)
		})

		JustBeforeEach(func() {
			err = cellState.ResourceMatch(&requiredResource)
		})

		Context("when insufficient memory", func() {
			BeforeEach(func() {
				requiredResource.MemoryMB = 5000
			})

			It("returns an error", func() {
				Expect(err).To(HaveOccurred())
				Expect(err).To(MatchError("insufficient resources: memory"))
			})
		})

		Context("when insufficient disk", func() {
			BeforeEach(func() {
				requiredResource.DiskMB = 5000
			})

			It("returns an error", func() {
				Expect(err).To(HaveOccurred())
				Expect(err).To(MatchError("insufficient resources: disk"))
			})
		})

		Context("when insufficient disk and memory", func() {
			BeforeEach(func() {
				requiredResource.MemoryMB = 5000
				requiredResource.DiskMB = 5000
			})

			It("returns an error", func() {
				Expect(err).To(HaveOccurred())
				Expect(err).To(MatchError("insufficient resources: disk, memory"))
			})
		})

		Context("when insufficient disk, memory and containers", func() {
			BeforeEach(func() {
				requiredResource.MemoryMB = 5000
				requiredResource.DiskMB = 5000
				cellState.AvailableResources.Containers = 0
			})

			It("returns an error", func() {
				Expect(err).To(HaveOccurred())
				Expect(err).To(MatchError("insufficient resources: containers, disk, memory"))
			})
		})

		Context("when there are no available containers", func() {
			BeforeEach(func() {
				cellState.AvailableResources.Containers = 0
			})

			It("returns an error", func() {
				Expect(err).To(HaveOccurred())
				Expect(err).To(MatchError("insufficient resources: containers"))
			})
		})

		Context("when there is sufficient room", func() {
			It("does not return an error", func() {
				Expect(err).NotTo(HaveOccurred())
			})
		})
	})
})

func BuildLRP(guid, domain string, index int, rootFS string, memoryMB, diskMB int32) *rep.LRP {
	lrpKey := models.NewActualLRPKey(guid, int32(index), domain)
	lrp := rep.NewLRP(lrpKey, rep.NewResource(memoryMB, diskMB), rep.PlacementConstraint{RootFs: rootFS})
	return &lrp
}

func BuildTask(taskGuid, domain, rootFS string, memoryMB, diskMB int32, volumeDrivers []string) *rep.Task {
	task := rep.NewTask(taskGuid, domain, rep.NewResource(memoryMB, diskMB), rep.PlacementConstraint{RootFs: rootFS, VolumeDrivers: volumeDrivers})
	return &task
}
