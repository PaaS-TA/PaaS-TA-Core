package cc_client_test

import (
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"

	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/stager/cc_client"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/ghttp"
)

var _ = Describe("CC Client", func() {
	var (
		fakeCC *ghttp.Server

		logger   lager.Logger
		ccClient cc_client.CcClient

		stagingGuid        string
		completionCallback string
	)

	BeforeEach(func() {
		fakeCC = ghttp.NewServer()

		logger = lager.NewLogger("fakelogger")
		logger.RegisterSink(lager.NewWriterSink(GinkgoWriter, lager.DEBUG))

		ccClient = cc_client.NewCcClient(fakeCC.URL(), "username", "password", true)

		stagingGuid = "the-staging-guid"
		completionCallback = ""
	})

	AfterEach(func() {
		if fakeCC.HTTPTestServer != nil {
			fakeCC.Close()
		}
	})

	Describe("Successfully calling the Cloud Controller", func() {
		It("calls the callback URL if it exists", func() {
			completionCallback = fmt.Sprintf("%s/awesome/potato/staging_complete", fakeCC.URL())

			fakeCC.AppendHandlers(
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("POST", "/awesome/potato/staging_complete"),
				),
			)

			err := ccClient.StagingComplete(stagingGuid, completionCallback, []byte(`{}`), logger)
			Expect(err).NotTo(HaveOccurred())
		})

		Context("When CC's staging request does not provide a callback URL", func() {
			var expectedBody = []byte(`{ "key": "value" }`)

			BeforeEach(func() {
				fakeCC.AppendHandlers(
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("POST", fmt.Sprintf("/internal/staging/%s/completed", stagingGuid)),
						ghttp.VerifyBasicAuth("username", "password"),
						ghttp.RespondWith(200, `{}`),
						func(w http.ResponseWriter, req *http.Request) {
							body, err := ioutil.ReadAll(req.Body)
							defer req.Body.Close()

							Expect(err).NotTo(HaveOccurred())
							Expect(body).To(Equal(expectedBody))
						},
					),
				)
			})

			It("sends the request payload to the CC without modification", func() {
				err := ccClient.StagingComplete(stagingGuid, completionCallback, expectedBody, logger)
				Expect(err).NotTo(HaveOccurred())
			})
		})
	})

	Describe("TLS certificate validation", func() {
		BeforeEach(func() {
			fakeCC = ghttp.NewTLSServer() // self-signed certificate
			fakeCC.AppendHandlers(
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("POST", fmt.Sprintf("/internal/staging/%s/completed", stagingGuid)),
					ghttp.VerifyBasicAuth("username", "password"),
					ghttp.RespondWith(200, `{}`),
				),
			)

			// muffle server-side log of certificate error
			fakeCC.HTTPTestServer.Config.ErrorLog = log.New(ioutil.Discard, "", log.Flags())
		})

		Context("when certificate verfication is enabled", func() {
			BeforeEach(func() {
				ccClient = cc_client.NewCcClient(fakeCC.URL(), "username", "password", false)
			})

			It("fails with a self-signed certificate", func() {
				err := ccClient.StagingComplete(stagingGuid, completionCallback, []byte(`{}`), logger)
				Expect(err).To(HaveOccurred())
			})
		})

		Context("when certificate verfication is disabled", func() {
			BeforeEach(func() {
				ccClient = cc_client.NewCcClient(fakeCC.URL(), "username", "password", true)
			})

			It("Attempts to validate SSL certificates", func() {
				err := ccClient.StagingComplete(stagingGuid, completionCallback, []byte(`{}`), logger)
				Expect(err).NotTo(HaveOccurred())
			})
		})
	})

	Describe("Error conditions", func() {
		Context("when the request couldn't be completed", func() {
			BeforeEach(func() {
				bogusURL := "http://0.0.0.0.0:80"
				ccClient = cc_client.NewCcClient(bogusURL, "username", "password", true)
			})

			It("percolates the error", func() {
				err := ccClient.StagingComplete(stagingGuid, completionCallback, []byte(`{}`), logger)
				Expect(err).To(HaveOccurred())
				Expect(err).To(BeAssignableToTypeOf(&url.Error{}))
			})
		})

		Context("when the response code is not StatusOK (200)", func() {
			BeforeEach(func() {
				fakeCC.AppendHandlers(
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("POST", fmt.Sprintf("/internal/staging/%s/completed", stagingGuid)),
						ghttp.RespondWith(500, `{}`),
					),
				)
			})

			It("returns an error with the actual status code", func() {
				err := ccClient.StagingComplete(stagingGuid, completionCallback, []byte(`{}`), logger)
				Expect(err).To(HaveOccurred())
				Expect(err).To(BeAssignableToTypeOf(&cc_client.BadResponseError{}))
				Expect(err.(*cc_client.BadResponseError).StatusCode).To(Equal(500))
			})
		})
	})
})

type testNetError struct {
	timeout   bool
	temporary bool
}

func (e *testNetError) Error() string   { return "test error" }
func (e *testNetError) Timeout() bool   { return e.timeout }
func (e *testNetError) Temporary() bool { return e.temporary }
