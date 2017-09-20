package handlers_test

import (
	"bytes"
	"errors"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"net/url"

	executorfakes "code.cloudfoundry.org/executor/fakes"
	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/lager/lagertest"
	"code.cloudfoundry.org/rep/handlers"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("StopLRPInstanceHandler", func() {
	var (
		stopInstanceHandler *handlers.StopLRPInstanceHandler
		fakeClient          *executorfakes.FakeClient
		resp                *httptest.ResponseRecorder
		req                 *http.Request
		logger              *lagertest.TestLogger
	)

	BeforeEach(func() {
		var err error
		fakeClient = &executorfakes.FakeClient{}

		logger = lagertest.NewTestLogger("test")
		logger.RegisterSink(lager.NewWriterSink(GinkgoWriter, lager.DEBUG))

		stopInstanceHandler = handlers.NewStopLRPInstanceHandler(fakeClient)

		resp = httptest.NewRecorder()

		req, err = http.NewRequest("POST", "", nil)
		Expect(err).NotTo(HaveOccurred())
	})

	JustBeforeEach(func() {
		stopInstanceHandler.ServeHTTP(resp, req, logger)
	})

	Context("when the request is valid", func() {
		var processGuid string
		var instanceGuid string

		BeforeEach(func() {
			processGuid = "process-guid"
			instanceGuid = "instance-guid"

			values := make(url.Values)
			values.Set(":process_guid", processGuid)
			values.Set(":instance_guid", instanceGuid)
			req.URL.RawQuery = values.Encode()
		})

		Context("and StopContainer succeeds", func() {
			It("responds with 202 Accepted", func() {
				Expect(resp.Code).To(Equal(http.StatusAccepted))
			})

			It("eventually stops the instance", func() {
				Eventually(fakeClient.StopContainerCallCount).Should(Equal(1))

				processGuid, instanceGuid := fakeClient.StopContainerArgsForCall(0)
				Expect(processGuid).To(Equal(processGuid))
				Expect(instanceGuid).To(Equal(instanceGuid))
			})
		})

		Context("but StopContainer fails", func() {
			BeforeEach(func() {
				fakeClient.StopContainerReturns(errors.New("fail"))
			})

			It("responds with 500 Internal Server Error", func() {
				Expect(resp.Code).To(Equal(http.StatusInternalServerError))
			})
		})
	})

	Context("when the request is invalid", func() {
		BeforeEach(func() {
			req.Body = ioutil.NopCloser(bytes.NewBufferString("foo"))
		})

		It("responds with 400 Bad Request", func() {
			Expect(resp.Code).To(Equal(http.StatusBadRequest))
		})

		It("does not attempt to stop the instance", func() {
			Expect(fakeClient.StopContainerCallCount()).To(Equal(0))
		})
	})
})
