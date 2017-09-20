package routing_table_test

import (
	"code.cloudfoundry.org/route-emitter/routing_table"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("MessagesToEmit", func() {
	var (
		messagesToEmit routing_table.MessagesToEmit
		messages1      []routing_table.RegistryMessage
	)

	BeforeEach(func() {
		messagesToEmit = routing_table.MessagesToEmit{}
		messages1 = []routing_table.RegistryMessage{
			{
				Host: "1.1.1.1",
				Port: 61000,
				App:  "log-guid-2",
				URIs: []string{"host1.example.com"},
			},
			{
				Host: "1.1.1.1",
				Port: 61001,
				App:  "log-guid-1",
				URIs: []string{"host1.example.com"},
			},
			{
				Host: "1.1.1.1",
				Port: 61003,
				App:  "log-guid-2",
				URIs: []string{"host2.example.com", "host3.example.com"},
			},
			{
				Host: "1.1.1.1",
				Port: 61004,
				App:  "log-guid-3",
				URIs: []string{"host3.example.com"},
			},
		}
	})

	Describe("RouteRegistrationCount", func() {
		Context("when there are registration messages", func() {
			BeforeEach(func() {
				messagesToEmit.RegistrationMessages = messages1
			})

			It("adds the number of hostnames in each route message", func() {
				Expect(messagesToEmit.RouteRegistrationCount()).To(BeEquivalentTo(5))
			})
		})

		Context("when registration messages is nil", func() {
			BeforeEach(func() {
				messagesToEmit.RegistrationMessages = nil
				messagesToEmit.UnregistrationMessages = messages1
			})

			It("adds the number of hostnames in each route message", func() {
				Expect(messagesToEmit.RouteRegistrationCount()).To(BeEquivalentTo(0))
			})
		})
	})

	Describe("RouteUnregistrationCount", func() {
		Context("when there are unregistration messages", func() {
			BeforeEach(func() {
				messagesToEmit.UnregistrationMessages = messages1
			})

			It("adds the number of hostnames in each route message", func() {
				Expect(messagesToEmit.RouteUnregistrationCount()).To(BeEquivalentTo(5))
			})
		})

		Context("when registration messages is nil", func() {
			BeforeEach(func() {
				messagesToEmit.RegistrationMessages = messages1
				messagesToEmit.UnregistrationMessages = nil
			})

			It("adds the number of hostnames in each route message", func() {
				Expect(messagesToEmit.RouteUnregistrationCount()).To(BeEquivalentTo(0))
			})
		})
	})
})
