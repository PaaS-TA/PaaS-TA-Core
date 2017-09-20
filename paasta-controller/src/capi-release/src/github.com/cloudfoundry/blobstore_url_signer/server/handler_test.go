package server_test

import (
	"net/http"
	"net/http/httptest"

	"github.com/cloudfoundry/blobstore_url_signer/server"
	"github.com/cloudfoundry/blobstore_url_signer/signer/fakes"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("handlers", func() {

	var (
		fakeSigner    *fakes.FakeSigner
		serverHandler server.ServerHandlers
		resp          *httptest.ResponseRecorder
		request       *http.Request
		err           error
	)

	BeforeEach(func() {
		fakeSigner = &fakes.FakeSigner{}
		serverHandler = server.NewServerHandlers(fakeSigner)
		resp = httptest.NewRecorder()
		request, err = http.NewRequest("GET", "http://127.0.0.1:8080/sign?expires=123123&secret=topSecret&path=1c/9a/3234-sdfs", nil)
		Expect(err).ToNot(HaveOccurred())
	})

	Describe("SignUrl()", func() {
		It("calls the signer to sign the url", func() {
			serverHandler.SignUrl(resp, request)
			Expect(fakeSigner.SignCallCount()).To(Equal(1))
		})

		It("sends the signer the correct params", func() {
			serverHandler.SignUrl(resp, request)
			expire, path := fakeSigner.SignArgsForCall(0)
			Expect(expire).To(Equal("123123"))
			Expect(path).To(Equal("1c/9a/3234-sdfs"))
		})

		It("writes the signed URL back to requester", func() {
			fakeSigner.SignReturns("/link/?md5=signedurl")
			serverHandler.SignUrl(resp, request)
			Expect(resp.Body.String()).To(ContainSubstring("/link/?md5=signedurl"))
		})
	})
})
