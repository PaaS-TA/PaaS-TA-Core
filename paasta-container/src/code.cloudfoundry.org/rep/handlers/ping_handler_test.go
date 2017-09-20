package handlers_test

import (
	"net/http"
	"net/http/httptest"

	"code.cloudfoundry.org/lager/lagertest"
	"code.cloudfoundry.org/rep/handlers"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("PingHandler", func() {
	var (
		pingHandler *handlers.PingHandler
		resp        *httptest.ResponseRecorder
		req         *http.Request
	)

	JustBeforeEach(func() {
		logger := lagertest.NewTestLogger("ping-handler")
		pingHandler = handlers.NewPingHandler()
		resp = httptest.NewRecorder()

		var err error
		req, err = http.NewRequest("GET", "/ping", nil)
		Expect(err).NotTo(HaveOccurred())

		pingHandler.ServeHTTP(resp, req, logger)
	})

	It("responds with 200 OK", func() {
		Expect(resp.Code).To(Equal(http.StatusOK))
	})
})
