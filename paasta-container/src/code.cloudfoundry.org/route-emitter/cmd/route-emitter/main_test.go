package main_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/http/httputil"
	"net/url"
	"sync"
	"time"

	"code.cloudfoundry.org/bbs/models"
	"code.cloudfoundry.org/route-emitter/routing_table"
	. "code.cloudfoundry.org/route-emitter/routing_table/matchers"
	"code.cloudfoundry.org/routing-info/cfroutes"
	"github.com/cloudfoundry/sonde-go/events"
	"github.com/nats-io/nats"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/types"
	"github.com/tedsuo/ifrit"
	"github.com/tedsuo/ifrit/ginkgomon"
)

const emitterInterruptTimeout = 5 * time.Second
const msgReceiveTimeout = 5 * time.Second

var _ = Describe("Route Emitter", func() {
	listenForRoutes := func(subject string) <-chan routing_table.RegistryMessage {
		routes := make(chan routing_table.RegistryMessage)

		natsClient.Subscribe(subject, func(msg *nats.Msg) {
			defer GinkgoRecover()

			var message routing_table.RegistryMessage
			err := json.Unmarshal(msg.Data, &message)
			Expect(err).NotTo(HaveOccurred())

			routes <- message
		})

		return routes
	}

	var (
		registeredRoutes   <-chan routing_table.RegistryMessage
		unregisteredRoutes <-chan routing_table.RegistryMessage

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
	)

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

		netInfo = models.NewActualLRPNetInfo("1.2.3.4", models.NewPortMapping(65100, 8080))
		registeredRoutes = listenForRoutes("router.register")
		unregisteredRoutes = listenForRoutes("router.unregister")

		natsClient.Subscribe("router.greet", func(msg *nats.Msg) {
			defer GinkgoRecover()

			greeting := routing_table.RouterGreetingMessage{
				MinimumRegisterInterval: 2,
				PruneThresholdInSeconds: 6,
			}

			response, err := json.Marshal(greeting)
			Expect(err).NotTo(HaveOccurred())

			err = natsClient.Publish(msg.Reply, response)
			Expect(err).NotTo(HaveOccurred())
		})
	})

	Context("Ping interval for nats client", func() {
		var runner *ginkgomon.Runner
		var emitter ifrit.Process

		BeforeEach(func() {
			runner = createEmitterRunner("emitter1")
			runner.StartCheck = "emitter1.started"
			emitter = ginkgomon.Invoke(runner)
		})

		AfterEach(func() {
			ginkgomon.Interrupt(emitter, emitterInterruptTimeout)
		})

		It("returns 20 second", func() {
			Expect(runner).To(gbytes.Say("setting-nats-ping-interval"))
			Expect(runner).To(gbytes.Say(`"duration-in-seconds":20`))
		})
	})

	Context("when the emitter is running", func() {
		var (
			emitter ifrit.Process
			runner  *ginkgomon.Runner
		)

		BeforeEach(func() {
			runner = createEmitterRunner("emitter1")
			runner.StartCheck = "emitter1.started"
			emitter = ginkgomon.Invoke(runner)
		})

		AfterEach(func() {
			ginkgomon.Interrupt(emitter, emitterInterruptTimeout)
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

				Context("when etcd loses its data", func() {
					var msg1 routing_table.RegistryMessage
					var msg2 routing_table.RegistryMessage
					var msg3 routing_table.RegistryMessage
					var msg4 routing_table.RegistryMessage

					BeforeEach(func() {
						// ensure it's seen the route at least once
						Eventually(registeredRoutes).Should(Receive(&msg1))
						Eventually(registeredRoutes).Should(Receive(&msg2))

						etcdRunner.Reset()

						// Only start actual LRP, do not repopulate Desired
						err := bbsClient.StartActualLRP(logger, &lrpKey, &instanceKey, &netInfo)
						Expect(err).NotTo(HaveOccurred())
					})

					It("continues to broadcast routes", func() {
						Eventually(registeredRoutes, 5).Should(Receive(&msg3))
						Eventually(registeredRoutes, 5).Should(Receive(&msg4))
						Expect([]routing_table.RegistryMessage{msg3, msg4}).To(ConsistOf(
							MatchRegistryMessage(msg1),
							MatchRegistryMessage(msg2),
						))
					})
				})

				It("emits its routes immediately", func() {
					var msg1, msg2 routing_table.RegistryMessage
					Eventually(registeredRoutes).Should(Receive(&msg1))
					Eventually(registeredRoutes).Should(Receive(&msg2))

					Expect([]routing_table.RegistryMessage{msg1, msg2}).To(ConsistOf(
						MatchRegistryMessage(routing_table.RegistryMessage{
							URIs:                 []string{hostnames[1]},
							Host:                 netInfo.Address,
							Port:                 netInfo.Ports[0].HostPort,
							App:                  desiredLRP.LogGuid,
							PrivateInstanceId:    instanceKey.InstanceGuid,
							PrivateInstanceIndex: "0",
							RouteServiceUrl:      "https://awesome.com",
							Tags:                 map[string]string{"component": "route-emitter"},
						}),
						MatchRegistryMessage(routing_table.RegistryMessage{
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
					var msg1, msg2 routing_table.RegistryMessage
					Eventually(registeredRoutes).Should(Receive(&msg1))
					Eventually(registeredRoutes).Should(Receive(&msg2))

					Expect([]routing_table.RegistryMessage{msg1, msg2}).To(ConsistOf(
						MatchRegistryMessage(routing_table.RegistryMessage{
							URIs:                 []string{hostnames[1]},
							Host:                 netInfo.Address,
							Port:                 netInfo.Ports[0].HostPort,
							App:                  desiredLRP.LogGuid,
							PrivateInstanceId:    instanceKey.InstanceGuid,
							PrivateInstanceIndex: "0",
							RouteServiceUrl:      "https://awesome.com",
							Tags:                 map[string]string{"component": "route-emitter"},
						}),
						MatchRegistryMessage(routing_table.RegistryMessage{
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
					var msg1 routing_table.RegistryMessage
					var msg2 routing_table.RegistryMessage
					Eventually(registeredRoutes).Should(Receive(&msg1))
					Eventually(registeredRoutes).Should(Receive(&msg2))
					t1 := time.Now()

					var msg3 routing_table.RegistryMessage
					var msg4 routing_table.RegistryMessage
					Eventually(registeredRoutes, 5).Should(Receive(&msg3))
					Eventually(registeredRoutes, 5).Should(Receive(&msg4))
					t2 := time.Now()

					Expect([]routing_table.RegistryMessage{msg3, msg4}).To(ConsistOf(
						MatchRegistryMessage(msg1),
						MatchRegistryMessage(msg2),
					))
					Expect(t2.Sub(t1)).To(BeNumerically("~", 2*syncInterval, 500*time.Millisecond))
				})

				Context("when etcd goes away", func() {
					var msg1 routing_table.RegistryMessage
					var msg2 routing_table.RegistryMessage
					var msg3 routing_table.RegistryMessage
					var msg4 routing_table.RegistryMessage

					BeforeEach(func() {
						// ensure it's seen the route at least once
						Eventually(registeredRoutes).Should(Receive(&msg1))
						Eventually(registeredRoutes).Should(Receive(&msg2))

						etcdRunner.Stop()
					})

					It("continues to broadcast routes", func() {
						Eventually(registeredRoutes, 5).Should(Receive(&msg3))
						Eventually(registeredRoutes, 5).Should(Receive(&msg4))
						Expect([]routing_table.RegistryMessage{msg3, msg4}).To(ConsistOf(
							MatchRegistryMessage(msg1),
							MatchRegistryMessage(msg2),
						))
					})
				})
			})
		})

		Context("and another emitter starts", func() {
			var (
				secondRunner  *ginkgomon.Runner
				secondEmitter ifrit.Process
			)

			BeforeEach(func() {
				secondRunner = createEmitterRunner("emitter2")
				secondRunner.StartCheck = "lock.acquiring-lock"

				secondEmitter = ginkgomon.Invoke(secondRunner)
			})

			AfterEach(func() {
				Expect(secondEmitter.Wait()).NotTo(Receive(), "Runner should not have exploded!")
				ginkgomon.Interrupt(secondEmitter, emitterInterruptTimeout)
			})

			Describe("the second emitter", func() {
				It("does not become active", func() {
					Consistently(secondRunner.Buffer, 5*time.Second).ShouldNot(gbytes.Say("emitter2.started"))
				})
			})

			Context("and the first emitter goes away", func() {
				BeforeEach(func() {
					ginkgomon.Interrupt(emitter, emitterInterruptTimeout)
				})

				Describe("the second emitter", func() {
					It("becomes active", func() {
						Eventually(secondRunner.Buffer, 10).Should(gbytes.Say("emitter2.started"))
					})
				})
			})
		})

		Context("and etcd goes away", func() {
			BeforeEach(func() {
				etcdRunner.Stop()
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
		)

		BeforeEach(func() {
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
			runner = createEmitterRunner("emitter1")
			runner.StartCheck = "emitter1.started"
			emitter = ginkgomon.Invoke(runner)
		})

		AfterEach(func() {
			fakeConsul.Close()
			ginkgomon.Interrupt(emitter, emitterInterruptTimeout)
		})

		Context("when consul goes down", func() {
			var (
				msg1 routing_table.RegistryMessage
				msg2 routing_table.RegistryMessage
			)

			BeforeEach(func() {
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
				var msg3 routing_table.RegistryMessage
				var msg4 routing_table.RegistryMessage
				Eventually(registeredRoutes, 5).Should(Receive(&msg3))
				Eventually(registeredRoutes, 5).Should(Receive(&msg4))

				Expect([]routing_table.RegistryMessage{msg3, msg4}).To(ConsistOf(
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
				emitter = ginkgomon.Invoke(createEmitterRunner("route-emitter"))
			})

			AfterEach(func() {
				ginkgomon.Interrupt(emitter, emitterInterruptTimeout)
			})

			It("immediately emits all routes", func() {
				var msg1, msg2 routing_table.RegistryMessage
				Eventually(registeredRoutes).Should(Receive(&msg1))
				Eventually(registeredRoutes).Should(Receive(&msg2))

				Expect([]routing_table.RegistryMessage{msg1, msg2}).To(ConsistOf(
					MatchRegistryMessage(routing_table.RegistryMessage{
						URIs:                 []string{"route-1"},
						Host:                 "1.2.3.4",
						Port:                 65100,
						App:                  "some-log-guid",
						PrivateInstanceId:    "iguid1",
						PrivateInstanceIndex: "0",
						RouteServiceUrl:      "https://awesome.com",
						Tags:                 map[string]string{"component": "route-emitter"},
					}),
					MatchRegistryMessage(routing_table.RegistryMessage{
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
					var msg1, msg2, msg3 routing_table.RegistryMessage
					Eventually(registeredRoutes).Should(Receive(&msg1))
					Eventually(registeredRoutes).Should(Receive(&msg2))
					Eventually(registeredRoutes).Should(Receive(&msg3))

					registryMessages := []routing_table.RegistryMessage{}
					for _, hostname := range hostnames {
						registryMessages = append(registryMessages, routing_table.RegistryMessage{
							URIs:                 []string{hostname},
							Host:                 "1.2.3.4",
							Port:                 65100,
							App:                  "some-log-guid",
							PrivateInstanceId:    "iguid1",
							PrivateInstanceIndex: "0",
							Tags:                 map[string]string{"component": "route-emitter"},
						})
					}
					Expect([]routing_table.RegistryMessage{msg1, msg2, msg3}).To(ConsistOf(
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
						MatchRegistryMessage(routing_table.RegistryMessage{
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
						MatchRegistryMessage(routing_table.RegistryMessage{
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
