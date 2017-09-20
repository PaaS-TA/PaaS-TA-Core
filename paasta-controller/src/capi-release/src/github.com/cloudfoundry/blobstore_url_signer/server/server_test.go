package server_test

import (
	"net/http"

	"github.com/cloudfoundry/blobstore_url_signer/server"
	"github.com/cloudfoundry/blobstore_url_signer/server/fakes"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Server", func() {
	var (
		fakeHandler *fakes.FakeServerHandlers
		s           server.Server
	)

	BeforeEach(func() {
		fakeHandler = &fakes.FakeServerHandlers{}
		s = server.NewServer("tcp", ":8080", fakeHandler)
		go s.Start()
	})

	AfterEach(func() {
		s.Stop()
	})

	It("serves with the SignUrl handler", func() {
		_, err := http.Get("http://127.0.0.1:8080")

		Expect(err).ToNot(HaveOccurred())
		Expect(fakeHandler.SignUrlCallCount()).To(Equal(1))
	})
})
