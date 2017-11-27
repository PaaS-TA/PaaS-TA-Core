package vollocal_test

import (
	"fmt"
	"os"
	"path"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"code.cloudfoundry.org/goshims/osshim/os_fake"
	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/lager/lagertest"
	"code.cloudfoundry.org/voldriver"
	"code.cloudfoundry.org/voldriver/voldriverfakes"
	"code.cloudfoundry.org/volman/vollocal"
)

var _ = Describe("DriverFactory", func() {
	var (
		testLogger lager.Logger
		driverName string
	)
	BeforeEach(func() {
		testLogger = lagertest.NewTestLogger("ClientTest")
	})

	Context("when a valid driver spec is discovered", func() {
		var (
			fakeRemoteClientFactory *voldriverfakes.FakeRemoteClientFactory
			localDriver             *voldriverfakes.FakeDriver
			driver                  voldriver.Driver
			driverFactory           vollocal.DriverFactory
		)
		BeforeEach(func() {
			driverName = "some-driver-name"
			fakeRemoteClientFactory = new(voldriverfakes.FakeRemoteClientFactory)
			localDriver = new(voldriverfakes.FakeDriver)
			fakeRemoteClientFactory.NewRemoteClientReturns(localDriver, nil)
			driverFactory = vollocal.NewDriverFactoryWithRemoteClientFactory(fakeRemoteClientFactory)

		})

		Context("when a json driver spec is discovered", func() {
			BeforeEach(func() {
				err := voldriver.WriteDriverSpec(testLogger, defaultPluginsDirectory, driverName, "json", []byte("{\"Addr\":\"http://0.0.0.0:8080\"}"))
				Expect(err).NotTo(HaveOccurred())
				driver, err = driverFactory.Driver(testLogger, driverName, defaultPluginsDirectory, driverName+".json", nil)
				Expect(err).ToNot(HaveOccurred())
			})
			It("should return the correct driver", func() {
				Expect(driver).To(Equal(localDriver))
				Expect(fakeRemoteClientFactory.NewRemoteClientArgsForCall(0)).To(Equal("http://0.0.0.0:8080"))
			})
			It("should fail if unable to open file", func() {
				fakeOs := new(os_fake.FakeOs)
				driverFactory := vollocal.NewDriverFactoryWithOs(fakeOs)
				fakeOs.OpenReturns(nil, fmt.Errorf("error opening file"))
				_, err := driverFactory.Driver(testLogger, driverName, defaultPluginsDirectory, driverName+".json", nil)
				Expect(err).To(HaveOccurred())
			})
		})

		Context("when a json driver spec is rediscovered", func() {
			var matchableDriver *voldriverfakes.FakeMatchableDriver
			BeforeEach(func() {
				matchableDriver = new(voldriverfakes.FakeMatchableDriver)
				err := voldriver.WriteDriverSpec(testLogger, defaultPluginsDirectory, driverName, "json", []byte("{\"Addr\":\"http://0.0.0.0:8080\"}"))
				Expect(err).NotTo(HaveOccurred())
			})

			Context("when the driver is not matchable", func() {
				BeforeEach(func() {
					var err error
					driver, err = driverFactory.Driver(testLogger, driverName, defaultPluginsDirectory, driverName+".json", map[string]voldriver.Driver{driverName: localDriver})
					Expect(err).ToNot(HaveOccurred())
				})

				It("should recreate the driver", func() {
					Expect(driver).To(Equal(localDriver))
					Expect(fakeRemoteClientFactory.NewRemoteClientCallCount()).To(Equal(1))
				})
			})
			Context("when the driver is matchable, but does not match", func() {
				BeforeEach(func() {
					var err error
					matchableDriver.MatchesReturns(false)
					driver, err = driverFactory.Driver(testLogger, driverName, defaultPluginsDirectory, driverName+".json", map[string]voldriver.Driver{driverName: matchableDriver})
					Expect(err).ToNot(HaveOccurred())
				})

				It("should recreate the driver", func() {
					Expect(driver).To(Equal(localDriver))
					Expect(fakeRemoteClientFactory.NewRemoteClientCallCount()).To(Equal(1))
				})
			})
			Context("when the driver matches", func() {
				BeforeEach(func() {
					var err error
					matchableDriver.MatchesReturns(true)
					driver, err = driverFactory.Driver(testLogger, driverName, defaultPluginsDirectory, driverName+".json", map[string]voldriver.Driver{driverName: matchableDriver})
					Expect(err).ToNot(HaveOccurred())
				})

				It("should not recreate the driver", func() {
					Expect(driver).To(Equal(matchableDriver))
					Expect(fakeRemoteClientFactory.NewRemoteClientCallCount()).To(Equal(0))
				})
			})
		})

		Context("when an invalid json spec is discovered", func() {
			BeforeEach(func() {
				err := voldriver.WriteDriverSpec(testLogger, defaultPluginsDirectory, driverName, "json", []byte("{\"invalid\"}"))
				Expect(err).NotTo(HaveOccurred())
			})
			It("should error", func() {
				_, err := driverFactory.Driver(testLogger, driverName, defaultPluginsDirectory, driverName+".json", nil)
				Expect(err).To(HaveOccurred())
			})
		})

		Context("when a spec driver spec is discovered", func() {
			BeforeEach(func() {
				err := voldriver.WriteDriverSpec(testLogger, defaultPluginsDirectory, driverName, "spec", []byte("http://0.0.0.0:8080"))
				Expect(err).NotTo(HaveOccurred())
				driver, err = driverFactory.Driver(testLogger, driverName, defaultPluginsDirectory, driverName+".spec", nil)
				Expect(err).ToNot(HaveOccurred())
			})
			It("should return the correct driver", func() {
				Expect(driver).To(Equal(localDriver))
				Expect(fakeRemoteClientFactory.NewRemoteClientArgsForCall(0)).To(Equal("http://0.0.0.0:8080"))
			})
			It("should fail if unable to open file", func() {
				fakeOs := new(os_fake.FakeOs)
				driverFactory := vollocal.NewDriverFactoryWithOs(fakeOs)
				fakeOs.OpenReturns(nil, fmt.Errorf("error opening file"))
				_, err := driverFactory.Driver(testLogger, driverName, defaultPluginsDirectory, driverName+".spec", nil)
				Expect(err).To(HaveOccurred())
			})

			It("should error if driver id doesn't match found driver", func() {
				fakeRemoteClientFactory := new(voldriverfakes.FakeRemoteClientFactory)
				driverFactory := vollocal.NewDriverFactoryWithRemoteClientFactory(fakeRemoteClientFactory)
				_, err := driverFactory.Driver(testLogger, "garbage", defaultPluginsDirectory, "garbage.garbage", nil)
				Expect(err).To(HaveOccurred())
			})
		})

		Context("when a sock driver spec is discovered", func() {
			BeforeEach(func() {
				f, err := os.Create(defaultPluginsDirectory + "/" + driverName + ".sock")
				defer f.Close()
				Expect(err).ToNot(HaveOccurred())
			})
			It("should return the correct driver", func() {
				driver, err := driverFactory.Driver(testLogger, driverName, defaultPluginsDirectory, driverName+".sock", nil)
				Expect(err).ToNot(HaveOccurred())
				Expect(driver).To(Equal(localDriver))
				address := path.Join(defaultPluginsDirectory, driverName+".sock")
				Expect(fakeRemoteClientFactory.NewRemoteClientArgsForCall(0)).To(Equal(address))
			})
			It("should error for invalid sock endpoint address", func() {
				fakeRemoteClientFactory.NewRemoteClientReturns(nil, fmt.Errorf("invalid address"))
				_, err := driverFactory.Driver(testLogger, driverName, defaultPluginsDirectory, driverName+".sock", nil)
				Expect(err).To(HaveOccurred())
			})
		})
	})

	Context("when valid driver spec is not discovered", func() {
		var (
			fakeRemoteClientFactory *voldriverfakes.FakeRemoteClientFactory
			fakeDriver              *voldriverfakes.FakeDriver
			driverFactory           vollocal.DriverFactory
		)
		BeforeEach(func() {
			driverName = "some-driver-name"
			fakeRemoteClientFactory = new(voldriverfakes.FakeRemoteClientFactory)
			fakeDriver = new(voldriverfakes.FakeDriver)
			fakeRemoteClientFactory.NewRemoteClientReturns(fakeDriver, nil)
			driverFactory = vollocal.NewDriverFactoryWithRemoteClientFactory(fakeRemoteClientFactory)

		})
		It("should error", func() {
			_, err := driverFactory.Driver(testLogger, driverName, defaultPluginsDirectory, driverName+".spec", nil)
			Expect(err).To(HaveOccurred())
		})
	})

})
