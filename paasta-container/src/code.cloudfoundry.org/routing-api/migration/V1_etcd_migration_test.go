package migration_test

import (
	"path"
	"path/filepath"
	"time"

	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/routing-api/cmd/routing-api/testrunner"
	"code.cloudfoundry.org/routing-api/config"
	"code.cloudfoundry.org/routing-api/db"
	"code.cloudfoundry.org/routing-api/matchers"
	"code.cloudfoundry.org/routing-api/migration"
	"code.cloudfoundry.org/routing-api/models"

	"github.com/cloudfoundry/storeadapter/storerunner/etcdstorerunner"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("V1EtcdMigration", func() {
	var (
		etcd           db.DB
		sqlDB          *db.SqlDB
		etcdRunner     *etcdstorerunner.ETCDClusterRunner
		mysqlAllocator testrunner.DbAllocator
		etcdConfig     *config.Etcd
		done           chan struct{}
		logger         lager.Logger
	)
	BeforeEach(func() {
		mysqlAllocator = testrunner.NewMySQLAllocator()
		mysqlSchema, err := mysqlAllocator.Create()
		Expect(err).NotTo(HaveOccurred())

		logger = lager.NewLogger("test-logger")

		sqlCfg := &config.SqlDB{
			Username: "root",
			Password: "password",
			Schema:   mysqlSchema,
			Host:     "localhost",
			Port:     3306,
			Type:     "mysql",
		}

		sqlDB, err = db.NewSqlDB(sqlCfg)
		Expect(err).ToNot(HaveOccurred())

		v0Migration := migration.NewV0InitMigration()
		err = v0Migration.Run(sqlDB)
		Expect(err).ToNot(HaveOccurred())

		etcdConfig = &config.Etcd{}

		done = make(chan struct{})
	})

	AfterEach(func() {
		err := mysqlAllocator.Delete()
		Expect(err).ToNot(HaveOccurred())
	})

	Context("when etcd is configured", func() {
		BeforeEach(func() {
			basePath, err := filepath.Abs(path.Join("..", "fixtures", "etcd-certs"))
			Expect(err).NotTo(HaveOccurred())

			serverSSLConfig := &etcdstorerunner.SSLConfig{
				CertFile: filepath.Join(basePath, "server.crt"),
				KeyFile:  filepath.Join(basePath, "server.key"),
				CAFile:   filepath.Join(basePath, "etcd-ca.crt"),
			}

			etcdPort := 4001 + GinkgoParallelNode()
			etcdRunner = etcdstorerunner.NewETCDClusterRunner(etcdPort, 1, serverSSLConfig)
			etcdRunner.Start()

			etcdConfig = &config.Etcd{
				RequireSSL: true,
				CertFile:   filepath.Join(basePath, "client.crt"),
				KeyFile:    filepath.Join(basePath, "client.key"),
				CAFile:     filepath.Join(basePath, "etcd-ca.crt"),
				NodeURLS:   etcdRunner.NodeURLS(),
			}

			etcd, err = db.NewETCD(etcdConfig)
			Expect(err).NotTo(HaveOccurred())
		})

		AfterEach(func() {
			select {
			case <-done:
			default:
				close(done)
			}

			etcdRunner.Stop()
			etcdRunner.KillWithFire()
			etcdRunner.GoAway()
		})

		Context("when there are events in etcd after migration started", func() {
			BeforeEach(func() {
				etcdMigration := migration.NewV1EtcdMigration(etcdConfig, done, logger)
				err := etcdMigration.Run(sqlDB)
				Expect(err).ToNot(HaveOccurred())
			})

			Context("when there are http events", func() {
				Context("when there are no expired events", func() {
					BeforeEach(func() {
						savedRoute := models.NewRoute("/route", 8333, "127.0.0.1", "log_guid", "rs", 10)
						err := etcd.SaveRoute(savedRoute)
						Expect(err).NotTo(HaveOccurred())
						Eventually(func() []models.Route {
							rt, _ := etcd.ReadRoutes()
							return rt
						}, "5s").Should(HaveLen(1))
					})

					It("should migrate http routes to sql database", func() {
						Eventually(func() []models.Route {
							routes, err := sqlDB.ReadRoutes()
							Expect(err).ToNot(HaveOccurred())
							return routes
						}, "10s", "500ms").Should(HaveLen(1))

					})
				})

				Context("when there are expired events", func() {
					var savedRoute models.Route
					BeforeEach(func() {
						savedRoute = models.NewRoute("/route", 8333, "127.0.0.1", "log_guid", "rs", 2)
						err := etcd.SaveRoute(savedRoute)
						Expect(err).NotTo(HaveOccurred())
					})

					It("should not migrate http route expirations to sql database", func() {
						var (
							routes []models.Route
							err    error
						)
						Eventually(func() []models.Route {
							routes, err = sqlDB.ReadRoutes()
							Expect(err).ToNot(HaveOccurred())
							return routes
						}, "5s", "100ms").Should(HaveLen(1))

						guid := routes[0].Guid

						ttl := 30
						savedRoute.TTL = &ttl
						err = sqlDB.SaveRoute(savedRoute)
						Expect(err).ToNot(HaveOccurred())

						Eventually(etcd.ReadRoutes, "3s").Should(HaveLen(0))

						Consistently(func() []models.Route {
							routes, err = sqlDB.ReadRoutes()
							Expect(err).ToNot(HaveOccurred())
							return routes
						}).Should(HaveLen(1))
						Expect(routes[0].Guid).To(Equal(guid))
					})
				})
			})

			Context("when there are tcp events", func() {
				Context("when there are no expired events", func() {
					BeforeEach(func() {
						tcpRoute := models.NewTcpRouteMapping("router-group-guid", 3056, "127.0.0.1", 2990, 30)
						err := etcd.SaveTcpRouteMapping(tcpRoute)
						Expect(err).NotTo(HaveOccurred())
					})

					It("should migrate tcp routes to sql database", func() {
						Eventually(func() []models.TcpRouteMapping {
							routes, err := sqlDB.ReadTcpRouteMappings()
							Expect(err).ToNot(HaveOccurred())
							return routes
						}, "3s", "500ms").Should(HaveLen(1))

					})
				})
				Context("when there are expired events", func() {
					var savedRoute models.TcpRouteMapping
					BeforeEach(func() {
						savedRoute = models.NewTcpRouteMapping("router-group-guid", 3056, "127.0.0.1", 2990, 2)
						err := etcd.SaveTcpRouteMapping(savedRoute)
						Expect(err).NotTo(HaveOccurred())
					})

					It("should not migrate tcp route expirations to sql database", func() {
						var (
							routes []models.TcpRouteMapping
							err    error
						)
						Eventually(func() []models.TcpRouteMapping {
							routes, err = sqlDB.ReadTcpRouteMappings()
							Expect(err).ToNot(HaveOccurred())
							return routes
						}, "5s", "100ms").Should(HaveLen(1))

						guid := routes[0].Guid

						ttl := 30
						savedRoute.TTL = &ttl
						err = sqlDB.SaveTcpRouteMapping(savedRoute)
						Expect(err).ToNot(HaveOccurred())

						Eventually(etcd.ReadRoutes).Should(HaveLen(0))

						Consistently(func() []models.TcpRouteMapping {
							routes, err = sqlDB.ReadTcpRouteMappings()
							Expect(err).ToNot(HaveOccurred())
							return routes
						}).Should(HaveLen(1))

						Expect(routes[0].Guid).To(Equal(guid))
					})

				})
			})

			Context("when there are router group events", func() {
				var rg models.RouterGroup
				BeforeEach(func() {
					rg = models.RouterGroup{
						Guid:            "some-guid",
						Name:            "some-name",
						Type:            "mysql",
						ReservablePorts: "1200",
					}
					err := sqlDB.SaveRouterGroup(rg)
					Expect(err).NotTo(HaveOccurred())
					err = etcd.SaveRouterGroup(rg)
					Expect(err).NotTo(HaveOccurred())
				})

				Context("when there are updates to router groups", func() {
					BeforeEach(func() {
						//update port range to create etcd update event
						rg.ReservablePorts = "1400"
						err := etcd.SaveRouterGroup(rg)
						Expect(err).NotTo(HaveOccurred())
					})

					It("should migrate events to sql database", func() {
						Eventually(func() string {
							rg, err := sqlDB.ReadRouterGroup("some-guid")
							Expect(err).ToNot(HaveOccurred())
							return string(rg.ReservablePorts)
						}, "3s", "500ms").Should(Equal("1400"))
					})
				})

				Context("when a router group is created in etcd", func() {
					BeforeEach(func() {
						//update port range to create etcd update event
						rg1 := models.RouterGroup{
							Guid:            "some-guid-1",
							Name:            "some-name-1",
							Type:            "mysql",
							ReservablePorts: "1200",
						}
						err := etcd.SaveRouterGroup(rg1)
						Expect(err).NotTo(HaveOccurred())
					})

					It("should not create the router group in the sql database", func() {
						Consistently(func() string {
							rg, err := sqlDB.ReadRouterGroup("some-guid-1")
							Expect(err).ToNot(HaveOccurred())
							return rg.Name
						}, "3s", "500ms").Should(BeEmpty())
					})
				})
			})

			Context("when the migration is signaled to stop", func() {
				BeforeEach(func() {
					close(done)
				})
				Context("with http route events", func() {
					BeforeEach(func() {
						savedRoute := models.NewRoute("/route", 8333, "127.0.0.1", "log_guid", "rs", 10)
						err := etcd.SaveRoute(savedRoute)
						Expect(err).NotTo(HaveOccurred())
						Eventually(func() []models.Route {
							rt, _ := etcd.ReadRoutes()
							return rt
						}, "5s").Should(HaveLen(1))
					})
					It("should no longer migrate http routes to sql", func() {
						Consistently(func() []models.Route {
							routes, err := sqlDB.ReadRoutes()
							Expect(err).ToNot(HaveOccurred())
							return routes
						}, "3s", "500ms").Should(BeEmpty())
					})
				})
				Context("with tcp route events", func() {
					BeforeEach(func() {
						tcpRoute := models.NewTcpRouteMapping("router-group-guid", 3056, "127.0.0.1", 2990, 30)
						err := etcd.SaveTcpRouteMapping(tcpRoute)
						Expect(err).NotTo(HaveOccurred())
					})

					It("should no longer migrate tcp routes to sql", func() {
						Consistently(func() []models.TcpRouteMapping {
							routes, err := sqlDB.ReadTcpRouteMappings()
							Expect(err).ToNot(HaveOccurred())
							return routes
						}, "3s", "500ms").Should(BeEmpty())

					})
				})
				Context("with router group events", func() {
					var rg models.RouterGroup
					BeforeEach(func() {
						rg = models.RouterGroup{
							Guid:            "some-guid",
							Name:            "some-name",
							Type:            "mysql",
							ReservablePorts: "1200",
						}
						err := sqlDB.SaveRouterGroup(rg)
						Expect(err).NotTo(HaveOccurred())
						err = etcd.SaveRouterGroup(rg)
						Expect(err).NotTo(HaveOccurred())

						rg.ReservablePorts = "1400"
						err = etcd.SaveRouterGroup(rg)
						Expect(err).NotTo(HaveOccurred())
					})

					It("should no longer migrate router group updates to sql", func() {
						Consistently(func() string {
							rg, err := sqlDB.ReadRouterGroup("some-guid")
							Expect(err).ToNot(HaveOccurred())
							return string(rg.ReservablePorts)
						}, "3s", "500ms").Should(Equal("1200"))
					})
				})
			})
		})

		Context("with router groups in etcd", func() {
			var savedRouterGroup models.RouterGroup
			BeforeEach(func() {
				savedRouterGroup = models.RouterGroup{
					Name:            "router-group-1",
					Type:            "tcp",
					Guid:            "1234567890",
					ReservablePorts: "10-20,25",
				}
				err := etcd.SaveRouterGroup(savedRouterGroup)
				Expect(err).NotTo(HaveOccurred())
			})
			It("should successfully migrate router groups to mysql", func() {
				etcdMigration := migration.NewV1EtcdMigration(etcdConfig, done, logger)
				err := etcdMigration.Run(sqlDB)
				Expect(err).ToNot(HaveOccurred())

				rg, err := sqlDB.ReadRouterGroup("1234567890")
				Expect(err).ToNot(HaveOccurred())
				Expect(rg).To(matchers.MatchRouterGroup(savedRouterGroup))
			})
		})

		Context("with http routes in etcd", func() {
			var savedRoute models.Route
			BeforeEach(func() {
				savedRoute = models.NewRoute("/route", 8333, "127.0.0.1", "log_guid", "rs", 10)
				for i := 0; i < 3; i += 1 {
					err := etcd.SaveRoute(savedRoute)
					Expect(err).NotTo(HaveOccurred())
				}
			})

			It("should successfully migrate http routes to mysql and retain their expiration", func() {
				time.Sleep(3 * time.Second)

				etcdMigration := migration.NewV1EtcdMigration(etcdConfig, done, logger)
				err := etcdMigration.Run(sqlDB)
				Expect(err).ToNot(HaveOccurred())

				routes, err := sqlDB.ReadRoutes()
				Expect(err).ToNot(HaveOccurred())
				Expect(routes).To(HaveLen(1))
				Expect(routes[0]).To(matchers.MatchHttpRoute(savedRoute))
				Expect(int(routes[0].ModificationTag.Index)).To(Equal(2))

				etcdRoutes, err := etcd.ReadRoutes()
				Expect(err).ToNot(HaveOccurred())
				Expect(routes[0].ExpiresAt.Second()).To(BeNumerically("~", etcdRoutes[0].ExpiresAt.Second(), 1))
			})
		})

		Context("with tcp routes in etcd", func() {
			var tcpRoute models.TcpRouteMapping
			BeforeEach(func() {
				tcpRoute = models.NewTcpRouteMapping("router-group-guid", 3056, "127.0.0.1", 2990, 30)
				for i := 0; i < 3; i += 1 {
					err := etcd.SaveTcpRouteMapping(tcpRoute)
					Expect(err).NotTo(HaveOccurred())
				}
			})

			It("should successfully migrate tcp routes to mysql and retain its expiration", func() {
				time.Sleep(3 * time.Second)
				etcdMigration := migration.NewV1EtcdMigration(etcdConfig, done, logger)
				err := etcdMigration.Run(sqlDB)
				Expect(err).ToNot(HaveOccurred())

				routes, err := sqlDB.ReadTcpRouteMappings()
				Expect(err).ToNot(HaveOccurred())
				Expect(routes).To(HaveLen(1))
				Expect(routes[0]).To(matchers.MatchTcpRoute(tcpRoute))
				Expect(int(routes[0].ModificationTag.Index)).To(Equal(2))

				etcdRoutes, err := etcd.ReadTcpRouteMappings()
				Expect(err).ToNot(HaveOccurred())
				Expect(routes[0].ExpiresAt.Second()).To(BeNumerically("~", etcdRoutes[0].ExpiresAt.Second(), 1))
			})
		})
	})

	Context("when etcd is not configured", func() {
		It("should not error when run", func() {
			etcdMigration := migration.NewV1EtcdMigration(etcdConfig, done, logger)
			err := etcdMigration.Run(sqlDB)
			Expect(err).ToNot(HaveOccurred())

			tcpRoutes, err := sqlDB.ReadTcpRouteMappings()
			Expect(err).ToNot(HaveOccurred())
			Expect(tcpRoutes).To(BeEmpty())

			routes, err := sqlDB.ReadRoutes()
			Expect(err).ToNot(HaveOccurred())
			Expect(routes).To(BeEmpty())

			routerGroups, err := sqlDB.ReadRouterGroups()
			Expect(err).ToNot(HaveOccurred())
			Expect(routerGroups).To(BeEmpty())
		})
	})
})
