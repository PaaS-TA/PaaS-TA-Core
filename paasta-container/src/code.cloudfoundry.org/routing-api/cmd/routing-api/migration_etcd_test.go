package main_test

import (
	"path"
	"path/filepath"

	"code.cloudfoundry.org/routing-api/cmd/routing-api/testrunner"
	"code.cloudfoundry.org/routing-api/config"
	"code.cloudfoundry.org/routing-api/db"
	"code.cloudfoundry.org/routing-api/matchers"
	"code.cloudfoundry.org/routing-api/models"
	"github.com/tedsuo/ifrit"
	"github.com/tedsuo/ifrit/ginkgomon"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("ETCD Migrations", func() {
	var (
		etcdClient        db.DB
		routingAPIProcess ifrit.Process
		etcdRouterGroups  []models.RouterGroup
	)

	BeforeEach(func() {
		var err error
		basePath, _ := filepath.Abs(path.Join("..", "..", "fixtures", "etcd-certs"))
		Expect(err).ToNot(HaveOccurred())
		etcdConfig := config.Etcd{
			RequireSSL: true,
			CertFile:   filepath.Join(basePath, "client.crt"),
			KeyFile:    filepath.Join(basePath, "client.key"),
			CAFile:     filepath.Join(basePath, "etcd-ca.crt"),
			NodeURLS:   []string{etcdUrl},
		}
		etcdClient, err = db.NewETCD(&etcdConfig)
		Expect(err).NotTo(HaveOccurred())

	})
	JustBeforeEach(func() {
		routingAPIRunner := testrunner.New(routingAPIBinPath, routingAPIArgs)
		routingAPIProcess = ginkgomon.Invoke(routingAPIRunner)
		Eventually(routingAPIProcess.Ready(), "5s").Should(BeClosed())
	})

	AfterEach(func() {
		ginkgomon.Kill(routingAPIProcess)

	})

	Context("when etcd already has router groups", func() {
		BeforeEach(func() {
			var err error
			routerGroup := models.RouterGroup{
				Name: "some-name", Type: "tcp", Guid: "some-guid", ReservablePorts: "2000",
			}

			err = etcdClient.SaveRouterGroup(routerGroup)
			Expect(err).ToNot(HaveOccurred())

			etcdRouterGroups, err = etcdClient.ReadRouterGroups()
			Expect(err).ToNot(HaveOccurred())
		})

		It("migrates all router groups with the original guids", func() {
			Eventually(func() error {
				_, err := client.RouterGroups()
				return err
			}, "60s", "1s").Should(BeNil())
			routerGroups, err := client.RouterGroups()
			Expect(err).NotTo(HaveOccurred())
			Expect(len(routerGroups)).To(Equal(1))
			Expect(routerGroups[0]).To(matchers.MatchRouterGroup(etcdRouterGroups[0]))
		})

		Context("when routes already exist", func() {
			var (
				tcpRoute models.TcpRouteMapping
				route    models.Route
				err      error
			)
			BeforeEach(func() {
				Expect(len(etcdRouterGroups)).To(Equal(1))
				routerGroupGuid := etcdRouterGroups[0].Guid
				tcpRoute = models.NewTcpRouteMapping(routerGroupGuid, 52001, "1.2.3.5", 60001, 30)
				route = models.NewRoute("a.b.c", 33, "1.1.1.1", "potato", "", 55)

				err = etcdClient.SaveTcpRouteMapping(tcpRoute)
				Expect(err).ToNot(HaveOccurred())
				err = etcdClient.SaveRoute(route)
				Expect(err).ToNot(HaveOccurred())

				routes, err := etcdClient.ReadRoutes()
				Expect(err).ToNot(HaveOccurred())
				route = routes[0]

				tcpRoutes, err := etcdClient.ReadTcpRouteMappings()
				Expect(err).ToNot(HaveOccurred())
				tcpRoute = tcpRoutes[0]
			})
			It("migrates all routes", func() {
				tcpRouteMappings, err := client.TcpRouteMappings()
				Expect(err).NotTo(HaveOccurred())
				Expect(tcpRouteMappings).NotTo(BeNil())
				Expect(tcpRouteMappings).To(ConsistOf(matchers.MatchTcpRoute(tcpRoute)))

				routes, err := client.Routes()
				Expect(err).ToNot(HaveOccurred())

				Expect(routes).To(ContainElement(matchers.MatchHttpRoute(route)))
			})
		})
	})
})
