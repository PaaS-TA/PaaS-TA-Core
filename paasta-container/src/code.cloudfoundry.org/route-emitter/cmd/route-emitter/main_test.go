package main_test

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"net/http/httputil"
	"net/url"
	"os"
	"os/exec"
	"sync"
	"time"

	"code.cloudfoundry.org/bbs/models"
	"code.cloudfoundry.org/durationjson"
	"code.cloudfoundry.org/lager/lagerflags"
	"code.cloudfoundry.org/locket"
	"code.cloudfoundry.org/route-emitter/cmd/route-emitter/config"
	"code.cloudfoundry.org/route-emitter/cmd/route-emitter/runners"
	"code.cloudfoundry.org/route-emitter/routingtable"
	. "code.cloudfoundry.org/route-emitter/routingtable/matchers"
	apimodels "code.cloudfoundry.org/routing-api/models"
	"code.cloudfoundry.org/routing-info/cfroutes"
	"code.cloudfoundry.org/routing-info/tcp_routes"
	"github.com/cloudfoundry/sonde-go/events"
	"github.com/nats-io/nats"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/types"
	"github.com/tedsuo/ifrit"
	"github.com/tedsuo/ifrit/ginkgomon"
)

const (
	emitterInterruptTimeout    = 5 * time.Second
	routingAPIInterruptTimeout = 10 * time.Second
	msgReceiveTimeout          = 5 * time.Second
)

var _ = Describe("Route Emitter", func() {
	var (
		registeredRoutes   <-chan routingtable.RegistryMessage
		unregisteredRoutes <-chan routingtable.RegistryMessage

		processGuid string
		domain      string
		desiredLRP  *models.DesiredLRP
		index       int32

		lrpKey      models.ActualLRPKey
		instanceKey models.ActualLRPInstanceKey
		netInfo     models.ActualLRPNetInfo

		hostnames     []string
		containerPort uint32
		routes        *models.Routes

		routingApiProcess ifrit.Process
		routingAPIRunner  *runners.RoutingAPIRunner
		routerGUID        string
		cfgs              []func(*config.RouteEmitterConfig)
	)

	createEmitterRunner := func(sessionName string, cellID string, modifyConfig ...func(*config.RouteEmitterConfig)) *ginkgomon.Runner {
		cfg := config.RouteEmitterConfig{
			CellID:               cellID,
			ConsulSessionName:    sessionName,
			DropsondePort:        dropsondePort,
			HealthCheckAddress:   healthCheckAddress,
			NATSAddresses:        fmt.Sprintf("127.0.0.1:%d", natsPort),
			BBSAddress:           bbsURL.String(),
			CommunicationTimeout: durationjson.Duration(100 * time.Millisecond),
			SyncInterval:         durationjson.Duration(syncInterval),
			LockRetryInterval:    durationjson.Duration(time.Second),
			LockTTL:              durationjson.Duration(5 * time.Second),
			ConsulCluster:        consulClusterAddress,
			LagerConfig: lagerflags.LagerConfig{
				LogLevel: lagerflags.DEBUG,
			},
		}
		for _, f := range modifyConfig {
			f(&cfg)
		}

		configFile, err := ioutil.TempFile("", "route-emitter-test")
		Expect(err).NotTo(HaveOccurred())

		defer configFile.Close()

		configPath := configFile.Name()
		encoder := json.NewEncoder(configFile)
		err = encoder.Encode(&cfg)
		Expect(err).NotTo(HaveOccurred())

		return ginkgomon.New(ginkgomon.Config{
			Command: exec.Command(
				string(emitterPath),
				"-config", configPath,
			),

			Name: sessionName,

			StartCheck: "route-emitter.watcher.sync.complete",

			AnsiColorCode: "97m",
			Cleanup: func() {
				os.RemoveAll(configPath)
			},
		})
	}

	listenForRoutes := func(subject string) <-chan routingtable.RegistryMessage {
		routes := make(chan routingtable.RegistryMessage)

		natsClient.Subscribe(subject, func(msg *nats.Msg) {
			defer GinkgoRecover()

			var message routingtable.RegistryMessage
			err := json.Unmarshal(msg.Data, &message)
			Expect(err).NotTo(HaveOccurred())

			routes <- message
		})

		return routes
	}

	BeforeEach(func() {
		processGuid = "guid1"
		domain = "tests"

		hostnames = []string{"route-1", "route-2"}
		containerPort = 8080
		routes = newRoutes(hostnames, containerPort, "https://awesome.com")

		desiredLRP = &models.DesiredLRP{
			Domain:      domain,
			ProcessGuid: processGuid,
			Ports:       []uint32{containerPort},
			Routes:      routes,
			Instances:   5,
			RootFs:      "some:rootfs",
			MemoryMb:    1024,
			DiskMb:      512,
			LogGuid:     "some-log-guid",
			Action: models.WrapAction(&models.RunAction{
				User: "me",
				Path: "ls",
			}),
		}

		index = 0
		lrpKey = models.NewActualLRPKey(processGuid, index, domain)
		instanceKey = models.NewActualLRPInstanceKey("iguid1", "cell-id")

		netInfo = models.NewActualLRPNetInfo("1.2.3.4", "2.2.2.2", models.NewPortMapping(65100, 8080))
		registeredRoutes = listenForRoutes("router.register")
		unregisteredRoutes = listenForRoutes("router.unregister")

		natsClient.Subscribe("router.greet", func(msg *nats.Msg) {
			defer GinkgoRecover()

			greeting := routingtable.RouterGreetingMessage{
				MinimumRegisterInterval: int(emitInterval / time.Second),
				PruneThresholdInSeconds: 6,
			}

			response, err := json.Marshal(greeting)
			Expect(err).NotTo(HaveOccurred())

			err = natsClient.Publish(msg.Reply, response)
			Expect(err).NotTo(HaveOccurred())
		})
		var err error
		sqlConfig := runners.SQLConfig{
			Port:       sqlRunner.Port(),
			DBName:     sqlRunner.DBName(),
			DriverName: sqlRunner.DriverName(),
			Username:   sqlRunner.Username(),
			Password:   sqlRunner.Password(),
		}
		routingAPIRunner, err = runners.NewRoutingAPIRunner(routingAPIPath, consulRunner.URL(), sqlConfig, func(cfg *runners.Config) {
			cfg.ConsulCluster.LockTTL = 5 * time.Second
		})

		Expect(err).NotTo(HaveOccurred())
		routingApiProcess = ginkgomon.Invoke(routingAPIRunner)

		Eventually(func() error {
			guid, err := routingAPIRunner.GetGUID()
			if err != nil {
				return err
			}
			routerGUID = guid
			return nil
		}).Should(Succeed())
		logger.Info("started-routing-api-server")
		cfgs = nil
		cfgs = append(cfgs, func(cfg *config.RouteEmitterConfig) {
			cfg.BBSAddress = bbsURL.String()
			cfg.RoutingAPI.URL = "http://127.0.0.1"
			cfg.RoutingAPI.Port = routingAPIRunner.Config.Port
		})
	})

	AfterEach(func() {
		logger.Info("shutting-down")
		ginkgomon.Kill(routingApiProcess, routingAPIInterruptTimeout)
	})

	Context("Ping interval for nats client", func() {
		var runner *ginkgomon.Runner
		var emitter ifrit.Process

		BeforeEach(func() {
			runner = createEmitterRunner("emitter1", "", cfgs...)
			runner.StartCheck = "emitter1.started"
			emitter = ginkgomon.Invoke(runner)
		})

		AfterEach(func() {
			ginkgomon.Kill(emitter, emitterInterruptTimeout)
		})

		It("returns 20 second", func() {
			Expect(runner).To(gbytes.Say("setting-nats-ping-interval"))
			Expect(runner).To(gbytes.Say(`"duration-in-seconds":20`))
		})
	})

	Context("when emitter is started with invalid configuration", func() {
		var (
			runner  *ginkgomon.Runner
			emitter ifrit.Process
		)

		JustBeforeEach(func() {
			runner = createEmitterRunner("emitter1", "", cfgs...)
			emitter = ifrit.Invoke(runner)
		})

		AfterEach(func() {
			ginkgomon.Interrupt(emitter, emitterInterruptTimeout)
		})

		Context("ttl is too high", func() {
			BeforeEach(func() {
				cfgs = append(cfgs, func(cfg *config.RouteEmitterConfig) {
					cfg.TCPRouteTTL = durationjson.Duration(24 * time.Hour)
				})
			})

			It("logs an error and exit", func() {
				var err error
				Eventually(emitter.Wait()).Should(Receive(&err))
				Expect(err).To(HaveOccurred())
				Expect(runner.Buffer()).To(gbytes.Say("invalid-route-ttl"))
			})
		})
	})

	Context("when the tcp route emitter is enabled", func() {
		var (
			expectedTcpRouteMapping    apimodels.TcpRouteMapping
			notExpectedTcpRouteMapping apimodels.TcpRouteMapping
			emitter                    ifrit.Process
			runner                     *ginkgomon.Runner
			cellID                     string
		)

		BeforeEach(func() {
			cfgs = append(cfgs, func(cfg *config.RouteEmitterConfig) {
				cfg.EnableTCPEmitter = true
			})
			expectedTcpRouteMapping = apimodels.NewTcpRouteMapping("", 5222, "some-ip", 62003, 120)
			notExpectedTcpRouteMapping = apimodels.NewTcpRouteMapping("", 1883, "some-ip-1", 62003, 120)
			expectedTcpRouteMapping.RouterGroupGuid = routerGUID
			notExpectedTcpRouteMapping.RouterGroupGuid = routerGUID
			cellID = ""
		})

		getDesiredLRP := func(processGuid, routerGroupGuid string, externalPort, containerPort uint32) models.DesiredLRP {
			tcpRoutes := tcp_routes.TCPRoutes{
				tcp_routes.TCPRoute{
					RouterGroupGuid: routerGroupGuid,
					ExternalPort:    externalPort,
					ContainerPort:   containerPort,
				},
			}

			return models.DesiredLRP{
				Domain:      domain,
				ProcessGuid: processGuid,
				Ports:       []uint32{containerPort},
				Routes:      tcpRoutes.RoutingInfo(),
				Instances:   5,
				RootFs:      "some:rootfs",
				MemoryMb:    1024,
				DiskMb:      512,
				LogGuid:     "some-log-guid",
				Action: models.WrapAction(&models.RunAction{
					User: "me",
					Path: "ls",
				}),
			}
		}

		Context("when UAA auth is enabled", func() {
			JustBeforeEach(func() {
				runner = createEmitterRunner("emitter1", cellID, cfgs...)
				runner.StartCheck = "emitter1.started"
				emitter = ifrit.Invoke(runner)
			})

			AfterEach(func() {
				ginkgomon.Kill(emitter, emitterInterruptTimeout)
			})

			BeforeEach(func() {
				cfgs = append(cfgs, func(cfg *config.RouteEmitterConfig) {
					cfg.RoutingAPI.AuthEnabled = true
				})
			})

			Context("and no configuration is provided", func() {
				BeforeEach(func() {
					cfgs = append(cfgs, func(cfg *config.RouteEmitterConfig) {
						cfg.OAuth = config.OAuthConfig{}
					})
				})

				It("fails", func() {
					Eventually(emitter.Wait()).Should(Receive())
					Expect(runner.ExitCode()).NotTo(Equal(0))
					Expect(runner).To(gbytes.Say("initialize-token-fetcher-error"))
				})
			})

			Context("and an invalid configuration is provided", func() {
				BeforeEach(func() {
					cfgs = append(cfgs, func(cfg *config.RouteEmitterConfig) {
						cfg.OAuth = config.OAuthConfig{
							UaaURL: "http://localhost:0",
						}
					})
				})

				It("fails", func() {
					Eventually(emitter.Wait()).Should(Receive())
					Expect(runner.ExitCode()).NotTo(Equal(0))
					Expect(runner).To(gbytes.Say("failed-fetching-uaa-key"))
				})
			})

			Context("and a valid configuration is provided", func() {
				verifyEmitterIsUP := func() {
					It("starts up successfully", func() {
						Expect(runner).To(gbytes.Say("emitter1.started"))
					})

					Context("when an lrp is desired", func() {
						BeforeEach(func() {
							desiredLRP := getDesiredLRP("some-guid", routerGUID, 5222, 5222)
							Expect(bbsClient.DesireLRP(logger, &desiredLRP)).NotTo(HaveOccurred())
							lrpKey := models.NewActualLRPKey("some-guid", 0, domain)
							instanceKey := models.NewActualLRPInstanceKey("instance-guid", "cell-id")
							netInfo := models.NewActualLRPNetInfo("some-ip", "container-ip", models.NewPortMapping(62003, 5222))
							Expect(bbsClient.StartActualLRP(logger, &lrpKey, &instanceKey, &netInfo))

						})

						It("requests a token from the server", func() {
							Eventually(func() []string {
								reqs := oauthServer.ReceivedRequests()
								paths := make([]string, len(reqs))
								for i, r := range reqs {
									paths[i] = r.URL.Path
								}
								return paths
							}, 5*time.Second).Should(ContainElement("/oauth/token"))
						})
					})
				}

				BeforeEach(func() {
					cfgs = append(cfgs, func(cfg *config.RouteEmitterConfig) {
						cfg.OAuth = config.OAuthConfig{
							UaaURL:       oauthServer.URL(),
							ClientName:   "someclient",
							ClientSecret: "somesecret",
							CACerts:      "fixtures/ca.crt",
						}
					})
				})

				verifyEmitterIsUP()

				Context("and skip cert verify is set", func() {
					BeforeEach(func() {
						cfgs = append(cfgs, func(cfg *config.RouteEmitterConfig) {
							cfg.OAuth.CACerts = ""
							cfg.OAuth.SkipCertVerify = true
						})
					})

					verifyEmitterIsUP()
				})

			})
		})

		Context("when UAA auth is disabled", func() {
			BeforeEach(func() {
				cfgs = append(cfgs, func(cfg *config.RouteEmitterConfig) {
					cfg.RoutingAPI.AuthEnabled = false
				})
			})

			JustBeforeEach(func() {
				runner = createEmitterRunner("emitter1", cellID, cfgs...)
				runner.StartCheck = "emitter1.started"
				emitter = ginkgomon.Invoke(runner)
			})

			AfterEach(func() {
				ginkgomon.Kill(emitter, emitterInterruptTimeout)
			})

			It("starts successfully without oauth config", func() {
				Expect(runner).To(gbytes.Say("creating-noop-uaa-client"))
			})

			Context("and an lrp with routes is desired", func() {
				var (
					expectedTCPProcessGUID string
					desiredLRP             models.DesiredLRP
				)

				BeforeEach(func() {
					cfgs = append(cfgs, func(cfg *config.RouteEmitterConfig) {
						cfg.BBSAddress = bbsURL.String()
						cfg.SyncInterval = durationjson.Duration(1 * time.Second)
						cfg.RoutingAPI.URL = "http://127.0.0.1"
						cfg.RoutingAPI.Port = routingAPIRunner.Config.Port
					})

					expectedTCPProcessGUID = "some-guid"
					desiredLRP = getDesiredLRP(expectedTCPProcessGUID, routerGUID, 5222, 5222)
				})

				JustBeforeEach(func() {
					Expect(bbsClient.DesireLRP(logger, &desiredLRP)).NotTo(HaveOccurred())
				})

				Context("and an instance is started", func() {
					BeforeEach(func() {
						lrpKey = models.NewActualLRPKey(expectedTCPProcessGUID, 0, domain)
						instanceKey = models.NewActualLRPInstanceKey("instance-guid", "cell-id")
						netInfo = models.NewActualLRPNetInfo("some-ip", "container-ip", models.NewPortMapping(62003, 5222))
						Expect(bbsClient.StartActualLRP(logger, &lrpKey, &instanceKey, &netInfo))
					})

					It("emits its routes immediately", func() {
						Eventually(func() bool {
							mappings, _ := routingAPIRunner.GetClient().TcpRouteMappings()
							return contains(mappings, expectedTcpRouteMapping)
						}, 5*time.Second).Should(BeTrue())
					})

					Context("and the route-emitter is running in direct instance routes mode", func() {
						BeforeEach(func() {
							cfgs = append(cfgs, func(cfg *config.RouteEmitterConfig) {
								cfg.RegisterDirectInstanceRoutes = true
							})
						})

						It("contains the container host and port", func() {
							expectedTcpRouteMapping.HostIP = netInfo.InstanceAddress
							expectedTcpRouteMapping.HostPort = uint16(netInfo.Ports[0].ContainerPort)
							Eventually(func() bool {
								mappings, _ := routingAPIRunner.GetClient().TcpRouteMappings()
								return contains(mappings, expectedTcpRouteMapping)
							}, 5*time.Second).Should(BeTrue())
						})
					})

					Context("when running in local mode", func() {
						BeforeEach(func() {
							consulClusterAddress = ""
							cellID = "cell-id"
						})

						It("emits the tcp route count", func() {
							Eventually(func() *events.Envelope {
								metric := <-testMetricsChan
								return metric
							}).Should(matchMetricAndValue(metricAndValue{Name: "TCPRouteCount", Value: int32(1)}))
						})
					})

					Context("and the route-emitter cell id doesn't match the actual lrp cell", func() {
						BeforeEach(func() {
							cellID = "some-random-cell-id"
						})

						It("does not emit the route", func() {
							Consistently(func() bool {
								mappings, _ := routingAPIRunner.GetClient().TcpRouteMappings()
								return contains(mappings, expectedTcpRouteMapping)
							}, 5*time.Second).Should(BeFalse())
						})
					})

					Context("the instance has no routes", func() {
						var (
							routes *models.Routes
						)

						BeforeEach(func() {
							routes = desiredLRP.Routes
							desiredLRP.Routes = &models.Routes{}
						})

						JustBeforeEach(func() {
							Consistently(func() bool {
								mappings, _ := routingAPIRunner.GetClient().TcpRouteMappings()
								return contains(mappings, expectedTcpRouteMapping)
							}, 5*time.Second).Should(BeFalse())
						})

						Context("and routes are added", func() {
							JustBeforeEach(func() {
								update := &models.DesiredLRPUpdate{
									Routes: routes,
								}
								err := bbsClient.UpdateDesiredLRP(logger, desiredLRP.ProcessGuid, update)
								Expect(err).NotTo(HaveOccurred())
							})

							It("immediately registers the route", func() {
								Eventually(func() error {
									mappings, err := routingAPIRunner.GetClient().TcpRouteMappings()
									if err != nil {
										return err
									}

									if !contains(mappings, expectedTcpRouteMapping) {
										return fmt.Errorf("%v does not contain the route mappings", mappings)
									}
									return nil
								}).Should(Succeed())
							})
						})
					})

					Context("and routes are removed", func() {
						JustBeforeEach(func() {
							Eventually(func() bool {
								mappings, _ := routingAPIRunner.GetClient().TcpRouteMappings()
								return contains(mappings, expectedTcpRouteMapping)
							}, 5*time.Second).Should(BeTrue())
							update := &models.DesiredLRPUpdate{
								Routes: &models.Routes{},
							}
							err := bbsClient.UpdateDesiredLRP(logger, desiredLRP.ProcessGuid, update)
							Expect(err).NotTo(HaveOccurred())
						})

						It("immediately unregisters the route", func() {
							Eventually(func() error {
								mappings, err := routingAPIRunner.GetClient().TcpRouteMappings()
								if err != nil {
									return err
								}

								if len(mappings) != 0 {
									return fmt.Errorf("%v is not empty", mappings)
								}

								return nil
							}).Should(Succeed())
						})
					})
				})

				Context("and an instance is claimed", func() {
					JustBeforeEach(func() {
						err := bbsClient.ClaimActualLRP(logger, expectedTCPProcessGUID, int(index), &instanceKey)
						Expect(err).NotTo(HaveOccurred())
					})

					It("does not emit routes", func() {
						Consistently(func() []apimodels.TcpRouteMapping {
							mappings, _ := routingAPIRunner.GetClient().TcpRouteMappings()
							return mappings
						}).Should(BeEmpty())
					})
				})

				Context("an actual lrp created event is received during sync", func() {
					var (
						fakeBBS    *httptest.Server
						blkChannel chan struct{}
					)

					BeforeEach(func() {
						blkChannel = make(chan struct{}, 1)

						proxy := httputil.NewSingleHostReverseProxy(bbsURL)
						proxy.FlushInterval = 100 * time.Millisecond
						fakeBBS = httptest.NewUnstartedServer(
							http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
								if r.URL.Path == "/v1/domains/list" {
									By("blocking the sync loop")
									<-blkChannel
								}
								proxy.ServeHTTP(w, r)
							}),
						)

						cfgs = append(cfgs, func(cfg *config.RouteEmitterConfig) {
							cfg.BBSAddress = fakeBBS.URL
							cfg.RoutingAPI.URL = "http://127.0.0.1"
							cfg.RoutingAPI.Port = routingAPIRunner.Config.Port
							cfg.CommunicationTimeout = durationjson.Duration(5 * time.Second)
							cfg.SyncInterval = durationjson.Duration(1 * time.Hour)
						})

						fakeBBS.Start()
						runner = createEmitterRunner("route-emitter", "cell-id", cfgs...)
						runner.StartCheck = "succeeded-getting-actual-lrps"
						desiredLRP.Instances = 1
						Expect(bbsClient.UpsertDomain(logger, domain, time.Hour)).To(Succeed())
					})

					It("should emit a route registration", func() {
						By("waiting for the sync loop to start")
						lrpKey = models.NewActualLRPKey(expectedTCPProcessGUID, 0, domain)
						instanceKey = models.NewActualLRPInstanceKey("instance-guid", "cell-id")
						netInfo = models.NewActualLRPNetInfo("some-ip", "container-ip", models.NewPortMapping(5222, 5222))
						Expect(bbsClient.StartActualLRP(logger, &lrpKey, &instanceKey, &netInfo)).To(Succeed())
						Eventually(runner).Should(gbytes.Say("caching-event"))

						By("unblocking the sync loop")
						close(blkChannel)

						expectedTcpRouteMapping = apimodels.NewTcpRouteMapping(routerGUID, 5222, "some-ip", 5222, 120)

						Eventually(func() bool {
							mappings, _ := routingAPIRunner.GetClient().TcpRouteMappings()
							return contains(mappings, expectedTcpRouteMapping)
						}, 5*time.Second).Should(BeTrue())
					})

					AfterEach(func() {
						// ensure the channel is closed
						select {
						case <-blkChannel:
						default:
							close(blkChannel)
						}

						fakeBBS.CloseClientConnections()
						fakeBBS.Close()
					})
				})
			})

			Context("when routing api server is down but bbs is running", func() {
				JustBeforeEach(func() {
					ginkgomon.Kill(routingApiProcess, routingAPIInterruptTimeout)
					desiredLRP := getDesiredLRP("some-guid-1", "some-guid", 1883, 1883)
					Expect(bbsClient.DesireLRP(logger, &desiredLRP)).NotTo(HaveOccurred())

					key := models.NewActualLRPKey("some-guid-1", 0, domain)
					instanceKey := models.NewActualLRPInstanceKey("instance-guid-1", "cell-id")
					netInfo := models.NewActualLRPNetInfo("some-ip-1", "container-ip-1", models.NewPortMapping(62003, 1883))
					Expect(bbsClient.StartActualLRP(logger, &key, &instanceKey, &netInfo))
				})

				It("starts an SSE connection to the bbs and continues to try to emit to routing api", func() {
					// Do not use Say matcher as ordering of 'subscribed-to-bbs-event' log message
					// is not defined in relation to the 'tcp-emitter.started' message
					Eventually(runner.Buffer().Contents).Should(ContainSubstring("subscribed-to-bbs-event"))
					Eventually(runner.Buffer()).Should(gbytes.Say("syncer.syncing"))
					Eventually(runner.Buffer()).Should(gbytes.Say("unable-to-upsert"))
					Consistently(runner.Buffer()).ShouldNot(gbytes.Say("successfully-emitted-event"))
					Consistently(emitter.Wait()).ShouldNot(Receive())

					By("starting routing api server")
					routingApiProcess = ginkgomon.Invoke(routingAPIRunner)
					Eventually(func() error {
						guid, err := routingAPIRunner.GetGUID()
						if err != nil {
							return err
						}
						expectedTcpRouteMapping.RouterGroupGuid = guid
						notExpectedTcpRouteMapping.RouterGroupGuid = guid
						return nil
					}).Should(Succeed())
					logger.Info("started-routing-api-server")
					Eventually(runner.Buffer()).Should(gbytes.Say("unable-to-upsert.*some-guid not found"))
				})
			})
		})
	})

	Context("when NATS is unreachable", func() {
		var runner *ginkgomon.Runner
		var emitter ifrit.Process

		JustBeforeEach(func() {
			cfgs = append(cfgs, func(cfg *config.RouteEmitterConfig) {
				// some invalid address
				cfg.NATSAddresses = "localhost:0"
			})
			runner = createEmitterRunner("emitter1", "", cfgs...)
			runner.StartCheck = "emitter1.started"

			emitter = ifrit.Invoke(runner)
		})

		AfterEach(func() {
			ginkgomon.Kill(emitter, emitterInterruptTimeout)
		})

		It("exits with non zero exit code", func() {
			Eventually(emitter.Wait(), 6*time.Second).Should(Receive())
			Expect(runner.ExitCode()).NotTo(Equal(0))
		})

		It("doesn't enable the healthcheck server", func() {
			client := http.Client{
				Timeout: time.Second,
			}
			Consistently(func() error {
				_, err := client.Get("http://" + healthCheckAddress)
				return err
			}, 6*time.Second).Should(HaveOccurred(), "healthcheck unexpectedly started up")
		})
	})

	Context("when only the nats emitter is running", func() {
		var (
			emitter ifrit.Process
			runner  *ginkgomon.Runner
			cellID  string
		)

		BeforeEach(func() {
			cellID = ""
		})

		JustBeforeEach(func() {
			cfgs = append(cfgs, func(cfg *config.RouteEmitterConfig) {
				cfg.EnableTCPEmitter = false
			})
			runner = createEmitterRunner("emitter1", cellID, cfgs...)
			runner.StartCheck = "emitter1.started"
			emitter = ginkgomon.Invoke(runner)
		})

		AfterEach(func() {
			By("killing the route-emitter")
			ginkgomon.Kill(emitter, emitterInterruptTimeout)
		})

		It("enables the healthcheck server", func() {
			client := http.Client{
				Timeout: time.Second,
			}
			Eventually(func() error {
				resp, err := client.Get("http://" + healthCheckAddress)
				if err != nil {
					return err
				}
				if resp.StatusCode != http.StatusOK {
					return errors.New("received a non-200 status code")
				}
				return nil
			}, 6*time.Second).ShouldNot(HaveOccurred(), "healthcheck server didn't start")
		})

		Context("and an lrp with routes is desired", func() {
			BeforeEach(func() {
				err := bbsClient.DesireLRP(logger, desiredLRP)
				Expect(err).NotTo(HaveOccurred())
			})

			Context("and an instance starts", func() {
				BeforeEach(func() {
					err := bbsClient.StartActualLRP(logger, &lrpKey, &instanceKey, &netInfo)
					Expect(err).NotTo(HaveOccurred())
				})

				Context("when running in local mode", func() {
					BeforeEach(func() {
						cellID = "cell-id"
						consulClusterAddress = ""
					})

					It("emits the http route count", func() {
						Eventually(func() *events.Envelope {
							metric := <-testMetricsChan
							return metric
						}, "2s").Should(matchMetricAndValue(metricAndValue{Name: "HTTPRouteCount", Value: int32(2)}))
					})
				})

				Context("when backing store loses its data", func() {
					var msg1 routingtable.RegistryMessage
					var msg2 routingtable.RegistryMessage
					var msg3 routingtable.RegistryMessage
					var msg4 routingtable.RegistryMessage

					JustBeforeEach(func() {
						// ensure it's seen the route at least once
						Eventually(registeredRoutes).Should(Receive(&msg1))
						Eventually(registeredRoutes).Should(Receive(&msg2))

						sqlRunner.Reset()

						// Only start actual LRP, do not repopulate Desired
						err := bbsClient.StartActualLRP(logger, &lrpKey, &instanceKey, &netInfo)
						Expect(err).NotTo(HaveOccurred())
					})

					It("continues to broadcast routes", func() {
						Eventually(registeredRoutes, 5).Should(Receive(&msg3))
						Eventually(registeredRoutes, 5).Should(Receive(&msg4))
						Expect([]routingtable.RegistryMessage{msg3, msg4}).To(ConsistOf(
							MatchRegistryMessage(msg1),
							MatchRegistryMessage(msg2),
						))
					})
				})

				It("emits its routes immediately", func() {
					var msg1, msg2 routingtable.RegistryMessage
					Eventually(registeredRoutes).Should(Receive(&msg1))
					Eventually(registeredRoutes).Should(Receive(&msg2))

					Expect([]routingtable.RegistryMessage{msg1, msg2}).To(ConsistOf(
						MatchRegistryMessage(routingtable.RegistryMessage{
							URIs:                 []string{hostnames[1]},
							Host:                 netInfo.Address,
							Port:                 netInfo.Ports[0].HostPort,
							App:                  desiredLRP.LogGuid,
							PrivateInstanceId:    instanceKey.InstanceGuid,
							PrivateInstanceIndex: "0",
							RouteServiceUrl:      "https://awesome.com",
							Tags:                 map[string]string{"component": "route-emitter"},
						}),
						MatchRegistryMessage(routingtable.RegistryMessage{
							URIs:                 []string{hostnames[0]},
							Host:                 netInfo.Address,
							Port:                 netInfo.Ports[0].HostPort,
							App:                  desiredLRP.LogGuid,
							PrivateInstanceId:    instanceKey.InstanceGuid,
							PrivateInstanceIndex: "0",
							RouteServiceUrl:      "https://awesome.com",
							Tags:                 map[string]string{"component": "route-emitter"},
						}),
					))
				})

				Context("and the route-emitter is running in direct instance routes mode", func() {
					BeforeEach(func() {
						cfgs = append(cfgs, func(cfg *config.RouteEmitterConfig) {
							cfg.RegisterDirectInstanceRoutes = true
						})
					})

					It("emits its routes immediately", func() {
						var msg1, msg2 routingtable.RegistryMessage
						Eventually(registeredRoutes).Should(Receive(&msg1))
						Eventually(registeredRoutes).Should(Receive(&msg2))

						Expect([]routingtable.RegistryMessage{msg1, msg2}).To(ConsistOf(
							MatchRegistryMessage(routingtable.RegistryMessage{
								URIs:                 []string{hostnames[1]},
								Host:                 netInfo.InstanceAddress,
								Port:                 netInfo.Ports[0].ContainerPort,
								App:                  desiredLRP.LogGuid,
								PrivateInstanceId:    instanceKey.InstanceGuid,
								PrivateInstanceIndex: "0",
								RouteServiceUrl:      "https://awesome.com",
								Tags:                 map[string]string{"component": "route-emitter"},
							}),
							MatchRegistryMessage(routingtable.RegistryMessage{
								URIs:                 []string{hostnames[0]},
								Host:                 netInfo.InstanceAddress,
								Port:                 netInfo.Ports[0].ContainerPort,
								App:                  desiredLRP.LogGuid,
								PrivateInstanceId:    instanceKey.InstanceGuid,
								PrivateInstanceIndex: "0",
								RouteServiceUrl:      "https://awesome.com",
								Tags:                 map[string]string{"component": "route-emitter"},
							}),
						))
					})

				})

				Context("and the route-emitter cell id doesn't match the actual lrp cell", func() {
					BeforeEach(func() {
						cellID = "some-random-cell-id"
					})

					It("does not emit the route", func() {
						Consistently(registeredRoutes).ShouldNot(Receive())
					})
				})
			})

			Context("and an instance is claimed", func() {
				BeforeEach(func() {
					err := bbsClient.ClaimActualLRP(logger, processGuid, int(index), &instanceKey)
					Expect(err).NotTo(HaveOccurred())
				})

				It("does not emit routes", func() {
					Consistently(registeredRoutes).ShouldNot(Receive())
				})
			})
		})

		Context("an actual lrp starts without a routed desired lrp", func() {
			BeforeEach(func() {
				desiredLRP.Routes = nil
				err := bbsClient.DesireLRP(logger, desiredLRP)
				Expect(err).NotTo(HaveOccurred())

				err = bbsClient.StartActualLRP(logger, &lrpKey, &instanceKey, &netInfo)
				Expect(err).NotTo(HaveOccurred())
			})

			Context("and a route is desired", func() {
				BeforeEach(func() {
					update := &models.DesiredLRPUpdate{
						Routes: routes,
					}
					err := bbsClient.UpdateDesiredLRP(logger, desiredLRP.ProcessGuid, update)
					Expect(err).NotTo(HaveOccurred())
				})

				It("emits its routes immediately", func() {
					var msg1, msg2 routingtable.RegistryMessage
					Eventually(registeredRoutes).Should(Receive(&msg1))
					Eventually(registeredRoutes).Should(Receive(&msg2))

					Expect([]routingtable.RegistryMessage{msg1, msg2}).To(ConsistOf(
						MatchRegistryMessage(routingtable.RegistryMessage{
							URIs:                 []string{hostnames[1]},
							Host:                 netInfo.Address,
							Port:                 netInfo.Ports[0].HostPort,
							App:                  desiredLRP.LogGuid,
							PrivateInstanceId:    instanceKey.InstanceGuid,
							PrivateInstanceIndex: "0",
							RouteServiceUrl:      "https://awesome.com",
							Tags:                 map[string]string{"component": "route-emitter"},
						}),
						MatchRegistryMessage(routingtable.RegistryMessage{
							URIs:                 []string{hostnames[0]},
							Host:                 netInfo.Address,
							Port:                 netInfo.Ports[0].HostPort,
							App:                  desiredLRP.LogGuid,
							PrivateInstanceId:    instanceKey.InstanceGuid,
							PrivateInstanceIndex: "0",
							RouteServiceUrl:      "https://awesome.com",
							Tags:                 map[string]string{"component": "route-emitter"},
						}),
					))
				})

				It("repeats the route message at the interval given by the router", func() {
					var msg1 routingtable.RegistryMessage
					var msg2 routingtable.RegistryMessage
					Eventually(registeredRoutes).Should(Receive(&msg1))
					Eventually(registeredRoutes).Should(Receive(&msg2))
					t1 := time.Now()

					var msg3 routingtable.RegistryMessage
					var msg4 routingtable.RegistryMessage
					Eventually(registeredRoutes, 5).Should(Receive(&msg3))
					Eventually(registeredRoutes, 5).Should(Receive(&msg4))
					t2 := time.Now()

					Expect([]routingtable.RegistryMessage{msg3, msg4}).To(ConsistOf(
						MatchRegistryMessage(msg1),
						MatchRegistryMessage(msg2),
					))
					Expect(t2.Sub(t1)).To(BeNumerically("~", emitInterval, 100*time.Millisecond))
				})

				Context("when backing store goes away", func() {
					var msg1 routingtable.RegistryMessage
					var msg2 routingtable.RegistryMessage
					var msg3 routingtable.RegistryMessage
					var msg4 routingtable.RegistryMessage

					JustBeforeEach(func() {
						// ensure it's seen the route at least once
						Eventually(registeredRoutes).Should(Receive(&msg1))
						Eventually(registeredRoutes).Should(Receive(&msg2))

						stopBBS()
					})

					It("continues to broadcast routes", func() {
						Eventually(registeredRoutes, 10).Should(Receive(&msg3))
						Eventually(registeredRoutes, 10).Should(Receive(&msg4))
						Expect([]routingtable.RegistryMessage{msg3, msg4}).To(ConsistOf(
							MatchRegistryMessage(msg1),
							MatchRegistryMessage(msg2),
						))
					})
				})
			})
		})

		Context("and another emitter starts", func() {
			var (
				secondRunner        *ginkgomon.Runner
				secondEmitter       ifrit.Process
				secondEmitterConfig []func(*config.RouteEmitterConfig)
			)

			BeforeEach(func() {
				secondEmitterConfig = append(cfgs, func(cfg *config.RouteEmitterConfig) {
					cfg.HealthCheckAddress = fmt.Sprintf("127.0.0.1:%d", 4600+GinkgoParallelNode())
				})
				secondRunner = createEmitterRunner("emitter2", "", secondEmitterConfig...)
				secondRunner.StartCheck = "consul-lock.acquiring-lock"
			})

			JustBeforeEach(func() {
				secondEmitter = ginkgomon.Invoke(secondRunner)
			})

			AfterEach(func() {
				Expect(secondEmitter.Wait()).NotTo(Receive(), "Runner should not have exploded!")
				ginkgomon.Kill(secondEmitter, emitterInterruptTimeout)
			})

			Describe("the second emitter", func() {
				It("does not become active", func() {
					Consistently(secondRunner.Buffer, 5*time.Second).ShouldNot(gbytes.Say("emitter2.started"))
				})

				Context("runs in local mode", func() {
					BeforeEach(func() {
						secondEmitterConfig = append(cfgs, func(cfg *config.RouteEmitterConfig) {
							cfg.ConsulCluster = ""
							cfg.HealthCheckAddress = fmt.Sprintf("127.0.0.1:%d", 4600+GinkgoParallelNode())
						})
						secondRunner = createEmitterRunner("emitter2", "some-cell-id", secondEmitterConfig...)
						secondRunner.StartCheck = "emitter2.watcher.sync.complete"
					})

					It("becomes active and does not connect to consul to acquire the lock", func() {
						Eventually(secondRunner.Buffer).Should(gbytes.Say("emitter2.started"))
					})
				})
			})

			Context("and the first emitter goes away", func() {
				JustBeforeEach(func() {
					ginkgomon.Kill(emitter, emitterInterruptTimeout)
				})

				Describe("the second emitter", func() {
					It("becomes active", func() {
						Eventually(secondRunner.Buffer, locket.DefaultSessionTTL*2).Should(gbytes.Say("emitter2.started"))
					})
				})
			})
		})

		Context("and backing store goes away", func() {
			BeforeEach(func() {
				stopBBS()
			})

			It("does not explode", func() {
				Consistently(emitter.Wait(), 5).ShouldNot(Receive())
			})
		})

		It("emits a metric to say that it is not in consul down mode", func() {
			Eventually(testMetricsChan).Should(Receive(matchMetricAndValue(metricAndValue{Name: "ConsulDownMode", Value: 0})))
		})
	})

	Describe("consul down mode", func() {
		var (
			emitter           ifrit.Process
			runner            *ginkgomon.Runner
			fakeConsul        *httptest.Server
			fakeConsulHandler http.HandlerFunc
			handlerWriteLock  *sync.Mutex
			cellID            string
		)

		BeforeEach(func() {
			cellID = ""
			consulClusterURL, err := url.Parse(consulRunner.ConsulCluster())
			Expect(err).NotTo(HaveOccurred())
			fakeConsulHandler = nil

			handlerWriteLock = &sync.Mutex{}
			proxy := httputil.NewSingleHostReverseProxy(consulClusterURL)
			fakeConsul = httptest.NewUnstartedServer(
				http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					handlerWriteLock.Lock()
					defer handlerWriteLock.Unlock()
					if fakeConsulHandler != nil {
						fakeConsulHandler(w, r)
					} else {
						proxy.ServeHTTP(w, r)
					}
				}),
			)
			fakeConsul.Start()

			consulClusterAddress = fakeConsul.URL
		})

		JustBeforeEach(func() {
			runner = createEmitterRunner("emitter1", cellID, cfgs...)
			runner.StartCheck = "emitter1.started"
			emitter = ginkgomon.Invoke(runner)
		})

		AfterEach(func() {
			fakeConsul.Close()
			ginkgomon.Kill(emitter, emitterInterruptTimeout)
		})

		Context("when consul goes down", func() {
			var (
				msg1 routingtable.RegistryMessage
				msg2 routingtable.RegistryMessage
			)

			JustBeforeEach(func() {
				err := bbsClient.DesireLRP(logger, desiredLRP)
				Expect(err).NotTo(HaveOccurred())

				err = bbsClient.StartActualLRP(logger, &lrpKey, &instanceKey, &netInfo)
				Expect(err).NotTo(HaveOccurred())

				Eventually(registeredRoutes).Should(Receive(&msg1))
				Eventually(registeredRoutes).Should(Receive(&msg2))

				handlerWriteLock.Lock()
				fakeConsulHandler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.WriteHeader(500)
					w.Write([]byte(`"No known Consul servers"`))
				})
				handlerWriteLock.Unlock()
				consulRunner.Stop()
			})

			Context("when in local mode", func() {
				var receiveCh chan struct{}

				BeforeEach(func() {
					cellID = "cell-local"
					instanceKey.CellId = cellID
				})

				JustBeforeEach(func() {

					receiveCh = make(chan struct{}, 1)
					handlerWriteLock.Lock()
					fakeConsulHandler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
						receiveCh <- struct{}{}
					})
					handlerWriteLock.Unlock()
				})

				It("does not connect to consul", func() {
					Consistently(receiveCh, 5*time.Second).ShouldNot(Receive())
				})
			})

			It("enters consul down mode and exits when consul comes back up", func() {
				lockTTL := 5
				retryInterval := 1
				Eventually(runner, lockTTL+3*retryInterval+1).Should(gbytes.Say("consul-down-mode.started"))
				consulRunner.Start()
				handlerWriteLock.Lock()
				fakeConsulHandler = nil
				handlerWriteLock.Unlock()
				Eventually(runner, 6*retryInterval+1).Should(gbytes.Say("consul-down-mode.exited"))
				var err error
				Eventually(emitter.Wait()).Should(Receive(&err))
				Expect(err).NotTo(HaveOccurred())
			})

			It("emits a metric to say that it has entered consul down mode", func() {
				lockTTL := 5
				retryInterval := 1
				Eventually(runner, lockTTL+3*retryInterval+1).Should(gbytes.Say("consul-down-mode.started"))

				Eventually(testMetricsChan, 3*retryInterval+1).Should(Receive(matchMetricAndValue(metricAndValue{Name: "ConsulDownMode", Value: 1})))
			})

			It("repeats the route message at the interval given by the router", func() {
				var msg3 routingtable.RegistryMessage
				var msg4 routingtable.RegistryMessage
				Eventually(registeredRoutes, 5).Should(Receive(&msg3))
				Eventually(registeredRoutes, 5).Should(Receive(&msg4))

				Expect([]routingtable.RegistryMessage{msg3, msg4}).To(ConsistOf(
					MatchRegistryMessage(msg1),
					MatchRegistryMessage(msg2),
				))
			})
		})
	})

	Context("when the legacyBBS has routes to emit in /desired and /actual", func() {
		var emitter ifrit.Process

		BeforeEach(func() {
			err := bbsClient.DesireLRP(logger, desiredLRP)
			Expect(err).NotTo(HaveOccurred())

			err = bbsClient.StartActualLRP(logger, &lrpKey, &instanceKey, &netInfo)
			Expect(err).NotTo(HaveOccurred())
		})

		Context("and the emitter is started", func() {
			BeforeEach(func() {
				emitter = ginkgomon.Invoke(createEmitterRunner("route-emitter", "", cfgs...))
			})

			AfterEach(func() {
				ginkgomon.Kill(emitter, emitterInterruptTimeout)
			})

			It("immediately emits all routes", func() {
				var msg1, msg2 routingtable.RegistryMessage
				Eventually(registeredRoutes).Should(Receive(&msg1))
				Eventually(registeredRoutes).Should(Receive(&msg2))

				Expect([]routingtable.RegistryMessage{msg1, msg2}).To(ConsistOf(
					MatchRegistryMessage(routingtable.RegistryMessage{
						URIs:                 []string{"route-1"},
						Host:                 "1.2.3.4",
						Port:                 65100,
						App:                  "some-log-guid",
						PrivateInstanceId:    "iguid1",
						PrivateInstanceIndex: "0",
						RouteServiceUrl:      "https://awesome.com",
						Tags:                 map[string]string{"component": "route-emitter"},
					}),
					MatchRegistryMessage(routingtable.RegistryMessage{
						URIs:                 []string{"route-2"},
						Host:                 "1.2.3.4",
						Port:                 65100,
						App:                  "some-log-guid",
						PrivateInstanceId:    "iguid1",
						PrivateInstanceIndex: "0",
						RouteServiceUrl:      "https://awesome.com",
						Tags:                 map[string]string{"component": "route-emitter"},
					}),
				))
			})

			Context("and a route is added", func() {
				BeforeEach(func() {
					Eventually(registeredRoutes).Should(Receive())
					Eventually(registeredRoutes).Should(Receive())

					hostnames = []string{"route-1", "route-2", "route-3"}

					updateRequest := &models.DesiredLRPUpdate{
						Routes:     newRoutes(hostnames, containerPort, ""),
						Instances:  &desiredLRP.Instances,
						Annotation: &desiredLRP.Annotation,
					}
					err := bbsClient.UpdateDesiredLRP(logger, processGuid, updateRequest)
					Expect(err).NotTo(HaveOccurred())
				})

				It("immediately emits router.register", func() {
					var msg1, msg2, msg3 routingtable.RegistryMessage
					Eventually(registeredRoutes).Should(Receive(&msg1))
					Eventually(registeredRoutes).Should(Receive(&msg2))
					Eventually(registeredRoutes).Should(Receive(&msg3))

					registryMessages := []routingtable.RegistryMessage{}
					for _, hostname := range hostnames {
						registryMessages = append(registryMessages, routingtable.RegistryMessage{
							URIs:                 []string{hostname},
							Host:                 "1.2.3.4",
							Port:                 65100,
							App:                  "some-log-guid",
							PrivateInstanceId:    "iguid1",
							PrivateInstanceIndex: "0",
							Tags:                 map[string]string{"component": "route-emitter"},
						})
					}
					Expect([]routingtable.RegistryMessage{msg1, msg2, msg3}).To(ConsistOf(
						MatchRegistryMessage(registryMessages[0]),
						MatchRegistryMessage(registryMessages[1]),
						MatchRegistryMessage(registryMessages[2]),
					))
				})
			})

			Context("and a route is removed", func() {
				BeforeEach(func() {
					updateRequest := &models.DesiredLRPUpdate{
						Routes:     newRoutes([]string{"route-2"}, containerPort, ""),
						Instances:  &desiredLRP.Instances,
						Annotation: &desiredLRP.Annotation,
					}
					err := bbsClient.UpdateDesiredLRP(logger, processGuid, updateRequest)
					Expect(err).NotTo(HaveOccurred())
				})

				It("immediately emits router.unregister when domain is fresh", func() {
					bbsClient.UpsertDomain(logger, domain, 2*time.Second)
					Eventually(unregisteredRoutes, msgReceiveTimeout).Should(Receive(
						MatchRegistryMessage(routingtable.RegistryMessage{
							URIs:                 []string{"route-1"},
							Host:                 "1.2.3.4",
							Port:                 65100,
							App:                  "some-log-guid",
							PrivateInstanceId:    "iguid1",
							PrivateInstanceIndex: "0",
							RouteServiceUrl:      "https://awesome.com",
							Tags:                 map[string]string{"component": "route-emitter"},
						}),
					))
					Eventually(registeredRoutes, msgReceiveTimeout).Should(Receive(
						MatchRegistryMessage(routingtable.RegistryMessage{
							URIs:                 []string{"route-2"},
							Host:                 "1.2.3.4",
							Port:                 65100,
							App:                  "some-log-guid",
							PrivateInstanceId:    "iguid1",
							PrivateInstanceIndex: "0",
							Tags:                 map[string]string{"component": "route-emitter"},
						}),
					))
				})
			})
		})
	})

	Context("when desired lrp is missing and actual lrp created event is received during the sync loop", func() {
		var (
			fakeBBS    *httptest.Server
			blkChannel chan struct{}
			runner     *ginkgomon.Runner
			emitter    ifrit.Process
		)

		BeforeEach(func() {
			blkChannel = make(chan struct{}, 1)

			proxy := httputil.NewSingleHostReverseProxy(bbsURL)
			proxy.FlushInterval = 100 * time.Millisecond
			fakeBBS = httptest.NewUnstartedServer(
				http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					if r.URL.Path == "/v1/domains/list" {
						By("blocking the sync loop")
						<-blkChannel
					}
					proxy.ServeHTTP(w, r)
				}),
			)

			cfgs = append(cfgs, func(cfg *config.RouteEmitterConfig) {
				cfg.BBSAddress = fakeBBS.URL
				cfg.RoutingAPI.URL = "http://127.0.0.1"
				cfg.RoutingAPI.Port = routingAPIRunner.Config.Port
				cfg.CommunicationTimeout = durationjson.Duration(5 * time.Second)
				cfg.SyncInterval = durationjson.Duration(1 * time.Hour)
			})

			fakeBBS.Start()
			runner = createEmitterRunner("route-emitter", "cell-id", cfgs...)
			Expect(bbsClient.UpsertDomain(logger, domain, time.Hour)).To(Succeed())
			Expect(bbsClient.DesireLRP(logger, desiredLRP)).To(Succeed())
		})

		It("should refresh the desired lrp and emit a route registration", func() {
			By("waiting for the sync loop to start")
			runner.StartCheck = "succeeded-getting-actual-lrps"
			emitter = ginkgomon.Invoke(runner)

			Expect(bbsClient.StartActualLRP(logger, &lrpKey, &instanceKey, &netInfo)).To(Succeed())
			Eventually(runner).Should(gbytes.Say("caching-event"))

			By("unblocking the sync loop")
			close(blkChannel)

			var msg1 routingtable.RegistryMessage
			Eventually(registeredRoutes).Should(Receive(&msg1))
			Expect(msg1.PrivateInstanceId).To(Equal(instanceKey.GetInstanceGuid()))
		})

		AfterEach(func() {
			ginkgomon.Kill(emitter, emitterInterruptTimeout)
			fakeBBS.Close()
		})
	})
})

func newRoutes(hosts []string, port uint32, routeServiceUrl string) *models.Routes {
	routingInfo := cfroutes.CFRoutes{
		{Hostnames: hosts, Port: port, RouteServiceUrl: routeServiceUrl},
	}.RoutingInfo()

	routes := models.Routes{}

	for key, message := range routingInfo {
		routes[key] = message
	}

	return &routes
}

type metricAndValue struct {
	Name  string
	Value int32
}

func matchMetricAndValue(target metricAndValue) types.GomegaMatcher {
	return SatisfyAll(
		WithTransform(func(source *events.Envelope) events.Envelope_EventType {
			return *source.EventType
		}, Equal(events.Envelope_ValueMetric)),
		WithTransform(func(source *events.Envelope) string {
			return *source.ValueMetric.Name
		}, Equal(target.Name)),
		WithTransform(func(source *events.Envelope) int32 {
			return int32(*source.ValueMetric.Value)
		}, Equal(target.Value)),
	)
}

func contains(ms []apimodels.TcpRouteMapping, tcpRouteMapping apimodels.TcpRouteMapping) bool {
	for _, m := range ms {
		if m.Matches(tcpRouteMapping) {
			return true
		}
	}
	return false
}
