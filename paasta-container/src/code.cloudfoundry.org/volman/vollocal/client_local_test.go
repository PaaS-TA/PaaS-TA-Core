package vollocal_test

import (
	"time"

	"fmt"

	"github.com/cloudfoundry/dropsonde/metric_sender/fake"
	"github.com/cloudfoundry/dropsonde/metrics"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"code.cloudfoundry.org/voldriver"
	"code.cloudfoundry.org/volman/vollocal"
	"code.cloudfoundry.org/volman/volmanfakes"

	"code.cloudfoundry.org/clock/fakeclock"
	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/lager/lagertest"
	"code.cloudfoundry.org/voldriver/voldriverfakes"
	"github.com/tedsuo/ifrit"
	"github.com/tedsuo/ifrit/ginkgomon"
)

var _ = Describe("Volman", func() {
	var (
		logger = lagertest.NewTestLogger("client-test")

		fakeDriverFactory *volmanfakes.FakeDriverFactory
		fakeDriver        *voldriverfakes.FakeDriver
		fakeClock         *fakeclock.FakeClock

		scanInterval time.Duration

		driverRegistry vollocal.DriverRegistry
		driverSyncer   vollocal.DriverSyncer

		process ifrit.Process
	)

	BeforeEach(func() {
		fakeDriverFactory = new(volmanfakes.FakeDriverFactory)
		fakeClock = fakeclock.NewFakeClock(time.Unix(123, 456))

		scanInterval = 1 * time.Second

		driverRegistry = vollocal.NewDriverRegistry()
	})

	Describe("ListDrivers", func() {
		BeforeEach(func() {
			driverSyncer = vollocal.NewDriverSyncerWithDriverFactory(logger, driverRegistry, []string{"/somePath"}, scanInterval, fakeClock, fakeDriverFactory)
			client = vollocal.NewLocalClient(logger, driverRegistry, fakeClock)

			process = ginkgomon.Invoke(driverSyncer.Runner())
		})

		It("should report empty list of drivers", func() {
			drivers, err := client.ListDrivers(logger)
			Expect(err).NotTo(HaveOccurred())
			Expect(len(drivers.Drivers)).To(Equal(0))
		})

		Context("has no drivers in location", func() {

			BeforeEach(func() {
				fakeDriverFactory = new(volmanfakes.FakeDriverFactory)
			})

			It("should report empty list of drivers", func() {
				drivers, err := client.ListDrivers(logger)
				Expect(err).NotTo(HaveOccurred())
				Expect(len(drivers.Drivers)).To(Equal(0))
			})

			AfterEach(func() {
				ginkgomon.Kill(process)
			})

		})

		Context("has driver in location", func() {
			BeforeEach(func() {
				err := voldriver.WriteDriverSpec(logger, defaultPluginsDirectory, "fakedriver", "spec", []byte("http://0.0.0.0:8080"))
				Expect(err).NotTo(HaveOccurred())

				driverSyncer = vollocal.NewDriverSyncerWithDriverFactory(logger, driverRegistry, []string{defaultPluginsDirectory}, scanInterval, fakeClock, fakeDriverFactory)
				client = vollocal.NewLocalClient(logger, driverRegistry, fakeClock)

				fakeDriver := new(voldriverfakes.FakeDriver)
				fakeDriverFactory.DriverReturns(fakeDriver, nil)

				fakeDriver.ActivateReturns(voldriver.ActivateResponse{Implements: []string{"VolumeDriver"}})
			})

			It("should report empty list of drivers", func() {
				drivers, err := client.ListDrivers(logger)
				Expect(err).NotTo(HaveOccurred())
				Expect(len(drivers.Drivers)).To(Equal(0))
			})

			Context("after running drivers discovery", func() {
				BeforeEach(func() {
					process = ginkgomon.Invoke(driverSyncer.Runner())
				})

				AfterEach(func() {
					ginkgomon.Kill(process)
				})

				It("should report fakedriver", func() {
					drivers, err := client.ListDrivers(logger)
					Expect(err).NotTo(HaveOccurred())
					Expect(len(drivers.Drivers)).ToNot(Equal(0))
					Expect(drivers.Drivers[0].Name).To(Equal("fakedriver"))
				})

			})
		})
	})

	Describe("Mount and Unmount", func() {
		Context("when given a driver", func() {
			BeforeEach(func() {
				fakeDriverFactory = new(volmanfakes.FakeDriverFactory)
				fakeDriver = new(voldriverfakes.FakeDriver)
				fakeDriverFactory.DriverReturns(fakeDriver, nil)

				drivers := make(map[string]voldriver.Driver)
				drivers["fakedriver"] = fakeDriver

				err := voldriver.WriteDriverSpec(logger, defaultPluginsDirectory, "fakedriver", "spec", []byte(fmt.Sprintf("http://0.0.0.0:%d", fakeDriver)))
				Expect(err).NotTo(HaveOccurred())

				fakeDriver.ActivateReturns(voldriver.ActivateResponse{Implements: []string{"VolumeDriver"}})

				driverSyncer = vollocal.NewDriverSyncerWithDriverFactory(logger, driverRegistry, []string{defaultPluginsDirectory}, scanInterval, fakeClock, fakeDriverFactory)
				client = vollocal.NewLocalClient(logger, driverRegistry, fakeClock)

			})

			JustBeforeEach(func() {
				process = ginkgomon.Invoke(driverSyncer.Runner())
			})

			AfterEach(func() {
				ginkgomon.Kill(process)
			})

			Context("mount", func() {
				It("should be able to mount", func() {
					volumeId := "fake-volume"

					mountPath, err := client.Mount(logger, "fakedriver", volumeId, map[string]interface{}{"volume_id": volumeId})
					Expect(err).NotTo(HaveOccurred())
					Expect(mountPath).NotTo(Equal(""))
				})

				It("should not be able to mount if mount fails", func() {
					mountResponse := voldriver.MountResponse{Err: "an error"}
					fakeDriver.MountReturns(mountResponse)

					volumeId := "fake-volume"
					_, err := client.Mount(logger, "fakedriver", volumeId, map[string]interface{}{"volume_id": volumeId})
					Expect(err).To(HaveOccurred())
				})
				Context("with metrics", func() {
					var sender *fake.FakeMetricSender

					BeforeEach(func() {
						sender = fake.NewFakeMetricSender()
						metrics.Initialize(sender, nil)

					})

					It("should emit mount time on successful mount", func() {
						volumeId := "fake-volume"

						client.Mount(logger, "fakedriver", volumeId, map[string]interface{}{"volume_id": volumeId})

						reportedDuration := sender.GetValue("VolmanMountDuration")
						Expect(reportedDuration.Unit).To(Equal("nanos"))
						Expect(reportedDuration.Value).NotTo(BeZero())
						reportedDuration = sender.GetValue("VolmanMountDurationForfakedriver")
						Expect(reportedDuration.Unit).To(Equal("nanos"))
						Expect(reportedDuration.Value).NotTo(BeZero())
					})

					It("should increment error count on mount failure", func() {
						mountResponse := voldriver.MountResponse{Err: "an error"}
						fakeDriver.MountReturns(mountResponse)

						volumeId := "fake-volume"
						client.Mount(logger, "fakedriver", volumeId, map[string]interface{}{"volume_id": volumeId})
						Expect(sender.GetCounter("VolmanMountErrors")).To(Equal(uint64(1)))
					})

				})
			})

			Context("umount", func() {
				It("should be able to unmount", func() {
					volumeId := "fake-volume"

					err := client.Unmount(logger, "fakedriver", volumeId)
					Expect(err).NotTo(HaveOccurred())
					Expect(fakeDriver.UnmountCallCount()).To(Equal(1))
					Expect(fakeDriver.RemoveCallCount()).To(Equal(0))
				})

				It("should not be able to unmount when driver unmount fails", func() {
					fakeDriver.UnmountReturns(voldriver.ErrorResponse{Err: "unmount failure"})
					volumeId := "fake-volume"

					err := client.Unmount(logger, "fakedriver", volumeId)
					Expect(err).To(HaveOccurred())
				})
				Context("with metrics", func() {
					var sender *fake.FakeMetricSender

					BeforeEach(func() {
						sender = fake.NewFakeMetricSender()
						metrics.Initialize(sender, nil)

					})

					It("should emit unmount time on successful unmount", func() {
						volumeId := "fake-volume"
						client.Unmount(logger, "fakedriver", volumeId)

						reportedDuration := sender.GetValue("VolmanUnmountDuration")
						Expect(reportedDuration.Unit).To(Equal("nanos"))
						Expect(reportedDuration.Value).NotTo(BeZero())
						reportedDuration = sender.GetValue("VolmanUnmountDurationForfakedriver")
						Expect(reportedDuration.Unit).To(Equal("nanos"))
						Expect(reportedDuration.Value).NotTo(BeZero())
					})

					It("should increment error count on unmount failure", func() {
						fakeDriver.UnmountReturns(voldriver.ErrorResponse{Err: "unmount failure"})
						volumeId := "fake-volume"

						client.Unmount(logger, "fakedriver", volumeId)
						Expect(sender.GetCounter("VolmanUnmountErrors")).To(Equal(uint64(1)))
					})

				})
			})

			Context("when driver is not found", func() {
				BeforeEach(func() {
					fakeDriverFactory.DriverReturns(nil, fmt.Errorf("driver not found"))
				})

				It("should not be able to mount", func() {
					_, err := client.Mount(logger, "fakedriver", "fake-volume", map[string]interface{}{"volume_id": "fake-volume"})
					Expect(err).To(HaveOccurred())
				})

				It("should not be able to unmount", func() {
					err := client.Unmount(logger, "fakedriver", "fake-volume")
					Expect(err).To(HaveOccurred())
				})
			})

			Context("when driver does not implement VolumeDriver", func() {
				BeforeEach(func() {
					fakeDriver.ActivateReturns(voldriver.ActivateResponse{Implements: []string{"nada"}})
				})

				It("should not be able to mount", func() {
					_, err := client.Mount(logger, "fakedriver", "fake-volume", map[string]interface{}{"volume_id": "fake-volume"})
					Expect(err).To(HaveOccurred())
				})

				It("should not be able to unmount", func() {
					err := client.Unmount(logger, "fakedriver", "fake-volume")
					Expect(err).To(HaveOccurred())
				})
			})
		})

		Context("after creating successfully driver is not found", func() {
			BeforeEach(func() {

				fakeDriverFactory = new(volmanfakes.FakeDriverFactory)
				fakeDriver = new(voldriverfakes.FakeDriver)
				mountReturn := voldriver.MountResponse{Err: "driver not found",
					Mountpoint: "",
				}
				fakeDriver.MountReturns(mountReturn)
				fakeDriverFactory.DriverReturns(fakeDriver, nil)

				driverRegistry := vollocal.NewDriverRegistry()
				driverSyncer = vollocal.NewDriverSyncerWithDriverFactory(logger, driverRegistry, []string{"/somePath"}, scanInterval, fakeClock, fakeDriverFactory)
				client = vollocal.NewLocalClient(logger, driverRegistry, fakeClock)

				process = ginkgomon.Invoke(driverSyncer.Runner())

				calls := 0
				fakeDriverFactory.DriverStub = func(lager.Logger, string, string, string) (voldriver.Driver, error) {
					calls++
					if calls > 1 {
						return nil, fmt.Errorf("driver not found")
					}
					return fakeDriver, nil
				}
			})

			AfterEach(func() {
				ginkgomon.Kill(process)
			})

			It("should not be able to mount", func() {
				_, err := client.Mount(logger, "fakedriver", "fake-volume", map[string]interface{}{"volume_id": "fake-volume"})
				Expect(err).To(HaveOccurred())
			})

		})

		Context("after unsuccessfully creating", func() {
			BeforeEach(func() {
				localDriverProcess = ginkgomon.Invoke(localDriverRunner)
				fakeDriver = new(voldriverfakes.FakeDriver)

				fakeDriverFactory = new(volmanfakes.FakeDriverFactory)
				fakeDriverFactory.DriverReturns(fakeDriver, nil)

				fakeDriver.CreateReturns(voldriver.ErrorResponse{"create fails"})

				driverRegistry := vollocal.NewDriverRegistry()
				driverSyncer = vollocal.NewDriverSyncerWithDriverFactory(logger, driverRegistry, []string{"/somePath"}, scanInterval, fakeClock, fakeDriverFactory)
				client = vollocal.NewLocalClient(logger, driverRegistry, fakeClock)

				process = ginkgomon.Invoke(driverSyncer.Runner())
			})

			AfterEach(func() {
				ginkgomon.Kill(process)
			})

			It("should not be able to mount", func() {
				_, err := client.Mount(logger, "fakedriver", "fake-volume", map[string]interface{}{"volume_id": "fake-volume"})
				Expect(err).To(HaveOccurred())
			})

		})
	})
})
