package main_test

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"mime/multipart"
	"net/http"
	"os"
	"time"

	"github.com/cloudfoundry/sonde-go/events"
	"github.com/gogo/protobuf/proto"
	"github.com/hashicorp/consul/api"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/ghttp"
	"github.com/tedsuo/ifrit/ginkgomon"
	"github.com/tedsuo/rata"

	"code.cloudfoundry.org/bbs/models"
	"code.cloudfoundry.org/runtimeschema/cc_messages"
	"code.cloudfoundry.org/tps"
)

var _ = Describe("TPS-Listener", func() {
	var (
		httpClient       *http.Client
		requestGenerator *rata.RequestGenerator

		desiredLRP, desiredLRP2 *models.DesiredLRP
	)

	BeforeEach(func() {
		requestGenerator = rata.NewRequestGenerator(fmt.Sprintf("http://%s", listenerAddr), tps.Routes)
		httpClient = &http.Client{
			Transport: &http.Transport{},
		}

		desiredLRP = &models.DesiredLRP{
			Domain:      "some-domain",
			ProcessGuid: "some-process-guid",
			Instances:   3,
			RootFs:      "some:rootfs",
			MemoryMb:    1024,
			DiskMb:      512,
			LogGuid:     "some-process-guid",
			Action: models.WrapAction(&models.RunAction{
				User: "me",
				Path: "ls",
			}),
		}
	})

	JustBeforeEach(func() {
		listener = ginkgomon.Invoke(runner)
	})

	AfterEach(func() {
		if listener != nil {
			listener.Signal(os.Kill)
			Eventually(listener.Wait()).Should(Receive())
		}
	})

	Describe("Initialization", func() {
		It("registers itself with consul", func() {
			services, err := consulRunner.NewClient().Agent().Services()
			Expect(err).NotTo(HaveOccurred())
			Expect(services).Should(HaveKeyWithValue("tps",
				&api.AgentService{
					Service: "tps",
					ID:      "tps",
					Port:    listenerPort,
				}))
		})

		It("registers a TTL healthcheck", func() {
			checks, err := consulRunner.NewClient().Agent().Checks()
			Expect(err).NotTo(HaveOccurred())
			Expect(checks).Should(HaveKeyWithValue("service:tps",
				&api.AgentCheck{
					Node:        "0",
					CheckID:     "service:tps",
					Name:        "Service 'tps' check",
					Status:      "passing",
					ServiceID:   "tps",
					ServiceName: "tps",
				}))
		})
	})

	Describe("GET /v1/actual_lrps/:guid", func() {
		Context("when the bbs is running", func() {
			JustBeforeEach(func() {
				fakeBBS.RouteToHandler("POST", "/v1/actual_lrp_groups/list_by_process_guid",
					ghttp.RespondWithProto(200, &models.ActualLRPGroupsResponse{
						ActualLrpGroups: []*models.ActualLRPGroup{
							{Instance: &models.ActualLRP{ActualLRPKey: models.ActualLRPKey{ProcessGuid: "some-process-guid", Index: 0}, ActualLRPInstanceKey: models.ActualLRPInstanceKey{InstanceGuid: "some-instance-guid-0"}, State: models.ActualLRPStateClaimed}},
							{Instance: &models.ActualLRP{ActualLRPKey: models.ActualLRPKey{ProcessGuid: "some-process-guid", Index: 1}, ActualLRPInstanceKey: models.ActualLRPInstanceKey{InstanceGuid: "some-instance-guid-1"}, State: models.ActualLRPStateRunning}},
							{Instance: &models.ActualLRP{ActualLRPKey: models.ActualLRPKey{ProcessGuid: "some-process-guid", Index: 2}, State: models.ActualLRPStateUnclaimed}},
						},
					}),
				)
			})

			It("reports the state of the given process guid's instances", func() {
				getLRPs, err := requestGenerator.CreateRequest(
					tps.LRPStatus,
					rata.Params{"guid": "some-process-guid"},
					nil,
				)
				Expect(err).NotTo(HaveOccurred())

				response, err := httpClient.Do(getLRPs)
				Expect(err).NotTo(HaveOccurred())

				var lrpInstances []cc_messages.LRPInstance
				err = json.NewDecoder(response.Body).Decode(&lrpInstances)
				Expect(err).NotTo(HaveOccurred())

				Expect(lrpInstances).To(HaveLen(3))
				for i, _ := range lrpInstances {
					Expect(lrpInstances[i]).NotTo(BeZero())
					lrpInstances[i].Since = 0

					Eventually(lrpInstances[i]).ShouldNot(BeZero())
					lrpInstances[i].Uptime = 0
				}

				Expect(lrpInstances).To(ContainElement(cc_messages.LRPInstance{
					ProcessGuid:  "some-process-guid",
					InstanceGuid: "some-instance-guid-0",
					Index:        0,
					State:        cc_messages.LRPInstanceStateStarting,
				}))

				Expect(lrpInstances).To(ContainElement(cc_messages.LRPInstance{
					ProcessGuid:  "some-process-guid",
					InstanceGuid: "some-instance-guid-1",
					Index:        1,
					State:        cc_messages.LRPInstanceStateRunning,
				}))

				Expect(lrpInstances).To(ContainElement(cc_messages.LRPInstance{
					ProcessGuid:  "some-process-guid",
					InstanceGuid: "",
					Index:        2,
					State:        cc_messages.LRPInstanceStateStarting,
				}))
			})
		})

		Context("when the bbs is not running", func() {
			JustBeforeEach(func() {
				fakeBBS.HTTPTestServer.Close()
			})

			It("returns 500", func() {
				getLRPs, err := requestGenerator.CreateRequest(
					tps.LRPStatus,
					rata.Params{"guid": "some-process-guid"},
					nil,
				)
				Expect(err).NotTo(HaveOccurred())

				response, err := httpClient.Do(getLRPs)
				Expect(err).NotTo(HaveOccurred())

				Expect(response.StatusCode).To(Equal(http.StatusInternalServerError))
			})
		})
	})

	Describe("GET /v1/actual_lrps/:guid/stats", func() {
		Context("when the bbs is running", func() {
			var netInfo models.ActualLRPNetInfo

			JustBeforeEach(func() {
				netInfo = models.NewActualLRPNetInfo("1.2.3.4", models.NewPortMapping(65100, 8080))

				fakeBBS.RouteToHandler("POST", "/v1/actual_lrp_groups/list_by_process_guid",
					ghttp.RespondWithProto(200, &models.ActualLRPGroupsResponse{
						ActualLrpGroups: []*models.ActualLRPGroup{
							{Instance: &models.ActualLRP{ActualLRPKey: models.ActualLRPKey{ProcessGuid: "some-process-guid", Index: 0}, ActualLRPInstanceKey: models.ActualLRPInstanceKey{InstanceGuid: "some-instance-guid-0"}, State: models.ActualLRPStateClaimed}},
							{Instance: &models.ActualLRP{ActualLRPKey: models.ActualLRPKey{ProcessGuid: "some-process-guid", Index: 1}, ActualLRPInstanceKey: models.ActualLRPInstanceKey{InstanceGuid: "some-instance-guid-1"}, State: models.ActualLRPStateRunning, ActualLRPNetInfo: netInfo}},
							{Instance: &models.ActualLRP{ActualLRPKey: models.ActualLRPKey{ProcessGuid: "some-process-guid", Index: 2}, State: models.ActualLRPStateUnclaimed}},
						},
					}),
				)
			})

			Context("when a DesiredLRP is not found", func() {
				BeforeEach(func() {
					fakeBBS.RouteToHandler("POST", "/v1/desired_lrps/get_by_process_guid.r2",
						ghttp.RespondWithProto(200, &models.DesiredLRPResponse{
							Error: models.ErrResourceNotFound,
						}),
					)
				})

				It("returns a NotFound", func() {
					getLRPStats, err := requestGenerator.CreateRequest(
						tps.LRPStats,
						rata.Params{"guid": "some-bogus-guid"},
						nil,
					)
					Expect(err).ToNot(HaveOccurred())
					getLRPStats.Header.Add("Authorization", "I can do this.")

					response, err := httpClient.Do(getLRPStats)
					Expect(err).ToNot(HaveOccurred())
					Expect(response.StatusCode).To(Equal(http.StatusNotFound))
				})
			})

			Context("when the traffic controller is running", func() {
				BeforeEach(func() {
					message1 := marshalMessage(createContainerMetric("some-process-guid", 0, 3.0, 1024, 2048, 0))
					message2 := marshalMessage(createContainerMetric("some-process-guid", 1, 4.0, 1024, 2048, 0))
					message3 := marshalMessage(createContainerMetric("some-process-guid", 2, 5.0, 1024, 2048, 0))

					messages := map[string][][]byte{}
					messages["some-process-guid"] = [][]byte{message1, message2, message3}
					fakeTrafficController.RouteToHandler("GET", "/apps/some-process-guid/containermetrics",
						func(rw http.ResponseWriter, r *http.Request) {
							mp := multipart.NewWriter(rw)
							defer mp.Close()

							guid := "some-process-guid"

							rw.Header().Set("Content-Type", `multipart/x-protobuf; boundary=`+mp.Boundary())

							for _, msg := range messages[guid] {
								partWriter, _ := mp.CreatePart(nil)
								partWriter.Write(msg)
							}
						},
					)

					fakeBBS.RouteToHandler("POST", "/v1/desired_lrps/get_by_process_guid.r2",
						ghttp.CombineHandlers(
							ghttp.VerifyProtoRepresenting(&models.DesiredLRPByProcessGuidRequest{ProcessGuid: "some-process-guid"}),
							ghttp.RespondWithProto(200, &models.DesiredLRPResponse{
								DesiredLrp: desiredLRP,
							}),
						),
					)
				})

				It("reports the state of the given process guid's instances", func() {
					getLRPStats, err := requestGenerator.CreateRequest(
						tps.LRPStats,
						rata.Params{"guid": "some-process-guid"},
						nil,
					)
					Expect(err).NotTo(HaveOccurred())
					getLRPStats.Header.Add("Authorization", "I can do this.")

					response, err := httpClient.Do(getLRPStats)
					Expect(err).NotTo(HaveOccurred())
					Expect(response.StatusCode).To(Equal(http.StatusOK))

					var lrpInstances []cc_messages.LRPInstance
					err = json.NewDecoder(response.Body).Decode(&lrpInstances)
					Expect(err).NotTo(HaveOccurred())

					Expect(lrpInstances).To(HaveLen(3))
					zeroTime := time.Unix(0, 0)
					for i, _ := range lrpInstances {
						Expect(lrpInstances[i].Stats.Time).NotTo(BeZero())
						lrpInstances[i].Stats.Time = zeroTime

						Expect(lrpInstances[i]).NotTo(BeZero())
						lrpInstances[i].Since = 0

						Eventually(lrpInstances[i]).ShouldNot(BeZero())
						lrpInstances[i].Uptime = 0
					}

					Expect(lrpInstances).To(ContainElement(cc_messages.LRPInstance{
						ProcessGuid:  "some-process-guid",
						InstanceGuid: "some-instance-guid-0",
						Index:        0,
						State:        cc_messages.LRPInstanceStateStarting,
						Stats: &cc_messages.LRPInstanceStats{
							Time:          zeroTime,
							CpuPercentage: 0.03,
							MemoryBytes:   1024,
							DiskBytes:     2048,
						},
					}))

					Expect(lrpInstances).To(ContainElement(cc_messages.LRPInstance{
						ProcessGuid:  "some-process-guid",
						InstanceGuid: "some-instance-guid-1",
						Index:        1,
						State:        cc_messages.LRPInstanceStateRunning,
						Host:         "1.2.3.4",
						Port:         65100,
						NetInfo:      netInfo,
						Stats: &cc_messages.LRPInstanceStats{
							Time:          zeroTime,
							CpuPercentage: 0.04,
							MemoryBytes:   1024,
							DiskBytes:     2048,
						},
					}))

					Expect(lrpInstances).To(ContainElement(cc_messages.LRPInstance{
						ProcessGuid:  "some-process-guid",
						InstanceGuid: "",
						Index:        2,
						State:        cc_messages.LRPInstanceStateStarting,
						Stats: &cc_messages.LRPInstanceStats{
							Time:          zeroTime,
							CpuPercentage: 0.05,
							MemoryBytes:   1024,
							DiskBytes:     2048,
						},
					}))
				})
			})

			Context("when the traffic controller is not running", func() {
				BeforeEach(func() {
					fakeBBS.RouteToHandler("POST", "/v1/desired_lrps/get_by_process_guid.r2",
						ghttp.CombineHandlers(
							ghttp.VerifyProtoRepresenting(&models.DesiredLRPByProcessGuidRequest{ProcessGuid: "some-process-guid"}),
							ghttp.RespondWithProto(200, &models.DesiredLRPResponse{
								DesiredLrp: desiredLRP,
							}),
						),
					)
					fakeTrafficController.HTTPTestServer.Close()
				})

				It("reports the status with nil stats", func() {
					getLRPStats, err := requestGenerator.CreateRequest(
						tps.LRPStats,
						rata.Params{"guid": "some-process-guid"},
						nil,
					)
					Expect(err).NotTo(HaveOccurred())
					getLRPStats.Header.Add("Authorization", "I can do this.")

					response, err := httpClient.Do(getLRPStats)
					Expect(err).NotTo(HaveOccurred())
					Expect(response.StatusCode).To(Equal(http.StatusOK))

					var lrpInstances []cc_messages.LRPInstance
					err = json.NewDecoder(response.Body).Decode(&lrpInstances)
					Expect(err).NotTo(HaveOccurred())

					Expect(lrpInstances).To(HaveLen(3))

					for _, instance := range lrpInstances {
						Expect(instance.Stats).To(BeNil())
					}
				})
			})
		})

		Context("when the bbs is not running", func() {
			JustBeforeEach(func() {
				fakeBBS.HTTPTestServer.Close()
			})

			It("returns internal server error", func() {
				getLRPs, err := requestGenerator.CreateRequest(
					tps.LRPStats,
					rata.Params{"guid": "some-process-guid"},
					nil,
				)
				Expect(err).NotTo(HaveOccurred())
				getLRPs.Header.Add("Authorization", "I can do this.")

				response, err := httpClient.Do(getLRPs)
				Expect(err).NotTo(HaveOccurred())

				Expect(response.StatusCode).To(Equal(http.StatusInternalServerError))
			})
		})
	})

	Describe("GET /v1/bulk_actual_lrp_status", func() {
		var netInfo models.ActualLRPNetInfo

		JustBeforeEach(func() {
			desiredLRP2 = &models.DesiredLRP{
				Domain:      "some-domain",
				ProcessGuid: "some-other-process-guid",
				Instances:   3,
				RootFs:      "some:rootfs",
				MemoryMb:    1024,
				DiskMb:      512,
				LogGuid:     "some-other-log-guid",
				Action: models.WrapAction(&models.RunAction{
					User: "me",
					Path: "ls",
				}),
			}

			fakeBBS.RouteToHandler("POST", "/v1/desired_lrps/get_by_process_guid.r2",
				func(w http.ResponseWriter, r *http.Request) {
					body, err := ioutil.ReadAll(r.Body)
					Expect(err).NotTo(HaveOccurred())
					r.Body.Close()

					req := &models.DesiredLRPByProcessGuidRequest{}
					err = proto.Unmarshal(body, req)
					Expect(err).NotTo(HaveOccurred(), "Failed to unmarshal protobuf")

					if req.ProcessGuid == "some-process-guid" {
						ghttp.RespondWithProto(200, &models.DesiredLRPResponse{
							DesiredLrp: desiredLRP,
						})(w, nil)
					} else if req.ProcessGuid == "some-other-process-guid" {
						ghttp.RespondWithProto(200, &models.DesiredLRPResponse{
							DesiredLrp: desiredLRP2,
						})(w, nil)
					}
				},
			)

			netInfo = models.NewActualLRPNetInfo("1.2.3.4", models.NewPortMapping(65100, 8080))

			fakeBBS.RouteToHandler("POST", "/v1/actual_lrp_groups/list_by_process_guid",
				func(w http.ResponseWriter, r *http.Request) {
					body, err := ioutil.ReadAll(r.Body)
					Expect(err).NotTo(HaveOccurred())
					r.Body.Close()

					req := &models.ActualLRPGroupsByProcessGuidRequest{}
					err = proto.Unmarshal(body, req)
					Expect(err).NotTo(HaveOccurred(), "Failed to unmarshal protobuf")

					if req.ProcessGuid == "some-process-guid" {
						ghttp.RespondWithProto(200, &models.ActualLRPGroupsResponse{
							ActualLrpGroups: []*models.ActualLRPGroup{
								{Instance: &models.ActualLRP{ActualLRPKey: models.ActualLRPKey{ProcessGuid: "some-process-guid", Index: 0}, ActualLRPInstanceKey: models.ActualLRPInstanceKey{InstanceGuid: "some-instance-guid-0"}, State: models.ActualLRPStateClaimed}},
								{Instance: &models.ActualLRP{ActualLRPKey: models.ActualLRPKey{ProcessGuid: "some-process-guid", Index: 1}, ActualLRPInstanceKey: models.ActualLRPInstanceKey{InstanceGuid: "some-instance-guid-1"}, State: models.ActualLRPStateRunning, ActualLRPNetInfo: netInfo}},
								{Instance: &models.ActualLRP{ActualLRPKey: models.ActualLRPKey{ProcessGuid: "some-process-guid", Index: 2}, State: models.ActualLRPStateUnclaimed}},
							},
						})(w, nil)
					} else if req.ProcessGuid == "some-other-process-guid" {
						ghttp.RespondWithProto(200, &models.ActualLRPGroupsResponse{
							ActualLrpGroups: []*models.ActualLRPGroup{
								{Instance: &models.ActualLRP{ActualLRPKey: models.ActualLRPKey{ProcessGuid: "some-other-process-guid", Index: 0}, ActualLRPInstanceKey: models.ActualLRPInstanceKey{InstanceGuid: "some-instance-guid-0"}, State: models.ActualLRPStateClaimed}},
								{Instance: &models.ActualLRP{ActualLRPKey: models.ActualLRPKey{ProcessGuid: "some-other-process-guid", Index: 1}, ActualLRPInstanceKey: models.ActualLRPInstanceKey{InstanceGuid: "some-instance-guid-1"}, State: models.ActualLRPStateRunning, ActualLRPNetInfo: netInfo}},
								{Instance: &models.ActualLRP{ActualLRPKey: models.ActualLRPKey{ProcessGuid: "some-other-process-guid", Index: 2}, State: models.ActualLRPStateUnclaimed}},
							},
						})(w, nil)
					}
				},
			)
		})

		It("reports the status for all the process guids supplied", func() {
			getLRPStatus, err := requestGenerator.CreateRequest(
				tps.BulkLRPStatus,
				nil,
				nil,
			)
			Expect(err).NotTo(HaveOccurred())
			getLRPStatus.Header.Add("Authorization", "I can do this.")

			query := getLRPStatus.URL.Query()
			query.Set("guids", "some-process-guid,some-other-process-guid")
			getLRPStatus.URL.RawQuery = query.Encode()

			response, err := httpClient.Do(getLRPStatus)
			Expect(err).NotTo(HaveOccurred())
			Expect(response.StatusCode).To(Equal(http.StatusOK))

			var lrpInstanceStatus map[string][]cc_messages.LRPInstance
			err = json.NewDecoder(response.Body).Decode(&lrpInstanceStatus)
			Expect(err).NotTo(HaveOccurred())

			Expect(lrpInstanceStatus).To(HaveLen(2))
			for guid, instances := range lrpInstanceStatus {
				for i, _ := range instances {
					Expect(instances[i]).NotTo(BeZero())
					instances[i].Since = 0

					Eventually(instances[i]).ShouldNot(BeZero())
					instances[i].Uptime = 0
				}

				Expect(instances).To(ContainElement(cc_messages.LRPInstance{
					ProcessGuid:  guid,
					InstanceGuid: "some-instance-guid-0",
					Index:        0,
					State:        cc_messages.LRPInstanceStateStarting,
				}))

				Expect(instances).To(ContainElement(cc_messages.LRPInstance{
					ProcessGuid:  guid,
					InstanceGuid: "some-instance-guid-1",
					Index:        1,
					NetInfo:      netInfo,
					State:        cc_messages.LRPInstanceStateRunning,
				}))

				Expect(instances).To(ContainElement(cc_messages.LRPInstance{
					ProcessGuid:  guid,
					InstanceGuid: "",
					Index:        2,
					State:        cc_messages.LRPInstanceStateStarting,
				}))
			}
		})
	})
})

func createContainerMetric(appId string, instanceIndex int32, cpuPercentage float64, memoryBytes uint64, diskByte uint64, timestamp int64) *events.Envelope {
	if timestamp == 0 {
		timestamp = time.Now().UnixNano()
	}

	cm := &events.ContainerMetric{
		ApplicationId: proto.String(appId),
		InstanceIndex: proto.Int32(instanceIndex),
		CpuPercentage: proto.Float64(cpuPercentage),
		MemoryBytes:   proto.Uint64(memoryBytes),
		DiskBytes:     proto.Uint64(diskByte),
	}

	return &events.Envelope{
		ContainerMetric: cm,
		EventType:       events.Envelope_ContainerMetric.Enum(),
		Origin:          proto.String("fake-origin-1"),
		Timestamp:       proto.Int64(timestamp),
	}
}

func marshalMessage(message *events.Envelope) []byte {
	data, err := proto.Marshal(message)
	if err != nil {
		log.Println(err.Error())
	}

	return data
}
