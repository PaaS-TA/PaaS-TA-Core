package handlers_test

import (
	"net/http"
	"net/http/httptest"

	"github.com/cloudfoundry-incubator/consul-release/src/acceptance-tests/testing/testconsumer/handlers"
	"github.com/onsi/gomega/gexec"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("dns", func() {
	Context("when ip addresses are bound to the service", func() {
		var pathToCheckARecord string

		BeforeEach(func() {
			var err error

			args := []string{
				"-ldflags",
				"-X main.Addresses=127.0.0.2,127.0.0.3,127.0.0.4 -X main.ServiceName=some-service",
			}

			pathToCheckARecord, err = gexec.Build("github.com/cloudfoundry-incubator/consul-release/src/acceptance-tests/testing/testconsumer/fakes/checkarecord", args...)
			Expect(err).NotTo(HaveOccurred())
		})

		It("returns an array of ip addresses", func() {
			request, err := http.NewRequest("GET", "/dns?service=some-service", nil)
			Expect(err).NotTo(HaveOccurred())

			handler := handlers.NewDNSHandler(pathToCheckARecord)
			recorder := httptest.NewRecorder()
			handler.ServeHTTP(recorder, request)

			Expect(recorder.Code).To(Equal(http.StatusOK))
			Expect(string(recorder.Body.Bytes())).To(Equal(`["127.0.0.2","127.0.0.3","127.0.0.4"]`))
		})
	})

	Context("when no ip addresses are bound to the service", func() {
		var pathToCheckARecord string

		BeforeEach(func() {
			var err error

			args := []string{
				"-ldflags",
				"-X main.Addresses= -X main.ServiceName=some-service",
			}

			pathToCheckARecord, err = gexec.Build("github.com/cloudfoundry-incubator/consul-release/src/acceptance-tests/testing/testconsumer/fakes/checkarecord", args...)
			Expect(err).NotTo(HaveOccurred())
		})

		It("returns an empty array", func() {
			request, err := http.NewRequest("GET", "/dns?service=some-service", nil)
			Expect(err).NotTo(HaveOccurred())

			handler := handlers.NewDNSHandler(pathToCheckARecord)
			recorder := httptest.NewRecorder()
			handler.ServeHTTP(recorder, request)

			Expect(recorder.Code).To(Equal(http.StatusOK))
			Expect(string(recorder.Body.Bytes())).To(Equal(`[]`))
		})
	})

	Context("failure cases", func() {
		It("returns a 500 when the check-a-record process does not run successfully", func() {
			request, err := http.NewRequest("GET", "/dns?service=some-service", nil)
			Expect(err).NotTo(HaveOccurred())

			handler := handlers.NewDNSHandler("/some/fake/process")
			recorder := httptest.NewRecorder()
			handler.ServeHTTP(recorder, request)

			Expect(recorder.Code).To(Equal(http.StatusInternalServerError))
			Expect(string(recorder.Body.Bytes())).To(Equal("fork/exec /some/fake/process: no such file or directory"))
		})

		It("returns a 400 when no service has been provided", func() {
			request, err := http.NewRequest("GET", "/dns", nil)
			Expect(err).NotTo(HaveOccurred())

			handler := handlers.NewDNSHandler("")
			recorder := httptest.NewRecorder()
			handler.ServeHTTP(recorder, request)

			Expect(recorder.Code).To(Equal(http.StatusBadRequest))
			Expect(string(recorder.Body.Bytes())).To(Equal("service is a required parameter"))
		})
	})
})
