package volman_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Given volman and localdriver", func() {

	var driverId string
	var volumeId string

	BeforeEach(func() {
		driverId = "localdriver"
		volumeId = "test-volume"
	})

	It("should mount a volume", func() {
		var err error
		someConfig := map[string]interface{}{"volume_id": "volman_test-someID"}
		mountPointResponse, err := volmanClient.Mount(logger, driverId, volumeId, someConfig)
		Expect(err).NotTo(HaveOccurred())
		Expect(mountPointResponse.Path).NotTo(BeEmpty())

		volmanClient.Unmount(logger, driverId, volumeId)
	})

	Context("and a mounted volman", func() {
		BeforeEach(func() {
			var err error
			someConfig := map[string]interface{}{"volume_id": "volman_test-someID"}
			mountPointResponse, err := volmanClient.Mount(logger, driverId, volumeId, someConfig)
			Expect(err).NotTo(HaveOccurred())
			Expect(mountPointResponse.Path).NotTo(BeEmpty())
		})

		It("should be able to unmount the volume", func() {
			err := volmanClient.Unmount(logger, driverId, volumeId)
			Expect(err).NotTo(HaveOccurred())
		})
	})
})
