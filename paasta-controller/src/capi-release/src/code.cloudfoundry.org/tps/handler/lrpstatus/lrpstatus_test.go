package lrpstatus_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"time"

	"code.cloudfoundry.org/bbs/fake_bbs"
	"code.cloudfoundry.org/bbs/models"
	"code.cloudfoundry.org/bbs/models/test/model_helpers"
	"code.cloudfoundry.org/clock/fakeclock"
	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/lager/lagertest"
	"code.cloudfoundry.org/runtimeschema/cc_messages"
	"code.cloudfoundry.org/stager/diego_errors"
	"code.cloudfoundry.org/tps/handler/lrpstatus"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("LRPStatus", func() {
	var (
		fakeClient *fake_bbs.FakeClient

		server *httptest.Server
	)

	BeforeEach(func() {
		fakeClient = new(fake_bbs.FakeClient)
		fakeClock := fakeclock.NewFakeClock(time.Now())

		handler := lrpstatus.NewHandler(fakeClient, fakeClock, lagertest.NewTestLogger("test"))
		server = httptest.NewServer(handler)
	})

	AfterEach(func() {
		server.Close()
	})

	Describe("Instance state", func() {
		BeforeEach(func() {
			fakeClient.ActualLRPGroupsByProcessGuidStub = func(lager.Logger, string) ([]*models.ActualLRPGroup, error) {
				return []*models.ActualLRPGroup{
					makeActualLRPGroup(1, models.ActualLRPStateUnclaimed, ""),
					makeActualLRPGroup(2, models.ActualLRPStateClaimed, ""),
					makeActualLRPGroup(3, models.ActualLRPStateRunning, ""),
					makeActualLRPGroup(4, models.ActualLRPStateCrashed, diego_errors.CELL_MISMATCH_MESSAGE),
				}, nil
			}
		})

		It("returns instance state", func() {
			res, err := http.Get(server.URL)
			Expect(err).NotTo(HaveOccurred())

			response := []cc_messages.LRPInstance{}
			err = json.NewDecoder(res.Body).Decode(&response)
			res.Body.Close()
			Expect(err).NotTo(HaveOccurred())

			Expect(response).To(HaveLen(4))
			Expect(response[0].State).To(Equal(cc_messages.LRPInstanceStateStarting))
			Expect(response[1].State).To(Equal(cc_messages.LRPInstanceStateStarting))
			Expect(response[2].State).To(Equal(cc_messages.LRPInstanceStateRunning))
			Expect(response[3].State).To(Equal(cc_messages.LRPInstanceStateCrashed))
			Expect(response[3].Details).To(Equal(diego_errors.CELL_MISMATCH_MESSAGE))
		})
	})
})

func makeActualLRPGroup(index int32, state string, placementError string) *models.ActualLRPGroup {
	actual := model_helpers.NewValidActualLRP("guid", index)
	actual.PlacementError = placementError
	actual.State = state

	return &models.ActualLRPGroup{Instance: actual}
}
