package handlers_test

import (
	"errors"
	"net/http"

	"code.cloudfoundry.org/rep"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("State", func() {
	Context("when the state call succeeds", func() {
		var repState rep.CellState
		BeforeEach(func() {
			repState = rep.CellState{
				RootFSProviders: rep.RootFSProviders{"docker": rep.ArbitraryRootFSProvider{}},
			}
			fakeLocalRep.StateReturns(repState, nil)
			Expect(fakeLocalRep.StateCallCount()).To(Equal(0))
		})

		It("it returns whatever the state call returns", func() {
			status, body := Request(rep.StateRoute, nil, nil)
			Expect(status).To(Equal(http.StatusOK))

			Expect(body).To(MatchJSON(JSONFor(repState)))

			Expect(fakeLocalRep.StateCallCount()).To(Equal(1))
		})
	})

	Context("when the state call fails", func() {
		It("fails", func() {
			fakeLocalRep.StateReturns(rep.CellState{}, errors.New("boom"))
			Expect(fakeLocalRep.StateCallCount()).To(Equal(0))

			status, body := Request(rep.StateRoute, nil, nil)
			Expect(status).To(Equal(http.StatusInternalServerError))
			Expect(body).To(BeEmpty())

			Expect(fakeLocalRep.StateCallCount()).To(Equal(1))
		})
	})
})
