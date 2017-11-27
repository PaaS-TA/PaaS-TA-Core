package main_test

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"path"
	"regexp"
	"runtime"
	"strings"
	"time"

	"code.cloudfoundry.org/bbs"
	bbstestrunner "code.cloudfoundry.org/bbs/cmd/bbs/testrunner"
	"code.cloudfoundry.org/bbs/models"
	"code.cloudfoundry.org/bbs/models/test/model_helpers"
	"code.cloudfoundry.org/cfhttp"
	"code.cloudfoundry.org/durationjson"
	"code.cloudfoundry.org/executor/gardenhealth"
	executorinit "code.cloudfoundry.org/executor/initializer"
	"code.cloudfoundry.org/garden"
	"code.cloudfoundry.org/garden/transport"
	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/lager/lagerflags"
	"code.cloudfoundry.org/lager/lagertest"
	"code.cloudfoundry.org/localip"
	"code.cloudfoundry.org/locket"
	locketconfig "code.cloudfoundry.org/locket/cmd/locket/config"
	locketrunner "code.cloudfoundry.org/locket/cmd/locket/testrunner"
	locketmodels "code.cloudfoundry.org/locket/models"
	"code.cloudfoundry.org/rep"
	"code.cloudfoundry.org/rep/cmd/rep/config"
	"code.cloudfoundry.org/rep/cmd/rep/testrunner"

	"github.com/hashicorp/consul/api"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	. "github.com/onsi/gomega/gexec"
	"github.com/onsi/gomega/ghttp"
	"github.com/tedsuo/ifrit"
	"github.com/tedsuo/ifrit/ginkgomon"
)

var runner *testrunner.Runner

var _ = Describe("The Rep", func() {
	var (
		repConfig                           config.RepConfig
		fakeGarden                          *ghttp.Server
		pollingInterval                     time.Duration
		evacuationTimeout                   time.Duration
		rootFSName                          string
		rootFSPath                          string
		logger                              *lagertest.TestLogger
		basePath                            string
		respondWithSuccessToCreateContainer bool

		flushEvents chan struct{}
	)

	var getActualLRPGroups = func(logger lager.Logger) func() []*models.ActualLRPGroup {
		return func() []*models.ActualLRPGroup {
			actualLRPGroups, err := bbsClient.ActualLRPGroups(logger, models.ActualLRPFilter{})
			Expect(err).NotTo(HaveOccurred())
			return actualLRPGroups
		}
	}

	BeforeEach(func() {
		logger = lagertest.NewTestLogger("test")

		respondWithSuccessToCreateContainer = true

		basePath = path.Join(os.Getenv("GOPATH"), "src/code.cloudfoundry.org/rep/cmd/rep/fixtures")

		Eventually(getActualLRPGroups(logger), 5*pollingInterval).Should(BeEmpty())
		flushEvents = make(chan struct{})
		fakeGarden = ghttp.NewUnstartedServer()
		// these tests only look for the start of a sequence of requests
		fakeGarden.AllowUnhandledRequests = false
		fakeGarden.RouteToHandler("GET", "/ping", ghttp.RespondWithJSONEncoded(http.StatusOK, struct{}{}))
		fakeGarden.RouteToHandler("GET", "/containers", ghttp.RespondWithJSONEncoded(http.StatusOK, struct{}{}))
		fakeGarden.RouteToHandler("GET", "/capacity", ghttp.RespondWithJSONEncoded(http.StatusOK,
			garden.Capacity{MemoryInBytes: 1024 * 1024 * 1024, DiskInBytes: 20 * 1024 * 1024 * 1024, MaxContainers: 4}))
		fakeGarden.RouteToHandler("GET", "/containers/bulk_info", ghttp.RespondWithJSONEncoded(http.StatusOK, struct{}{}))

		// The following handlers are needed to fake out the healthcheck containers
		fakeGarden.RouteToHandler("DELETE", regexp.MustCompile("/containers/executor-healthcheck-[-a-f0-9]+"), ghttp.RespondWithJSONEncoded(http.StatusOK, struct{}{}))
		fakeGarden.RouteToHandler("POST", "/containers/healthcheck-container/processes", func() http.HandlerFunc {
			firstResponse, err := json.Marshal(transport.ProcessPayload{})
			Expect(err).NotTo(HaveOccurred())

			exitStatus := 0
			secondResponse, err := json.Marshal(transport.ProcessPayload{ExitStatus: &exitStatus})
			Expect(err).NotTo(HaveOccurred())

			headers := http.Header{"Content-Type": []string{"application/json"}}
			response := string(firstResponse) + string(secondResponse)
			return ghttp.RespondWith(http.StatusOK, response, headers)
		}())

		pollingInterval = 50 * time.Millisecond
		evacuationTimeout = 200 * time.Millisecond

		rootFSName = "the-rootfs"
		rootFSPath = "/path/to/rootfs"

		repConfig = config.RepConfig{
			PreloadedRootFS:       map[string]string{rootFSName: rootFSPath},
			SupportedProviders:    []string{"docker"},
			PlacementTags:         []string{"test"},
			OptionalPlacementTags: []string{"optional_tag"},
			CellID:                cellID,
			BBSAddress:            bbsURL.String(),
			ListenAddr:            fmt.Sprintf("0.0.0.0:%d", serverPort),
			ListenAddrSecurable:   fmt.Sprintf("0.0.0.0:%d", serverPortSecurable),
			RequireTLS:            false,
			LockRetryInterval:     durationjson.Duration(1 * time.Second),
			ExecutorConfig: executorinit.ExecutorConfig{
				GardenAddr:                   fakeGarden.HTTPTestServer.Listener.Addr().String(),
				GardenNetwork:                "tcp",
				GardenHealthcheckProcessUser: "me",
				GardenHealthcheckProcessPath: "ls",
				ContainerMaxCpuShares:        1024,
			},
			LagerConfig: lagerflags.LagerConfig{
				LogLevel: "debug",
			},
			ConsulCluster:         consulRunner.ConsulCluster(),
			PollingInterval:       durationjson.Duration(pollingInterval),
			EvacuationTimeout:     durationjson.Duration(evacuationTimeout),
			EnableLegacyAPIServer: true,
		}

		runner = testrunner.New(representativePath, repConfig)
	})

	JustBeforeEach(func() {
		if respondWithSuccessToCreateContainer {
			fakeGarden.RouteToHandler("POST", "/containers", ghttp.RespondWithJSONEncoded(http.StatusOK, map[string]string{"handle": "healthcheck-container"}))
		}

		runner.Start()
	})

	AfterEach(func(done Done) {
		close(flushEvents)
		runner.KillWithFire()
		fakeGarden.Close()
		close(done)
	})

	Context("when Garden is available", func() {
		BeforeEach(func() {
			fakeGarden.Start()
		})

		Context("when a value is provided caCertsForDownloads", func() {
			var certFile *os.File

			BeforeEach(func() {
				var err error
				certFile, err = ioutil.TempFile("", "")
				Expect(err).NotTo(HaveOccurred())
			})

			AfterEach(func() {
				os.Remove(certFile.Name())
			})

			Context("when the file has a valid cert bundle", func() {
				BeforeEach(func() {
					fileContents := []byte(`-----BEGIN CERTIFICATE-----
MIIBdzCCASOgAwIBAgIBADALBgkqhkiG9w0BAQUwEjEQMA4GA1UEChMHQWNtZSBD
bzAeFw03MDAxMDEwMDAwMDBaFw00OTEyMzEyMzU5NTlaMBIxEDAOBgNVBAoTB0Fj
bWUgQ28wWjALBgkqhkiG9w0BAQEDSwAwSAJBAN55NcYKZeInyTuhcCwFMhDHCmwa
IUSdtXdcbItRB/yfXGBhiex00IaLXQnSU+QZPRZWYqeTEbFSgihqi1PUDy8CAwEA
AaNoMGYwDgYDVR0PAQH/BAQDAgCkMBMGA1UdJQQMMAoGCCsGAQUFBwMBMA8GA1Ud
EwEB/wQFMAMBAf8wLgYDVR0RBCcwJYILZXhhbXBsZS5jb22HBH8AAAGHEAAAAAAA
AAAAAAAAAAAAAAEwCwYJKoZIhvcNAQEFA0EAAoQn/ytgqpiLcZu9XKbCJsJcvkgk
Se6AbGXgSlq+ZCEVo0qIwSgeBqmsJxUu7NCSOwVJLYNEBO2DtIxoYVk+MA==
-----END CERTIFICATE-----
-----BEGIN CERTIFICATE-----
MIIFATCCAuugAwIBAgIBATALBgkqhkiG9w0BAQswEjEQMA4GA1UEAxMHZGllZ29D
QTAeFw0xNjAyMTYyMTU1MzNaFw0yNjAyMTYyMTU1NDZaMBIxEDAOBgNVBAMTB2Rp
ZWdvQ0EwggIiMA0GCSqGSIb3DQEBAQUAA4ICDwAwggIKAoICAQC7N7lGx7QGqkMd
wjqgkr09CPoV3HW+GL+YOPajf//CCo15t3mLu9Npv7O7ecb+g/6DxEOtHFpQBSbQ
igzHZkdlBJEGknwH2bsZ4wcVT2vcv2XPAIMDrnT7VuF1S2XD7BJK3n6BeXkFsVPA
OUjC/v0pM/rCFRId5CwtRD/0IHFC/qgEtFQx+zejXXEn1AJMzvNNJ3B0bd8VQGEX
ppemZXS1QvTP7/j2h7fJjosyoL6+76k4mcoScmWFNJHKcG4qcAh8rdnDlw+hJ+5S
z73CadYI2BTnlZ/fxEcsZ/kcteFSf0mFpMYX6vs9/us/rgGwjUNzg+JlzvF43TYY
VQ+TRkFUYHhDv3xwuRHnPNe0Nm30esKpqvbSXtoS6jcnpHn9tMOU0+4NW4aEdy9s
7l4lcGyih4qZfHbYTsRDk1Nrq5EzQbhlZSPC3nxMrLxXri7j22rVCY/Rj9IgAxwC
R3KcCdADGJeNOw44bK/BsRrB+Hxs9yNpXc2V2dez+w3hKNuzyk7WydC3fgXxX6x8
66xnlhFGor7fvM0OSMtGUBD16igh4ySdDiEMNUljqQ1DuMglT1eGdg+Kh+1YYWpz
v3JkNTX96C80IivbZyunZ2CczFhW2HlGWZLwNKeuM0hxt6AmiEa+KJQkx73dfg3L
tkDWWp9TXERPI/6Y2696INi0wElBUQIDAQABo2YwZDAOBgNVHQ8BAf8EBAMCAAYw
EgYDVR0TAQH/BAgwBgEB/wIBADAdBgNVHQ4EFgQU5xGtUKEzsfGmk/Siqo4fgAMs
TBwwHwYDVR0jBBgwFoAU5xGtUKEzsfGmk/Siqo4fgAMsTBwwCwYJKoZIhvcNAQEL
A4ICAQBkWgWl2t5fd4PZ1abpSQNAtsb2lfkkpxcKw+Osn9MeGpcrZjP8XoVTxtUs
GMpeVn2dUYY1sxkVgUZ0Epsgl7eZDK1jn6QfWIjltlHvDtJMh0OrxmdJUuHTGIHc
lsI9NGQRUtbyFHmy6jwIF7q925OmPQ/A6Xgkb45VUJDGNwOMUL5I9LbdBXcjmx6F
ZifEON3wxDBVMIAoS/mZYjP4zy2k1qE2FHoitwDccnCG5Wya+AHdZv/ZlfJcuMtU
U82oyHOctH29BPwASs3E1HUKof6uxJI+Y1M2kBDeuDS7DWiTt3JIVCjewIIhyYYw
uTPbQglqhqHr1RWohliDmKSroIil68s42An0fv9sUr0Btf4itKS1gTb4rNiKTZC/
8sLKs+CA5MB+F8lCllGGFfv1RFiUZBQs9+YEE+ru+yJw39lHeZQsEUgHbLjbVHs1
WFqiKTO8VKl1/eGwG0l9dI26qisIAa/I7kLjlqboKycGDmAAarsmcJBLPzS+ytiu
hoxA/fLhSWJvPXbdGemXLWQGf5DLN/8QGB63Rjp9WC3HhwSoU0NvmNmHoh+AdRRT
dYbCU/DMZjsv+Pt9flhj7ELLo+WKHyI767hJSq9A7IT3GzFt8iGiEAt1qj2yS0DX
36hwbfc1Gh/8nKgFeLmPOlBfKncjTjL2FvBNap6a8tVHXO9FvQ==
-----END CERTIFICATE-----`)

					err := ioutil.WriteFile(certFile.Name(), fileContents, os.ModePerm)
					Expect(err).NotTo(HaveOccurred())
					repConfig.PathToCACertsForDownloads = certFile.Name()
					runner = testrunner.New(
						representativePath,
						repConfig,
					)

					runner.StartCheck = "started"
				})

				It("should start", func() {
					Consistently(runner.Session).ShouldNot(Exit())
				})
			})
		})

		Describe("when an interrupt signal is sent to the representative", func() {
			JustBeforeEach(func() {
				if runtime.GOOS == "windows" {
					Skip("Interrupt isn't supported on windows")
				}

				runner.Stop()
			})

			It("should die", func() {
				Eventually(runner.Session.ExitCode).Should(Equal(0))
			})
		})

		Context("when the bbs is down", func() {
			BeforeEach(func() {
				ginkgomon.Kill(bbsProcess)
			})

			AfterEach(func() {
				bbsRunner = bbstestrunner.New(bbsBinPath, bbsConfig)
			})

			It("starts", func() {
				Consistently(runner.Session).ShouldNot(Exit())
			})
		})

		Context("when starting", func() {
			var deleteChan chan struct{}
			BeforeEach(func() {
				fakeGarden.RouteToHandler("GET", "/containers",
					func(w http.ResponseWriter, r *http.Request) {
						r.ParseForm()
						healthcheckTagQueryParam := gardenhealth.HealthcheckTag
						if r.FormValue(healthcheckTagQueryParam) == gardenhealth.HealthcheckTagValue {
							ghttp.RespondWithJSONEncoded(http.StatusOK, struct{}{})(w, r)
						} else {
							ghttp.RespondWithJSONEncoded(http.StatusOK, map[string][]string{"handles": []string{"cnr1", "cnr2"}})(w, r)
						}
					},
				)
				deleteChan = make(chan struct{}, 2)
				fakeGarden.RouteToHandler("DELETE", "/containers/cnr1",
					ghttp.CombineHandlers(
						func(http.ResponseWriter, *http.Request) {
							deleteChan <- struct{}{}
						},
						ghttp.RespondWithJSONEncoded(http.StatusOK, &struct{}{})))
				fakeGarden.RouteToHandler("DELETE", "/containers/cnr2",
					ghttp.CombineHandlers(
						func(http.ResponseWriter, *http.Request) {
							deleteChan <- struct{}{}
						},
						ghttp.RespondWithJSONEncoded(http.StatusOK, &struct{}{})))
			})

			It("destroys any existing containers", func() {
				Eventually(deleteChan).Should(Receive())
				Eventually(deleteChan).Should(Receive())
			})
		})

		Describe("maintaining presence", func() {
			Context("with consul", func() {
				BeforeEach(func() {
					repConfig.LocketAddress = ""
				})

				It("should maintain presence", func() {
					Eventually(fetchCells(logger)).Should(HaveLen(1))

					cells, err := bbsClient.Cells(logger)
					Expect(err).NotTo(HaveOccurred())

					cellSet := models.NewCellSetFromList(cells)

					cellPresence := cellSet[cellID]
					Expect(cellPresence.CellId).To(Equal(cellID))
					Expect(cellPresence.RepAddress).To(MatchRegexp(fmt.Sprintf(`http\:\/\/.*\:%d`, serverPort)))
					Expect(cellPresence.PlacementTags).To(Equal([]string{"test"}))
					Expect(cellPresence.OptionalPlacementTags).To(Equal([]string{"optional_tag"}))
				})
			})

			Context("with locket", func() {
				var (
					locketRunner  ifrit.Runner
					locketProcess ifrit.Process
					locketAddress string
				)

				BeforeEach(func() {
					locketPort, err := localip.LocalPort()
					Expect(err).NotTo(HaveOccurred())

					locketAddress = fmt.Sprintf("localhost:%d", locketPort)
					locketRunner = locketrunner.NewLocketRunner(locketBinPath, func(cfg *locketconfig.LocketConfig) {
						cfg.ConsulCluster = consulRunner.ConsulCluster()
						cfg.DatabaseConnectionString = sqlRunner.ConnectionString()
						cfg.DatabaseDriver = sqlRunner.DriverName()
						cfg.ListenAddress = locketAddress
					})
					locketProcess = ginkgomon.Invoke(locketRunner)

					repConfig.ClientLocketConfig = locketrunner.ClientLocketConfig()
					repConfig.LocketAddress = locketAddress

					runner = testrunner.New(representativePath, repConfig)
				})

				AfterEach(func() {
					ginkgomon.Kill(bbsProcess)
					ginkgomon.Kill(locketProcess)
				})

				It("should maintain presence", func() {
					locketClient, err := locket.NewClient(logger, repConfig.ClientLocketConfig)
					Expect(err).NotTo(HaveOccurred())

					var response *locketmodels.FetchResponse
					Eventually(func() error {
						response, err = locketClient.Fetch(context.Background(), &locketmodels.FetchRequest{Key: repConfig.CellID})
						return err
					}, 10*time.Second).Should(Succeed())
					Expect(response.Resource.Key).To(Equal(repConfig.CellID))
					Expect(response.Resource.Type).To(Equal(locketmodels.PresenceType))
					Expect(response.Resource.TypeCode).To(Equal(locketmodels.PRESENCE))
					value := &models.CellPresence{}
					err = json.Unmarshal([]byte(response.Resource.Value), value)
					Expect(err).NotTo(HaveOccurred())
					Expect(value.Zone).To(Equal(repConfig.Zone))
					Expect(value.CellId).To(Equal(repConfig.CellID))
				})

				Context("when it loses its presence", func() {
					var locketClient locketmodels.LocketClient

					JustBeforeEach(func() {
						var err error
						locketClient, err = locket.NewClient(logger, repConfig.ClientLocketConfig)
						Expect(err).NotTo(HaveOccurred())

						Eventually(func() error {
							_, err = locketClient.Fetch(context.Background(), &locketmodels.FetchRequest{Key: repConfig.CellID})
							return err
						}, 10*time.Second).Should(Succeed())
					})

					It("does not exit", func() {
						ginkgomon.Kill(locketProcess)
						Consistently(runner.Session, locket.RetryInterval).ShouldNot(Exit())
					})
				})
			})
		})

		Context("acting as an auction representative", func() {
			var client rep.Client

			JustBeforeEach(func() {
				Eventually(fetchCells(logger)).Should(HaveLen(1))
				cells, err := bbsClient.Cells(logger)
				cellSet := models.NewCellSetFromList(cells)
				Expect(err).NotTo(HaveOccurred())

				factory, err := rep.NewClientFactory(http.DefaultClient, cfhttp.NewCustomTimeoutClient(100*time.Millisecond), nil)
				Expect(err).NotTo(HaveOccurred())
				client, err = factory.CreateClient(cellSet[cellID].RepAddress, "")
				Expect(err).NotTo(HaveOccurred())
			})

			Context("Capacity with a container", func() {
				It("returns total capacity and state information", func() {
					state, err := client.State(logger)
					Expect(err).NotTo(HaveOccurred())
					Expect(state.TotalResources).To(Equal(rep.Resources{
						MemoryMB:   1024,
						DiskMB:     10 * 1024,
						Containers: 3,
					}))
					Expect(state.PlacementTags).To(Equal([]string{"test"}))
					Expect(state.OptionalPlacementTags).To(Equal([]string{"optional_tag"}))
				})

				Context("when the container is removed", func() {
					It("returns available capacity == total capacity", func() {
						fakeGarden.RouteToHandler("GET", "/containers", ghttp.RespondWithJSONEncoded(http.StatusOK, struct{}{}))
						fakeGarden.RouteToHandler("GET", "/containers/bulk_info", ghttp.RespondWithJSONEncoded(http.StatusOK, struct{}{}))

						Eventually(func() rep.Resources {
							state, err := client.State(logger)
							Expect(err).NotTo(HaveOccurred())
							return state.AvailableResources
						}).Should(Equal(rep.Resources{
							MemoryMB:   1024,
							DiskMB:     10 * 1024,
							Containers: 3,
						}))
					})
				})
			})
		})

		Describe("polling the BBS for tasks to reap", func() {
			var task *models.Task

			JustBeforeEach(func() {
				task = model_helpers.NewValidTask("task-guid")
				err := bbsClient.DesireTask(logger, task.TaskGuid, task.Domain, task.TaskDefinition)
				Expect(err).NotTo(HaveOccurred())

				_, err = bbsClient.StartTask(logger, task.TaskGuid, cellID)
				Expect(err).NotTo(HaveOccurred())
			})

			It("eventually marks tasks with no corresponding container as failed", func() {
				Eventually(func() []*models.Task {
					return getTasksByState(logger, bbsClient, models.Task_Completed)
				}, 5*pollingInterval).Should(HaveLen(1))

				completedTasks := getTasksByState(logger, bbsClient, models.Task_Completed)

				Expect(completedTasks[0].TaskGuid).To(Equal(task.TaskGuid))
				Expect(completedTasks[0].Failed).To(BeTrue())
			})
		})

		Describe("polling the BBS for actual LRPs to reap", func() {
			JustBeforeEach(func() {
				desiredLRP := &models.DesiredLRP{
					ProcessGuid: "process-guid",
					RootFs:      "some:rootfs",
					Domain:      "some-domain",
					Instances:   1,
					Action: models.WrapAction(&models.RunAction{
						User: "me",
						Path: "the-path",
						Args: []string{},
					}),
				}
				index := 0

				err := bbsClient.DesireLRP(logger, desiredLRP)
				Expect(err).NotTo(HaveOccurred())

				instanceKey := models.NewActualLRPInstanceKey("some-instance-guid", cellID)
				err = bbsClient.ClaimActualLRP(logger, desiredLRP.ProcessGuid, index, &instanceKey)
				Expect(err).NotTo(HaveOccurred())
			})

			It("eventually reaps actual LRPs with no corresponding container", func() {
				Eventually(getActualLRPGroups(logger), 5*pollingInterval).Should(BeEmpty())
			})
		})

		Describe("Evacuation", func() {
			JustBeforeEach(func() {
				resp, err := http.Post(fmt.Sprintf("http://127.0.0.1:%d/evacuate", serverPort), "text/html", nil)
				Expect(err).NotTo(HaveOccurred())
				resp.Body.Close()
				Expect(resp.StatusCode).To(Equal(http.StatusAccepted))
			})

			Context("when exceeding the evacuation timeout", func() {
				It("shuts down gracefully", func() {
					// wait longer than expected to let OS and Go runtime reap process
					Eventually(runner.Session.ExitCode, 2*evacuationTimeout+2*time.Second).Should(Equal(0))
				})
			})

			Context("when signaled to stop", func() {
				JustBeforeEach(func() {
					runner.Stop()
				})

				It("shuts down gracefully", func() {
					Eventually(runner.Session.ExitCode).Should(Equal(0))
				})
			})
		})

		Describe("when a Ping request comes in", func() {
			It("responds with 200 OK", func() {
				resp, err := http.Get(fmt.Sprintf("http://127.0.0.1:%d/ping", serverPort))
				Expect(err).NotTo(HaveOccurred())
				Expect(resp.StatusCode).To(Equal(http.StatusOK))
			})
		})

		Describe("ServiceRegistration", func() {
			It("registers itself with consul", func() {
				consulClient := consulRunner.NewClient()
				services, err := consulClient.Agent().Services()
				Expect(err).ToNot(HaveOccurred())

				Expect(services).To(HaveKeyWithValue("cell",
					&api.AgentService{
						Service: "cell",
						ID:      "cell",
						Port:    serverPort,
						Tags:    []string{strings.Replace(cellID, "_", "-", -1)},
					}))
			})

			It("registers a TTL healthcheck", func() {
				consulClient := consulRunner.NewClient()
				checks, err := consulClient.Agent().Checks()
				Expect(err).ToNot(HaveOccurred())

				Expect(checks).To(HaveKeyWithValue("service:cell",
					&api.AgentCheck{
						Node:        "0",
						CheckID:     "service:cell",
						Name:        "Service 'cell' check",
						Status:      "passing",
						ServiceID:   "cell",
						ServiceName: "cell",
					}))
			})
		})

		Describe("Secure Server", func() {
			var (
				clientFactory                                            rep.ClientFactory
				client                                                   *http.Client
				tlsConfig                                                *rep.TLSConfig
				err                                                      error
				caFile, certFile, keyFile, clientCertFile, clientKeyFile string
			)

			BeforeEach(func() {
				client = cfhttp.NewClient()
				caFile = path.Join(basePath, "green-certs", "server-ca.crt")
				certFile = path.Join(basePath, "green-certs", "server.crt")
				keyFile = path.Join(basePath, "green-certs", "server.key")
				clientCertFile = path.Join(basePath, "green-certs", "client.crt")
				clientKeyFile = path.Join(basePath, "green-certs", "client.key")
				tlsConfig = &rep.TLSConfig{}
			})

			JustBeforeEach(func() {
				clientFactory, err = rep.NewClientFactory(client, client, tlsConfig)
			})

			Context("when requireTLS is set to false", func() {
				var (
					addr string
				)

				BeforeEach(func() {
					addr = fmt.Sprintf("http://127.0.0.1:%d", serverPortSecurable)
				})

				It("creates an insecure server", func() {
					client, err := clientFactory.CreateClient(addr, "")
					Expect(err).NotTo(HaveOccurred())
					_, err = client.State(logger)
					Expect(err).NotTo(HaveOccurred())
				})

				Context("ClientFactory", func() {
					canConnectSuccessfully := func() {
						client, err := clientFactory.CreateClient("", addr)
						Expect(err).NotTo(HaveOccurred())
						_, err = client.State(logger)
						Expect(err).NotTo(HaveOccurred())
					}

					Context("doesn't support tls", func() {
						It("can create a new client using the address", func() {
							canConnectSuccessfully()
						})
					})

					Context("prefers tls", func() {
						BeforeEach(func() {
							tlsConfig = &rep.TLSConfig{
								RequireTLS: false,
								CertFile:   clientCertFile,
								KeyFile:    clientKeyFile,
								CaCertFile: caFile,
							}
						})

						It("can connect to the insecure url", func() {
							canConnectSuccessfully()
						})
					})

					Context("requires tls", func() {
						BeforeEach(func() {
							tlsConfig = &rep.TLSConfig{RequireTLS: true}
						})

						It("returns an error when creating a new client", func() {
							_, err := clientFactory.CreateClient("", addr)
							Expect(err).To(MatchError(ContainSubstring("https scheme is required")))
						})
					})
				})
			})

			Context("when requireTLS is set to true", func() {
				BeforeEach(func() {
					repConfig.RequireTLS = true
				})

				Context("when invalid values for certificates are supplied", func() {
					BeforeEach(func() {
						repConfig.CaCertFile = ""
						repConfig.ServerCertFile = ""
						repConfig.ServerKeyFile = ""
						runner = testrunner.New(
							representativePath,
							repConfig,
						)
						runner.StartCheck = ""
					})

					It("fails to start secure server", func() {
						Eventually(runner.Session.Buffer()).Should(gbytes.Say("tls-configuration-failed"))
						Eventually(runner.Session.ExitCode).Should(Equal(2))
					})
				})

				Context("when an incorrect server key is supplied", func() {
					BeforeEach(func() {
						repConfig.CaCertFile = path.Join(basePath, "green-certs", "server-ca.crt")
						repConfig.ServerCertFile = path.Join(basePath, "green-certs", "server.crt")
						repConfig.ServerKeyFile = path.Join(basePath, "blue-certs", "server.key")
						runner = testrunner.New(
							representativePath,
							repConfig,
						)
						runner.StartCheck = ""
					})

					It("fails to start secure server", func() {
						Eventually(runner.Session.Buffer()).Should(gbytes.Say("tls-configuration-failed"))
						Eventually(runner.Session.ExitCode).Should(Equal(2))
					})
				})

				Context("when correct server cert and key are supplied", func() {
					BeforeEach(func() {
						repConfig.CaCertFile = caFile
						repConfig.ServerCertFile = certFile
						repConfig.ServerKeyFile = keyFile
						runner = testrunner.New(
							representativePath,
							repConfig,
						)
					})

					It("runs secure server", func() {
						Eventually(runner.Session.Buffer()).ShouldNot(gbytes.Say("tls-configuration-failed"))
					})

					Context("when server is insecurable", func() {
						BeforeEach(func() {
							repConfig.EnableLegacyAPIServer = true

							runner = testrunner.New(
								representativePath,
								repConfig,
							)
						})

						Context("for the secure server", func() {
							var client *http.Client
							BeforeEach(func() {
								tlsConfig = &rep.TLSConfig{
									RequireTLS: true,
									CaCertFile: caFile,
									KeyFile:    clientKeyFile,
									CertFile:   clientCertFile,
								}
								config, err := cfhttp.NewTLSConfig(tlsConfig.CertFile, tlsConfig.KeyFile, tlsConfig.CaCertFile)
								Expect(err).NotTo(HaveOccurred())
								client = &http.Client{Transport: &http.Transport{TLSClientConfig: config}}
							})

							It("does not have unsecured routes on the secure server", func() {
								resp, err := client.Get(fmt.Sprintf("https://127.0.0.1:%d/ping", serverPortSecurable))
								Expect(err).NotTo(HaveOccurred())
								Expect(resp.StatusCode).To(Equal(http.StatusNotFound))
							})

							It("has secured routes on the secure server", func() {
								resp, err := client.Get(fmt.Sprintf("https://127.0.0.1:%d/state", serverPortSecurable))
								Expect(err).NotTo(HaveOccurred())
								Expect(resp.StatusCode).To(Equal(http.StatusOK))
							})
						})

						It("has all secure and insecure routes on the unsecured server", func() {
							resp, err := http.Get(fmt.Sprintf("http://127.0.0.1:%d/ping", serverPort))
							Expect(err).NotTo(HaveOccurred())
							Expect(resp.StatusCode).To(Equal(http.StatusOK))

							resp, err = http.Get(fmt.Sprintf("http://127.0.0.1:%d/state", serverPort))
							Expect(err).NotTo(HaveOccurred())
							Expect(resp.StatusCode).To(Equal(http.StatusOK))
						})
					})

					Context("when server is not insecurable", func() {
						BeforeEach(func() {
							repConfig.EnableLegacyAPIServer = false

							runner = testrunner.New(
								representativePath,
								repConfig,
							)
						})

						Context("for the secure server", func() {
							var client *http.Client
							BeforeEach(func() {
								tlsConfig = &rep.TLSConfig{
									RequireTLS: true,
									CaCertFile: caFile,
									KeyFile:    clientKeyFile,
									CertFile:   clientCertFile,
								}
								config, err := cfhttp.NewTLSConfig(tlsConfig.CertFile, tlsConfig.KeyFile, tlsConfig.CaCertFile)
								Expect(err).NotTo(HaveOccurred())
								client = &http.Client{Transport: &http.Transport{TLSClientConfig: config}}
							})

							It("does not have insecure routes on the secure server", func() {
								resp, _ := client.Get(fmt.Sprintf("https://127.0.0.1:%d/ping", serverPortSecurable))
								Expect(err).NotTo(HaveOccurred())
								Expect(resp.StatusCode).To(Equal(http.StatusNotFound))
							})

							It("has secure routes on the secure server", func() {
								resp, err := client.Get(fmt.Sprintf("https://127.0.0.1:%d/state", serverPortSecurable))
								Expect(err).NotTo(HaveOccurred())
								Expect(resp.StatusCode).To(Equal(http.StatusOK))
							})
						})

						It("has only insecure routes on the unsecured server", func() {
							resp, err := http.Get(fmt.Sprintf("http://127.0.0.1:%d/ping", serverPort))
							Expect(err).NotTo(HaveOccurred())
							Expect(resp.StatusCode).To(Equal(http.StatusOK))

							resp, err = http.Get(fmt.Sprintf("http://127.0.0.1:%d/state", serverPort))
							Expect(err).NotTo(HaveOccurred())
							Expect(resp.StatusCode).To(Equal(http.StatusNotFound))
						})
					})

					Context("ClientFactory", func() {
						var (
							addr string
						)

						BeforeEach(func() {
							addr = fmt.Sprintf("https://127.0.0.1:%d", serverPortSecurable)
						})

						canConnectSuccessfully := func() {
							client, err := clientFactory.CreateClient("", addr)
							Expect(err).NotTo(HaveOccurred())
							_, err = client.State(logger)
							Expect(err).NotTo(HaveOccurred())
						}

						Context("doesn't support tls", func() {
							It("cannot create a new client using the address", func() {
								_, err := clientFactory.CreateClient("", addr)
								Expect(err).To(MatchError(ContainSubstring("https scheme not supported")))
							})
						})

						Context("prefers tls", func() {
							BeforeEach(func() {
								tlsConfig = &rep.TLSConfig{
									RequireTLS: false,
									CertFile:   clientCertFile,
									KeyFile:    clientKeyFile,
									CaCertFile: caFile,
								}
							})

							It("can connect to the secure url", func() {
								canConnectSuccessfully()
							})
						})

						Context("requires tls", func() {
							BeforeEach(func() {
								tlsConfig = &rep.TLSConfig{
									RequireTLS: true,
									CaCertFile: caFile,
									KeyFile:    clientKeyFile,
									CertFile:   clientCertFile,
								}
							})

							It("can connect to the secure url", func() {
								canConnectSuccessfully()
							})

							It("sets a session cache", func() {
								tlsConfig := client.Transport.(*http.Transport).TLSClientConfig
								Expect(tlsConfig.ClientSessionCache).NotTo(BeNil())
							})
						})
					})
				})
			})
		})

		Describe("RootFS for garden healthcheck", func() {
			var createRequestReceived chan struct{}

			BeforeEach(func() {
				respondWithSuccessToCreateContainer = false
				createRequestReceived = make(chan struct{})
				fakeGarden.AppendHandlers(
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("POST", "/containers", ""),
						func(w http.ResponseWriter, req *http.Request) {
							body, err := ioutil.ReadAll(req.Body)
							req.Body.Close()
							Expect(err).ShouldNot(HaveOccurred())
							Expect(string(body)).Should(ContainSubstring(`executor-healthcheck`))
							Expect(string(body)).Should(ContainSubstring(`"rootfs":"/path/to/rootfs"`))
							createRequestReceived <- struct{}{}
						},
					),
				)

				fakeGarden.AllowUnhandledRequests = true
				runner.StartCheck = ""
			})

			It("sends the correct rootfs when creating the container", func() {
				Eventually(createRequestReceived).Should(Receive())
			})
		})
	})

	Context("when Garden is unavailable", func() {
		BeforeEach(func() {
			runner.StartCheck = ""
		})

		It("should not exit and continue waiting for a connection", func() {
			Consistently(runner.Session.Buffer()).ShouldNot(gbytes.Say("started"))
			Consistently(runner.Session).ShouldNot(Exit())
		})

		Context("when Garden starts", func() {
			JustBeforeEach(func() {
				fakeGarden.Start()
				// these tests only look for the start of a sequence of requests
				fakeGarden.AllowUnhandledRequests = false
			})

			It("should connect", func() {
				Eventually(runner.Session.Buffer(), 5*time.Second).Should(gbytes.Say("started"))
			})
		})
	})
})

func getTasksByState(logger lager.Logger, client bbs.InternalClient, state models.Task_State) []*models.Task {
	tasks, err := client.Tasks(logger)
	Expect(err).NotTo(HaveOccurred())

	filteredTasks := make([]*models.Task, 0)
	for _, task := range tasks {
		if task.State == state {
			filteredTasks = append(filteredTasks, task)
		}
	}
	return filteredTasks
}

func fetchCells(logger lager.Logger) func() ([]*models.CellPresence, error) {
	return func() ([]*models.CellPresence, error) {
		return bbsClient.Cells(logger)
	}
}
