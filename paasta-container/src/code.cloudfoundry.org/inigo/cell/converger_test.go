package cell_test

import (
	"os"
	"path/filepath"

	"code.cloudfoundry.org/bbs/models"
	"code.cloudfoundry.org/inigo/fixtures"
	"code.cloudfoundry.org/inigo/helpers"
	"github.com/tedsuo/ifrit"
	"github.com/tedsuo/ifrit/ginkgomon"
	"github.com/tedsuo/ifrit/grouper"

	archive_helper "code.cloudfoundry.org/archiver/extractor/test_helper"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Convergence to desired state", func() {
	var (
		runtime    ifrit.Process
		auctioneer ifrit.Process
		rep        ifrit.Process

		appId       string
		processGuid string

		runningLRPsPoller        func() []models.ActualLRP
		helloWorldInstancePoller func() []string
	)

	BeforeEach(func() {
		fileServer, fileServerStaticDir := componentMaker.FileServer()

		runtime = ginkgomon.Invoke(grouper.NewParallel(os.Kill, grouper.Members{
			{"file-server", fileServer},
			{"route-emitter", componentMaker.RouteEmitter()},
			{"router", componentMaker.Router()},
		}))

		archive_helper.CreateZipArchive(
			filepath.Join(fileServerStaticDir, "lrp.zip"),
			fixtures.GoServerApp(),
		)

		appId = helpers.GenerateGuid()

		processGuid = helpers.GenerateGuid()

		runningLRPsPoller = func() []models.ActualLRP {
			return helpers.ActiveActualLRPs(logger, bbsClient, processGuid)
		}

		helloWorldInstancePoller = helpers.HelloWorldInstancePoller(componentMaker.Addresses.Router, helpers.DefaultHost)
	})

	AfterEach(func() {
		By("Stopping all the processes")
		helpers.StopProcesses(auctioneer, rep, runtime)
	})

	Describe("Executor fault tolerance", func() {
		BeforeEach(func() {
			auctioneer = ginkgomon.Invoke(componentMaker.Auctioneer())
		})

		Context("when an rep, and converger are running", func() {
			BeforeEach(func() {
				rep = ginkgomon.Invoke(componentMaker.Rep())
				By("restarting the bbs with smaller convergeRepeatInterval")
				ginkgomon.Interrupt(bbsProcess)
				bbsProcess = ginkgomon.Invoke(componentMaker.BBS(
					"-convergeRepeatInterval", "1s",
				))
			})

			Context("and an LRP is desired", func() {
				var initialInstanceGuids []string

				BeforeEach(func() {
					err := bbsClient.DesireLRP(logger, helpers.DefaultLRPCreateRequest(processGuid, appId, 2))
					Expect(err).NotTo(HaveOccurred())

					Eventually(runningLRPsPoller).Should(HaveLen(2))
					Eventually(helloWorldInstancePoller).Should(Equal([]string{"0", "1"}))
					initialActuals := runningLRPsPoller()
					initialInstanceGuids = []string{initialActuals[0].InstanceGuid, initialActuals[1].InstanceGuid}
				})

				Context("and the LRP goes away because its rep dies", func() {
					BeforeEach(func() {
						ginkgomon.Interrupt(rep)

						Eventually(runningLRPsPoller).Should(BeEmpty())
						Eventually(helloWorldInstancePoller).Should(BeEmpty())
					})

					Context("once the rep comes back", func() {
						BeforeEach(func() {
							rep = ginkgomon.Invoke(componentMaker.Rep())
						})

						It("eventually brings the long-running process up", func() {
							Eventually(runningLRPsPoller).Should(HaveLen(2))
							Eventually(helloWorldInstancePoller).Should(Equal([]string{"0", "1"}))

							currentActuals := runningLRPsPoller()
							instanceGuids := []string{currentActuals[0].InstanceGuid, currentActuals[1].InstanceGuid}
							Expect(instanceGuids).NotTo(ContainElement(initialInstanceGuids[0]))
							Expect(instanceGuids).NotTo(ContainElement(initialInstanceGuids[1]))
						})
					})
				})

				Context("and a new rep is introduced", func() {
					var firstActualLRPs []models.ActualLRP
					var rep2 ifrit.Process

					BeforeEach(func() {
						firstActualLRPs = runningLRPsPoller()
						rep2 = ginkgomon.Invoke(componentMaker.RepN(1))
					})

					AfterEach(func() {
						helpers.StopProcesses(rep2)
					})

					Context("and the first rep goes away", func() {
						BeforeEach(func() {
							ginkgomon.Interrupt(rep)
						})

						It("eventually brings up the LRP on the new rep", func() {
							Eventually(func() bool {
								secondActualLRPs := runningLRPsPoller()
								if len(secondActualLRPs) != 2 {
									return false
								}
								return secondActualLRPs[0].CellId != firstActualLRPs[0].CellId &&
									secondActualLRPs[1].CellId != firstActualLRPs[1].CellId
							}).Should(BeTrue())
						})
					})
				})
			})
		})

		Context("when a converger is running without a rep", func() {
			BeforeEach(func() {
				By("restarting the bbs with smaller convergeRepeatInterval")
				ginkgomon.Interrupt(bbsProcess)
				bbsProcess = ginkgomon.Invoke(componentMaker.BBS(
					"-convergeRepeatInterval", "1s",
				))
			})

			Context("and an LRP is desired", func() {
				BeforeEach(func() {
					err := bbsClient.DesireLRP(logger, helpers.DefaultLRPCreateRequest(processGuid, appId, 1))
					Expect(err).NotTo(HaveOccurred())

					Consistently(runningLRPsPoller).Should(BeEmpty())
					Consistently(helloWorldInstancePoller).Should(BeEmpty())
				})

				Context("and then a rep come up", func() {
					BeforeEach(func() {
						rep = ginkgomon.Invoke(componentMaker.Rep())
					})

					It("eventually brings the LRP up", func() {
						Eventually(runningLRPsPoller).Should(HaveLen(1))
						Eventually(helloWorldInstancePoller).Should(Equal([]string{"0"}))
					})
				})
			})
		})
	})

	Describe("Auctioneer Fault Tolerance", func() {
		BeforeEach(func() {
			By("restarting the bbs with smaller convergeRepeatInterval")
			ginkgomon.Interrupt(bbsProcess)
			bbsProcess = ginkgomon.Invoke(componentMaker.BBS(
				"-convergeRepeatInterval", "1s",
			))
		})

		Context("when a rep is running with no auctioneer", func() {
			BeforeEach(func() {
				rep = ginkgomon.Invoke(componentMaker.Rep())
			})

			Context("and an LRP is desired", func() {
				BeforeEach(func() {
					err := bbsClient.DesireLRP(logger, helpers.DefaultLRPCreateRequest(processGuid, appId, 1))
					Expect(err).NotTo(HaveOccurred())

					Consistently(runningLRPsPoller).Should(BeEmpty())
					Consistently(helloWorldInstancePoller).Should(BeEmpty())
				})

				Context("and then an auctioneer comes up", func() {
					BeforeEach(func() {
						auctioneer = ginkgomon.Invoke(componentMaker.Auctioneer())
					})

					It("eventually brings it up", func() {
						Eventually(runningLRPsPoller).Should(HaveLen(1))
						Eventually(helloWorldInstancePoller).Should(Equal([]string{"0"}))
					})
				})
			})
		})

		Context("when an auctioneer is running with no rep", func() {
			BeforeEach(func() {
				auctioneer = ginkgomon.Invoke(componentMaker.Auctioneer())
			})

			Context("and an LRP is desired", func() {
				BeforeEach(func() {
					err := bbsClient.DesireLRP(logger, helpers.DefaultLRPCreateRequest(processGuid, appId, 1))
					Expect(err).NotTo(HaveOccurred())

					Consistently(runningLRPsPoller).Should(BeEmpty())
					Consistently(helloWorldInstancePoller).Should(BeEmpty())
				})

				Context("and the rep come up", func() {
					BeforeEach(func() {
						rep = ginkgomon.Invoke(componentMaker.Rep())
					})

					It("eventually brings it up", func() {
						Eventually(runningLRPsPoller).Should(HaveLen(1))
						Eventually(helloWorldInstancePoller).Should(Equal([]string{"0"}))
					})
				})
			})
		})
	})
})
