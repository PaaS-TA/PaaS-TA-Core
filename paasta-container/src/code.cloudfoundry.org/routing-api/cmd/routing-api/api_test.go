package main_test

import (
	"fmt"

	"github.com/tedsuo/ifrit"
	"github.com/tedsuo/ifrit/ginkgomon"

	"code.cloudfoundry.org/routing-api"
	. "code.cloudfoundry.org/routing-api/cmd/routing-api/test_helpers"
	"code.cloudfoundry.org/routing-api/cmd/routing-api/testrunner"
	"code.cloudfoundry.org/routing-api/config"
	"code.cloudfoundry.org/routing-api/db"
	"code.cloudfoundry.org/routing-api/models"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Routes API", func() {
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

				Expect(routes).To(HaveLen(3))
				Expect(Routes(routes).ContainsAll(route1, route2, routingAPIRoute)).To(BeTrue())
			})

			It("deletes a route", func() {
				err := client.DeleteRoutes([]models.Route{route1})

				Expect(err).NotTo(HaveOccurred())

				routes, err = client.Routes()
				Expect(err).NotTo(HaveOccurred())
				Expect(Routes(routes).Contains(route1)).To(BeFalse())
			})

			It("rejects bad routes", func() {
				route3 := models.NewRoute("foo/b ar", 35, "2.2.2.2", "banana", "", 66)

				err := client.UpsertRoutes([]models.Route{route3})
				Expect(err).To(HaveOccurred())

				routes, err = client.Routes()

				Expect(err).ToNot(HaveOccurred())
				Expect(Routes(routes).Contains(route1)).To(BeTrue())
				Expect(Routes(routes).Contains(route2)).To(BeTrue())
				Expect(Routes(routes).Contains(route3)).To(BeFalse())
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
					Expect(Routes(routes).Contains(routeWithPath)).To(BeTrue())
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

			getRouterGroupGuid := func() string {
				client := routing_api.NewClient(fmt.Sprintf("http://127.0.0.1:%d", routingAPIPort), false)
				routerGroups, err := client.RouterGroups()
				Expect(err).NotTo(HaveOccurred())
				Expect(routerGroups).ToNot(HaveLen(0))
				return routerGroups[0].Guid
			}

			BeforeEach(func() {
				routerGroupGuid = getRouterGroupGuid()
			})
			Context("POST", func() {
				It("allows to create given tcp route mappings", func() {
					client := routing_api.NewClient(fmt.Sprintf("http://127.0.0.1:%d", routingAPIPort), false)
					var err error
					tcpRouteMapping1 = models.NewTcpRouteMapping(routerGroupGuid, 52000, "1.2.3.4", 60000, 60)
					tcpRouteMapping2 = models.NewTcpRouteMapping(routerGroupGuid, 52001, "1.2.3.5", 60001, 1)

					tcpRouteMappings := []models.TcpRouteMapping{tcpRouteMapping1, tcpRouteMapping2}
					err = client.UpsertTcpRouteMappings(tcpRouteMappings)
					Expect(err).NotTo(HaveOccurred())
					tcpRouteMappingsResponse, err := client.TcpRouteMappings()
					Expect(err).NotTo(HaveOccurred())
					Expect(tcpRouteMappingsResponse).NotTo(BeNil())
					mappings := TcpRouteMappings(tcpRouteMappingsResponse)
					Expect(mappings.ContainsAll(tcpRouteMappings...)).To(BeTrue())

					By("letting route expire")
					Eventually(func() bool {
						tcpRouteMappingsResponse, err := client.TcpRouteMappings()
						Expect(err).NotTo(HaveOccurred())
						mappings := TcpRouteMappings(tcpRouteMappingsResponse)
						return mappings.Contains(tcpRouteMapping2)
					}, 3, 1).Should(BeFalse())
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
					err := client.DeleteTcpRouteMappings(tcpRouteMappings)
					Expect(err).NotTo(HaveOccurred())

					tcpRouteMappingsResponse, err := client.TcpRouteMappings()
					Expect(err).NotTo(HaveOccurred())
					Expect(tcpRouteMappingsResponse).NotTo(BeNil())
					Expect(tcpRouteMappingsResponse).NotTo(ConsistOf(tcpRouteMappings))
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
					Expect(TcpRouteMappings(tcpRouteMappingsResponse).ContainsAll(tcpRouteMappings...)).To(BeTrue())
				})
			})
		})
	}
	TestRouterGroups := func() {
		Context("Router Groups", func() {
			Context("GET (LIST)", func() {
				It("returns seeded router groups", func() {
					client := routing_api.NewClient(fmt.Sprintf("http://127.0.0.1:%d", routingAPIPort), false)
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
					client = routing_api.NewClient(fmt.Sprintf("http://127.0.0.1:%d", routingAPIPort), false)
					routerGroups, err := client.RouterGroups()
					Expect(err).NotTo(HaveOccurred())
					Expect(len(routerGroups)).To(Equal(1))
					routerGroup := routerGroups[0]

					routerGroup.ReservablePorts = "6000-8000"
					err = client.UpdateRouterGroup(routerGroup)
					Expect(err).NotTo(HaveOccurred())

					routerGroups, err = client.RouterGroups()
					Expect(err).NotTo(HaveOccurred())
					Expect(len(routerGroups)).To(Equal(1))
					Expect(routerGroups[0].ReservablePorts).To(Equal(models.ReservablePorts("6000-8000")))
				})
			})
		})
	}

	cleanupSQLRouterGroups := func() {
		sqlCfg := config.SqlDB{
			Username: "root",
			Password: "password",
			Schema:   sqlDBName,
			Host:     "localhost",
			Port:     3306,
			Type:     "mysql",
		}
		dbSQLInterface, err := db.NewSqlDB(&sqlCfg)
		Expect(err).ToNot(HaveOccurred())
		dbSQL := dbSQLInterface.(*db.SqlDB)
		dbSQL.Client.Where("type = ?", "tcp").Delete(models.RouterGroupsDB{})
	}

	Describe("API with ETCD + MySQL", func() {
		var routingAPIProcess ifrit.Process

		BeforeEach(func() {
			routingAPIRunner := testrunner.New(routingAPIBinPath, routingAPIArgs)
			routingAPIProcess = ginkgomon.Invoke(routingAPIRunner)
		})

		AfterEach(func() {
			ginkgomon.Kill(routingAPIProcess)
			cleanupSQLRouterGroups()
		})

		TestTCPRoutes()
	})

	Describe("API with MySQL Only", func() {
		var routingAPIProcess ifrit.Process

		BeforeEach(func() {
			routingAPIRunner := testrunner.New(routingAPIBinPath, routingAPIArgs)
			routingAPIProcess = ginkgomon.Invoke(routingAPIRunner)
		})

		AfterEach(func() {
			ginkgomon.Kill(routingAPIProcess)
			cleanupSQLRouterGroups()
		})

		TestRouterGroups()
	})

	Describe("API with ETCD Only", func() {
		var routingAPIProcess ifrit.Process

		BeforeEach(func() {
			routingAPIRunner := testrunner.New(routingAPIBinPath, routingAPIArgsNoSQL)
			routingAPIProcess = ginkgomon.Invoke(routingAPIRunner)
		})

		AfterEach(func() {
			ginkgomon.Kill(routingAPIProcess)
		})

		TestHTTPRoutes()
		TestTCPRoutes()
		TestRouterGroups()
	})
})
