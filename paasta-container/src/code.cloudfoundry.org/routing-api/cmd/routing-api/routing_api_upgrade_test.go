package main_test

import (
	"fmt"
	"os"

	"code.cloudfoundry.org/routing-api"
	"code.cloudfoundry.org/routing-api/cmd/routing-api/testrunner"
	"code.cloudfoundry.org/routing-api/matchers"
	"code.cloudfoundry.org/routing-api/models"
	"github.com/tedsuo/ifrit"
	"github.com/tedsuo/ifrit/ginkgomon"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("RoutingApiUpgrade", func() {
	var (
		oldRoutingAPIProcess    ifrit.Process
		routingAPIProcess       ifrit.Process
		routingAPINoETCDProcess ifrit.Process
		etcdRouterGroups        []models.RouterGroup
		tcpRoute                models.TcpRouteMapping
		route                   models.Route
		err                     error
	)

	BeforeEach(func() {
		oldRoutingAPIBinPath := os.Getenv("OLD_ROUTING_API_BIN_PATH")
		if oldRoutingAPIBinPath == "" {
			Skip("Skipping Upgrade Test: OLD_ROUTING_API_BIN_PATH not set")
		}
		if _, err := os.Stat(oldRoutingAPIBinPath); os.IsNotExist(err) {
			Skip(fmt.Sprintf("Skipping Upgrade Test: Cannot find %s", oldRoutingAPIBinPath))
		}

		// Bring up an old version of routing_api
		oldRoutingAPIRunner := testrunner.New(oldRoutingAPIBinPath, routingAPIArgsNoSQL)
		oldRoutingAPIProcess = ginkgomon.Invoke(oldRoutingAPIRunner)
		Eventually(oldRoutingAPIProcess.Ready(), "5s").Should(BeClosed())

		Eventually(func() error {
			_, err := client.RouterGroups()
			return err
		}, "60s", "1s").Should(BeNil())
	})

	AfterEach(func() {
		ginkgomon.Interrupt(routingAPINoETCDProcess)
	})

	It("migrates successfully", func() {

		// Verify Router Group Exists and grab GUID
		etcdRouterGroups, err = client.RouterGroups()
		Expect(err).ToNot(HaveOccurred())
		Expect(etcdRouterGroups).To(HaveLen(1))

		routerGroupGUID := etcdRouterGroups[0].Guid

		// Create a tcp and http route
		tcpRoute = models.NewTcpRouteMapping(routerGroupGUID, 52001, "1.2.3.5", 60001, 120)
		route = models.NewRoute("a.b.c", 33, "1.1.1.1", "potato", "", 120)

		err = client.UpsertTcpRouteMappings([]models.TcpRouteMapping{tcpRoute})
		Expect(err).ToNot(HaveOccurred())
		err = client.UpsertRoutes([]models.Route{route})
		Expect(err).ToNot(HaveOccurred())

		etcdRoutes, err := client.Routes()
		Expect(err).ToNot(HaveOccurred())
		Expect(etcdRoutes).To(HaveLen(2)) //http route + routing-api route

		etcdTCPRoutes, err := client.TcpRouteMappings()
		Expect(err).ToNot(HaveOccurred())
		Expect(etcdTCPRoutes).To(HaveLen(1))

		// Kill Old Routing API
		ginkgomon.Interrupt(oldRoutingAPIProcess)

		// Bring up Routing API with etcd and sql
		routingAPIRunner := testrunner.New(routingAPIBinPath, routingAPIArgs)
		routingAPIProcess = ginkgomon.Invoke(routingAPIRunner)
		Eventually(routingAPIProcess.Ready(), "5s").Should(BeClosed())
		Eventually(func() error {
			_, err := client.RouterGroups()
			return err
		}, "60s", "1s").Should(BeNil())

		verifyMigration(client, etcdRouterGroups, etcdRoutes, etcdTCPRoutes)

		// Kill Old Routing API
		ginkgomon.Interrupt(routingAPIProcess)

		// Bring up Routing API with sql and NO etcd
		routingAPIRunner = testrunner.New(routingAPIBinPath, routingAPIArgsOnlySQL)
		routingAPINoETCDProcess := ginkgomon.Invoke(routingAPIRunner)
		Eventually(routingAPINoETCDProcess.Ready(), "5s").Should(BeClosed())
		Eventually(func() error {
			_, err := client.RouterGroups()
			return err
		}, "60s", "1s").Should(BeNil())

		verifyMigration(client, etcdRouterGroups, etcdRoutes, etcdTCPRoutes)
	})
})

func verifyMigration(
	client routing_api.Client,
	originalRouterGroups []models.RouterGroup,
	originalRoutes []models.Route,
	originalTCPRoutes []models.TcpRouteMapping,
) {
	routerGroups, err := client.RouterGroups()
	Expect(err).NotTo(HaveOccurred())
	Expect(routerGroups).To(Equal(originalRouterGroups))

	tcpRouteMappings, err := client.TcpRouteMappings()
	Expect(err).NotTo(HaveOccurred())
	Expect(tcpRouteMappings).To(Equal(originalTCPRoutes))

	routes, err := client.Routes()
	Expect(err).ToNot(HaveOccurred())
	Expect(routes).To(ConsistOf(
		matchers.MatchHttpRoute(originalRoutes[0]),
		matchers.MatchHttpRoute(originalRoutes[1]),
	))
}
