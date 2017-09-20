package vizzini_test

import (
	"fmt"
	"net/http"
	"net/http/cookiejar"
	"time"

	"code.cloudfoundry.org/bbs/models"
	"code.cloudfoundry.org/routing-info/cfroutes"

	. "code.cloudfoundry.org/vizzini/matchers"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Routing Related Tests", func() {
	var lrp *models.DesiredLRP

	Describe("sticky sessions", func() {
		var httpClient *http.Client

		BeforeEach(func() {
			jar, err := cookiejar.New(nil)
			Expect(err).NotTo(HaveOccurred())

			httpClient = &http.Client{
				Jar: jar,
			}

			lrp = DesiredLRPWithGuid(guid)
			lrp.Instances = 3

			Expect(bbsClient.DesireLRP(logger, lrp)).To(Succeed())
			Eventually(IndexCounter(guid, httpClient)).Should(Equal(3))
		})

		It("should only route to the stuck instance", func() {
			resp, err := httpClient.Get("http://" + RouteForGuid(guid) + "/stick")
			Expect(err).NotTo(HaveOccurred())
			resp.Body.Close()

			//for some reason this isn't always 1!  it's sometimes 2....
			Expect(IndexCounter(guid, httpClient)()).To(BeNumerically("<", 3))

			resp, err = httpClient.Get("http://" + RouteForGuid(guid) + "/unstick")
			Expect(err).NotTo(HaveOccurred())
			resp.Body.Close()

			Expect(IndexCounter(guid, httpClient)()).To(Equal(3))
		})
	})

	Describe("supporting multiple ports", func() {
		var primaryURL string
		BeforeEach(func() {
			lrp = DesiredLRPWithGuid(guid)
			lrp.Ports = []uint32{8080, 9999}
			primaryURL = "http://" + RouteForGuid(guid) + "/env"

			Expect(bbsClient.DesireLRP(logger, lrp)).To(Succeed())
			Eventually(EndpointCurler(primaryURL)).Should(Equal(http.StatusOK))
		})

		It("should be able to route to multiple ports", func() {
			By("updating the LRP with a new route to a port 9999")
			newRoute := RouteForGuid(NewGuid())
			routes, err := cfroutes.CFRoutesFromRoutingInfo(*lrp.Routes)
			Expect(err).NotTo(HaveOccurred())
			routes = append(routes, cfroutes.CFRoute{
				Hostnames: []string{newRoute},
				Port:      9999,
			})
			routingInfo := routes.RoutingInfo()
			Expect(bbsClient.UpdateDesiredLRP(logger, guid, &models.DesiredLRPUpdate{
				Routes: &routingInfo,
			})).To(

				Succeed())

			By("verifying that the new route is hooked up to the port")
			Eventually(EndpointContentCurler("http://" + newRoute)).Should(Equal("grace side-channel"))

			By("verifying that the original route is fine")
			Expect(EndpointContentCurler(primaryURL)()).To(ContainSubstring("DAQUIRI"), "something on the original endpoint that's not in the new one")

			By("adding a new route to the new port")
			veryNewRoute := RouteForGuid(NewGuid())
			routes[1].Hostnames = append(routes[1].Hostnames, veryNewRoute)
			routingInfo = routes.RoutingInfo()
			Expect(bbsClient.UpdateDesiredLRP(logger, guid, &models.DesiredLRPUpdate{
				Routes: &routingInfo,
			})).To(

				Succeed())

			Eventually(EndpointContentCurler("http://" + veryNewRoute)).Should(Equal("grace side-channel"))
			Expect(EndpointContentCurler("http://" + newRoute)()).To(Equal("grace side-channel"))
			Expect(EndpointContentCurler(primaryURL)()).To(ContainSubstring("DAQUIRI"), "something on the original endpoint that's not in the new one")

			By("tearing down the new port")
			Expect(bbsClient.UpdateDesiredLRP(logger, guid, &models.DesiredLRPUpdate{
				Routes: lrp.Routes,
			})).To(

				Succeed())

			Eventually(EndpointCurler("http://" + newRoute)).ShouldNot(Equal(http.StatusOK))
		})
	})

	Context("as containers come and go", func() {
		var url string
		var lrp *models.DesiredLRP

		BeforeEach(func() {
			url = "http://" + RouteForGuid(guid) + "/env"
			lrp = DesiredLRPWithGuid(guid)
			lrp.Instances = 3
			Expect(bbsClient.DesireLRP(logger, lrp)).To(Succeed())
			Eventually(ActualByProcessGuidGetter(logger, guid)).Should(ConsistOf(
				BeActualLRPWithState(guid, 0, models.ActualLRPStateRunning),
				BeActualLRPWithState(guid, 1, models.ActualLRPStateRunning),
				BeActualLRPWithState(guid, 2, models.ActualLRPStateRunning),
			))
		})

		It("{SLOW} should only route to running containers", func() {
			done := make(chan struct{})
			badCodes := []int{}
			attempts := 0

			go func() {
				t := time.NewTicker(10 * time.Millisecond)
				for {
					select {
					case <-done:
						t.Stop()
					case <-t.C:
						attempts += 1
						code, _ := EndpointCurler(url)()
						if code != http.StatusOK {
							badCodes = append(badCodes, code)
						}
					}
				}
			}()

			var three = int32(3)
			var one = int32(1)

			updateToThree := models.DesiredLRPUpdate{
				Instances: &three,
			}

			updateToOne := models.DesiredLRPUpdate{
				Instances: &one,
			}

			for i := 0; i < 4; i++ {
				By(fmt.Sprintf("Scaling down then back up #%d", i+1))
				Expect(bbsClient.UpdateDesiredLRP(logger, guid, &updateToOne)).To(Succeed())
				Eventually(ActualByProcessGuidGetter(logger, guid)).Should(ConsistOf(
					BeActualLRPWithState(guid, 0, models.ActualLRPStateRunning),
				))

				time.Sleep(200 * time.Millisecond)

				Expect(bbsClient.UpdateDesiredLRP(logger, guid, &updateToThree)).To(Succeed())
				Eventually(ActualByProcessGuidGetter(logger, guid)).Should(ConsistOf(
					BeActualLRPWithState(guid, 0, models.ActualLRPStateRunning),
					BeActualLRPWithState(guid, 1, models.ActualLRPStateRunning),
					BeActualLRPWithState(guid, 2, models.ActualLRPStateRunning),
				))
			}

			close(done)

			fmt.Fprintf(GinkgoWriter, "%d bad codes out of %d attempts (%.3f%%)", len(badCodes), attempts, float64(len(badCodes))/float64(attempts)*100)
			Expect(len(badCodes)).To(BeNumerically("<", float64(attempts)*0.01))
		})
	})

	Describe("scaling down an LRP", func() {

		BeforeEach(func() {
			lrp = DesiredLRPWithGuid(guid)
			lrp.Action = models.WrapAction(&models.RunAction{
				Path: "/tmp/grace/grace",
				Args: []string{"-catchTerminate"},
				User: "vcap",
				Env:  []*models.EnvironmentVariable{{Name: "PORT", Value: "8080"}},
			})
			lrp.Instances = 5

			Expect(bbsClient.DesireLRP(logger, lrp)).To(Succeed())
			Eventually(IndexCounter(guid)).Should(Equal(int(lrp.Instances)))
		})

		It("quickly stops routing to the removed indices", func() {
			instanceCount := int32(1)
			Expect(bbsClient.UpdateDesiredLRP(logger, lrp.ProcessGuid,
				&models.DesiredLRPUpdate{
					Instances:  &instanceCount,
					Routes:     lrp.Routes,
					Annotation: &lrp.Annotation})).To(Succeed())

			Eventually(IndexCounterWithAttempts(guid, 10), 2*time.Second).Should(Equal(1))
			Consistently(IndexCounterWithAttempts(guid, 10), 5*time.Second).Should(Equal(1))
		})
	})
})
