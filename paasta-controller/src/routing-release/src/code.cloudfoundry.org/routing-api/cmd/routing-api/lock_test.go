package main_test

import (
	"fmt"
	"time"

	"code.cloudfoundry.org/routing-api"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/gexec"
)

var _ = Describe("Locking", func() {
	Describe("vieing for the lock", func() {
		Context("when two long-lived processes try to run", func() {
			It("one waits for the other to exit and then grabs the lock", func() {
				args := routingAPIArgs
				args.DevMode = true
				session1 := RoutingApi(args.ArgSlice()...)
				Eventually(session1, 10*time.Second).Should(gbytes.Say("acquire-lock-succeeded"))

				args.Port = uint16(5500 + GinkgoParallelNode())

				session2 := RoutingApi(args.ArgSlice()...)

				defer func() {
					session1.Interrupt().Wait(5 * time.Second)
					session2.Interrupt().Wait(10 * time.Second)
				}()

				Eventually(session2, 10*time.Second).Should(gbytes.Say("acquiring-lock"))
				Consistently(session2).ShouldNot(gbytes.Say("acquire-lock-succeeded"))

				session1.Interrupt().Wait(10 * time.Second)

				Eventually(session1, 10*time.Second).Should(gbytes.Say("releasing-lock"))
				Eventually(session2, 10*time.Second).Should(gbytes.Say("acquire-lock-succeeded"))
			})
		})
	})

	Context("when the lock disappears", func() {
		Context("long-lived processes", func() {
			It("should exit 1", func() {
				args := routingAPIArgs
				args.DevMode = true
				session1 := RoutingApi(args.ArgSlice()...)
				defer func() {
					session1.Interrupt().Wait(5 * time.Second)
				}()

				Eventually(session1, 10*time.Second).Should(gbytes.Say("acquire-lock-succeeded"))

				err := consulRunner.Reset()
				Expect(err).ToNot(HaveOccurred())

				consulRunner.WaitUntilReady()
				Eventually(session1, 10*time.Second).Should(gbytes.Say("lost-lock"))
				Eventually(session1, 20*time.Second).Should(gexec.Exit(1))
			})
		})
	})
	Context("when a rolling deploy occurs", func() {
		It("ensures there is no downtime", func() {
			args := routingAPIArgs
			args.DevMode = true

			session1 := RoutingApi(args.ArgSlice()...)
			Eventually(session1, 10*time.Second).Should(gbytes.Say("routing-api.started"))

			args.Port = uint16(5500 + GinkgoParallelNode())
			session2 := RoutingApi(args.ArgSlice()...)
			defer func() { session2.Interrupt().Wait(10 * time.Second) }()
			Eventually(session2, 10*time.Second).Should(gbytes.Say("acquiring-lock"))

			done := make(chan struct{})
			goRoutineFinished := make(chan struct{})
			client2 := routing_api.NewClient(fmt.Sprintf("http://127.0.0.1:%d", args.Port), false)

			go func() {
				defer GinkgoRecover()

				client1 := routing_api.NewClient(fmt.Sprintf("http://127.0.0.1:%d", routingAPIPort), false)

				var err1, err2 error

				ticker := time.NewTicker(time.Second)
				for range ticker.C {
					select {
					case <-done:
						close(goRoutineFinished)
						ticker.Stop()
						return
					default:
						_, err1 = client1.Routes()
						_, err2 = client2.Routes()
						Expect([]error{err1, err2}).To(ContainElement(Not(HaveOccurred())), "At least one of the errors should not have occurred")
					}
				}
			}()

			session1.Interrupt().Wait(10 * time.Second)

			Eventually(session2, 10*time.Second).Should(gbytes.Say("acquire-lock-succeeded"))
			Eventually(session2, 10*time.Second).Should(gbytes.Say("routing-api.started"))

			close(done)
			Eventually(done).Should(BeClosed())
			Eventually(goRoutineFinished).Should(BeClosed())

			_, err := client2.Routes()
			Expect(err).ToNot(HaveOccurred())
		})
	})
})
