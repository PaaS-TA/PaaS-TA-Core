package locket_test

import (
	"net/http"
	"net/http/httptest"
	"net/http/httputil"
	"net/url"
	"strings"
	"time"

	"code.cloudfoundry.org/cfhttp"
	"code.cloudfoundry.org/consuladapter"
	"code.cloudfoundry.org/locket"
	"github.com/cloudfoundry/dropsonde/metric_sender/fake"
	"github.com/cloudfoundry/dropsonde/metrics"
	"github.com/hashicorp/consul/api"

	"code.cloudfoundry.org/clock/fakeclock"
	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/lager/lagertest"
	"github.com/tedsuo/ifrit"
	"github.com/tedsuo/ifrit/ginkgomon"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gbytes"
)

var _ = Describe("Lock", func() {
	var (
		lockKey              string
		lockHeldMetricName   string
		lockUptimeMetricName string
		lockValue            []byte

		consulClient consuladapter.Client

		lockRunner    ifrit.Runner
		lockProcess   ifrit.Process
		retryInterval time.Duration
		lockTTL       time.Duration
		logger        lager.Logger

		sender *fake.FakeMetricSender
		clock  *fakeclock.FakeClock
	)

	getLockValue := func() ([]byte, error) {
		kvPair, _, err := consulClient.KV().Get(lockKey, nil)
		if err != nil {
			return nil, err
		}

		if kvPair == nil || kvPair.Session == "" {
			return nil, consuladapter.NewKeyNotFoundError(lockKey)
		}

		return kvPair.Value, nil
	}

	BeforeEach(func() {
		consulClient = consulRunner.NewClient()

		lockKey = locket.LockSchemaPath("some-key")
		lockKeyMetric := strings.Replace(lockKey, "/", "-", -1)
		lockHeldMetricName = "LockHeld." + lockKeyMetric
		lockUptimeMetricName = "LockHeldDuration." + lockKeyMetric
		lockValue = []byte("some-value")

		retryInterval = 500 * time.Millisecond
		lockTTL = 5 * time.Second
		logger = lagertest.NewTestLogger("locket")

		sender = fake.NewFakeMetricSender()
		metrics.Initialize(sender, nil)
	})

	JustBeforeEach(func() {
		clock = fakeclock.NewFakeClock(time.Now())
		lockRunner = locket.NewLock(logger, consulClient, lockKey, lockValue, clock, retryInterval, lockTTL)
	})

	AfterEach(func() {
		ginkgomon.Kill(lockProcess)
	})

	var shouldEventuallyHaveNumSessions = func(numSessions int) {
		Eventually(func() int {
			sessions, _, err := consulClient.Session().List(nil)
			Expect(err).NotTo(HaveOccurred())
			return len(sessions)
		}).Should(Equal(numSessions))
	}

	Context("When consul is running", func() {
		Context("an error occurs while acquiring the lock", func() {
			BeforeEach(func() {
				lockKey = ""
			})

			It("continues to retry", func() {
				lockProcess = ifrit.Background(lockRunner)
				shouldEventuallyHaveNumSessions(1)
				Consistently(lockProcess.Ready()).ShouldNot(BeClosed())
				Consistently(lockProcess.Wait()).ShouldNot(Receive())

				clock.Increment(retryInterval)
				Eventually(logger).Should(Say("acquire-lock-failed"))
				Eventually(logger).Should(Say("retrying-acquiring-lock"))
				Expect(sender.GetValue(lockHeldMetricName).Value).To(Equal(float64(0)))
			})
		})

		Context("and the lock is available", func() {
			It("acquires the lock", func() {
				lockProcess = ifrit.Background(lockRunner)
				Eventually(lockProcess.Ready()).Should(BeClosed())
				Expect(sender.GetValue(lockUptimeMetricName).Value).Should(Equal(float64(0)))
				Expect(getLockValue()).To(Equal(lockValue))
				Expect(sender.GetValue(lockHeldMetricName).Value).To(Equal(float64(1)))
			})

			Context("and we have acquired the lock", func() {
				JustBeforeEach(func() {
					lockProcess = ifrit.Background(lockRunner)
					Eventually(lockProcess.Ready()).Should(BeClosed())
				})

				It("continues to emit lock metric", func() {
					clock.IncrementBySeconds(30)
					Eventually(func() float64 {
						return sender.GetValue(lockUptimeMetricName).Value
					}, 2).Should(Equal(float64(30 * time.Second)))
					clock.IncrementBySeconds(30)
					Eventually(func() float64 {
						return sender.GetValue(lockUptimeMetricName).Value
					}, 2).Should(Equal(float64(60 * time.Second)))
					clock.IncrementBySeconds(30)
					Eventually(func() float64 {
						return sender.GetValue(lockUptimeMetricName).Value
					}, 2).Should(Equal(float64(90 * time.Second)))
				})

				Context("when consul shuts down", func() {
					JustBeforeEach(func() {
						consulRunner.Stop()
					})

					AfterEach(func() {
						consulRunner.Start()
						consulRunner.WaitUntilReady()
					})

					It("loses the lock and exits", func() {
						var err error
						Eventually(lockProcess.Wait()).Should(Receive(&err))
						Expect(err).To(Equal(locket.ErrLockLost))
						Expect(sender.GetValue(lockHeldMetricName).Value).To(Equal(float64(0)))
					})
				})

				Context("and the process is shutting down", func() {
					It("releases the lock and exits", func() {
						ginkgomon.Interrupt(lockProcess)
						Eventually(lockProcess.Wait()).Should(Receive(BeNil()))
						_, err := getLockValue()
						Expect(err).To(Equal(consuladapter.NewKeyNotFoundError(lockKey)))
					})
				})

				Context("and consul goes through a period of instability", func() {
					var serveFiveHundreds chan struct{}
					var fakeConsul *httptest.Server

					BeforeEach(func() {
						serveFiveHundreds = make(chan struct{}, 4)

						consulClusterURL, err := url.Parse(consulRunner.URL())
						Expect(err).NotTo(HaveOccurred())
						proxy := httputil.NewSingleHostReverseProxy(consulClusterURL)
						fakeConsul = httptest.NewServer(
							http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
								// We only want to return 500's on the lock monitor query
								if !strings.Contains(r.URL.Path, "/v1/lock") {
									proxy.ServeHTTP(w, r)
									return
								}

								select {
								case <-serveFiveHundreds:
									w.WriteHeader(http.StatusInternalServerError)
								default:
									proxy.ServeHTTP(w, r)
								}
							}),
						)

						fakeConsulURL, err := url.Parse(fakeConsul.URL)
						Expect(err).NotTo(HaveOccurred())

						client, err := api.NewClient(&api.Config{
							Address:    fakeConsulURL.Host,
							Scheme:     fakeConsulURL.Scheme,
							HttpClient: cfhttp.NewStreamingClient(),
						})
						Expect(err).NotTo(HaveOccurred())

						consulClient = consuladapter.NewConsulClient(client)
						lockTTL = 6 * time.Second
					})

					Context("for longer than the MonitorRetries * MonitorRetryTime", func() {
						It("loses lock", func() {
							Expect(sender.GetValue(lockUptimeMetricName).Value).Should(Equal(float64(0)))
							Expect(getLockValue()).To(Equal(lockValue))
							Expect(sender.GetValue(lockHeldMetricName).Value).To(Equal(float64(1)))

							// Serve 500's to simulate a leader election. We know that we need
							// to serve more than lockTTL / 2 500's to lose the lock.
							for i := 0; i < 4; i++ {
								Eventually(serveFiveHundreds).Should(BeSent(struct{}{}))
							}
							// Close the existing connection with consul so that the
							// lock monitor is forced to retry. This is because consul
							// performs a blocking query until the lock index is changed.
							fakeConsul.CloseClientConnections()

							Eventually(lockProcess.Wait(), 7*time.Second).Should(Receive())
						})
					})

					Context("for less than the MonitorRetries * MonitorRetryTime", func() {
						It("does not lose the lock", func() {
							Expect(sender.GetValue(lockUptimeMetricName).Value).Should(Equal(float64(0)))
							Expect(getLockValue()).To(Equal(lockValue))
							Expect(sender.GetValue(lockHeldMetricName).Value).To(Equal(float64(1)))

							// Serve 500's to simulate a leader election. We know that if we
							// serve less than lockTTL / 2 500's, we will not lose the lock.
							for i := 0; i < 2; i++ {
								Eventually(serveFiveHundreds).Should(BeSent(struct{}{}))
							}
							// Close the existing connection with consul so that the
							// lock monitor is forced to retry. This is because consul
							// performs a blocking query until the lock index is changed.
							fakeConsul.CloseClientConnections()

							Consistently(lockProcess.Wait(), 7*time.Second).ShouldNot(Receive())
						})
					})
				})
			})
		})

		Context("and the lock is unavailable", func() {
			var (
				otherProcess ifrit.Process
				otherValue   []byte
			)

			BeforeEach(func() {
				otherValue = []byte("doppel-value")
				otherClock := fakeclock.NewFakeClock(time.Now())

				otherRunner := locket.NewLock(logger, consulClient, lockKey, otherValue, otherClock, retryInterval, 5*time.Second)
				otherProcess = ifrit.Background(otherRunner)

				Eventually(otherProcess.Ready()).Should(BeClosed())
				Expect(getLockValue()).To(Equal(otherValue))
			})

			AfterEach(func() {
				ginkgomon.Interrupt(otherProcess)
			})

			It("waits for the lock to become available", func() {
				lockProcess = ifrit.Background(lockRunner)
				Consistently(lockProcess.Ready()).ShouldNot(BeClosed())
				Expect(getLockValue()).To(Equal(otherValue))
			})

			Context("when consul shuts down", func() {
				JustBeforeEach(func() {
					lockProcess = ifrit.Background(lockRunner)
					shouldEventuallyHaveNumSessions(1)

					consulRunner.Stop()
				})

				AfterEach(func() {
					consulRunner.Start()
					consulRunner.WaitUntilReady()
				})

				It("continues to wait for the lock", func() {
					Consistently(lockProcess.Ready()).ShouldNot(BeClosed())
					Consistently(lockProcess.Wait()).ShouldNot(Receive())

					Eventually(logger).Should(Say("acquire-lock-failed"))
					clock.Increment(retryInterval)
					Eventually(logger).Should(Say("retrying-acquiring-lock"))
					Expect(sender.GetValue(lockHeldMetricName).Value).To(Equal(float64(0)))
				})
			})

			Context("and the session is destroyed", func() {
				It("should recreate the session and continue to retry", func() {
					lockProcess = ifrit.Background(lockRunner)

					shouldEventuallyHaveNumSessions(2)

					sessions, _, err := consulClient.Session().List(nil)
					Expect(err).NotTo(HaveOccurred())
					var mostRecentSession *api.SessionEntry
					for _, session := range sessions {
						if mostRecentSession == nil {
							mostRecentSession = session
						} else if session.CreateIndex > mostRecentSession.CreateIndex {
							mostRecentSession = session
						}
					}

					_, err = consulClient.Session().Destroy(mostRecentSession.ID, nil)
					Expect(err).NotTo(HaveOccurred())

					Eventually(logger, 10*time.Second).Should(Say("consul-error"))
					Eventually(logger, 15*time.Second).Should(Say("acquire-lock-failed"))
					clock.Increment(retryInterval)
					Eventually(logger).Should(Say("retrying-acquiring-lock"))
					shouldEventuallyHaveNumSessions(2)
				})
			})

			Context("and the process is shutting down", func() {
				It("exits", func() {
					lockProcess = ifrit.Background(lockRunner)
					shouldEventuallyHaveNumSessions(2)

					ginkgomon.Interrupt(lockProcess)
					Eventually(lockProcess.Wait()).Should(Receive(BeNil()))
				})
			})

			Context("and the lock is released", func() {
				It("acquires the lock", func() {
					lockProcess = ifrit.Background(lockRunner)
					Consistently(lockProcess.Ready()).ShouldNot(BeClosed())
					Expect(getLockValue()).To(Equal(otherValue))

					ginkgomon.Interrupt(otherProcess)

					Eventually(lockProcess.Ready(), 6*time.Second).Should(BeClosed())
					Expect(getLockValue()).To(Equal(lockValue))
				})
			})
		})
	})

	Context("When consul is down", func() {
		BeforeEach(func() {
			consulRunner.Stop()
		})

		AfterEach(func() {
			consulRunner.Start()
			consulRunner.WaitUntilReady()
		})

		It("continues to retry acquiring the lock", func() {
			lockProcess = ifrit.Background(lockRunner)

			Consistently(lockProcess.Ready()).ShouldNot(BeClosed())
			Consistently(lockProcess.Wait()).ShouldNot(Receive())

			Eventually(logger).Should(Say("acquire-lock-failed"))
			clock.WaitForWatcherAndIncrement(retryInterval)
			Eventually(logger).Should(Say("retrying-acquiring-lock"))
			clock.WaitForWatcherAndIncrement(retryInterval)
			Eventually(logger).Should(Say("retrying-acquiring-lock"))
		})

		Context("when consul starts up", func() {
			It("acquires the lock", func() {
				lockProcess = ifrit.Background(lockRunner)

				Eventually(logger).Should(Say("acquire-lock-failed"))
				clock.Increment(retryInterval)
				Eventually(logger).Should(Say("retrying-acquiring-lock"))
				Consistently(lockProcess.Ready()).ShouldNot(BeClosed())
				Consistently(lockProcess.Wait()).ShouldNot(Receive())

				consulRunner.Start()
				consulRunner.WaitUntilReady()

				clock.Increment(retryInterval)
				Eventually(lockProcess.Ready()).Should(BeClosed())
				Expect(getLockValue()).To(Equal(lockValue))
			})
		})
	})
})
