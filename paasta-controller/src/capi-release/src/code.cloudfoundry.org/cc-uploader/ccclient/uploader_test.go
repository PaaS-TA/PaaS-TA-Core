package ccclient_test

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"net/url"

	"code.cloudfoundry.org/cc-uploader/ccclient"
	"code.cloudfoundry.org/cc-uploader/ccclient/test_helpers"
	"code.cloudfoundry.org/lager/lagertest"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Uploader", func() {
	var (
		u         ccclient.Uploader
		transport http.RoundTripper
	)

	Describe("Upload", func() {
		var (
			response  *http.Response
			uploadErr error

			uploadURL       *url.URL
			filename        string
			incomingRequest *http.Request
		)

		BeforeEach(func() {
			uploadURL, _ = url.Parse("http://example.com")
			filename = "filename"
			incomingRequest = createValidRequest()
		})

		Context("when not cancelling", func() {
			JustBeforeEach(func() {
				httpClient := &http.Client{
					Transport: transport,
				}

				u = ccclient.NewUploader(lagertest.NewTestLogger("test"), httpClient)
				fmt.Fprintf(GinkgoWriter, "Uploading to URL %s\n", uploadURL.String())
				response, uploadErr = u.Upload(uploadURL, filename, incomingRequest, make(chan struct{}))
			})

			Context("Validating the content length of the request", func() {
				BeforeEach(func() {
					transport = http.DefaultTransport
					filename = "filename"
					incomingRequest = &http.Request{}
				})

				It("fails early if the content length is 0", func() {
					Expect(response.StatusCode).To(Equal(http.StatusLengthRequired))

					Expect(uploadErr).To(HaveOccurred())
				})
			})

			Context("When it can create a valid multipart request to the upload URL", func() {
				var uploadRequestChan chan *http.Request

				BeforeEach(func() {
					uploadRequestChan = make(chan *http.Request, 3)
					transport = test_helpers.NewFakeRoundTripper(
						uploadRequestChan,
						map[string]test_helpers.RespErrorPair{
							"example.com": {responseWithCode(http.StatusOK), nil},
						},
					)
				})

				It("Makes an upload request using that multipart request", func() {
					var uploadRequest *http.Request
					Eventually(uploadRequestChan).Should(Receive(&uploadRequest))
					Expect(uploadRequest.Header.Get("Content-Type")).To(ContainSubstring("multipart/form-data; boundary="))
				})

				It("Forwards Content-MD5 header onto the upload request", func() {
					var uploadRequest *http.Request
					Eventually(uploadRequestChan).Should(Receive(&uploadRequest))
					Expect(uploadRequest.Header.Get("Content-MD5")).To(Equal("the-md5"))
				})

				Context("When the upload URL has basic auth credentials", func() {
					BeforeEach(func() {
						uploadURL.User = url.UserPassword("bob", "cobb")
					})

					It("Forwards the basic auth credentials", func() {
						var uploadRequest *http.Request
						Eventually(uploadRequestChan).Should(Receive(&uploadRequest))
						Expect(uploadRequest.URL.User).To(Equal(url.UserPassword("bob", "cobb")))
					})
				})

				Context("When uploading to the upload URL succeeds", func() {
					It("Returns the respons, and no error", func() {
						Expect(response).To(Equal(responseWithCode(http.StatusOK))) // assumes (*http.Client).do doesn't modify the response from the roundtripper
						Expect(uploadErr).NotTo(HaveOccurred())
					})
				})

				Context("Whenuploading to the upload URL fails with a dial error", func() {
					BeforeEach(func() {
						transport = test_helpers.NewFakeRoundTripper(
							uploadRequestChan,
							map[string]test_helpers.RespErrorPair{
								"example.com": {nil, &net.OpError{Op: "dial"}},
							},
						)
					})

					It("Retries the upload three times", func() {
						for i := 0; i < 3; i++ {
							var uploadRequest *http.Request
							Eventually(uploadRequestChan).Should(Receive(&uploadRequest))
							Expect(uploadRequest.URL).To(Equal(uploadURL))
						}
					})

					It("Returns the network error", func() {
						Expect(uploadErr).To(HaveOccurred())

						urlErr, ok := uploadErr.(*url.Error)
						Expect(ok).To(BeTrue())

						Expect(urlErr.Err).To(Equal(&net.OpError{Op: "dial"}))
					})
				})

				Context("When the request to the upload URL fails with something other than a dial error", func() {
					BeforeEach(func() {
						transport = test_helpers.NewFakeRoundTripper(
							uploadRequestChan,
							map[string]test_helpers.RespErrorPair{
								"example.com": {nil, &net.OpError{Op: "not-dial"}},
							},
						)
					})

					It("Returns the network error", func() {
						Expect(uploadErr).To(HaveOccurred())

						urlErr, ok := uploadErr.(*url.Error)
						Expect(ok).To(BeTrue())

						Expect(urlErr.Err).To(Equal(&net.OpError{Op: "not-dial"}))
					})
				})

				Context("When request to the upload URL fails due to a bad response", func() {
					BeforeEach(func() {
						transport = test_helpers.NewFakeRoundTripper(
							uploadRequestChan,
							map[string]test_helpers.RespErrorPair{
								"example.com": {responseWithCode(http.StatusUnauthorized), nil},
							},
						)
					})

					It("Returns the response", func() {
						Expect(response).To(Equal(responseWithCode(http.StatusUnauthorized))) // assumes (*http.Client).do doesn't modify the response from the roundtripper
					})
				})
			})
		})

		Context("when cancelling uploads", func() {
			var cancelChan chan struct{}
			var uploadCompleted chan struct{}
			var uploadRequestChan chan *http.Request

			BeforeEach(func() {
				cancelChan = make(chan struct{})
				uploadRequestChan = make(chan *http.Request)
				transport = test_helpers.NewFakeRoundTripper(
					uploadRequestChan,
					map[string]test_helpers.RespErrorPair{
						"example.com": test_helpers.RespErrorPair{responseWithCode(http.StatusOK), nil},
					},
				)
			})

			JustBeforeEach(func() {
				uploadCompleted = make(chan struct{})

				go func() {
					httpClient := &http.Client{
						Transport: transport,
					}
					u = ccclient.NewUploader(lagertest.NewTestLogger("test"), httpClient)
					response, uploadErr = u.Upload(uploadURL, filename, incomingRequest, cancelChan)
					close(uploadCompleted)
				}()
			})

			It("will fail with the uploadURL", func() {
				Consistently(uploadCompleted).ShouldNot(BeClosed())
				close(cancelChan)

				Eventually(uploadCompleted).Should(BeClosed())
				Expect(uploadErr).To(HaveOccurred())
			})
		})
	})
})

func createValidRequest() *http.Request {
	buffer := bytes.NewBufferString("file-upload-contents")
	request, err := http.NewRequest("POST", "", buffer)
	Expect(err).NotTo(HaveOccurred())

	request.Header.Set("Content-MD5", "the-md5")
	request.Body = ioutil.NopCloser(bytes.NewBufferString(""))

	fmt.Fprintf(GinkgoWriter, "Content-length %d\n", request.ContentLength)

	return request
}

func responseWithCode(code int) *http.Response {
	return &http.Response{StatusCode: code, Body: ioutil.NopCloser(bytes.NewBufferString(""))}
}
