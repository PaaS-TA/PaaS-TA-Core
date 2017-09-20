package main_test

import (
	"fmt"
	"net"
	"os"
	"os/exec"
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
	"github.com/onsi/gomega/gbytes"
)

var _ = Describe("ETCD Event Migrations", func() {
	var (
		etcdClient        db.DB
		routingAPIProcess ifrit.Process
		routingAPIRunner  *ginkgomon.Runner
		lockHolderProcess ifrit.Process
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

		routerGroup := models.RouterGroup{
			Name: "some-name", Type: "tcp", Guid: "some-guid", ReservablePorts: "2000",
		}

		err = etcdClient.SaveRouterGroup(routerGroup)
		Expect(err).ToNot(HaveOccurred())

		etcdRouterGroups, err = etcdClient.ReadRouterGroups()
		Expect(err).ToNot(HaveOccurred())

		lockHolderArgs := routingAPIArgsNoSQL
		lockHolderArgs.Port = uint16(7000 + GinkgoParallelNode())
		lockHolderRunner := testrunner.New(routingAPIBinPath, lockHolderArgs)
		validatePort(lockHolderArgs.Port)
		lockHolderProcess = ginkgomon.Invoke(lockHolderRunner)

		routingAPIRunner = ginkgomon.New(ginkgomon.Config{
			Name:       "routing-api",
			Command:    exec.Command(routingAPIBinPath, routingAPIArgs.ArgSlice()...),
			StartCheck: "routing-api.lock.acquiring-lock",
		})
		validatePort(routingAPIArgs.Port)
		routingAPIProcess = ginkgomon.Invoke(routingAPIRunner)
		Eventually(routingAPIProcess.Ready(), "5s").Should(BeClosed())
	})

	AfterEach(func() {
		ginkgomon.Kill(lockHolderProcess)
		ginkgomon.Kill(routingAPIProcess)
	})

	Context("when another routing API process is currently the active node", func() {
		It("migrates route changes from etcd to SQL", func() {
			Expect(len(etcdRouterGroups)).To(Equal(1))
			routerGroupGuid := etcdRouterGroups[0].Guid
			tcpRoute := models.NewTcpRouteMapping(routerGroupGuid, 52001, "1.2.3.5", 60001, 30)
			route := models.NewRoute("a.b.c", 33, "1.1.1.1", "potato", "", 55)

			err := etcdClient.SaveTcpRouteMapping(tcpRoute)
			Expect(err).ToNot(HaveOccurred())
			err = etcdClient.SaveRoute(route)
			Expect(err).ToNot(HaveOccurred())

			Consistently(func() []models.Route {
				routes, err := etcdClient.ReadRoutes()
				Expect(err).ToNot(HaveOccurred())
				return routes
			}, "3s", "500ms").Should(ContainElement(matchers.MatchHttpRoute(route)))

			Consistently(func() []models.TcpRouteMapping {
				tcpRoutes, err := etcdClient.ReadTcpRouteMappings()
				Expect(err).ToNot(HaveOccurred())
				return tcpRoutes
			}, "3s", "500ms").Should(ConsistOf(matchers.MatchTcpRoute(tcpRoute)))

			lockHolderProcess.Signal(os.Interrupt)
			Eventually(routingAPIRunner.Buffer()).Should(gbytes.Say(`routing-api.started`))

			Eventually(func() []models.TcpRouteMapping {
				tcpRouteMappings, err := client.TcpRouteMappings()
				Expect(err).NotTo(HaveOccurred())
				return tcpRouteMappings
			}).Should(ConsistOf(matchers.MatchTcpRoute(tcpRoute)))

			Eventually(func() []models.Route {
				httpRoutes, err := client.Routes()
				Expect(err).ToNot(HaveOccurred())
				return httpRoutes
			}).Should(ContainElement(matchers.MatchHttpRoute(route)))
		})
	})

	Context("the current Routing API connected to SQL is the active node", func() {
		BeforeEach(func() {
			lockHolderProcess.Signal(os.Interrupt)

			Eventually(routingAPIRunner.Buffer()).Should(gbytes.Say(`routing-api.started`))
			err := <-lockHolderProcess.Wait()
			Expect(err).ToNot(HaveOccurred())
		})

		It("does migrate route changes from etcd to SQL", func() {
			Expect(len(etcdRouterGroups)).To(Equal(1))
			routerGroupGuid := etcdRouterGroups[0].Guid
			tcpRoute := models.NewTcpRouteMapping(routerGroupGuid, 52001, "1.2.3.5", 60001, 30)
			route := models.NewRoute("a.b.c", 33, "1.1.1.1", "potato", "", 55)

			err := etcdClient.SaveTcpRouteMapping(tcpRoute)
			Expect(err).ToNot(HaveOccurred())
			err = etcdClient.SaveRoute(route)
			Expect(err).ToNot(HaveOccurred())

			routes, err := etcdClient.ReadRoutes()
			Expect(err).ToNot(HaveOccurred())
			route = routes[0]

			tcpRoutes, err := etcdClient.ReadTcpRouteMappings()
			Expect(err).ToNot(HaveOccurred())
			tcpRoute = tcpRoutes[0]

			Eventually(func() []models.TcpRouteMapping {
				tcpRouteMappings, err := client.TcpRouteMappings()
				Expect(err).NotTo(HaveOccurred())
				return tcpRouteMappings
			}).Should(ConsistOf(matchers.MatchTcpRoute(tcpRoute)))

			Eventually(func() []models.Route {
				httpRoutes, err := client.Routes()
				Expect(err).ToNot(HaveOccurred())
				return httpRoutes
			}).Should(ContainElement(matchers.MatchHttpRoute(route)))
		})
	})
})

func validatePort(port uint16) {
	Eventually(func() error {
		l, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
		if l != nil {
			_ = l.Close()
		}
		return err
	}, "60s", "1s").Should(BeNil())
}
