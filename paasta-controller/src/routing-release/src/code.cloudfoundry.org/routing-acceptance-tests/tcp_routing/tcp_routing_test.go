package tcp_routing_test

import (
	"errors"
	"fmt"
	"net"
	"strings"
	"time"

	"code.cloudfoundry.org/cf-routing-test-helpers/helpers"
	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/routing-acceptance-tests/helpers/assets"
	"github.com/cloudfoundry-incubator/cf-test-helpers/cf"
	cfworkflow_helpers "github.com/cloudfoundry-incubator/cf-test-helpers/workflowhelpers"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Tcp Routing", func() {
	BeforeEach(func() {
		updateOrgQuota(context)
	})

	Context("single app port", func() {
		var (
			appName            string
			tcpDropletReceiver = assets.NewAssets().TcpDropletReceiver
			serverId1          string
			externalPort1      uint16
			spaceName          string
		)

		BeforeEach(func() {
			appName = helpers.GenerateAppName()
			serverId1 = "server1"
			cmd := fmt.Sprintf("tcp-droplet-receiver --serverId=%s", serverId1)
			spaceName = context.RegularUserContext().Space
			externalPort1 = helpers.CreateTcpRouteWithRandomPort(spaceName, domainName, DEFAULT_TIMEOUT)

			// Uses --no-route flag so there is no HTTP route
			helpers.PushAppNoStart(appName, tcpDropletReceiver, routingConfig.GoBuildpackName, domainName, CF_PUSH_TIMEOUT, "256M", "-c", cmd, "--no-route")
			helpers.EnableDiego(appName, DEFAULT_TIMEOUT)
			helpers.UpdatePorts(appName, []uint16{3333}, DEFAULT_TIMEOUT)
			helpers.CreateRouteMapping(appName, "", externalPort1, 3333, DEFAULT_TIMEOUT)
			helpers.StartApp(appName, DEFAULT_TIMEOUT)
		})

		AfterEach(func() {
			helpers.AppReport(appName, DEFAULT_TIMEOUT)
			helpers.DeleteApp(appName, DEFAULT_TIMEOUT)
		})

		It("maps a single external port to an application's container port", func() {
			for _, routerAddr := range routingConfig.Addresses {
				Eventually(func() error {
					_, err := sendAndReceive(routerAddr, externalPort1)
					return err
				}, DEFAULT_TIMEOUT, DEFAULT_POLLING_INTERVAL).ShouldNot(HaveOccurred())

				resp, err := sendAndReceive(routerAddr, externalPort1)
				Expect(err).ToNot(HaveOccurred())
				Expect(resp).To(ContainSubstring(serverId1))
			}
		})

		Context("single external port to two different apps", func() {
			var (
				secondAppName string
				serverId2     string
			)

			BeforeEach(func() {
				secondAppName = helpers.GenerateAppName()
				serverId2 = "server2"
				cmd := fmt.Sprintf("tcp-droplet-receiver --serverId=%s", serverId2)

				// Uses --no-route flag so there is no HTTP route
				helpers.PushAppNoStart(secondAppName, tcpDropletReceiver, routingConfig.GoBuildpackName, domainName, CF_PUSH_TIMEOUT, "256M", "-c", cmd, "--no-route")
				helpers.EnableDiego(secondAppName, DEFAULT_TIMEOUT)
				helpers.UpdatePorts(secondAppName, []uint16{3333}, DEFAULT_TIMEOUT)
				helpers.CreateRouteMapping(secondAppName, "", externalPort1, 3333, DEFAULT_TIMEOUT)
				helpers.StartApp(secondAppName, DEFAULT_TIMEOUT)
			})

			AfterEach(func() {
				helpers.AppReport(secondAppName, DEFAULT_TIMEOUT)
				helpers.DeleteApp(secondAppName, DEFAULT_TIMEOUT)
			})

			It("maps single external port to both applications", func() {
				for _, routerAddr := range routingConfig.Addresses {
					Eventually(func() error {
						_, err := sendAndReceive(routerAddr, externalPort1)
						return err
					}, "30s", DEFAULT_POLLING_INTERVAL).ShouldNot(HaveOccurred())

					Eventually(func() string {
						serverId, err := getServerResponse(routerAddr, externalPort1)
						Expect(err).ToNot(HaveOccurred())
						return serverId
					}, "20s", DEFAULT_POLLING_INTERVAL).Should(Equal(serverId1))

					Eventually(func() string {
						serverId, err := getServerResponse(routerAddr, externalPort1)
						Expect(err).ToNot(HaveOccurred())
						return serverId
					}, DEFAULT_TIMEOUT, DEFAULT_POLLING_INTERVAL).Should(Equal(serverId2))

				}
			})
		})

		Context("when multiple external ports are mapped to a single app port", func() {
			var (
				externalPort2 uint16
			)

			BeforeEach(func() {
				externalPort2 = helpers.CreateTcpRouteWithRandomPort(spaceName, domainName, DEFAULT_TIMEOUT)
				helpers.CreateRouteMapping(appName, "", externalPort2, 3333, DEFAULT_TIMEOUT)
			})

			It("routes traffic from two external ports to the app", func() {
				for _, routerAddr := range routingConfig.Addresses {
					Eventually(func() string {
						serverId, _ := sendAndReceive(routerAddr, externalPort1)
						return serverId
					}, DEFAULT_TIMEOUT, DEFAULT_POLLING_INTERVAL).Should(ContainSubstring(serverId1))

					Eventually(func() string {
						serverId, _ := sendAndReceive(routerAddr, externalPort2)
						return serverId
					}, DEFAULT_TIMEOUT, DEFAULT_POLLING_INTERVAL).Should(ContainSubstring(serverId1))
				}
			})
		})

	})

	Context("multiple-app ports", func() {

		var (
			appName           string
			tcpSampleReceiver = assets.NewAssets().TcpSampleReceiver
			serverId1         string
			externalPort1     uint16
			appPort1          uint16
			appPort2          uint16
			spaceName         string
		)

		BeforeEach(func() {
			appName = helpers.GenerateAppName()
			serverId1 = "server1"
			appPort1 = 3434
			appPort2 = 3535
			cmd := fmt.Sprintf("tcp-sample-receiver --address=0.0.0.0:%d,0.0.0.0:%d --serverId=%s", appPort1, appPort2, serverId1)
			spaceName = context.RegularUserContext().Space
			externalPort1 = helpers.CreateTcpRouteWithRandomPort(spaceName, domainName, DEFAULT_TIMEOUT)

			// Uses --no-route flag so there is no HTTP route
			helpers.PushAppNoStart(appName, tcpSampleReceiver, routingConfig.GoBuildpackName, domainName, CF_PUSH_TIMEOUT, "256M", "-c", cmd, "--no-route")
			helpers.EnableDiego(appName, DEFAULT_TIMEOUT)
			helpers.UpdatePorts(appName, []uint16{appPort1, appPort2}, DEFAULT_TIMEOUT)
			helpers.CreateRouteMapping(appName, "", externalPort1, appPort1, DEFAULT_TIMEOUT)
			helpers.StartApp(appName, DEFAULT_TIMEOUT)
		})

		AfterEach(func() {
			helpers.AppReport(appName, DEFAULT_TIMEOUT)
			helpers.DeleteApp(appName, DEFAULT_TIMEOUT)
		})

		Context("single external port with multiple app ports", func() {
			BeforeEach(func() {
				helpers.CreateRouteMapping(appName, "", externalPort1, appPort2, DEFAULT_TIMEOUT)
			})

			It("should switch between ports", func() {

				for _, routerAddr := range routingConfig.Addresses {
					Eventually(func() error {
						_, err := sendAndReceive(routerAddr, externalPort1)
						return err
					}, DEFAULT_TIMEOUT, DEFAULT_POLLING_INTERVAL).ShouldNot(HaveOccurred())

					Eventually(func() string {
						resp, err := sendAndReceive(routerAddr, externalPort1)
						Expect(err).ToNot(HaveOccurred())
						return resp
					}, DEFAULT_TIMEOUT, DEFAULT_POLLING_INTERVAL).Should(ContainSubstring(fmt.Sprintf("%d", appPort1)))

					Eventually(func() string {
						resp, err := sendAndReceive(routerAddr, externalPort1)
						Expect(err).ToNot(HaveOccurred())
						return resp
					}, DEFAULT_TIMEOUT, DEFAULT_POLLING_INTERVAL).Should(ContainSubstring(fmt.Sprintf("%d", appPort2)))
				}
			})
		})

		Context("multiple external ports with multiple app ports", func() {
			var (
				externalPort2 uint16
			)

			BeforeEach(func() {
				externalPort2 = helpers.CreateTcpRouteWithRandomPort(spaceName, domainName, DEFAULT_TIMEOUT)
				helpers.CreateRouteMapping(appName, "", externalPort2, appPort2, DEFAULT_TIMEOUT)
			})

			It("should maps first external port to the first app port", func() {

				for _, routerAddr := range routingConfig.Addresses {
					Eventually(func() error {
						_, err := sendAndReceive(routerAddr, externalPort1)
						return err
					}, DEFAULT_TIMEOUT, DEFAULT_POLLING_INTERVAL).ShouldNot(HaveOccurred())

					Eventually(func() string {
						resp, err := sendAndReceive(routerAddr, externalPort1)
						Expect(err).ToNot(HaveOccurred())
						return resp
					}, DEFAULT_TIMEOUT, DEFAULT_POLLING_INTERVAL).Should(ContainSubstring(fmt.Sprintf("%d", appPort1)))
				}
			})

			It("should maps second external port to the second app port", func() {
				for _, routerAddr := range routingConfig.Addresses {
					Eventually(func() error {
						_, err := sendAndReceive(routerAddr, externalPort2)
						return err
					}, DEFAULT_TIMEOUT, DEFAULT_POLLING_INTERVAL).ShouldNot(HaveOccurred())

					Eventually(func() string {
						resp, err := sendAndReceive(routerAddr, externalPort2)
						Expect(err).ToNot(HaveOccurred())
						return resp
					}, DEFAULT_TIMEOUT, DEFAULT_POLLING_INTERVAL).Should(ContainSubstring(fmt.Sprintf("%d", appPort2)))
				}
			})
		})
	})

})

const (
	DEFAULT_CONNECT_TIMEOUT = 5 * time.Second
	DEFAULT_RW_TIMEOUT      = 2 * time.Second
	CONN_TYPE               = "tcp"
	BUFFER_SIZE             = 1024
)

func getServerResponse(addr string, externalPort uint16) (string, error) {
	response, err := sendAndReceive(addr, externalPort)
	if err != nil {
		return "", err
	}
	tokens := strings.Split(response, ":")
	if len(tokens) == 0 {
		return "", errors.New("Could not extract server id from response")
	}
	return tokens[0], nil
}

func sendAndReceive(addr string, externalPort uint16) (string, error) {
	address := fmt.Sprintf("%s:%d", addr, externalPort)

	conn, err := net.DialTimeout(CONN_TYPE, address, DEFAULT_CONNECT_TIMEOUT)
	if err != nil {
		return "", err
	}
	logger.Info("connected", lager.Data{"address": conn.RemoteAddr()})

	message := []byte(fmt.Sprintf("Time is %d", time.Now().Nanosecond()))
	err = conn.SetWriteDeadline(time.Now().Add(DEFAULT_RW_TIMEOUT))
	_, err = conn.Write(message)
	if err != nil {
		return "", err
	}
	logger.Info("wrote-message", lager.Data{"address": conn.RemoteAddr(), "message": string(message)})

	buff := make([]byte, BUFFER_SIZE)
	err = conn.SetReadDeadline(time.Now().Add(DEFAULT_RW_TIMEOUT))
	if err != nil {
		return "", err
	}
	n, err := conn.Read(buff)
	if err != nil {
		conn.Close()
		return "", err
	}
	logger.Info("read-message", lager.Data{"address": conn.RemoteAddr(), "message": string(buff[:n])})

	return string(buff), conn.Close()
}

func updateOrgQuota(context cfworkflow_helpers.SuiteContext) {
	cfworkflow_helpers.AsUser(context.AdminUserContext(), context.ShortTimeout(), func() {
		orgGuid := cf.Cf("org", context.RegularUserContext().Org, "--guid").Wait(context.ShortTimeout()).Out.Contents()

		quotaUrl, err := helpers.GetOrgQuotaDefinitionUrl(string(orgGuid), context.ShortTimeout())
		Expect(err).NotTo(HaveOccurred())

		cf.Cf("curl", quotaUrl, "-X", "PUT", "-d", "'{\"total_reserved_route_ports\":-1}'").Wait(context.ShortTimeout())
	})
}
