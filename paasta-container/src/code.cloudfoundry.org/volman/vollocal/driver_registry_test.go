package vollocal_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "code.cloudfoundry.org/volman/vollocal"

	"code.cloudfoundry.org/voldriver"
	"code.cloudfoundry.org/voldriver/voldriverfakes"
)

var _ = Describe("DriverRegistry", func() {
	var (
		emptyRegistry, oneRegistry, manyRegistry DriverRegistry
	)

	BeforeEach(func() {
		emptyRegistry = NewDriverRegistry()

		oneRegistry = NewDriverRegistryWith(map[string]voldriver.Driver{
			"one": new(voldriverfakes.FakeDriver),
		})

		manyRegistry = NewDriverRegistryWith(map[string]voldriver.Driver{
			"one": new(voldriverfakes.FakeDriver),
			"two": new(voldriverfakes.FakeDriver),
		})
	})

	Describe("#Driver", func() {
		It("sets the driver to new value", func() {
			oneDriver, exists := oneRegistry.Driver("one")
			Expect(exists).To(BeTrue())
			Expect(oneDriver).NotTo(BeNil())
		})

		It("returns nil and false if the driver doesn't exist", func() {
			oneDriver, exists := oneRegistry.Driver("doesnotexist")
			Expect(exists).To(BeFalse())
			Expect(oneDriver).To(BeNil())
		})
	})

	Describe("#Drivers", func() {
		It("should return return empty map for emptyRegistry", func() {
			drivers := emptyRegistry.Drivers()
			Expect(len(drivers)).To(Equal(0))
		})

		It("should return return one driver for oneRegistry", func() {
			drivers := oneRegistry.Drivers()
			Expect(len(drivers)).To(Equal(1))
		})
	})

	Describe("#Set", func() {
		It("replaces driver if it already exists", func() {
			newDriver := map[string]voldriver.Driver{
				"one": new(voldriverfakes.FakeDriver),
			}
			oneRegistry.Set(newDriver)
			oneDriver, exists := oneRegistry.Driver("one")
			Expect(exists).To(BeTrue())
			Expect(oneDriver).NotTo(BeNil())
		})

		It("adds driver that does not exists", func() {
			newDriver := map[string]voldriver.Driver{
				"one":   new(voldriverfakes.FakeDriver),
				"two":   new(voldriverfakes.FakeDriver),
				"three": new(voldriverfakes.FakeDriver),
			}
			manyRegistry.Set(newDriver)
			threeDriver, exists := manyRegistry.Driver("three")
			Expect(exists).To(BeTrue())
			Expect(threeDriver).NotTo(BeNil())
		})
	})

	Describe("#Keys", func() {
		It("should return return {'one'} for oneRegistry keys", func() {
			keys := emptyRegistry.Keys()
			Expect(len(keys)).To(Equal(0))
		})

		It("should return return {'one'} for oneRegistry keys", func() {
			keys := oneRegistry.Keys()
			Expect(len(keys)).To(Equal(1))
			Expect(keys[0]).To(Equal("one"))
		})
	})
})
