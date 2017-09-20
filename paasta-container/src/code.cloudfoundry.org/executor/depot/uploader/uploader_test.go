package uploader_test

import (
	"crypto/md5"
	"encoding/base64"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strconv"
	"sync"
	"time"

	"code.cloudfoundry.org/executor/depot/uploader"
	"code.cloudfoundry.org/lager/lagertest"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
)

var _ = Describe("Uploader", func() {
	var upldr uploader.Uploader
	var testServer *httptest.Server
	var serverRequests []*http.Request
	var serverRequestBody []string
	var logger *lagertest.TestLogger

	BeforeEach(func() {
		testServer = nil
		serverRequestBody = []string{}
		serverRequests = []*http.Request{}
		logger = lagertest.NewTestLogger("test")

		upldr = uploader.New(100*time.Millisecond, false, logger)
	})

	Describe("Upload", func() {
		var url *url.URL
		var file *os.File
		var expectedBytes int
		var expectedMD5 string

		BeforeEach(func() {
			file, _ = ioutil.TempFile("", "foo")
			contentString := "content that we can check later"
			expectedBytes, _ = file.WriteString(contentString)
			rawMD5 := md5.Sum([]byte(contentString))
			expectedMD5 = base64.StdEncoding.EncodeToString(rawMD5[:])
			file.Close()
		})

		AfterEach(func() {
			file.Close()
			if testServer != nil {
				testServer.Close()
			}
			os.Remove(file.Name())
		})

		Context("when the upload is successful", func() {
			BeforeEach(func() {
				testServer = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					serverRequests = append(serverRequests, r)

					data, err := ioutil.ReadAll(r.Body)
					Expect(err).NotTo(HaveOccurred())
					serverRequestBody = append(serverRequestBody, string(data))

					fmt.Fprintln(w, "Hello, client")
				}))

				serverUrl := testServer.URL + "/somepath"
				url, _ = url.Parse(serverUrl)
			})

			var err error
			var numBytes int64
			JustBeforeEach(func() {
				numBytes, err = upldr.Upload(file.Name(), url, nil)
			})

			It("uploads the file to the url", func() {
				Expect(len(serverRequests)).To(Equal(1))

				request := serverRequests[0]
				data := serverRequestBody[0]

				Expect(request.URL.Path).To(Equal("/somepath"))
				Expect(request.Header.Get("Content-Type")).To(Equal("application/octet-stream"))
				Expect(request.Header.Get("Content-MD5")).To(Equal(expectedMD5))
				Expect(strconv.Atoi(request.Header.Get("Content-Length"))).To(BeNumerically("==", 31))
				Expect(string(data)).To(Equal("content that we can check later"))
			})

			It("returns the number of bytes written", func() {
				Expect(numBytes).To(Equal(int64(expectedBytes)))
			})

			It("does not return an error", func() {
				Expect(err).NotTo(HaveOccurred())
			})
		})

		Context("when the upload is canceled", func() {
			var flushRequests chan struct{}
			var requestsInFlight *sync.WaitGroup

			BeforeEach(func() {
				flushRequests = make(chan struct{})

				requestsInFlight = new(sync.WaitGroup)
				testServer = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					defer requestsInFlight.Done()
					<-flushRequests
				}))

				serverUrl := testServer.URL + "/somepath"
				url, _ = url.Parse(serverUrl)
			})

			AfterEach(func() {
				close(flushRequests)
				requestsInFlight.Wait()
			})

			It("interrupts the client and returns an error", func() {
				upldrWithoutTimeout := uploader.New(0, false, logger)

				cancel := make(chan struct{})
				errs := make(chan error)

				requestsInFlight.Add(1)

				go func() {
					_, err := upldrWithoutTimeout.Upload(file.Name(), url, cancel)
					errs <- err
				}()

				Consistently(errs).ShouldNot(Receive())

				close(cancel)

				Eventually(errs).Should(Receive(Equal(uploader.ErrUploadCancelled)))
			})
		})

		Context("when the upload times out", func() {
			var requestInitiated chan struct{}

			BeforeEach(func() {
				requestInitiated = make(chan struct{})

				testServer = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					requestInitiated <- struct{}{}

					time.Sleep(300 * time.Millisecond)
					fmt.Fprintln(w, "Hello, client")
				}))

				serverUrl := testServer.URL + "/somepath"
				url, _ = url.Parse(serverUrl)
			})

			It("should retry and log 3 times and return an error", func() {
				errs := make(chan error)

				go func() {
					_, err := upldr.Upload(file.Name(), url, nil)
					errs <- err
				}()

				Eventually(logger.TestSink.Buffer).Should(gbytes.Say("attempt"))
				Eventually(requestInitiated).Should(Receive())

				Eventually(logger.TestSink.Buffer).Should(gbytes.Say("attempt"))
				Eventually(requestInitiated).Should(Receive())

				Eventually(logger.TestSink.Buffer).Should(gbytes.Say("attempt"))
				Eventually(requestInitiated).Should(Receive())

				Eventually(logger.TestSink.Buffer).Should(gbytes.Say("failed-upload"))

				Expect(<-errs).To(HaveOccurred())
			})
		})

		Context("when the upload fails with a protocol error", func() {
			BeforeEach(func() {
				// No server to handle things!

				serverUrl := "http://127.0.0.1:54321/somepath"
				url, _ = url.Parse(serverUrl)
			})

			It("should return the error", func() {
				_, err := upldr.Upload(file.Name(), url, nil)
				Expect(err).NotTo(BeNil())
			})
		})

		Context("when the upload fails with a status code error", func() {
			BeforeEach(func() {
				testServer = httptest.NewServer(http.NotFoundHandler())

				serverUrl := testServer.URL + "/somepath"
				url, _ = url.Parse(serverUrl)
			})

			It("should return the error", func() {
				_, err := upldr.Upload(file.Name(), url, nil)
				Expect(err).NotTo(BeNil())
			})
		})
	})
})
