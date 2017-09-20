package vizzini_test

import (
	"fmt"
	"net/http"
	"time"

	. "code.cloudfoundry.org/vizzini/matchers"

	"code.cloudfoundry.org/bbs/models"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func MakeGraceExit(baseURL string, status int) {
	//make sure Grace is up first
	Eventually(EndpointCurler(baseURL + "/env")).Should(Equal(http.StatusOK))

	//make Grace exit
	for i := 0; i < 3; i++ {
		url := fmt.Sprintf("%s/exit/%d", baseURL, status)
		resp, err := http.Post(url, "application/octet-stream", nil)
		Expect(err).NotTo(HaveOccurred())
		resp.Body.Close()
		if resp.StatusCode == http.StatusOK {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	Fail("failed to make grace exit")
}

func TellGraceToDeleteFile(baseURL string, filename string) {
	url := fmt.Sprintf("%s/file/%s", baseURL, filename)
	req, err := http.NewRequest("DELETE", url, nil)
	Expect(err).NotTo(HaveOccurred())
	resp, err := http.DefaultClient.Do(req)
	Expect(err).NotTo(HaveOccurred())
	resp.Body.Close()
	Expect(resp.StatusCode).To(Equal(http.StatusOK))
}

var _ = Describe("Crashes", func() {
	var lrp *models.DesiredLRP
	var url string

	BeforeEach(func() {
		url = fmt.Sprintf("http://%s", RouteForGuid(guid))
		lrp = DesiredLRPWithGuid(guid)
		lrp.Monitor = nil
	})

	Describe("Annotating the Crash Reason", func() {
		BeforeEach(func() {
			Expect(bbsClient.DesireLRP(logger, lrp)).To(Succeed())
			Eventually(EndpointCurler(url + "/env")).Should(Equal(http.StatusOK))
		})

		It("adds the crash reason to the application", func() {
			MakeGraceExit(url, 17)
			Eventually(ActualGetter(logger, guid, 0)).Should(BeActualLRPWithStateAndCrashCount(guid, 0, models.ActualLRPStateRunning, 1))
			actualLRP, err := ActualLRPByProcessGuidAndIndex(logger, guid, 0)
			Expect(err).NotTo(HaveOccurred())
			Expect(actualLRP.CrashReason).To(ContainSubstring("Exited with status 17"))
		})
	})

	Describe("backoff behavior", func() {
		BeforeEach(func() {
			Expect(bbsClient.DesireLRP(logger, lrp)).To(Succeed())
			Eventually(EndpointCurler(url + "/env")).Should(Equal(http.StatusOK))
		})

		It("{SLOW} restarts the application immediately twice, and then starts backing it off, and updates the modification tag as it goes", func() {
			actualLRP, err := ActualLRPByProcessGuidAndIndex(logger, guid, 0)
			Expect(err).NotTo(HaveOccurred())
			tag := actualLRP.ModificationTag

			By("immediately restarting #1")
			MakeGraceExit(url, 1)
			Eventually(ActualGetter(logger, guid, 0)).Should(BeActualLRPWithStateAndCrashCount(guid, 0, models.ActualLRPStateRunning, 1))

			restartedActualLRP, err := ActualLRPByProcessGuidAndIndex(logger, guid, 0)
			Expect(err).NotTo(HaveOccurred())
			Expect(restartedActualLRP.InstanceGuid).NotTo(Equal(actualLRP.InstanceGuid))
			Expect(restartedActualLRP.ModificationTag.Epoch).To(Equal(tag.Epoch))
			Expect(restartedActualLRP.ModificationTag.Index).To(BeNumerically(">", tag.Index))

			By("immediately restarting #2")
			MakeGraceExit(url, 1)
			Eventually(ActualGetter(logger, guid, 0)).Should(BeActualLRPWithStateAndCrashCount(guid, 0, models.ActualLRPStateRunning, 2))

			By("eventually restarting #3 (slow)")
			MakeGraceExit(url, 1)
			Eventually(ActualGetter(logger, guid, 0), ConvergerInterval).Should(BeActualLRPWithStateAndCrashCount(guid, 0, models.ActualLRPStateCrashed, 3))
			Consistently(ActualGetter(logger, guid, 0), CrashRestartTimeout-5*time.Second).Should(BeActualLRPWithStateAndCrashCount(guid, 0, models.ActualLRPStateCrashed, 3))
			Eventually(ActualGetter(logger, guid, 0), ConvergerInterval*2).Should(BeActualLRPWithStateAndCrashCount(guid, 0, models.ActualLRPStateRunning, 3))
			Eventually(EndpointCurler(url+"/env")).Should(Equal(http.StatusOK), "This can be removed when #89463754 lands")
		})

		It("deletes the crashed ActualLRP when scaling down", func() {
			By("immediately restarting #1")
			MakeGraceExit(url, 1)
			Eventually(ActualGetter(logger, guid, 0)).Should(BeActualLRPWithStateAndCrashCount(guid, 0, models.ActualLRPStateRunning, 1))

			By("immediately restarting #2")
			MakeGraceExit(url, 1)
			Eventually(ActualGetter(logger, guid, 0)).Should(BeActualLRPWithStateAndCrashCount(guid, 0, models.ActualLRPStateRunning, 2))

			By("eventually restarting #3")
			MakeGraceExit(url, 1)
			Eventually(ActualGetter(logger, guid, 0), ConvergerInterval).Should(BeActualLRPWithStateAndCrashCount(guid, 0, models.ActualLRPStateCrashed, 3))

			By("deleting the DesiredLRP")
			Expect(bbsClient.RemoveDesiredLRP(logger, guid)).To(Succeed())
			Eventually(ActualByProcessGuidGetter(logger, guid)).Should(BeEmpty())
		})
	})

	Describe("killing crashed applications", func() {
		BeforeEach(func() {
			Expect(bbsClient.DesireLRP(logger, lrp)).To(Succeed())
			Eventually(EndpointCurler(url + "/env")).Should(Equal(http.StatusOK))
		})

		It("should delete the Crashed ActualLRP succesfully", func() {
			By("immediately restarting #1")
			MakeGraceExit(url, 1)
			Eventually(ActualGetter(logger, guid, 0)).Should(BeActualLRPWithStateAndCrashCount(guid, 0, models.ActualLRPStateRunning, 1))

			By("immediately restarting #2")
			MakeGraceExit(url, 1)
			Eventually(ActualGetter(logger, guid, 0)).Should(BeActualLRPWithStateAndCrashCount(guid, 0, models.ActualLRPStateRunning, 2))

			By("eventually restarting #3")
			MakeGraceExit(url, 1)
			Eventually(ActualGetter(logger, guid, 0), ConvergerInterval).Should(BeActualLRPWithStateAndCrashCount(guid, 0, models.ActualLRPStateCrashed, 3))

			actualLRPKey := models.NewActualLRPKey(guid, 0, domain)
			Expect(bbsClient.RetireActualLRP(logger, &actualLRPKey)).To(Succeed())
			Eventually(ActualByProcessGuidGetter(logger, guid)).Should(BeEmpty())
		})
	})

	Context("with no monitor action", func() {
		Context("when running a single action", func() {
			BeforeEach(func() {
				Expect(bbsClient.DesireLRP(logger, lrp)).To(Succeed())
				Eventually(EndpointCurler(url + "/env")).Should(Equal(http.StatusOK))
			})

			It("comes up as soon as the process starts", func() {
				Eventually(ActualGetter(logger, guid, 0)).Should(BeActualLRPWithState(guid, 0, models.ActualLRPStateRunning))
			})

			Context("when the process dies with exit code 0", func() {
				BeforeEach(func() {
					MakeGraceExit(url, 0)
				})

				It("gets restarted immediately", func() {
					Eventually(ActualGetter(logger, guid, 0)).Should(BeActualLRPWithStateAndCrashCount(guid, 0, models.ActualLRPStateRunning, 1))
					Eventually(EndpointCurler(url+"/env")).Should(Equal(http.StatusOK), "This can be removed when #89463754 lands")
				})
			})

			Context("when the process dies with exit code 1", func() {
				BeforeEach(func() {
					MakeGraceExit(url, 1)
				})

				It("gets restarted immediately", func() {
					Eventually(ActualGetter(logger, guid, 0)).Should(BeActualLRPWithStateAndCrashCount(guid, 0, models.ActualLRPStateRunning, 1))
					Eventually(EndpointCurler(url+"/env")).Should(Equal(http.StatusOK), "This can be removed when #89463754 lands")
				})
			})
		})

		Context("when running several actions", func() {
			Context("codependently", func() {
				BeforeEach(func() {
					lrp.Action = models.WrapAction(models.Codependent(
						&models.RunAction{
							Path: "bash",
							Args: []string{"-c", "while true; do sleep 1; done"},
							User: "vcap",
						},
						&models.RunAction{
							Path: "/tmp/grace/grace",
							Env:  []*models.EnvironmentVariable{{Name: "PORT", Value: "8080"}},
							User: "vcap",
						},
					))
				})

				JustBeforeEach(func() {
					Expect(bbsClient.DesireLRP(logger, lrp)).To(Succeed())
				})

				Context("when one of the actions finishes", func() {
					JustBeforeEach(func() {
						Eventually(EndpointCurler(url + "/env")).Should(Equal(http.StatusOK))
						MakeGraceExit(url, 0)
					})

					It("gets restarted immediately", func() {
						Eventually(ActualGetter(logger, guid, 0)).Should(BeActualLRPWithStateAndCrashCount(guid, 0, models.ActualLRPStateRunning, 1))
						Eventually(EndpointCurler(url+"/env")).Should(Equal(http.StatusOK), "This can be removed when #89463754 lands")
					})
				})

				Context("when lot of subprocesses fail", func() {
					BeforeEach(func() {
						actions := []models.ActionInterface{}
						for i := 0; i < 200; i++ {
							actions = append(actions, &models.RunAction{
								Path: "bash",
								Args: []string{"-c", "exit 1"},
								User: "vcap",
							})
						}
						lrp.Action = models.WrapAction(models.Codependent(actions...))
					})

					It("the crash count is incremented", func() {
						Eventually(ActualGetter(logger, guid, 0), 20*time.Second).Should(BeActualLRPWithCrashCount(guid, 0, 1))
					})
				})
			})

			Context("in parallel", func() {
				BeforeEach(func() {
					lrp.Action = models.WrapAction(models.Parallel(
						&models.RunAction{
							Path: "bash",
							Args: []string{"-c", "while true; do sleep 1; done"},
							User: "vcap",
						},
						&models.RunAction{
							Path: "/tmp/grace/grace",
							Env:  []*models.EnvironmentVariable{{Name: "PORT", Value: "8080"}},
							User: "vcap",
						},
					))
					Expect(bbsClient.DesireLRP(logger, lrp)).To(Succeed())
					Eventually(EndpointCurler(url + "/env")).Should(Equal(http.StatusOK))
				})

				Context("when one of the actions finishes", func() {
					BeforeEach(func() {
						MakeGraceExit(url, 2)
					})

					It("does not crash", func() {
						Consistently(ActualGetter(logger, guid, 0), 5).Should(BeActualLRPWithState(guid, 0, models.ActualLRPStateRunning))
					})
				})
			})
		})
	})

	Context("with a monitor action", func() {
		Context("when the monitor eventually succeeds", func() {
			var directURL string
			var indirectURL string
			BeforeEach(func() {
				lrp.Action = models.WrapAction(&models.RunAction{
					Path: "/tmp/grace/grace",
					Args: []string{"-upFile=up"},
					User: "vcap",
					Env:  []*models.EnvironmentVariable{{Name: "PORT", Value: "8080"}},
				})

				lrp.Monitor = models.WrapAction(&models.RunAction{
					Path: "cat",
					Args: []string{"/tmp/up"},
					User: "vcap",
				})

				Expect(bbsClient.DesireLRP(logger, lrp)).To(Succeed())
				Eventually(ActualGetter(logger, guid, 0)).Should(BeActualLRPWithState(guid, 0, models.ActualLRPStateRunning))
				Eventually(EndpointCurler(url + "/env")).Should(Equal(http.StatusOK))
				directURL = "http://" + DirectAddressFor(guid, 0, 8080)
				indirectURL = "http://" + RouteForGuid(guid)
			})

			It("enters the running state", func() {
				Expect(ActualGetter(logger, guid, 0)()).To(BeActualLRPWithState(guid, 0, models.ActualLRPStateRunning))
			})

			Context("when the process dies with exit code 0", func() {
				BeforeEach(func() {
					MakeGraceExit(indirectURL, 0)
				})

				It("does not get marked as crashed (may have daemonized)", func() {
					Consistently(ActualGetter(logger, guid, 0), 3).Should(BeActualLRPWithStateAndCrashCount(guid, 0, models.ActualLRPStateRunning, 0))
				})
			})

			Context("when the process dies with exit code 0 and the monitor subsequently fails", func() {
				BeforeEach(func() {
					//tell grace to delete the file then exit, it's highly unlikely that the health check will run
					//between these two lines so the test should actually be covering the edge case in question
					TellGraceToDeleteFile(url, "up")
					MakeGraceExit(indirectURL, 0)
				})

				It("{SLOW} is marked as crashed", func() {
					Consistently(ActualGetter(logger, guid, 0), 2).Should(BeActualLRPWithState(guid, 0, models.ActualLRPStateRunning), "Banking on the fact that the health check runs every thirty seconds and is unlikely to run immediately")
					Eventually(ActualGetter(logger, guid, 0), HealthyCheckInterval+5*time.Second).Should(BeActualLRPWithCrashCount(guid, 0, 1))
				})
			})

			Context("when the process dies with exit code 1", func() {
				BeforeEach(func() {
					MakeGraceExit(indirectURL, 1)
				})

				It("is marked as crashed (immediately)", func() {
					Eventually(ActualGetter(logger, guid, 0), HealthyCheckInterval/3).Should(BeActualLRPWithCrashCount(guid, 0, 1))
				})
			})

			//{LOCAL} because: this test attempts to communicate with the container *directly* to ensure the process has been torn down
			//this is not possible against a remote installation as it entails connecting directly into the VPC
			Context("{LOCAL} when the monitor subsequently fails", func() {
				BeforeEach(func() {
					TellGraceToDeleteFile(indirectURL, "up")
				})

				It("{SLOW} is marked as crashed (and reaped)", func() {
					httpClient := &http.Client{
						Timeout: time.Second,
					}

					By("first validate that we can connect to the container directly")
					_, err := httpClient.Get(directURL + "/env")
					Expect(err).NotTo(HaveOccurred())

					By("being marked as crashed")
					Eventually(ActualGetter(logger, guid, 0), HealthyCheckInterval+5*time.Second).Should(BeActualLRPWithCrashCount(guid, 0, 1))

					By("tearing down the process -- this reaches out to the container's direct address and ensures we can't reach it")
					_, err = httpClient.Get(directURL + "/env")
					Expect(err).To(HaveOccurred())
				})
			})
		})

		Context("when the monitor never succeeds", func() {
			JustBeforeEach(func() {
				lrp.Monitor = models.WrapAction(&models.RunAction{
					Path: "false",
					User: "vcap",
				})

				Expect(bbsClient.DesireLRP(logger, lrp)).To(Succeed())
				Eventually(ActualGetter(logger, guid, 0)).Should(BeActualLRPWithState(guid, 0, models.ActualLRPStateClaimed))
			})

			Context("when the process dies with exit code 0", func() {
				BeforeEach(func() {
					lrp.Action = models.WrapAction(&models.RunAction{
						Path: "/tmp/grace/grace",
						Args: []string{"-exitAfter=2s", "-exitAfterCode=0"},
						User: "vcap",
						Env:  []*models.EnvironmentVariable{{Name: "PORT", Value: "8080"}},
					})
				})

				It("does not get marked as crash, as it has presumably daemonized and we are waiting on the health check", func() {
					Consistently(ActualGetter(logger, guid, 0), 3).Should(BeActualLRPWithStateAndCrashCount(guid, 0, models.ActualLRPStateClaimed, 0))
				})
			})

			Context("when the process dies with exit code 1", func() {
				BeforeEach(func() {
					lrp.Action = models.WrapAction(&models.RunAction{
						Path: "/tmp/grace/grace",
						Args: []string{"-exitAfter=2s", "-exitAfterCode=1"},
						User: "vcap",
						Env:  []*models.EnvironmentVariable{{Name: "PORT", Value: "8080"}},
					})
				})

				It("gets marked as crashed (immediately)", func() {
					Eventually(ActualGetter(logger, guid, 0)).Should(BeActualLRPWithCrashCount(guid, 0, 1))
				})
			})

			Context("and there is a StartTimeout", func() {
				BeforeEach(func() {
					lrp.StartTimeoutMs = 5000
				})

				It("never enters the running state and is marked as crashed after the StartTimeout", func() {
					Consistently(ActualGetter(logger, guid, 0), 3).Should(BeActualLRPWithState(guid, 0, models.ActualLRPStateClaimed))
					Eventually(ActualGetter(logger, guid, 0)).Should(BeActualLRPWithCrashCount(guid, 0, 1))
				})
			})

			Context("and there is no start timeout", func() {
				BeforeEach(func() {
					lrp.StartTimeoutMs = 0
				})

				It("never enters the running state, and never crashes", func() {
					Consistently(ActualGetter(logger, guid, 0), 5).Should(BeActualLRPWithStateAndCrashCount(guid, 0, models.ActualLRPStateClaimed, 0))
				})
			})
		})
	})
})
