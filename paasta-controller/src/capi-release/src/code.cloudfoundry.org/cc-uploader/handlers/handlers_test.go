package handlers_test

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"time"

	"code.cloudfoundry.org/cc-uploader/ccclient"
	"code.cloudfoundry.org/cc-uploader/handlers"
	"code.cloudfoundry.org/lager/lagertest"
	"code.cloudfoundry.org/runtimeschema/cc_messages"
	"code.cloudfoundry.org/urljoiner"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/ghttp"
)

var _ = Describe("Handlers", func() {
	var (
		logger *lagertest.TestLogger

		fakeCloudController *ghttp.Server
		uploadURL           *url.URL

		incomingRequest  *http.Request
		outgoingResponse *httptest.ResponseRecorder

		handler http.Handler

		postStatusCode   int
		postResponseBody string
		uploadedBytes    []byte
		uploadedFileName string
		uploadedHeaders  http.Header
	)

	BeforeEach(func() {
		var err error

		logger = lagertest.NewTestLogger("test")

		buffer := bytes.NewBufferString("the file I'm uploading")
		incomingRequest, err = http.NewRequest("POST", "", buffer)
		Expect(err).NotTo(HaveOccurred())
		incomingRequest.Header.Set("Content-MD5", "the-md5")

		fakeCloudController = ghttp.NewServer()

		uploader := ccclient.NewUploader(logger, http.DefaultClient)
		poller := ccclient.NewPoller(logger, http.DefaultClient, 100*time.Millisecond)

		handler, err = handlers.New(uploader, poller, logger)
		Expect(err).NotTo(HaveOccurred())

		postStatusCode = http.StatusCreated
		uploadedBytes = nil
		uploadedFileName = ""
		uploadedHeaders = nil
	})

	AfterEach(func() {
		fakeCloudController.Close()
	})

	Describe("UploadDroplet", func() {
		var (
			timeClicker chan time.Time
			startTime   time.Time
			endTime     time.Time
		)

		BeforeEach(func() {
			var err error

			timeClicker = make(chan time.Time, 4)
			fakeCloudController.AppendHandlers(ghttp.CombineHandlers(
				ghttp.VerifyRequest("POST", "/staging/droplet/app-guid/upload"),
				ghttp.VerifyBasicAuth("bob", "password"),
				ghttp.RespondWithPtr(&postStatusCode, &postResponseBody),
				func(w http.ResponseWriter, r *http.Request) {
					uploadedHeaders = r.Header
					file, fileHeader, err := r.FormFile(ccclient.FormField)
					Expect(err).NotTo(HaveOccurred())
					uploadedBytes, err = ioutil.ReadAll(file)
					Expect(err).NotTo(HaveOccurred())
					uploadedFileName = fileHeader.Filename
					Expect(r.ContentLength).To(BeNumerically(">", len(uploadedBytes)))
				},
			))

			uploadURL, err = url.Parse(fakeCloudController.URL())
			Expect(err).NotTo(HaveOccurred())

			uploadURL.User = url.UserPassword("bob", "password")
			uploadURL.Path = "/staging/droplet/app-guid/upload"
			uploadURL.RawQuery = url.Values{"async": []string{"true"}}.Encode()
		})

		JustBeforeEach(func() {
			u, err := url.Parse("http://cc-uploader.com/v1/droplet/app-guid")
			Expect(err).NotTo(HaveOccurred())

			v := url.Values{cc_messages.CcDropletUploadUriKey: []string{uploadURL.String()}}
			u.RawQuery = v.Encode()
			incomingRequest.URL = u

			outgoingResponse = httptest.NewRecorder()

			startTime = time.Now()
			handler.ServeHTTP(outgoingResponse, incomingRequest)
			endTime = time.Now()
		})

		Context("uploading the file, when all is well", func() {
			BeforeEach(func() {
				postStatusCode = http.StatusCreated
				postResponseBody = pollingResponseBody("my-job-guid", "queued", fakeCloudController.URL())
				fakeCloudController.AppendHandlers(
					verifyPollingRequest("my-job-guid", "queued", timeClicker),
					verifyPollingRequest("my-job-guid", "running", timeClicker),
					verifyPollingRequest("my-job-guid", "finished", timeClicker),
				)
			})

			It("responds with 201 CREATED", func() {
				Expect(outgoingResponse.Code).To(Equal(http.StatusCreated))
			})

			It("forwards the content-md5 header", func() {
				Expect(uploadedHeaders.Get("Content-MD5")).To(Equal("the-md5"))
			})

			It("uploads the correct file", func() {
				Expect(uploadedBytes).To(Equal([]byte("the file I'm uploading")))
				Expect(uploadedFileName).To(Equal("droplet.tgz"))
			})

			It("should wait between polls", func() {
				var firstTime, secondTime, thirdTime time.Time
				Eventually(timeClicker).Should(Receive(&firstTime))
				Eventually(timeClicker).Should(Receive(&secondTime))
				Eventually(timeClicker).Should(Receive(&thirdTime))

				Expect(secondTime.Sub(firstTime)).To(BeNumerically(">", 75*time.Millisecond))
				Expect(thirdTime.Sub(secondTime)).To(BeNumerically(">", 75*time.Millisecond))
			})
		})

		Context("uploading the file, when the job fails", func() {
			BeforeEach(func() {
				postStatusCode = http.StatusCreated
				postResponseBody = pollingResponseBody("my-job-guid", "queued", fakeCloudController.URL())
				fakeCloudController.AppendHandlers(
					verifyPollingRequest("my-job-guid", "queued", timeClicker),
					verifyPollingRequest("my-job-guid", "failed", timeClicker),
				)
			})

			It("stops polling after the first fail", func() {
				Expect(fakeCloudController.ReceivedRequests()).To(HaveLen(3))

				Expect(outgoingResponse.Code).To(Equal(http.StatusInternalServerError))
			})
		})

		Context("uploading the file, when the inbound upload request is missing content length", func() {
			BeforeEach(func() {
				incomingRequest.ContentLength = -1
			})

			It("does not make the request to CC", func() {
				Expect(fakeCloudController.ReceivedRequests()).To(HaveLen(0))
			})

			It("responds with 411", func() {
				Expect(outgoingResponse.Code).To(Equal(http.StatusLengthRequired))
			})
		})

		Context("when CC returns a non-succesful status code", func() {
			BeforeEach(func() {
				postStatusCode = http.StatusForbidden
			})

			It("makes the request to CC", func() {
				Expect(fakeCloudController.ReceivedRequests()).To(HaveLen(1))
			})

			It("responds with the status code from the CC request", func() {
				Expect(outgoingResponse.Code).To(Equal(http.StatusForbidden))

				data, err := ioutil.ReadAll(outgoingResponse.Body)
				Expect(err).NotTo(HaveOccurred())
				Expect(string(data)).To(ContainSubstring(strconv.Itoa(http.StatusForbidden)))
			})
		})
	})

	Describe("Uploading Build Artifacts", func() {
		BeforeEach(func() {
			var err error

			fakeCloudController.AppendHandlers(ghttp.CombineHandlers(
				ghttp.VerifyRequest("POST", "/staging/buildpack_cache/app-guid/upload"),
				ghttp.VerifyBasicAuth("bob", "password"),
				ghttp.RespondWithPtr(&postStatusCode, &postResponseBody),
				func(w http.ResponseWriter, r *http.Request) {
					uploadedHeaders = r.Header
					file, fileHeader, err := r.FormFile(ccclient.FormField)
					Expect(err).NotTo(HaveOccurred())
					uploadedBytes, err = ioutil.ReadAll(file)
					Expect(err).NotTo(HaveOccurred())
					uploadedFileName = fileHeader.Filename
					Expect(r.ContentLength).To(BeNumerically(">", len(uploadedBytes)))
				},
			))

			uploadURL, err = url.Parse(fakeCloudController.URL())
			Expect(err).NotTo(HaveOccurred())

			uploadURL.User = url.UserPassword("bob", "password")
			uploadURL.Path = "/staging/buildpack_cache/app-guid/upload"
		})

		JustBeforeEach(func() {
			u, err := url.Parse("http://cc-uploader.com/v1/build_artifacts/app-guid")
			Expect(err).NotTo(HaveOccurred())
			v := url.Values{cc_messages.CcBuildArtifactsUploadUriKey: []string{uploadURL.String()}}
			u.RawQuery = v.Encode()
			incomingRequest.URL = u

			outgoingResponse = httptest.NewRecorder()

			handler.ServeHTTP(outgoingResponse, incomingRequest)
		})

		Context("uploading the file, when all is well", func() {
			It("responds with 200 OK", func() {
				Expect(outgoingResponse.Code).To(Equal(http.StatusOK))
			})

			It("uploads the correct file", func() {
				Expect(uploadedBytes).To(Equal([]byte("the file I'm uploading")))
				Expect(uploadedFileName).To(Equal("buildpack_cache.tgz"))
			})

			It("forwards the content-md5 header", func() {
				Expect(uploadedHeaders.Get("Content-MD5")).To(Equal("the-md5"))
			})
		})

		Context("uploading the file, when the inbound upload request is missing content length", func() {
			BeforeEach(func() {
				incomingRequest.ContentLength = -1
			})

			It("does not make the request to CC", func() {
				Expect(fakeCloudController.ReceivedRequests()).To(HaveLen(0))
			})

			It("responds with 411", func() {
				Expect(outgoingResponse.Code).To(Equal(http.StatusLengthRequired))
			})
		})

		Context("when CC returns a non-succesful status code", func() {
			BeforeEach(func() {
				postStatusCode = http.StatusForbidden
			})

			It("makes the request to CC", func() {
				Expect(fakeCloudController.ReceivedRequests()).To(HaveLen(1))
			})

			It("responds with the status code from the CC request", func() {
				Expect(outgoingResponse.Code).To(Equal(http.StatusForbidden))

				data, err := ioutil.ReadAll(outgoingResponse.Body)
				Expect(err).NotTo(HaveOccurred())
				Expect(string(data)).To(ContainSubstring(strconv.Itoa(http.StatusForbidden)))
			})
		})
	})
})

func pollingResponseBody(jobGuid, status string, baseUrl string) string {
	url := urljoiner.Join("/v2/jobs", jobGuid)
	if baseUrl != "" {
		url = urljoiner.Join(baseUrl, url)
	}
	return fmt.Sprintf(`
				{
					"metadata":{
						"guid": "%s",
						"url": "%s"
					},
					"entity": {
						"status": "%s"
					}
				}
			`, jobGuid, url, status)
}

func verifyPollingRequest(jobGuid, status string, timeClicker chan time.Time) http.HandlerFunc {
	return ghttp.CombineHandlers(
		ghttp.VerifyRequest("GET", urljoiner.Join("/v2/jobs/", jobGuid)),
		ghttp.RespondWith(http.StatusOK, pollingResponseBody(jobGuid, status, "")),
		func(w http.ResponseWriter, r *http.Request) {
			timeClicker <- time.Now()
		},
	)
}
