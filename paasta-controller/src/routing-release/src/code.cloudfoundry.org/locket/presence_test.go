package locket_test

import (
	"time"

	"code.cloudfoundry.org/consuladapter"
	"code.cloudfoundry.org/locket"
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

var _ = Describe("Presence", func() {
	var (
		presenceKey   string
		presenceValue []byte

		consulClient consuladapter.Client

		presenceRunner  ifrit.Runner
		presenceProcess ifrit.Process
		retryInterval   time.Duration
		logger          lager.Logger
		clock           *fakeclock.FakeClock
	)

	getPresenceValue := func() ([]byte, error) {
		kvPair, _, err := consulClient.KV().Get(presenceKey, nil)
		if err != nil {
			return nil, err
		}

		if kvPair == nil || kvPair.Session == "" {
			return nil, consuladapter.NewKeyNotFoundError(presenceKey)
		}

		return kvPair.Value, nil
	}

	BeforeEach(func() {
		consulClient = consulRunner.NewClient()

		presenceKey = "some-key"
		presenceValue = []byte("some-value")

		retryInterval = 500 * time.Millisecond
		logger = lagertest.NewTestLogger("locket")
	})

	JustBeforeEach(func() {
		clock = fakeclock.NewFakeClock(time.Now())
		presenceRunner = locket.NewPresence(logger, consulClient, presenceKey, presenceValue, clock, retryInterval, 5*time.Second)
	})

	AfterEach(func() {
		ginkgomon.Kill(presenceProcess)
	})

	Context("When consul is running", func() {
		Context("an error occurs while acquiring the presence", func() {
			BeforeEach(func() {
				presenceKey = ""
			})

			It("continues to retry", func() {
				presenceProcess = ifrit.Background(presenceRunner)

				Consistently(presenceProcess.Ready()).ShouldNot(BeClosed())
				Consistently(presenceProcess.Wait()).ShouldNot(Receive())

				Eventually(logger).Should(Say("failed-setting-presence"))
				clock.WaitForWatcherAndIncrement(6 * time.Second)
				Eventually(logger).Should(Say("recreating-session"))
			})
		})

		Context("and the presence is available", func() {
			It("acquires the presence", func() {
				presenceProcess = ifrit.Background(presenceRunner)
				Eventually(presenceProcess.Ready()).Should(BeClosed())
				Eventually(getPresenceValue).Should(Equal(presenceValue))
			})

			Context("and we have acquired the presence", func() {
				JustBeforeEach(func() {
					presenceProcess = ifrit.Background(presenceRunner)
					Eventually(presenceProcess.Ready()).Should(BeClosed())
				})

				Context("when consul shuts down", func() {
					JustBeforeEach(func() {
						consulRunner.Stop()
					})

					AfterEach(func() {
						consulRunner.Start()
						consulRunner.WaitUntilReady()
					})

					It("loses the presence and retries", func() {
						Eventually(presenceProcess.Wait()).ShouldNot(Receive())
						clock.WaitForWatcherAndIncrement(6 * time.Second)
						Eventually(logger).Should(Say("recreating-session"))
					})
				})

				Context("when consul goes down and comes back up", func() {
					JustBeforeEach(func() {
						consulRunner.Stop()
					})

					It("reacquires presence", func() {
						Eventually(presenceProcess.Wait()).ShouldNot(Receive())
						clock.WaitForWatcherAndIncrement(6 * time.Second)
						Eventually(logger).Should(Say("recreating-session"))

						consulRunner.Start()
						consulRunner.WaitUntilReady()

						clock.WaitForWatcherAndIncrement(6 * time.Second)
						Eventually(logger).Should(Say("succeeded-recreating-session"))
						Eventually(presenceProcess.Ready()).Should(BeClosed())
					})
				})

				Context("and the process is shutting down", func() {
					It("releases the presence and exits", func() {
						ginkgomon.Interrupt(presenceProcess)
						Eventually(presenceProcess.Wait()).Should(Receive(BeNil()))
						_, err := getPresenceValue()
						Expect(err).To(Equal(consuladapter.NewKeyNotFoundError(presenceKey)))
					})
				})
			})
		})

		Context("and the presence is unavailable", func() {
			var (
				otherSession *locket.Session
				otherValue   []byte
			)

			BeforeEach(func() {
				otherValue = []byte("doppel-value")
				var err error
				otherSession, err = locket.NewSession("other-session", 10*time.Second, consulClient)
				Expect(err).NotTo(HaveOccurred())

				_, err = otherSession.SetPresence(presenceKey, otherValue)
				Expect(err).NotTo(HaveOccurred())
				Expect(getPresenceValue()).To(Equal(otherValue))
			})

			AfterEach(func() {
				otherSession.Destroy()
			})

			It("waits for the presence to become available", func() {
				presenceProcess = ifrit.Background(presenceRunner)
				Consistently(presenceProcess.Ready()).ShouldNot(BeClosed())
				Expect(getPresenceValue()).To(Equal(otherValue))
			})

			Context("when consul shuts down", func() {
				JustBeforeEach(func() {
					presenceProcess = ifrit.Background(presenceRunner)
					Consistently(presenceProcess.Ready()).ShouldNot(BeClosed())

					consulRunner.Stop()
				})

				AfterEach(func() {
					consulRunner.Start()
					consulRunner.WaitUntilReady()
				})

				It("continues to wait for the presence", func() {
					Consistently(presenceProcess.Ready()).ShouldNot(BeClosed())
					Consistently(presenceProcess.Wait()).ShouldNot(Receive())

					Eventually(logger).Should(Say("failed-setting-presence"))
					clock.WaitForWatcherAndIncrement(6 * time.Second)
					Eventually(logger).Should(Say("recreating-session"))
				})
			})

			Context("and the session is destroyed", func() {
				It("should recreate the session and continue to retry", func() {
					var err error
					presenceProcess = ifrit.Background(presenceRunner)
					Consistently(presenceProcess.Ready()).ShouldNot(BeClosed())

					var sessions []*api.SessionEntry
					Eventually(func() int {
						sessions, _, err = consulClient.Session().List(nil)
						Expect(err).NotTo(HaveOccurred())
						return len(sessions)
					}).Should(Equal(2))

					sessions, _, err = consulClient.Session().List(nil)
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

					Eventually(logger, 6*time.Second).Should(Say("consul-error"))

					clock.WaitForWatcherAndIncrement(6 * time.Second)
					Eventually(logger).Should(Say("recreating-session"))

					Eventually(func() int {
						sessions, _, err = consulClient.Session().List(nil)
						Expect(err).NotTo(HaveOccurred())
						return len(sessions)
					}).Should(Equal(2))
				})
			})

			Context("and the process is shutting down", func() {
				It("exits", func() {
					presenceProcess = ifrit.Background(presenceRunner)
					Consistently(presenceProcess.Ready()).ShouldNot(BeClosed())

					ginkgomon.Interrupt(presenceProcess)
					Eventually(presenceProcess.Wait()).Should(Receive(BeNil()))
				})
			})

			Context("and the presence is released", func() {
				It("acquires the presence", func() {
					presenceProcess = ifrit.Background(presenceRunner)
					Consistently(presenceProcess.Ready()).ShouldNot(BeClosed())
					Expect(getPresenceValue()).To(Equal(otherValue))

					otherSession.Destroy()

					Eventually(presenceProcess.Ready(), 7*time.Second).Should(BeClosed())
					Expect(getPresenceValue()).To(Equal(presenceValue))
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

		It("continues to retry creating the session", func() {
			presenceProcess = ifrit.Background(presenceRunner)

			Consistently(presenceProcess.Ready()).ShouldNot(BeClosed())
			Consistently(presenceProcess.Wait()).ShouldNot(Receive())

			Eventually(logger).Should(Say("failed-setting-presence"))
			clock.WaitForWatcherAndIncrement(6 * time.Second)
			Eventually(logger).Should(Say("recreating-session"))
			clock.WaitForWatcherAndIncrement(6 * time.Second)
			Eventually(logger).Should(Say("recreating-session"))
		})

		Context("when consul starts up", func() {
			It("acquires the presence", func() {
				presenceProcess = ifrit.Background(presenceRunner)
				Consistently(presenceProcess.Ready()).ShouldNot(BeClosed())

				Eventually(logger).Should(Say("failed-setting-presence"))
				clock.WaitForWatcherAndIncrement(6 * time.Second)
				Eventually(logger).Should(Say("recreating-session"))
				Consistently(presenceProcess.Wait()).ShouldNot(Receive())

				consulRunner.Start()
				consulRunner.WaitUntilReady()

				clock.WaitForWatcherAndIncrement(6 * time.Second)

				Eventually(presenceProcess.Ready()).Should(BeClosed())
				Eventually(getPresenceValue).Should(Equal(presenceValue))
			})
		})
	})
})
