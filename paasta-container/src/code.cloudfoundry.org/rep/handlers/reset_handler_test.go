package handlers_test

import (
	"errors"
	"net/http"

	"code.cloudfoundry.org/rep"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Reset Handler", func() {
	Describe("Reset", func() {
		Context("when the reset succeeds", func() {
			It("succeeds", func() {
				Expect(fakeLocalRep.ResetCallCount()).To(Equal(0))

				status, body := Request(rep.Sim_ResetRoute, nil, nil)
				Expect(status).To(Equal(http.StatusOK))
				Expect(body).To(BeEmpty())

				Expect(fakeLocalRep.ResetCallCount()).To(Equal(1))
			})
		})

		Context("when the reset fails", func() {
			It("fails", func() {
				fakeLocalRep.ResetReturns(errors.New("boom"))
				Expect(fakeLocalRep.ResetCallCount()).To(Equal(0))

				status, body := Request(rep.Sim_ResetRoute, nil, nil)
				Expect(status).To(Equal(http.StatusInternalServerError))
				Expect(body).To(BeEmpty())

				Expect(fakeLocalRep.ResetCallCount()).To(Equal(1))
			})
		})
	})
})
