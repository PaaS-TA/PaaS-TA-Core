package handlers_test

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"

	"github.com/cloudfoundry-incubator/consul-release/src/acceptance-tests/testing/testconsumer/handlers"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("HealthCheckHandler", func() {
	It("returns a 200 if the state is good", func() {
		request, err := http.NewRequest("GET", "/health_check", strings.NewReader(""))
		Expect(err).NotTo(HaveOccurred())

		handler := handlers.NewHealthCheckHandler()
		recorder := httptest.NewRecorder()
		handler.ServeHTTP(recorder, request)

		Expect(recorder.Code).To(Equal(http.StatusOK))
	})

	It("returns a 405 when an unknown method has been received", func() {
		request, err := http.NewRequest("DELETE", "/health_check", strings.NewReader(""))
		Expect(err).NotTo(HaveOccurred())

		handler := handlers.NewHealthCheckHandler()
		recorder := httptest.NewRecorder()
		handler.ServeHTTP(recorder, request)

		Expect(recorder.Code).To(Equal(http.StatusMethodNotAllowed))
	})

	It("returns a 500 when a bad read occurs", func() {
		request, err := http.NewRequest("POST", "/health_check", badReader{})
		Expect(err).NotTo(HaveOccurred())

		handler := handlers.NewHealthCheckHandler()
		recorder := httptest.NewRecorder()
		handler.ServeHTTP(recorder, request)

		Expect(recorder.Code).To(Equal(http.StatusInternalServerError))
	})

	It("returns a 503 if the state is bad", func() {
		handler := handlers.NewHealthCheckHandler()
		By("checking the initial state", func() {
			request, err := http.NewRequest("GET", "/health_check", strings.NewReader(""))
			Expect(err).NotTo(HaveOccurred())

			recorder := httptest.NewRecorder()
			handler.ServeHTTP(recorder, request)

			Expect(recorder.Code).To(Equal(http.StatusOK))
		})

		By("setting the check to a failure mode", func() {
			request, err := http.NewRequest("POST", "/health_check", strings.NewReader("false"))
			Expect(err).NotTo(HaveOccurred())

			recorder := httptest.NewRecorder()
			handler.ServeHTTP(recorder, request)

			Expect(recorder.Code).To(Equal(http.StatusOK))
		})

		By("returning a 503 error code", func() {
			request, err := http.NewRequest("GET", "/health_check", strings.NewReader(""))
			Expect(err).NotTo(HaveOccurred())

			recorder := httptest.NewRecorder()
			handler.ServeHTTP(recorder, request)

			Expect(recorder.Code).To(Equal(http.StatusServiceUnavailable))
		})

		By("setting the check to a success mode", func() {
			request, err := http.NewRequest("POST", "/health_check", strings.NewReader("true"))
			Expect(err).NotTo(HaveOccurred())

			recorder := httptest.NewRecorder()
			handler.ServeHTTP(recorder, request)

			Expect(recorder.Code).To(Equal(http.StatusOK))
		})

		By("returning a 200 error code", func() {
			request, err := http.NewRequest("GET", "/health_check", strings.NewReader(""))
			Expect(err).NotTo(HaveOccurred())

			recorder := httptest.NewRecorder()
			handler.ServeHTTP(recorder, request)

			Expect(recorder.Code).To(Equal(http.StatusOK))
		})
	})
})

type badReader struct{}

func (badReader) Read([]byte) (int, error) {
	return 0, errors.New("failed to read")
}
