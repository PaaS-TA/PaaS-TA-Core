package main_test

import (
	"fmt"
	"time"

	"github.com/tedsuo/ifrit"
	"github.com/tedsuo/ifrit/ginkgomon"

	"code.cloudfoundry.org/routing-api"
	"code.cloudfoundry.org/routing-api/cmd/routing-api/testrunner"
	"code.cloudfoundry.org/routing-api/matchers"
	"code.cloudfoundry.org/routing-api/models"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Routes API", func() {
	getRouterGroupGuid := func() string {
		client := routing_api.NewClient(fmt.Sprintf("http://127.0.0.1:%d", routingAPIPort), false)
		var routerGroups []models.RouterGroup
		Eventually(func() error {
			var err error
			routerGroups, err = client.RouterGroups()
			return err
		}, "30s", "1s").ShouldNot(HaveOccurred(), "Failed to connect to Routing API server after 30s.")
		Expect(routerGroups).ToNot(HaveLen(0))
		return routerGroups[0].Guid
	}

	TestTCPEvents := func() {
		Context("TCP Events", func() {
			var (
				routerGroupGuid string
				eventStream     routing_api.TcpEventSource
				err             error
				route1          models.TcpRouteMapping
			)

			BeforeEach(func() {
				routerGroupGuid = getRouterGroupGuid()

				route1 = models.NewTcpRouteMapping(routerGroupGuid, 3000, "1.1.1.1", 1234, 60)
				eventStream, err = client.SubscribeToTcpEvents()
				Expect(err).NotTo(HaveOccurred())
			})

			AfterEach(func() {
				err = eventStream.Close()
				Expect(err).ToNot(HaveOccurred())
			})

			It("returns an eventstream", func() {
				expectedEvent := routing_api.TcpEvent{
					Action:          "Upsert",
					TcpRouteMapping: route1,
				}
				routesToInsert := []models.TcpRouteMapping{route1}
				err := client.UpsertTcpRouteMappings(routesToInsert)
				Expect(err).NotTo(HaveOccurred())

				event, err := eventStream.Next()
				Expect(err).NotTo(HaveOccurred())
				Expect(event.Action).To(Equal(expectedEvent.Action))
				Expect(event.TcpRouteMapping).To(matchers.MatchTcpRoute(expectedEvent.TcpRouteMapping))
			})

			It("gets events for updated routes", func(done Done) {
				defer close(done)
				routeUpdated := models.NewTcpRouteMapping(routerGroupGuid, 3000, "1.1.1.1", 1234, 75)

				routesToInsert := []models.TcpRouteMapping{route1}

				err := client.UpsertTcpRouteMappings(routesToInsert)
				Expect(err).NotTo(HaveOccurred())
				event, err := eventStream.Next()
				Expect(err).NotTo(HaveOccurred())
				Expect(event.Action).To(Equal("Upsert"))
				Expect(event.TcpRouteMapping).To(matchers.MatchTcpRoute(route1))

				err = client.UpsertTcpRouteMappings([]models.TcpRouteMapping{routeUpdated})
				Expect(err).NotTo(HaveOccurred())
				event, err = eventStream.Next()
				Expect(err).NotTo(HaveOccurred())
				Expect(event.Action).To(Equal("Upsert"))
				Expect(event.TcpRouteMapping).To(matchers.MatchTcpRoute(routeUpdated))
			}, 5.0)

			It("gets events for deleted routes", func(done Done) {
				defer close(done)
				routesToInsert := []models.TcpRouteMapping{route1}

				err := client.UpsertTcpRouteMappings(routesToInsert)
				Expect(err).NotTo(HaveOccurred())
				event, err := eventStream.Next()
				Expect(err).NotTo(HaveOccurred())

				expectedEvent := routing_api.TcpEvent{
					Action:          "Delete",
					TcpRouteMapping: route1,
				}

				err = client.DeleteTcpRouteMappings(routesToInsert)
				Expect(err).NotTo(HaveOccurred())
				event, err = eventStream.Next()
				Expect(err).NotTo(HaveOccurred())
				Expect(event.Action).To(Equal(expectedEvent.Action))
				Expect(event.TcpRouteMapping).To(matchers.MatchTcpRoute(expectedEvent.TcpRouteMapping))
			}, 5.0)

			It("gets events for expired routes", func() {
				routeExpire := models.NewTcpRouteMapping(routerGroupGuid, 3000, "1.1.1.1", 1234, 1)

				err := client.UpsertTcpRouteMappings([]models.TcpRouteMapping{routeExpire})
				Expect(err).NotTo(HaveOccurred())
				_, err = eventStream.Next()
				Expect(err).NotTo(HaveOccurred())

				expectedEvent := routing_api.TcpEvent{
					Action:          "Delete",
					TcpRouteMapping: routeExpire,
				}

				Eventually(func() models.TcpRouteMapping {
					event, err := eventStream.Next()
					Expect(err).NotTo(HaveOccurred())
					Expect(event.Action).To(Equal(expectedEvent.Action))
					return event.TcpRouteMapping
				}).Should(matchers.MatchTcpRoute(expectedEvent.TcpRouteMapping))
			})
		})
	}

	TestHTTPEvents := func() {
		Context("HTTP Events", func() {
			var (
				eventStream routing_api.EventSource
				err         error
				route1      models.Route
			)

			BeforeEach(func() {
				eventStream, err = client.SubscribeToEvents()
				Expect(err).NotTo(HaveOccurred())

				route1 = models.NewRoute("a.b.c", 33, "1.1.1.1", "potato", "", 55)
			})

			AfterEach(func() {
				err = eventStream.Close()
				Expect(err).NotTo(HaveOccurred())
			})

			It("returns an eventstream", func() {
				expectedEvent := routing_api.Event{
					Action: "Upsert",
					Route:  route1,
				}
				routesToInsert := []models.Route{route1}
				err := client.UpsertRoutes(routesToInsert)
				Expect(err).NotTo(HaveOccurred())

				Eventually(func() routing_api.Event {
					event, err := eventStream.Next()
					Expect(err).NotTo(HaveOccurred())
					return event
				}).Should(matchers.MatchHttpEvent(expectedEvent))
			})

			It("gets events for updated routes", func() {
				routeUpdated := models.NewRoute("a.b.c", 33, "1.1.1.1", "potato", "", 85)

				expectedEvent := routing_api.Event{
					Action: "Upsert",
					Route:  route1,
				}
				err := client.UpsertRoutes([]models.Route{route1})
				Expect(err).NotTo(HaveOccurred())
				Eventually(func() routing_api.Event {
					event, err := eventStream.Next()
					Expect(err).NotTo(HaveOccurred())
					return event
				}).Should(matchers.MatchHttpEvent(expectedEvent))

				expectedEvent.Route = routeUpdated
				err = client.UpsertRoutes([]models.Route{routeUpdated})
				Expect(err).NotTo(HaveOccurred())
				Eventually(func() routing_api.Event {
					event, err := eventStream.Next()
					Expect(err).NotTo(HaveOccurred())
					return event
				}).Should(matchers.MatchHttpEvent(expectedEvent))
			})

			It("gets events for deleted routes", func() {
				err := client.UpsertRoutes([]models.Route{route1})
				Expect(err).NotTo(HaveOccurred())

				expectedEvent := routing_api.Event{
					Action: "Delete",
					Route:  route1,
				}
				err = client.DeleteRoutes([]models.Route{route1})
				Expect(err).NotTo(HaveOccurred())
				Eventually(func() routing_api.Event {
					event, err := eventStream.Next()
					Expect(err).NotTo(HaveOccurred())
					return event
				}).Should(matchers.MatchHttpEvent(expectedEvent))
			})

			It("gets events for expired routes", func() {
				routeExpire := models.NewRoute("z.a.k", 63, "42.42.42.42", "Tomato", "", 1)

				err := client.UpsertRoutes([]models.Route{routeExpire})
				Expect(err).NotTo(HaveOccurred())
				_, err = eventStream.Next()
				Expect(err).NotTo(HaveOccurred())

				expectedEvent := routing_api.Event{
					Action: "Delete",
					Route:  routeExpire,
				}

				Eventually(func() routing_api.Event {
					event, err := eventStream.Next()
					Expect(err).NotTo(HaveOccurred())
					return event
				}).Should(matchers.MatchHttpEvent(expectedEvent))
			})
		})
	}

	TestHTTPRoutes := func() {
		Context("HTTP Routes", func() {
			var routes []models.Route
			var getErr error
			var route1, route2 models.Route

			BeforeEach(func() {
				route1 = models.NewRoute("a.b.c", 33, "1.1.1.1", "potato", "", 55)
				route2 = models.NewRoute("d.e.f", 35, "1.1.1.1", "banana", "", 66)

				routesToInsert := []models.Route{route1, route2}
				upsertErr := client.UpsertRoutes(routesToInsert)
				Expect(upsertErr).NotTo(HaveOccurred())
				routes, getErr = client.Routes()
			})

			It("responds without an error", func() {
				Expect(getErr).NotTo(HaveOccurred())
			})

			It("fetches all of the routes", func() {
				routingAPIRoute := models.NewRoute(fmt.Sprintf("api.%s/routing", routingAPISystemDomain), routingAPIPort, routingAPIIP, "my_logs", "", 120)
				Eventually(func() int {
					routes, getErr = client.Routes()
					Expect(getErr).ToNot(HaveOccurred())
					return len(routes)
				}, 2*time.Second).Should(BeNumerically("==", 3))
				Expect(routes).To(ConsistOf(
					matchers.MatchHttpRoute(route1),
					matchers.MatchHttpRoute(route2),
					matchers.MatchHttpRoute(routingAPIRoute),
				))
			})

			It("deletes a route", func() {
				err := client.DeleteRoutes([]models.Route{route1})

				Expect(err).NotTo(HaveOccurred())

				routes, err = client.Routes()
				Expect(err).NotTo(HaveOccurred())
				Expect(routes).ToNot(ContainElement(matchers.MatchHttpRoute(route1)))
			})

			It("rejects bad routes", func() {
				route3 := models.NewRoute("foo/b ar", 35, "2.2.2.2", "banana", "", 66)

				err := client.UpsertRoutes([]models.Route{route3})
				Expect(err).To(HaveOccurred())

				routes, err = client.Routes()

				Expect(err).ToNot(HaveOccurred())
				Expect(routes).To(ContainElement(matchers.MatchHttpRoute(route1)))
				Expect(routes).To(ContainElement(matchers.MatchHttpRoute(route2)))
				Expect(routes).ToNot(ContainElement(matchers.MatchHttpRoute(route3)))
			})

			Context("when a route has a context path", func() {
				var routeWithPath models.Route

				BeforeEach(func() {
					routeWithPath = models.NewRoute("host.com/path", 51480, "1.2.3.4", "logguid", "", 60)
					err := client.UpsertRoutes([]models.Route{routeWithPath})
					Expect(err).ToNot(HaveOccurred())
				})

				It("is present in the routes list", func() {
					var err error
					routes, err = client.Routes()
					Expect(err).ToNot(HaveOccurred())
					Expect(routes).To(ContainElement(matchers.MatchHttpRoute(routeWithPath)))
				})
			})
		})
	}
	TestTCPRoutes := func() {
		Context("TCP Routes", func() {
			var (
				routerGroupGuid string

				tcpRouteMapping1 models.TcpRouteMapping
				tcpRouteMapping2 models.TcpRouteMapping
			)

			BeforeEach(func() {
				routerGroupGuid = getRouterGroupGuid()
			})

			Context("POST", func() {
				It("allows to create given tcp route mappings", func() {
					client := routing_api.NewClient(fmt.Sprintf("http://127.0.0.1:%d", routingAPIPort), false)
					var err error
					tcpRouteMapping1 = models.NewTcpRouteMapping(routerGroupGuid, 52000, "1.2.3.4", 60000, 60)
					tcpRouteMapping2 = models.NewTcpRouteMapping(routerGroupGuid, 52001, "1.2.3.5", 60001, 3)

					tcpRouteMappings := []models.TcpRouteMapping{tcpRouteMapping1, tcpRouteMapping2}
					err = client.UpsertTcpRouteMappings(tcpRouteMappings)
					Expect(err).NotTo(HaveOccurred())

					Eventually(func() []models.TcpRouteMapping {
						tcpRouteMappingsResponse, err := client.TcpRouteMappings()
						Expect(err).ToNot(HaveOccurred())
						return tcpRouteMappingsResponse
					}, "10s", 1).Should(ConsistOf(matchers.MatchTcpRoute(tcpRouteMapping1)))

				})
			})

			Context("DELETE", func() {
				var (
					tcpRouteMappings []models.TcpRouteMapping
					client           routing_api.Client
					err              error
				)

				BeforeEach(func() {
					client = routing_api.NewClient(fmt.Sprintf("http://127.0.0.1:%d", routingAPIPort), false)
					routerGroupGuid = getRouterGroupGuid()
				})

				JustBeforeEach(func() {
					tcpRouteMapping1 = models.NewTcpRouteMapping(routerGroupGuid, 52000, "1.2.3.4", 60000, 60)
					tcpRouteMapping2 = models.NewTcpRouteMapping(routerGroupGuid, 52001, "1.2.3.5", 60001, 60)
					tcpRouteMappings = []models.TcpRouteMapping{tcpRouteMapping1, tcpRouteMapping2}
					err = client.UpsertTcpRouteMappings(tcpRouteMappings)

					Expect(err).NotTo(HaveOccurred())
				})

				It("allows to delete given tcp route mappings", func() {
					err = client.DeleteTcpRouteMappings(tcpRouteMappings)
					Expect(err).NotTo(HaveOccurred())

					tcpRouteMappingsResponse, err := client.TcpRouteMappings()
					Expect(err).NotTo(HaveOccurred())
					Expect(tcpRouteMappingsResponse).NotTo(BeNil())
					Expect(tcpRouteMappingsResponse).NotTo(ContainElement(matchers.MatchTcpRoute(tcpRouteMapping1)))
					Expect(tcpRouteMappingsResponse).NotTo(ContainElement(matchers.MatchTcpRoute(tcpRouteMapping2)))
				})
			})

			Context("GET (LIST)", func() {
				var (
					tcpRouteMappings []models.TcpRouteMapping
					client           routing_api.Client
				)

				JustBeforeEach(func() {
					client = routing_api.NewClient(fmt.Sprintf("http://127.0.0.1:%d", routingAPIPort), false)

					tcpRouteMapping1 = models.NewTcpRouteMapping(routerGroupGuid, 52000, "1.2.3.4", 60000, 60)
					tcpRouteMapping2 = models.NewTcpRouteMapping(routerGroupGuid, 52001, "1.2.3.5", 60001, 60)
					tcpRouteMappings = []models.TcpRouteMapping{tcpRouteMapping1, tcpRouteMapping2}
					err := client.UpsertTcpRouteMappings(tcpRouteMappings)

					Expect(err).NotTo(HaveOccurred())
				})

				It("allows to retrieve tcp route mappings", func() {
					tcpRouteMappingsResponse, err := client.TcpRouteMappings()
					Expect(err).NotTo(HaveOccurred())
					Expect(tcpRouteMappingsResponse).NotTo(BeNil())
					Expect(tcpRouteMappingsResponse).To(ContainElement(matchers.MatchTcpRoute(tcpRouteMapping1)))
					Expect(tcpRouteMappingsResponse).To(ContainElement(matchers.MatchTcpRoute(tcpRouteMapping2)))
				})
			})
		})
	}

	TestRouterGroups := func() {
		Context("Router Groups", func() {
			Context("GET (LIST)", func() {
				It("returns seeded router groups", func() {
					client := routing_api.NewClient(fmt.Sprintf("http://127.0.0.1:%d", routingAPIPort), false)
					Eventually(func() error {
						_, err := client.RouterGroups()
						return err
					}, "30s", "1s")
					routerGroups, err := client.RouterGroups()
					Expect(err).NotTo(HaveOccurred())
					Expect(len(routerGroups)).To(Equal(1))
					Expect(routerGroups[0].Guid).ToNot(BeNil())
					Expect(routerGroups[0].Name).To(Equal(DefaultRouterGroupName))
					Expect(routerGroups[0].Type).To(Equal(models.RouterGroupType("tcp")))
					Expect(routerGroups[0].ReservablePorts).To(Equal(models.ReservablePorts("1024-65535")))
				})
			})

			Context("PUT", func() {
				It("returns updated router groups", func() {
					var routerGroups models.RouterGroups
					client = routing_api.NewClient(fmt.Sprintf("http://127.0.0.1:%d", routingAPIPort), false)
					Eventually(func() error {
						var err error
						routerGroups, err = client.RouterGroups()
						return err
					}, "30s", "1s").ShouldNot(HaveOccurred(), "Failed to connect to Routing API server after 30s.")
					Expect(len(routerGroups)).To(Equal(1))
					routerGroup := routerGroups[0]

					routerGroup.ReservablePorts = "6000-8000"
					err := client.UpdateRouterGroup(routerGroup)
					Expect(err).NotTo(HaveOccurred())

					routerGroups, err = client.RouterGroups()
					Expect(err).NotTo(HaveOccurred())
					Expect(len(routerGroups)).To(Equal(1))
					Expect(routerGroups[0].ReservablePorts).To(Equal(models.ReservablePorts("6000-8000")))
				})
			})
		})
	}

	Describe("API with MySQL", func() {
		var routingAPIProcess ifrit.Process

		BeforeEach(func() {
			routingAPIRunner := testrunner.New(routingAPIBinPath, routingAPIArgs)
			routingAPIProcess = ginkgomon.Invoke(routingAPIRunner)
		})

		AfterEach(func() {
			ginkgomon.Kill(routingAPIProcess)
		})

		TestRouterGroups()
		TestTCPRoutes()
		TestTCPEvents()
		TestHTTPRoutes()
		TestHTTPEvents()
	})

	Describe("API with ETCD", func() {
		var routingAPIProcess ifrit.Process

		BeforeEach(func() {
			routingAPIRunner := testrunner.New(routingAPIBinPath, routingAPIArgsNoSQL)
			routingAPIProcess = ginkgomon.Invoke(routingAPIRunner)
		})

		AfterEach(func() {
			ginkgomon.Kill(routingAPIProcess)
		})

		TestHTTPEvents()
		TestHTTPRoutes()
		TestTCPRoutes()
		TestTCPEvents()
		TestRouterGroups()
	})
})
