package main_test

import (
	"fmt"
	"io/ioutil"
	"os"
	"time"

	yaml "gopkg.in/yaml.v2"

	"code.cloudfoundry.org/clock"
	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/lager/lagertest"
	"code.cloudfoundry.org/localip"
	"code.cloudfoundry.org/locket"
	"code.cloudfoundry.org/routing-api"
	"code.cloudfoundry.org/routing-api/cmd/routing-api/testrunner"
	"code.cloudfoundry.org/routing-api/config"

	locketconfig "code.cloudfoundry.org/locket/cmd/locket/config"
	locketrunner "code.cloudfoundry.org/locket/cmd/locket/testrunner"
	"code.cloudfoundry.org/locket/lock"
	locketmodels "code.cloudfoundry.org/locket/models"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/gexec"
	"github.com/tedsuo/ifrit"
	"github.com/tedsuo/ifrit/ginkgomon"
)

var _ = Describe("SqlLock", func() {
	var (
		locketRunner     ifrit.Runner
		locketProcess    ifrit.Process
		locketAddress    string
		routingAPIConfig config.Config
		configFilePath   string
		session          *gexec.Session
		args             testrunner.Args

		logger lager.Logger
	)

	routingAPIShouldBeReachable := func() {
		Eventually(func() error {
			_, err := client.Routes()
			return err
		}).Should(Succeed())
	}

	BeforeEach(func() {
		logger = lagertest.NewTestLogger("sql-lock-test")

		locketPort, err := localip.LocalPort()
		Expect(err).NotTo(HaveOccurred())

		locketAddress = fmt.Sprintf("localhost:%d", locketPort)
		locketRunner = locketrunner.NewLocketRunner(locketBinPath, func(cfg *locketconfig.LocketConfig) {
			cfg.ConsulCluster = consulRunner.ConsulCluster()
			cfg.DatabaseConnectionString = mysqlAllocator.ConnectionString() + sqlDBName
			cfg.DatabaseDriver = "mysql"
			cfg.ListenAddress = locketAddress
		})
		locketProcess = ginkgomon.Invoke(locketRunner)

		configBytes, err := ioutil.ReadFile(routingAPIArgsOnlySQL.ConfigPath)
		Expect(err).ToNot(HaveOccurred())
		err = yaml.Unmarshal(configBytes, &routingAPIConfig)
		Expect(err).ToNot(HaveOccurred())

		routingAPIConfig.Locket = locketrunner.ClientLocketConfig()
		routingAPIConfig.Locket.LocketAddress = locketAddress
	})

	JustBeforeEach(func() {
		configBytes, err := yaml.Marshal(routingAPIConfig)
		Expect(err).ToNot(HaveOccurred())

		configFile, err := ioutil.TempFile("", "routing-api-config")
		Expect(err).NotTo(HaveOccurred())

		configFilePath = configFile.Name()

		_, err = configFile.Write(configBytes)
		Expect(err).NotTo(HaveOccurred())

		err = configFile.Close()
		Expect(err).NotTo(HaveOccurred())

		args = routingAPIArgsOnlySQL
		args.ConfigPath = configFilePath
		session = RoutingApi(args.ArgSlice()...)
	})

	AfterEach(func() {
		if session != nil {
			session.Kill().Wait(10 * time.Second)
		}

		ginkgomon.Interrupt(locketProcess)
		locketProcess.Wait()

		err := os.RemoveAll(configFilePath)
		Expect(err).ToNot(HaveOccurred())
	})

	Context("with invalid configuration", func() {
		Context("and the locket address is not configured", func() {
			BeforeEach(func() {
				routingAPIConfig.Locket.LocketAddress = ""
				routingAPIConfig.SkipConsulLock = true
			})

			It("exits with an error", func() {
				Eventually(session).Should(gexec.Exit(2))
			})
		})
	})

	Context("with valid configuration", func() {
		It("acquires the lock in locket and becomes active", func() {
			routingAPIShouldBeReachable()
		})

		Context("and the locking server becomes unreachable after grabbing the lock", func() {
			JustBeforeEach(func() {
				routingAPIShouldBeReachable()

				ginkgomon.Interrupt(locketProcess)
			})

			It("exits", func() {
				Eventually(session, 30).Should(gexec.Exit(1))
			})
		})

		Context("when consul lock isn't required", func() {
			var competingLockProcess ifrit.Process

			BeforeEach(func() {
				routingAPIConfig.SkipConsulLock = true
				competingLock := locket.NewLock(
					logger,
					consulRunner.NewClient(),
					locket.LockSchemaPath("routing_api_lock"),
					[]byte{}, clock.NewClock(),
					locket.RetryInterval,
					locket.DefaultSessionTTL,
				)
				competingLockProcess = ifrit.Invoke(competingLock)
			})

			AfterEach(func() {
				ginkgomon.Kill(competingLockProcess)
			})

			It("does not acquire the consul lock", func() {
				routingAPIShouldBeReachable()
			})
		})

		Context("when the lock is not available", func() {
			var competingProcess ifrit.Process

			BeforeEach(func() {
				locketClient, err := locket.NewClient(logger, routingAPIConfig.Locket)
				Expect(err).NotTo(HaveOccurred())

				lockIdentifier := &locketmodels.Resource{
					Key:   "routing_api_lock",
					Owner: "Your worst enemy.",
					Value: "Something",
				}

				clock := clock.NewClock()
				competingRunner := lock.NewLockRunner(logger, locketClient, lockIdentifier, 5, clock, locket.RetryInterval)
				competingProcess = ginkgomon.Invoke(competingRunner)
			})

			AfterEach(func() {
				ginkgomon.Interrupt(competingProcess)
			})

			It("does not become active", func() {
				Consistently(func() error {
					_, err := client.Routes()
					return err
				}).ShouldNot(Succeed())
			})

			Context("and the lock becomes available", func() {
				JustBeforeEach(func() {
					Consistently(func() error {
						_, err := client.Routes()
						return err
					}).ShouldNot(Succeed())

					ginkgomon.Interrupt(competingProcess)
				})

				It("grabs the lock and becomes active", func() {
					routingAPIShouldBeReachable()
				})
			})
		})
	})

	Context("when a rolling deploy occurs", func() {
		BeforeEach(func() {
			routingAPIConfig.SkipConsulLock = true
		})

		It("ensures there is no downtime", func() {
			Eventually(session, 10*time.Second).Should(gbytes.Say("routing-api.started"))

			args.Port = uint16(5500 + GinkgoParallelNode())
			session2 := RoutingApi(args.ArgSlice()...)
			defer func() { session2.Interrupt().Wait(10 * time.Second) }()
			Eventually(session2, 10*time.Second).Should(gbytes.Say("locket-lock.started"))

			done := make(chan struct{})
			goRoutineFinished := make(chan struct{})
			client2 := routing_api.NewClient(fmt.Sprintf("http://127.0.0.1:%d", args.Port), false)

			go func() {
				defer GinkgoRecover()

				var err1, err2 error

				ticker := time.NewTicker(time.Second)
				for range ticker.C {
					select {
					case <-done:
						close(goRoutineFinished)
						ticker.Stop()
						return
					default:
						_, err1 = client.Routes()
						_, err2 = client2.Routes()
						Expect([]error{err1, err2}).To(ContainElement(Not(HaveOccurred())), "At least one of the errors should not have occurred")
					}
				}
			}()

			session.Interrupt().Wait(10 * time.Second)

			Eventually(session2, 10*time.Second).Should(gbytes.Say("locket-lock.acquired-lock"))
			Eventually(session2, 10*time.Second).Should(gbytes.Say("routing-api.started"))

			close(done)
			Eventually(done).Should(BeClosed())
			Eventually(goRoutineFinished).Should(BeClosed())

			_, err := client2.Routes()
			Expect(err).ToNot(HaveOccurred())
		})
	})
})
