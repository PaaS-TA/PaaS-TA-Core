package buffered_test

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"strconv"

	"github.com/cloudfoundry-incubator/consul-release/src/acceptance-tests/testing/testconsumer/buffered"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ http.ResponseWriter = buffered.NewResponseWriter(nil, nil)

var _ = Describe("ResponseBuffer", func() {
	Describe("Copy", func() {
		var (
			bufferedWriter *buffered.ResponseWriter
			actualWriter   *httptest.ResponseRecorder
			logBuffer      *bytes.Buffer
		)

		BeforeEach(func() {
			logBuffer = bytes.NewBuffer([]byte{})
			actualWriter = httptest.NewRecorder()
			bufferedWriter = buffered.NewResponseWriter(actualWriter, logBuffer)
		})

		It("copies all the headers to the real response writer", func() {
			bufferedWriter.Header().Set("Content-Length", "0")
			bufferedWriter.Header().Set("Content-Type", "application/json")

			bufferedWriter.Copy()

			Expect(actualWriter.Header()).To(Equal(http.Header{
				"Content-Type":   []string{"application/json"},
				"Content-Length": []string{"0"},
			}))
		})

		It("copies the status code to the real status code", func() {
			bufferedWriter.WriteHeader(http.StatusTeapot)

			bufferedWriter.Copy()

			Expect(actualWriter.Code).To(Equal(http.StatusTeapot))
		})

		It("copies the body to the real body", func() {
			bufferedWriter.Write([]byte("some-data"))

			bufferedWriter.Copy()

			Expect(actualWriter.Body.String()).To(Equal("some-data"))
			Expect(actualWriter.Header().Get("Content-Length")).To(Equal(strconv.Itoa(len("some-data"))))
		})

		It("prepends the logbuffer into the body for 500 errors", func() {
			logBuffer.Write([]byte("some-log-data\n"))

			bufferedWriter.WriteHeader(http.StatusInternalServerError)
			bufferedWriter.Write([]byte("some-data"))

			bufferedWriter.Copy()

			Expect(actualWriter.Body.String()).To(Equal("some-log-data some-data"))

			contentLength := len("some-log-data") + len(" ") + len("some-data")

			Expect(actualWriter.Header().Get("Content-Length")).To(Equal(strconv.Itoa(contentLength)))
		})
	})
})
