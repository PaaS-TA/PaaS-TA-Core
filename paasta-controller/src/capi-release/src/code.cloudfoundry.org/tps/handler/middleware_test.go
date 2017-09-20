package handler_test

import (
	"net/http"
	"net/http/httptest"

	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/lager/lagertest"
	"code.cloudfoundry.org/tps/handler"
	"code.cloudfoundry.org/tps/handler/handler_fakes"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
)

var _ = Describe("Middleware", func() {
	var httpHandler http.Handler
	var wrappedHandler *handler_fakes.FakeHandler
	var req *http.Request
	var res *httptest.ResponseRecorder
	var logger lager.Logger

	BeforeEach(func() {
		req = newTestRequest("")
		res = httptest.NewRecorder()
		wrappedHandler = new(handler_fakes.FakeHandler)
		logger = lagertest.NewTestLogger("test")
	})

	Describe("LogWrap", func() {
		BeforeEach(func() {
			httpHandler = handler.LogWrap(wrappedHandler, logger)
		})

		Context("when the handler serves request", func() {
			BeforeEach(func() {
				httpHandler.ServeHTTP(res, req)
			})

			It("calls the wrapped handler", func() {
				Expect(wrappedHandler.ServeHTTPCallCount()).To(Equal(1))
			})

			It("logs before serving", func() {
				Expect(logger).To(gbytes.Say("serving"))
			})

			It("logs after serving", func() {
				Expect(logger).To(gbytes.Say("done"))
			})
		})
	})
})
