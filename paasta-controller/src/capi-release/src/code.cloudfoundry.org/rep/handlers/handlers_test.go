package handlers_test

import (
	"code.cloudfoundry.org/rep"
	"code.cloudfoundry.org/rep/evacuation/evacuation_context/fake_evacuation_context"
	"code.cloudfoundry.org/rep/handlers"

	executorfakes "code.cloudfoundry.org/executor/fakes"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/tedsuo/rata"
)

var _ = Describe("NewLegacy", func() {
	It("has all routes", func() {

		fakeExecutorClient := new(executorfakes.FakeClient)
		fakeEvacuatable := new(fake_evacuation_context.FakeEvacuatable)
		handlers := handlers.NewLegacy(fakeLocalRep, fakeExecutorClient, fakeEvacuatable, logger)

		for _, route := range rep.Routes {
			Expect(handlers[route.Name]).NotTo(BeNil())
		}
	})
})

var _ = Describe("New", func() {
	var test_handlers rata.Handlers

	Context("an unsecure server", func() {
		BeforeEach(func() {
			fakeExecutorClient := new(executorfakes.FakeClient)
			fakeEvacuatable := new(fake_evacuation_context.FakeEvacuatable)
			test_handlers = handlers.New(fakeLocalRep, fakeExecutorClient, fakeEvacuatable, logger, false)
		})

		It("has no secure routes", func() {
			for _, route := range rep.RoutesSecure {
				Expect(test_handlers[route.Name]).To(BeNil())
			}
		})

		It("has all unsecure routes", func() {
			for _, route := range rep.RoutesInsecure {
				Expect(test_handlers[route.Name]).NotTo(BeNil())
			}
		})
	})

	Context("a secure server", func() {
		BeforeEach(func() {
			fakeExecutorClient := new(executorfakes.FakeClient)
			fakeEvacuatable := new(fake_evacuation_context.FakeEvacuatable)
			test_handlers = handlers.New(fakeLocalRep, fakeExecutorClient, fakeEvacuatable, logger, true)
		})

		It("has all the secure routes", func() {
			for _, route := range rep.RoutesSecure {
				Expect(test_handlers[route.Name]).NotTo(BeNil())
			}
		})

		It("has no unsecure routes", func() {
			for _, route := range rep.RoutesInsecure {
				Expect(test_handlers[route.Name]).To(BeNil())
			}
		})
	})
})
