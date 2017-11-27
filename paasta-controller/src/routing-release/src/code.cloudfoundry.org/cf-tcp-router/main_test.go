package main_test

import (
	"crypto/tls"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"path"
	"strings"
	"time"

	"code.cloudfoundry.org/cf-tcp-router/testrunner"
	"code.cloudfoundry.org/cf-tcp-router/testutil"
	"code.cloudfoundry.org/cf-tcp-router/utils"
	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/lager/lagertest"
	"code.cloudfoundry.org/routing-api"
	routingtestrunner "code.cloudfoundry.org/routing-api/cmd/routing-api/testrunner"
	"code.cloudfoundry.org/routing-api/models"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/gexec"
	"github.com/onsi/gomega/ghttp"
	"github.com/tedsuo/ifrit"
	"github.com/tedsuo/ifrit/ginkgomon"
)

var _ = Describe("Main", func() {

	var (
		routerGroupGuid string
	)

	getServerPort := func(serverURL string) string {
		endpoints := strings.Split(serverURL, ":")
		Expect(endpoints).To(HaveLen(3))
		return endpoints[2]
	}

	oAuthServer := func(logger lager.Logger) *ghttp.Server {
		server := ghttp.NewUnstartedServer()
		cert, err := tls.LoadX509KeyPair("fixtures/certs/server.pem", "fixtures/certs/server.key")
		Expect(err).ToNot(HaveOccurred())

		tlsConfig := &tls.Config{
			Certificates: []tls.Certificate{cert},
		}
		server.HTTPTestServer.TLS = tlsConfig
		server.AllowUnhandledRequests = true
		server.HTTPTestServer.StartTLS()

		publicKey := "-----BEGIN PUBLIC KEY-----\\n" +
			"MIGfMA0GCSqGSIb3DQEBAQUAA4GNADCBiQKBgQDHFr+KICms+tuT1OXJwhCUmR2d\\n" +
			"KVy7psa8xzElSyzqx7oJyfJ1JZyOzToj9T5SfTIq396agbHJWVfYphNahvZ/7uMX\\n" +
			"qHxf+ZH9BL1gk9Y6kCnbM5R60gfwjyW1/dQPjOzn9N394zd2FJoFHwdq9Qs0wBug\\n" +
			"spULZVNRxq7veq/fzwIDAQAB\\n" +
			"-----END PUBLIC KEY-----"

		data := fmt.Sprintf("{\"alg\":\"rsa\", \"value\":\"%s\"}", publicKey)
		server.RouteToHandler("GET", "/token_key",
			ghttp.CombineHandlers(
				ghttp.VerifyRequest("GET", "/token_key"),
				ghttp.RespondWith(http.StatusOK, data)),
		)

		server.RouteToHandler("POST", "/oauth/token",
			func(w http.ResponseWriter, req *http.Request) {
				jsonBytes := []byte(`{"access_token":"some-token", "expires_in":10}`)
				w.Write(jsonBytes)
			},
		)
		logger.Info("starting-oauth-server", lager.Data{"address": server.URL()})
		return server
	}

	getRouterGroupGuid := func(port uint16) string {
		client := routing_api.NewClient(fmt.Sprintf("http://127.0.0.1:%d", port), false)
		var routerGroups []models.RouterGroup
		Eventually(func() error {
			var err error
			routerGroups, err = client.RouterGroups()
			return err
		}, "30s", "1s").ShouldNot(HaveOccurred(), "Failed to connect to Routing API server after 30s.")
		Expect(routerGroups).ToNot(HaveLen(0))
		return routerGroups[0].Guid
	}

	routingApiServer := func(logger lager.Logger) ifrit.Process {
		server := routingtestrunner.New(routingAPIBinPath, routingAPIArgs)
		logger.Info("starting-routing-api-server")
		process := ginkgomon.Invoke(server)
		routerGroupGuid = getRouterGroupGuid(routingAPIArgs.Port)
		return process
	}

	generateConfigFile := func(oauthServerPort, routingApiServerPort string, routingApiAuthDisabled bool) string {
		randomConfigFileName := testutil.RandomFileName("tcp_router", ".yml")
		configFile := path.Join(os.TempDir(), randomConfigFileName)

		cfgString := `---
oauth:
  token_endpoint: "127.0.0.1"
  skip_ssl_validation: false
  ca_certs: %s
  client_name: "someclient"
  client_secret: "somesecret"
  port: %s
routing_api:
  auth_disabled: %t
  uri: http://127.0.0.1
  port: %s
haproxy_pid_file: %s
isolation_segments: ["foo-iso-seg"]
`
		cfg := fmt.Sprintf(cfgString, "fixtures/certs/uaa-ca.pem", oauthServerPort, routingApiAuthDisabled, routingApiServerPort, longRunningProcessPidFile)

		err := utils.WriteToFile([]byte(cfg), configFile)
		Expect(err).ShouldNot(HaveOccurred())
		Expect(utils.FileExists(configFile)).To(BeTrue())
		return configFile
	}

	verifyHaProxyConfigContent := func(haproxyFileName, expectedContent string, present bool) {
		Eventually(func() bool {
			data, err := ioutil.ReadFile(haproxyFileName)
			Expect(err).ShouldNot(HaveOccurred())
			return strings.Contains(string(data), expectedContent)
		}, 6, 1).Should(Equal(present))
	}

	var (
		oauthServer *ghttp.Server
		server      ifrit.Process
		logger      *lagertest.TestLogger
		session     *gexec.Session
	)

	BeforeEach(func() {
		logger = lagertest.NewTestLogger("test")
	})

	AfterEach(func() {
		logger.Info("shutting-down")
		session.Signal(os.Interrupt)
		Eventually(session.Exited, 5*time.Second).Should(BeClosed())

		ginkgomon.Interrupt(server, "10s")
		if oauthServer != nil {
			oauthServer.Close()
		}
	})

	Context("when both oauth and routing api servers are up and running", func() {
		BeforeEach(func() {
			oauthServer = oAuthServer(logger)
			server = routingApiServer(logger)
			oauthServerPort := getServerPort(oauthServer.URL())
			configFile := generateConfigFile(oauthServerPort, fmt.Sprintf("%d", routingAPIPort), false)
			tcpRouterArgs := testrunner.Args{
				BaseLoadBalancerConfigFilePath: haproxyBaseConfigFile,
				LoadBalancerConfigFilePath:     haproxyConfigFile,
				ConfigFilePath:                 configFile,
			}

			tcpRouteMapping := models.NewTcpRouteMapping(routerGroupGuid, 5222, "some-ip-1", 61000, 120)
			err := routingApiClient.UpsertTcpRouteMappings([]models.TcpRouteMapping{tcpRouteMapping})
			Expect(err).ToNot(HaveOccurred())

			tcpRouteMappings, err := routingApiClient.TcpRouteMappings()
			Expect(err).NotTo(HaveOccurred())
			Expect(contains(tcpRouteMappings, tcpRouteMapping)).To(BeTrue())

			allOutput := logger.Buffer()
			runner := testrunner.New(tcpRouterPath, tcpRouterArgs)
			session, err = gexec.Start(runner.Command, allOutput, allOutput)
			Expect(err).ToNot(HaveOccurred())
		})

		It("syncs with routing api", func() {
			Eventually(session.Out, 5*time.Second).Should(gbytes.Say("applied-fetched-routes-to-routing-table"))
			expectedConfigEntry := "\nlisten listen_cfg_5222\n  mode tcp\n  bind :5222\n"
			serverConfigEntry := "server server_some-ip-1_61000 some-ip-1:61000"
			verifyHaProxyConfigContent(haproxyConfigFile, expectedConfigEntry, true)
			verifyHaProxyConfigContent(haproxyConfigFile, serverConfigEntry, true)
		})

		It("starts an SSE connection to the server", func() {
			Eventually(session.Out, 5*time.Second).Should(gbytes.Say("Subscribing-to-routing-api-event-stream"))
			Eventually(session.Out, 5*time.Second).Should(gbytes.Say("Successfully-subscribed-to-routing-api-event-stream"))
			tcpRouteMapping := models.NewTcpRouteMapping(routerGroupGuid, 5222, "some-ip-2", 61000, 120)
			err := routingApiClient.UpsertTcpRouteMappings([]models.TcpRouteMapping{tcpRouteMapping})
			Expect(err).ToNot(HaveOccurred())
			Eventually(session.Out, 5*time.Second).Should(gbytes.Say("handle-event.finished"))
			expectedConfigEntry := "\nlisten listen_cfg_5222\n  mode tcp\n  bind :5222\n"
			verifyHaProxyConfigContent(haproxyConfigFile, expectedConfigEntry, true)
			oldServerConfigEntry := "server server_some-ip-1_61000 some-ip-1:61000"
			verifyHaProxyConfigContent(haproxyConfigFile, oldServerConfigEntry, true)
			newServerConfigEntry := "server server_some-ip-2_61000 some-ip-2:61000"
			verifyHaProxyConfigContent(haproxyConfigFile, newServerConfigEntry, true)
		})

		It("prunes stale routes", func() {
			Eventually(session.Out, 5*time.Second).Should(gbytes.Say("Subscribing-to-routing-api-event-stream"))
			Eventually(session.Out, 5*time.Second).Should(gbytes.Say("Successfully-subscribed-to-routing-api-event-stream"))
			tcpRouteMapping := models.NewTcpRouteMapping(routerGroupGuid, 5222, "some-ip-3", 61000, 6)
			err := routingApiClient.UpsertTcpRouteMappings([]models.TcpRouteMapping{tcpRouteMapping})
			Expect(err).ToNot(HaveOccurred())
			Eventually(session.Out, 5*time.Second).Should(gbytes.Say("handle-event.finished"))
			expectedConfigEntry := "\nlisten listen_cfg_5222\n  mode tcp\n  bind :5222\n"
			verifyHaProxyConfigContent(haproxyConfigFile, expectedConfigEntry, true)
			oldServerConfigEntry := "server server_some-ip-1_61000 some-ip-1:61000"
			verifyHaProxyConfigContent(haproxyConfigFile, oldServerConfigEntry, true)
			newServerConfigEntry := "server server_some-ip-3_61000 some-ip-3:61000"
			verifyHaProxyConfigContent(haproxyConfigFile, newServerConfigEntry, true)
			Eventually(session.Out, 10*time.Second, 1*time.Second).Should(gbytes.Say("prune-stale-routes.starting"))
			Eventually(session.Out, 10*time.Second, 1*time.Second).Should(gbytes.Say("prune-stale-routes.completed"))
			verifyHaProxyConfigContent(haproxyConfigFile, newServerConfigEntry, false)
		})

	})

	Context("Oauth server is down", func() {
		var (
			tcpRouterArgs   testrunner.Args
			configFile      string
			oauthServerPort string
		)
		BeforeEach(func() {
			server = routingApiServer(logger)
			oauthServerPort = "1111"
		})

		JustBeforeEach(func() {
			allOutput := logger.Buffer()
			runner := testrunner.New(tcpRouterPath, tcpRouterArgs)
			var err error
			session, err = gexec.Start(runner.Command, allOutput, allOutput)
			Expect(err).ToNot(HaveOccurred())
		})

		Context("routing api auth is enabled", func() {
			BeforeEach(func() {
				configFile = generateConfigFile(oauthServerPort, fmt.Sprintf("%d", routingAPIPort), false)
				tcpRouterArgs = testrunner.Args{
					BaseLoadBalancerConfigFilePath: haproxyBaseConfigFile,
					LoadBalancerConfigFilePath:     haproxyConfigFile,
					ConfigFilePath:                 configFile,
				}
			})

			It("exits with error", func() {
				Eventually(session.Out, 5*time.Second).Should(gbytes.Say("failed-connecting-to-uaa"))
				Eventually(session.Exited).Should(BeClosed())
			})
		})

		Context("routing api auth is disabled", func() {
			BeforeEach(func() {
				configFile = generateConfigFile(oauthServerPort, fmt.Sprintf("%d", routingAPIPort), true)
				tcpRouterArgs = testrunner.Args{
					BaseLoadBalancerConfigFilePath: haproxyBaseConfigFile,
					LoadBalancerConfigFilePath:     haproxyConfigFile,
					ConfigFilePath:                 configFile,
				}
			})

			It("does not call oauth server to get auth token and starts SSE connection with routing api", func() {
				Eventually(session.Out, 5*time.Second).Should(gbytes.Say("creating-noop-uaa-client"))
				Eventually(session.Out, 5*time.Second).Should(gbytes.Say("Successfully-subscribed-to-routing-api-event-stream"))
			})
		})
	})

	Context("Routing API server is down", func() {
		BeforeEach(func() {
			oauthServer = oAuthServer(logger)
			oauthServerPort := getServerPort(oauthServer.URL())
			configFile := generateConfigFile(oauthServerPort, fmt.Sprintf("%d", routingAPIPort), false)
			tcpRouterArgs := testrunner.Args{
				BaseLoadBalancerConfigFilePath: haproxyBaseConfigFile,
				LoadBalancerConfigFilePath:     haproxyConfigFile,
				ConfigFilePath:                 configFile,
			}
			allOutput := logger.Buffer()
			runner := testrunner.New(tcpRouterPath, tcpRouterArgs)
			var err error
			session, err = gexec.Start(runner.Command, allOutput, allOutput)
			Expect(err).ToNot(HaveOccurred())
		})

		It("keeps trying to connect and doesn't blow up", func() {
			Eventually(session.Out, 5*time.Second).Should(gbytes.Say("Subscribing-to-routing-api-event-stream"))
			Consistently(session.Exited).ShouldNot(BeClosed())
			Consistently(session.Out, 5*time.Second).ShouldNot(gbytes.Say("Successfully-subscribed-to-routing-api-event-stream"))
			By("starting routing api server")
			server = routingApiServer(logger)
			Eventually(session.Out, 5*time.Second).Should(gbytes.Say("Successfully-subscribed-to-routing-api-event-stream"))
			tcpRouteMapping := models.NewTcpRouteMapping(routerGroupGuid, 5222, "some-ip-3", 61000, 120)
			err := routingApiClient.UpsertTcpRouteMappings([]models.TcpRouteMapping{tcpRouteMapping})
			Expect(err).ToNot(HaveOccurred())
			Eventually(session.Out, 5*time.Second).Should(gbytes.Say("handle-event.finished"))
			expectedConfigEntry := "\nlisten listen_cfg_5222\n  mode tcp\n  bind :5222\n"
			verifyHaProxyConfigContent(haproxyConfigFile, expectedConfigEntry, true)
			newServerConfigEntry := "server server_some-ip-3_61000 some-ip-3:61000"
			verifyHaProxyConfigContent(haproxyConfigFile, newServerConfigEntry, true)
		})
	})

	Context("when haproxy is down", func() {
		BeforeEach(func() {
			oauthServer = oAuthServer(logger)
			server = routingApiServer(logger)
			oauthServerPort := getServerPort(oauthServer.URL())
			configFile := generateConfigFile(oauthServerPort, fmt.Sprintf("%d", routingAPIPort), false)
			tcpRouterArgs := testrunner.Args{
				BaseLoadBalancerConfigFilePath: haproxyBaseConfigFile,
				LoadBalancerConfigFilePath:     haproxyConfigFile,
				ConfigFilePath:                 configFile,
			}

			runner := testrunner.New(tcpRouterPath, tcpRouterArgs)

			var err error
			session, err = gexec.Start(runner.Command, GinkgoWriter, GinkgoWriter)
			Expect(err).NotTo(HaveOccurred())
		})

		AfterEach(func() {
			setupLongRunningProcess()
		})

		It("exits", func() {
			Eventually(session.Out, 5*time.Second).Should(gbytes.Say("Subscribing-to-routing-api-event-stream"))
			Consistently(session.Exited).ShouldNot(BeClosed())

			killLongRunningProcess()

			Eventually(session.Exited, "5s").Should(BeClosed())
		})
	})
})

func contains(ms []models.TcpRouteMapping, tcpRouteMapping models.TcpRouteMapping) bool {
	for _, m := range ms {
		if m.Matches(tcpRouteMapping) {
			return true
		}
	}
	return false
}
