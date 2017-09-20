package vollocal_test

import (
	"code.cloudfoundry.org/volman/vollocal"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"code.cloudfoundry.org/voldriver"
	"code.cloudfoundry.org/voldriver/voldriverfakes"
	"github.com/tedsuo/ifrit/ginkgomon"
	"code.cloudfoundry.org/lager/lagertest"
	"code.cloudfoundry.org/clock"
	"code.cloudfoundry.org/clock/fakeclock"
	"time"
	"code.cloudfoundry.org/volman/volmanfakes"
	"github.com/tedsuo/ifrit"
)

var _ = Describe("MountPurger", func() {

	var (
		logger *lagertest.TestLogger

		driverRegistry vollocal.DriverRegistry
		driverSyncer vollocal.DriverSyncer
		purger vollocal.MountPurger

		fakeDriverFactory *volmanfakes.FakeDriverFactory
		fakeDriver        *voldriverfakes.FakeDriver
		fakeClock clock.Clock

		scanInterval time.Duration

		process ifrit.Process
	)

	BeforeEach(func() {
		logger = lagertest.NewTestLogger("mount-purger")

		driverRegistry = vollocal.NewDriverRegistry()

		purger = vollocal.NewMountPurger(logger, driverRegistry)
	})

	It("should succeed when there are no drivers", func() {
		err := purger.PurgeMounts(logger)
		Expect(err).NotTo(HaveOccurred())
	})

	Context("when there is a driver", func() {
		BeforeEach(func() {
			err := voldriver.WriteDriverSpec(logger, defaultPluginsDirectory, "fakedriver", "spec", []byte("http://0.0.0.0:8080"))
			Expect(err).NotTo(HaveOccurred())

			fakeDriverFactory = new(volmanfakes.FakeDriverFactory)
			fakeClock = fakeclock.NewFakeClock(time.Unix(123, 456))

			scanInterval = 1 * time.Second

			driverSyncer = vollocal.NewDriverSyncerWithDriverFactory(logger, driverRegistry, []string{defaultPluginsDirectory}, scanInterval, fakeClock, fakeDriverFactory)
			client = vollocal.NewLocalClient(logger, driverRegistry, fakeClock)

			fakeDriver = new(voldriverfakes.FakeDriver)
			fakeDriverFactory.DriverReturns(fakeDriver, nil)

			fakeDriver.ActivateReturns(voldriver.ActivateResponse{Implements: []string{"VolumeDriver"}})

			process = ginkgomon.Invoke(driverSyncer.Runner())
		})

		AfterEach(func() {
			ginkgomon.Kill(process)
		})

		It("should succeed when there are no mounts", func() {
			err := purger.PurgeMounts(logger)
			Expect(err).NotTo(HaveOccurred())
		})

		Context("when there is a mount", func() {
			BeforeEach(func() {
				fakeDriver.ListReturns(voldriver.ListResponse{Volumes: []voldriver.VolumeInfo{
					{
						Name: "a-volume",
						Mountpoint: "foo",
					},
				}})
			})

			It("should unmount the volume", func() {
				err := purger.PurgeMounts(logger)
				Expect(err).NotTo(HaveOccurred())

				Expect(fakeDriver.UnmountCallCount()).To(Equal(1))
			})

			Context("when the unmount fails", func() {
				BeforeEach(func() {
					fakeDriver.UnmountReturns(voldriver.ErrorResponse{Err: "badness"})
				})

				It("should log but not fail", func() {
					err := purger.PurgeMounts(logger)
					Expect(err).NotTo(HaveOccurred())

					Expect(logger.TestSink.LogMessages()).To(ContainElement("mount-purger.purge-mounts.failed-purging-volume-mount"))
				})
			})
		})
	})
})
