package main_test

import (
	"code.cloudfoundry.org/routing-api"
	"github.com/tedsuo/ifrit"
	"github.com/tedsuo/ifrit/ginkgomon"

	"code.cloudfoundry.org/routing-api/cmd/routing-api/testrunner"
	"code.cloudfoundry.org/routing-api/models"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Routes API", func() {
	var routingAPIProcess ifrit.Process

	BeforeEach(func() {
		routingAPIRunner := testrunner.New(routingAPIBinPath, routingAPIArgs)
		routingAPIProcess = ginkgomon.Invoke(routingAPIRunner)
	})

	AfterEach(func() {
		ginkgomon.Kill(routingAPIProcess)
	})

	var (
		eventStream routing_api.EventSource
		err         error
		route1      models.Route
	)

	Describe("SubscribeToEvents", func() {
		BeforeEach(func() {
			eventStream, err = client.SubscribeToEvents()
			Expect(err).NotTo(HaveOccurred())

			route1 = models.NewRoute("a.b.c", 33, "1.1.1.1", "potato", "", 55)
		})

		AfterEach(func() {
			eventStream.Close()
		})

		It("returns an eventstream", func() {
			expectedEvent := routing_api.Event{
				Action: "Upsert",
				Route:  route1,
			}
			routesToInsert := []models.Route{route1}
			client.UpsertRoutes(routesToInsert)

			Eventually(func() bool {
				event, err := eventStream.Next()
				Expect(err).NotTo(HaveOccurred())
				return event.Action == expectedEvent.Action && event.Route.Matches(expectedEvent.Route)
			}).Should(BeTrue())
		})

		It("gets events for updated routes", func() {
			routeUpdated := models.NewRoute("a.b.c", 33, "1.1.1.1", "potato", "", 85)

			client.UpsertRoutes([]models.Route{route1})
			Eventually(func() bool {
				event, err := eventStream.Next()
				Expect(err).NotTo(HaveOccurred())
				return event.Action == "Upsert" && event.Route.Matches(route1)
			}).Should(BeTrue())

			client.UpsertRoutes([]models.Route{routeUpdated})
			Eventually(func() bool {
				event, err := eventStream.Next()
				Expect(err).NotTo(HaveOccurred())
				return event.Action == "Upsert" && event.Route.Matches(routeUpdated)
			}).Should(BeTrue())
		})

		It("gets events for deleted routes", func() {
			client.UpsertRoutes([]models.Route{route1})

			expectedEvent := routing_api.Event{
				Action: "Delete",
				Route:  route1,
			}
			client.DeleteRoutes([]models.Route{route1})
			Eventually(func() bool {
				event, err := eventStream.Next()
				Expect(err).NotTo(HaveOccurred())
				return event.Action == expectedEvent.Action && event.Route.Matches(expectedEvent.Route)
			}).Should(BeTrue())
		})

		It("gets events for expired routes", func() {
			routeExpire := models.NewRoute("z.a.k", 63, "42.42.42.42", "Tomato", "", 1)

			client.UpsertRoutes([]models.Route{routeExpire})

			expectedEvent := routing_api.Event{
				Action: "Delete",
				Route:  routeExpire,
			}

			Eventually(func() bool {
				event, err := eventStream.Next()
				Expect(err).NotTo(HaveOccurred())
				return event.Action == expectedEvent.Action && event.Route.Matches(expectedEvent.Route)
			}).Should(BeTrue())
		})
	})
})
