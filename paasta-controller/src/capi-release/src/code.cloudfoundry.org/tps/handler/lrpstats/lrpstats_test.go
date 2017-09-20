package lrpstats_test

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"time"

	"code.cloudfoundry.org/bbs/fake_bbs"
	"code.cloudfoundry.org/bbs/models"
	"code.cloudfoundry.org/clock/fakeclock"
	"code.cloudfoundry.org/lager/lagertest"
	"code.cloudfoundry.org/nsync/recipebuilder"
	"code.cloudfoundry.org/runtimeschema/cc_messages"
	"code.cloudfoundry.org/tps/handler/lrpstats"
	"code.cloudfoundry.org/tps/handler/lrpstats/fakes"
	"github.com/cloudfoundry/sonde-go/events"
	"github.com/gogo/protobuf/proto"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gbytes"
)

var _ = Describe("Stats", func() {
	const authorization = "something good"
	const guid = "my-guid"
	const logGuid = "log-guid"

	var (
		handler    http.Handler
		response   *httptest.ResponseRecorder
		request    *http.Request
		noaaClient *fakes.FakeNoaaClient
		bbsClient  *fake_bbs.FakeClient
		logger     *lagertest.TestLogger
		fakeClock  *fakeclock.FakeClock
	)

	BeforeEach(func() {
		var err error

		bbsClient = new(fake_bbs.FakeClient)
		noaaClient = &fakes.FakeNoaaClient{}
		logger = lagertest.NewTestLogger("test")
		fakeClock = fakeclock.NewFakeClock(time.Date(2008, 8, 8, 8, 8, 8, 8, time.UTC))
		handler = lrpstats.NewHandler(bbsClient, noaaClient, fakeClock, logger)
		response = httptest.NewRecorder()
		request, err = http.NewRequest("GET", "/v1/actual_lrps/:guid/stats", nil)
		Expect(err).NotTo(HaveOccurred())
	})

	JustBeforeEach(func() {
		handler.ServeHTTP(response, request)
	})

	Describe("Validation", func() {
		It("fails with a missing authorization header", func() {
			Expect(response.Code).To(Equal(http.StatusUnauthorized))
		})

		Context("with an authorization header", func() {
			BeforeEach(func() {
				request.Header.Set("Authorization", authorization)
			})

			It("fails with no guid", func() {
				Expect(response.Code).To(Equal(http.StatusBadRequest))
			})
		})
	})

	Describe("retrieve container metrics", func() {
		var netInfo models.ActualLRPNetInfo
		var actualLRP *models.ActualLRP

		BeforeEach(func() {
			request.Header.Set("Authorization", authorization)
			request.Form = url.Values{}
			request.Form.Add(":guid", guid)

			noaaClient.ContainerMetricsReturns([]*events.ContainerMetric{
				{
					ApplicationId: proto.String("appId"),
					InstanceIndex: proto.Int32(5),
					CpuPercentage: proto.Float64(4),
					MemoryBytes:   proto.Uint64(1024),
					DiskBytes:     proto.Uint64(2048),
				},
			}, nil)

			bbsClient.DesiredLRPByProcessGuidReturns(&models.DesiredLRP{
				LogGuid:     logGuid,
				ProcessGuid: guid,
			}, nil)

			netInfo = models.NewActualLRPNetInfo(
				"host",
				models.NewPortMapping(5432, 7890),
				models.NewPortMapping(1234, uint32(recipebuilder.DefaultPort)),
			)

			actualLRP = &models.ActualLRP{
				ActualLRPKey:         models.NewActualLRPKey(guid, 5, "some-domain"),
				ActualLRPInstanceKey: models.NewActualLRPInstanceKey("instanceId", "some-cell"),
				ActualLRPNetInfo:     netInfo,
				State:                models.ActualLRPStateRunning,
				Since:                fakeClock.Now().UnixNano(),
			}

			bbsClient.ActualLRPGroupsByProcessGuidReturns([]*models.ActualLRPGroup{{
				Instance: actualLRP},
			}, nil)
		})

		Context("when the LRP has crashed", func() {
			var expectedSinceTime int64

			BeforeEach(func() {
				expectedSinceTime = fakeClock.Now().Unix()
				fakeClock.Increment(5 * time.Second)
				actualLRP.State = models.ActualLRPStateCrashed
			})

			It("returns a map of stats & status per index in the correct units", func() {
				expectedLRPInstance := cc_messages.LRPInstance{
					ProcessGuid:  guid,
					InstanceGuid: "instanceId",
					Index:        5,
					State:        cc_messages.LRPInstanceStateCrashed,
					Host:         "host",
					Port:         1234,
					NetInfo:      netInfo,
					Since:        expectedSinceTime,
					Uptime:       0,
					Stats: &cc_messages.LRPInstanceStats{
						Time:          time.Unix(0, 0),
						CpuPercentage: 0,
						MemoryBytes:   0,
						DiskBytes:     0,
					},
				}
				var stats []cc_messages.LRPInstance

				Expect(response.Code).To(Equal(http.StatusOK))
				Expect(response.Header().Get("Content-Type")).To(Equal("application/json"))
				err := json.Unmarshal(response.Body.Bytes(), &stats)
				Expect(err).NotTo(HaveOccurred())
				Expect(stats[0].Stats.Time).NotTo(BeZero())
				expectedLRPInstance.Stats.Time = stats[0].Stats.Time
				Expect(stats).To(ConsistOf(expectedLRPInstance))
			})
		})

		Context("when the LRP has been running for a while", func() {
			var expectedSinceTime int64

			BeforeEach(func() {
				expectedSinceTime = fakeClock.Now().Unix()
				fakeClock.Increment(5 * time.Second)
			})

			It("returns a map of stats & status per index in the correct units", func() {
				expectedLRPInstance := cc_messages.LRPInstance{
					ProcessGuid:  guid,
					InstanceGuid: "instanceId",
					Index:        5,
					State:        cc_messages.LRPInstanceStateRunning,
					Host:         "host",
					Port:         1234,
					NetInfo:      netInfo,
					Since:        expectedSinceTime,
					Uptime:       5,
					Stats: &cc_messages.LRPInstanceStats{
						Time:          time.Unix(0, 0),
						CpuPercentage: 0.04,
						MemoryBytes:   1024,
						DiskBytes:     2048,
					},
				}
				var stats []cc_messages.LRPInstance

				Expect(response.Code).To(Equal(http.StatusOK))
				Expect(response.Header().Get("Content-Type")).To(Equal("application/json"))
				err := json.Unmarshal(response.Body.Bytes(), &stats)
				Expect(err).NotTo(HaveOccurred())
				Expect(stats[0].Stats.Time).NotTo(BeZero())
				expectedLRPInstance.Stats.Time = stats[0].Stats.Time
				Expect(stats).To(ConsistOf(expectedLRPInstance))
			})
		})

		It("calls ContainerMetrics", func() {
			Expect(noaaClient.ContainerMetricsCallCount()).To(Equal(1))
			guid, token := noaaClient.ContainerMetricsArgsForCall(0)
			Expect(guid).To(Equal(logGuid))
			Expect(token).To(Equal(authorization))
		})

		Context("when ContainerMetrics fails", func() {
			var expectedLRPInstance cc_messages.LRPInstance
			BeforeEach(func() {
				noaaClient.ContainerMetricsReturns(nil, errors.New("bad stuff happened"))
				expectedLRPInstance = cc_messages.LRPInstance{
					ProcessGuid:  guid,
					InstanceGuid: "instanceId",
					Index:        5,
					State:        cc_messages.LRPInstanceStateRunning,
					Host:         "host",
					Port:         1234,
					NetInfo:      netInfo,
					Since:        fakeClock.Now().Unix(),
					Uptime:       0,
					Stats:        nil,
				}
			})

			It("responds with empty stats", func() {
				var stats []cc_messages.LRPInstance
				Expect(response.Code).To(Equal(http.StatusOK))
				Expect(response.Header().Get("Content-Type")).To(Equal("application/json"))
				err := json.Unmarshal(response.Body.Bytes(), &stats)
				Expect(err).NotTo(HaveOccurred())
				Expect(stats).To(ConsistOf(expectedLRPInstance))
			})

			It("logs the failure", func() {
				Expect(logger).To(Say("container-metrics-failed"))
			})

			Context("when the instance is crashing", func() {
				BeforeEach(func() {
					actualLRP.State = models.ActualLRPStateCrashed
					expectedLRPInstance.State = models.ActualLRPStateCrashed
				})

				It("response with empty stats", func() {
					var stats []cc_messages.LRPInstance
					Expect(response.Code).To(Equal(http.StatusOK))
					Expect(response.Header().Get("Content-Type")).To(Equal("application/json"))
					err := json.Unmarshal(response.Body.Bytes(), &stats)
					Expect(err).NotTo(HaveOccurred())
					Expect(stats).To(ConsistOf(expectedLRPInstance))
				})
			})
		})

		Context("when fetching the desiredLRP fails", func() {
			Context("when the desiredLRP is not found", func() {
				BeforeEach(func() {
					bbsClient.DesiredLRPByProcessGuidReturns(&models.DesiredLRP{}, models.ErrResourceNotFound)
				})

				It("responds with a 404", func() {
					Expect(response.Code).To(Equal(http.StatusNotFound))
				})
			})

			Context("when another type of error occurs", func() {
				BeforeEach(func() {
					bbsClient.DesiredLRPByProcessGuidReturns(&models.DesiredLRP{}, errors.New("garbage"))
				})

				It("responds with a 500", func() {
					Expect(response.Code).To(Equal(http.StatusInternalServerError))
				})
			})
		})

		Context("when fetching actualLRPs fails", func() {
			BeforeEach(func() {
				bbsClient.ActualLRPGroupsByProcessGuidReturns(nil, errors.New("bad stuff happened"))
			})

			It("responds with a 500", func() {
				Expect(response.Code).To(Equal(http.StatusInternalServerError))
			})

			It("logs the failure", func() {
				Expect(logger).To(Say("fetching-actual-lrp-info-failed"))
			})
		})
	})
})
