package bulklrpstatus_test

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"time"

	"code.cloudfoundry.org/bbs/fake_bbs"
	"code.cloudfoundry.org/bbs/models"
	"code.cloudfoundry.org/clock/fakeclock"
	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/lager/lagertest"
	"code.cloudfoundry.org/nsync/recipebuilder"
	"code.cloudfoundry.org/runtimeschema/cc_messages"
	"code.cloudfoundry.org/tps/handler/bulklrpstatus"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gbytes"
)

var _ = Describe("Bulk Status", func() {
	const authorization = "something good"
	const guid1 = "my-guid1"
	const guid2 = "my-guid2"
	const logGuid1 = "log-guid1"
	const logGuid2 = "log-guid2"

	var (
		handler   http.Handler
		response  *httptest.ResponseRecorder
		request   *http.Request
		bbsClient *fake_bbs.FakeClient
		logger    *lagertest.TestLogger
		fakeClock *fakeclock.FakeClock
	)

	BeforeEach(func() {
		var err error

		bbsClient = new(fake_bbs.FakeClient)
		logger = lagertest.NewTestLogger("test")
		fakeClock = fakeclock.NewFakeClock(time.Date(2008, 8, 8, 8, 8, 8, 8, time.UTC))
		handler = bulklrpstatus.NewHandler(bbsClient, fakeClock, 15, logger)
		response = httptest.NewRecorder()
		url := "/v1/bulk_actual_lrp_status"
		request, err = http.NewRequest("GET", url, nil)
		Expect(err).NotTo(HaveOccurred())
	})

	JustBeforeEach(func() {
		handler.ServeHTTP(response, request)
	})

	Describe("Validation", func() {
		BeforeEach(func() {
			request.Header.Set("Authorization", authorization)
		})

		Context("with no process guids", func() {
			It("fails with missing process guids", func() {
				Expect(response.Code).To(Equal(http.StatusBadRequest))
			})
		})

		Context("with malformed process guids", func() {
			BeforeEach(func() {
				query := request.URL.Query()
				query.Set("guids", fmt.Sprintf("%s,,%s", guid1, guid2))
				request.URL.RawQuery = query.Encode()
			})

			It("fails", func() {
				Expect(response.Code).To(Equal(http.StatusBadRequest))
			})
		})
	})

	Describe("retrieves instance state for lrps specified", func() {
		var (
			expectedSinceTime, actualSinceTime int64
			netInfo1, netInfo2                 models.ActualLRPNetInfo
		)

		BeforeEach(func() {
			expectedSinceTime = fakeClock.Now().Unix()
			actualSinceTime = fakeClock.Now().UnixNano()
			fakeClock.Increment(5 * time.Second)

			netInfo1 = models.NewActualLRPNetInfo(
				"host",
				models.NewPortMapping(5432, 7890),
				models.NewPortMapping(1234, uint32(recipebuilder.DefaultPort)),
			)
			netInfo2 = models.NewActualLRPNetInfo(
				"host2",
				models.NewPortMapping(5432, 7890),
				models.NewPortMapping(1234, uint32(recipebuilder.DefaultPort)),
			)

			request.Header.Set("Authorization", authorization)

			query := request.URL.Query()
			query.Set("guids", fmt.Sprintf("%s,%s", guid1, guid2))
			request.URL.RawQuery = query.Encode()

			bbsClient.ActualLRPGroupsByProcessGuidStub = func(logger lager.Logger, processGuid string) ([]*models.ActualLRPGroup, error) {
				switch processGuid {

				case guid1:
					actualLRP := &models.ActualLRP{
						ActualLRPKey:         models.NewActualLRPKey(processGuid, 5, "some-domain"),
						ActualLRPInstanceKey: models.NewActualLRPInstanceKey("instanceId", "some-cell"),
						ActualLRPNetInfo:     netInfo1,
						State:                models.ActualLRPStateRunning,
						Since:                actualSinceTime,
					}
					return []*models.ActualLRPGroup{{Instance: actualLRP}}, nil

				case guid2:
					actualLRP := &models.ActualLRP{
						ActualLRPKey:         models.NewActualLRPKey(processGuid, 6, "some-domain"),
						ActualLRPInstanceKey: models.NewActualLRPInstanceKey("instanceId", "some-cell"),
						ActualLRPNetInfo:     netInfo2,
						State:                models.ActualLRPStateRunning,
						Since:                actualSinceTime,
					}
					return []*models.ActualLRPGroup{{Instance: actualLRP}}, nil

				default:
					return nil, errors.New("WHAT?")
				}
			}
		})

		Context("when the LRPs have been running for a while", func() {
			It("returns a map of status per index", func() {
				expectedLRPInstance1 := cc_messages.LRPInstance{
					ProcessGuid:  guid1,
					InstanceGuid: "instanceId",
					NetInfo:      netInfo1,
					Index:        5,
					State:        cc_messages.LRPInstanceStateRunning,
					Since:        expectedSinceTime,
					Uptime:       5,
				}
				expectedLRPInstance2 := cc_messages.LRPInstance{
					ProcessGuid:  guid2,
					InstanceGuid: "instanceId",
					NetInfo:      netInfo2,
					Index:        6,
					State:        cc_messages.LRPInstanceStateRunning,
					Since:        expectedSinceTime,
					Uptime:       5,
				}

				status := make(map[string][]cc_messages.LRPInstance)

				Expect(response.Code).To(Equal(http.StatusOK))
				Expect(response.Header().Get("Content-Type")).To(Equal("application/json"))

				err := json.Unmarshal(response.Body.Bytes(), &status)
				Expect(err).NotTo(HaveOccurred())

				Expect(status[guid1][0]).To(Equal(expectedLRPInstance1))
				Expect(status[guid2][0]).To(Equal(expectedLRPInstance2))
			})
		})

		Context("when fetching one of the actualLRPs fails", func() {
			BeforeEach(func() {
				bbsClient.ActualLRPGroupsByProcessGuidStub = func(logger lager.Logger, processGuid string) ([]*models.ActualLRPGroup, error) {
					switch processGuid {

					case guid1:
						actualLRP := &models.ActualLRP{
							ActualLRPKey:         models.NewActualLRPKey(processGuid, 5, "some-domain"),
							ActualLRPInstanceKey: models.NewActualLRPInstanceKey("instanceId", "some-cell"),
							ActualLRPNetInfo: models.NewActualLRPNetInfo(
								"host",
								models.NewPortMapping(5432, 7890),
								models.NewPortMapping(1234, uint32(recipebuilder.DefaultPort)),
							),
							State: models.ActualLRPStateRunning,
							Since: actualSinceTime,
						}
						return []*models.ActualLRPGroup{{Instance: actualLRP}}, nil

					case guid2:
						return nil, errors.New("boom")

					default:
						return nil, errors.New("UNEXPECTED GUID YO")
					}
				}
			})

			It("it is excluded from the result and logs the failure", func() {
				status := make(map[string][]cc_messages.LRPInstance)

				Expect(response.Code).To(Equal(http.StatusOK))
				Expect(response.Header().Get("Content-Type")).To(Equal("application/json"))

				err := json.Unmarshal(response.Body.Bytes(), &status)
				Expect(err).NotTo(HaveOccurred())

				Expect(len(status)).To(Equal(1))
				Expect(status[guid2]).To(BeNil())
				Expect(logger).To(Say("fetching-actual-lrps-info-failed"))
			})
		})
	})
})
