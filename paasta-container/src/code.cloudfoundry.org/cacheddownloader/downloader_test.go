package cacheddownloader_test

import (
	"bytes"
	"crypto/tls"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strconv"
	"sync"
	"time"

	"code.cloudfoundry.org/cacheddownloader"
	"github.com/cloudfoundry/systemcerts"
	"github.com/onsi/gomega/ghttp"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Downloader", func() {
	var downloader *cacheddownloader.Downloader
	var testServer *httptest.Server
	var serverRequestUrls []string
	var lock *sync.Mutex
	var cancelChan chan struct{}

	createDestFile := func() (*os.File, error) {
		return ioutil.TempFile("", "foo")
	}

	BeforeEach(func() {
		testServer = nil
		downloader = cacheddownloader.NewDownloader(100*time.Millisecond, 10, false, nil)
		lock = &sync.Mutex{}
		cancelChan = make(chan struct{}, 0)
	})

	Describe("Download", func() {
		var serverUrl *url.URL

		BeforeEach(func() {
			serverRequestUrls = []string{}
		})

		AfterEach(func() {
			if testServer != nil {
				testServer.Close()
			}
		})

		Context("when the download is successful", func() {
			var (
				downloadErr    error
				downloadedFile string

				downloadCachingInfo cacheddownloader.CachingInfoType
				expectedCachingInfo cacheddownloader.CachingInfoType
				expectedEtag        string
			)

			AfterEach(func() {
				if downloadedFile != "" {
					os.Remove(downloadedFile)
				}
			})

			JustBeforeEach(func() {
				serverUrl, _ = url.Parse(testServer.URL + "/somepath")
				downloadedFile, downloadCachingInfo, downloadErr = downloader.Download(serverUrl, createDestFile, cacheddownloader.CachingInfoType{}, cacheddownloader.ChecksumInfoType{}, cancelChan)
			})

			Context("and contains a matching MD5 Hash in the Etag", func() {
				var attempts int
				BeforeEach(func() {
					attempts = 0
					testServer = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
						lock.Lock()
						serverRequestUrls = append(serverRequestUrls, r.RequestURI)
						attempts++
						lock.Unlock()

						msg := "Hello, client"
						hexMsg, err := cacheddownloader.HexValue("md5", msg)
						Expect(err).NotTo(HaveOccurred())
						expectedCachingInfo = cacheddownloader.CachingInfoType{
							ETag:         hexMsg,
							LastModified: "The 70s",
						}
						w.Header().Set("ETag", expectedCachingInfo.ETag)
						w.Header().Set("Last-Modified", expectedCachingInfo.LastModified)

						fmt.Fprint(w, msg)
					}))
				})

				It("does not return an error", func() {
					Expect(downloadErr).NotTo(HaveOccurred())
				})

				It("only tries once", func() {
					Expect(attempts).To(Equal(1))
				})

				It("claims to have downloaded", func() {
					Expect(downloadedFile).NotTo(BeEmpty())
				})

				It("gets a file from a url", func() {
					lock.Lock()
					urlFromServer := testServer.URL + serverRequestUrls[0]
					Expect(urlFromServer).To(Equal(serverUrl.String()))
					lock.Unlock()
				})

				It("should use the provided file as the download location", func() {
					fileContents, _ := ioutil.ReadFile(downloadedFile)
					Expect(fileContents).To(ContainSubstring("Hello, client"))
				})

				It("returns the ETag", func() {
					Expect(downloadCachingInfo).To(Equal(expectedCachingInfo))
				})
			})

			Context("and contains an Etag that is not an MD5 Hash ", func() {
				BeforeEach(func() {
					expectedEtag = "not the hex you are looking for"
					testServer = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
						w.Header().Set("ETag", expectedEtag)
						fmt.Fprint(w, "Hello, client")
					}))
				})

				It("succeeds without doing a checksum", func() {
					Expect(downloadedFile).NotTo(BeEmpty())
					Expect(downloadErr).NotTo(HaveOccurred())
				})

				It("should returns the ETag in the caching info", func() {
					Expect(downloadCachingInfo.ETag).To(Equal(expectedEtag))
				})
			})

			Context("and contains no Etag at all", func() {
				BeforeEach(func() {
					testServer = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
						fmt.Fprint(w, "Hello, client")
					}))
				})

				It("succeeds without doing a checksum", func() {
					Expect(downloadedFile).NotTo(BeEmpty())
					Expect(downloadErr).NotTo(HaveOccurred())
				})

				It("should returns no ETag in the caching info", func() {
					Expect(downloadCachingInfo).To(BeZero())
				})
			})
		})

		Context("when the download times out", func() {
			var requestInitiated chan struct{}

			BeforeEach(func() {
				requestInitiated = make(chan struct{})

				testServer = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					requestInitiated <- struct{}{}

					time.Sleep(300 * time.Millisecond)
					fmt.Fprint(w, "Hello, client")
				}))

				serverUrl, _ = url.Parse(testServer.URL + "/somepath")
			})

			It("should retry 3 times and return an error", func() {
				errs := make(chan error)
				downloadedFiles := make(chan string)

				go func() {
					downloadedFile, _, err := downloader.Download(serverUrl, createDestFile, cacheddownloader.CachingInfoType{}, cacheddownloader.ChecksumInfoType{}, cancelChan)
					errs <- err
					downloadedFiles <- downloadedFile
				}()

				Eventually(requestInitiated).Should(Receive())
				Eventually(requestInitiated).Should(Receive())
				Eventually(requestInitiated).Should(Receive())

				Expect(<-errs).To(HaveOccurred())
				Expect(<-downloadedFiles).To(BeEmpty())
			})
		})

		Context("when the download fails with a protocol error", func() {
			BeforeEach(func() {
				// No server to handle things!
				serverUrl, _ = url.Parse("http://127.0.0.1:54321/somepath")
			})

			It("should return the error", func() {
				downloadedFile, _, err := downloader.Download(serverUrl, createDestFile, cacheddownloader.CachingInfoType{}, cacheddownloader.ChecksumInfoType{}, cancelChan)
				Expect(err).To(HaveOccurred())
				Expect(downloadedFile).To(BeEmpty())
			})
		})

		Context("when the download fails with a status code error", func() {
			BeforeEach(func() {
				testServer = httptest.NewServer(http.NotFoundHandler())

				serverUrl, _ = url.Parse(testServer.URL + "/somepath")
			})

			It("should return the error", func() {
				downloadedFile, _, err := downloader.Download(serverUrl, createDestFile, cacheddownloader.CachingInfoType{}, cacheddownloader.ChecksumInfoType{}, cancelChan)
				Expect(err).To(HaveOccurred())
				Expect(downloadedFile).To(BeEmpty())
			})
		})

		Context("when the read exceeds the deadline timeout", func() {
			var done chan struct{}

			BeforeEach(func() {
				done = make(chan struct{}, 3)
				downloader = cacheddownloader.NewDownloaderWithIdleTimeout(1*time.Second, 30*time.Millisecond, 10, false, nil)

				testServer = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					time.Sleep(100 * time.Millisecond)
					done <- struct{}{}
				}))

				serverUrl, _ = url.Parse(testServer.URL + "/somepath")
			})

			AfterEach(func() {
				Eventually(done).Should(HaveLen(3))
			})

			It("fails with a nested read error", func() {
				errs := make(chan error)

				go func() {
					_, _, err := downloader.Download(serverUrl, createDestFile, cacheddownloader.CachingInfoType{}, cacheddownloader.ChecksumInfoType{}, cancelChan)
					errs <- err
				}()

				var err error
				Eventually(errs).Should(Receive(&err))
				uErr, ok := err.(*url.Error)
				Expect(ok).To(BeTrue())
				opErr, ok := uErr.Err.(*net.OpError)
				Expect(ok).To(BeTrue())
				Expect(opErr.Op).To(Equal("read"))
			})
		})

		Context("when the Content-Length does not match the downloaded file size", func() {
			BeforeEach(func() {
				testServer = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					realMsg := "Hello, client"
					incompleteMsg := "Hello, clientsss"

					w.Header().Set("Content-Length", strconv.Itoa(len(realMsg)))

					fmt.Fprint(w, incompleteMsg)
				}))

				serverUrl, _ = url.Parse(testServer.URL + "/somepath")
			})

			It("should return an error", func() {
				downloadedFile, cachingInfo, err := downloader.Download(serverUrl, createDestFile, cacheddownloader.CachingInfoType{}, cacheddownloader.ChecksumInfoType{}, cancelChan)
				Expect(err).To(HaveOccurred())
				Expect(downloadedFile).To(BeEmpty())
				Expect(cachingInfo).To(BeZero())
			})
		})

		Context("when cancelling", func() {
			var requestInitiated chan struct{}
			var completeRequest chan struct{}

			BeforeEach(func() {
				requestInitiated = make(chan struct{})
				completeRequest = make(chan struct{})

				testServer = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					requestInitiated <- struct{}{}
					<-completeRequest
					w.Write(bytes.Repeat([]byte("a"), 1024))
					w.(http.Flusher).Flush()
					<-completeRequest
				}))

				serverUrl, _ = url.Parse(testServer.URL + "/somepath")
			})

			It("cancels the request", func() {
				errs := make(chan error)

				go func() {
					_, _, err := downloader.Download(serverUrl, createDestFile, cacheddownloader.CachingInfoType{}, cacheddownloader.ChecksumInfoType{}, cancelChan)
					errs <- err
				}()

				Eventually(requestInitiated).Should(Receive())
				close(cancelChan)

				Eventually(errs).Should(Receive(BeAssignableToTypeOf(cacheddownloader.NewDownloadCancelledError("", 0, cacheddownloader.NoBytesReceived))))

				close(completeRequest)
			})

			It("stops the download", func() {
				errs := make(chan error)

				go func() {
					_, _, err := downloader.Download(serverUrl, createDestFile, cacheddownloader.CachingInfoType{}, cacheddownloader.ChecksumInfoType{}, cancelChan)
					errs <- err
				}()

				Eventually(requestInitiated).Should(Receive())
				completeRequest <- struct{}{}
				close(cancelChan)

				Eventually(errs).Should(Receive(BeAssignableToTypeOf(cacheddownloader.NewDownloadCancelledError("", 0, cacheddownloader.NoBytesReceived))))
				close(completeRequest)
			})
		})

		Context("when using TLS", func() {
			var (
				downloadErr    error
				downloadedFile string

				localhostCert = []byte(`-----BEGIN CERTIFICATE-----
MIIBdzCCASOgAwIBAgIBADALBgkqhkiG9w0BAQUwEjEQMA4GA1UEChMHQWNtZSBD
bzAeFw03MDAxMDEwMDAwMDBaFw00OTEyMzEyMzU5NTlaMBIxEDAOBgNVBAoTB0Fj
bWUgQ28wWjALBgkqhkiG9w0BAQEDSwAwSAJBAN55NcYKZeInyTuhcCwFMhDHCmwa
IUSdtXdcbItRB/yfXGBhiex00IaLXQnSU+QZPRZWYqeTEbFSgihqi1PUDy8CAwEA
AaNoMGYwDgYDVR0PAQH/BAQDAgCkMBMGA1UdJQQMMAoGCCsGAQUFBwMBMA8GA1Ud
EwEB/wQFMAMBAf8wLgYDVR0RBCcwJYILZXhhbXBsZS5jb22HBH8AAAGHEAAAAAAA
AAAAAAAAAAAAAAEwCwYJKoZIhvcNAQEFA0EAAoQn/ytgqpiLcZu9XKbCJsJcvkgk
Se6AbGXgSlq+ZCEVo0qIwSgeBqmsJxUu7NCSOwVJLYNEBO2DtIxoYVk+MA==
-----END CERTIFICATE-----`)
				localhostKey = []byte(`-----BEGIN RSA PRIVATE KEY-----
MIIBPAIBAAJBAN55NcYKZeInyTuhcCwFMhDHCmwaIUSdtXdcbItRB/yfXGBhiex0
0IaLXQnSU+QZPRZWYqeTEbFSgihqi1PUDy8CAwEAAQJBAQdUx66rfh8sYsgfdcvV
NoafYpnEcB5s4m/vSVe6SU7dCK6eYec9f9wpT353ljhDUHq3EbmE4foNzJngh35d
AekCIQDhRQG5Li0Wj8TM4obOnnXUXf1jRv0UkzE9AHWLG5q3AwIhAPzSjpYUDjVW
MCUXgckTpKCuGwbJk7424Nb8bLzf3kllAiA5mUBgjfr/WtFSJdWcPQ4Zt9KTMNKD
EUO0ukpTwEIl6wIhAMbGqZK3zAAFdq8DD2jPx+UJXnh0rnOkZBzDtJ6/iN69AiEA
1Aq8MJgTaYsDQWyU/hDq5YkDJc9e9DSCvUIzqxQWMQE=
-----END RSA PRIVATE KEY-----`)
				wrongCA = []byte(`-----BEGIN CERTIFICATE-----
MIIFATCCAuugAwIBAgIBATALBgkqhkiG9w0BAQswEjEQMA4GA1UEAxMHZGllZ29D
QTAeFw0xNjAyMTYyMTU1MzNaFw0yNjAyMTYyMTU1NDZaMBIxEDAOBgNVBAMTB2Rp
ZWdvQ0EwggIiMA0GCSqGSIb3DQEBAQUAA4ICDwAwggIKAoICAQC7N7lGx7QGqkMd
wjqgkr09CPoV3HW+GL+YOPajf//CCo15t3mLu9Npv7O7ecb+g/6DxEOtHFpQBSbQ
igzHZkdlBJEGknwH2bsZ4wcVT2vcv2XPAIMDrnT7VuF1S2XD7BJK3n6BeXkFsVPA
OUjC/v0pM/rCFRId5CwtRD/0IHFC/qgEtFQx+zejXXEn1AJMzvNNJ3B0bd8VQGEX
ppemZXS1QvTP7/j2h7fJjosyoL6+76k4mcoScmWFNJHKcG4qcAh8rdnDlw+hJ+5S
z73CadYI2BTnlZ/fxEcsZ/kcteFSf0mFpMYX6vs9/us/rgGwjUNzg+JlzvF43TYY
VQ+TRkFUYHhDv3xwuRHnPNe0Nm30esKpqvbSXtoS6jcnpHn9tMOU0+4NW4aEdy9s
7l4lcGyih4qZfHbYTsRDk1Nrq5EzQbhlZSPC3nxMrLxXri7j22rVCY/Rj9IgAxwC
R3KcCdADGJeNOw44bK/BsRrB+Hxs9yNpXc2V2dez+w3hKNuzyk7WydC3fgXxX6x8
66xnlhFGor7fvM0OSMtGUBD16igh4ySdDiEMNUljqQ1DuMglT1eGdg+Kh+1YYWpz
v3JkNTX96C80IivbZyunZ2CczFhW2HlGWZLwNKeuM0hxt6AmiEa+KJQkx73dfg3L
tkDWWp9TXERPI/6Y2696INi0wElBUQIDAQABo2YwZDAOBgNVHQ8BAf8EBAMCAAYw
EgYDVR0TAQH/BAgwBgEB/wIBADAdBgNVHQ4EFgQU5xGtUKEzsfGmk/Siqo4fgAMs
TBwwHwYDVR0jBBgwFoAU5xGtUKEzsfGmk/Siqo4fgAMsTBwwCwYJKoZIhvcNAQEL
A4ICAQBkWgWl2t5fd4PZ1abpSQNAtsb2lfkkpxcKw+Osn9MeGpcrZjP8XoVTxtUs
GMpeVn2dUYY1sxkVgUZ0Epsgl7eZDK1jn6QfWIjltlHvDtJMh0OrxmdJUuHTGIHc
lsI9NGQRUtbyFHmy6jwIF7q925OmPQ/A6Xgkb45VUJDGNwOMUL5I9LbdBXcjmx6F
ZifEON3wxDBVMIAoS/mZYjP4zy2k1qE2FHoitwDccnCG5Wya+AHdZv/ZlfJcuMtU
U82oyHOctH29BPwASs3E1HUKof6uxJI+Y1M2kBDeuDS7DWiTt3JIVCjewIIhyYYw
uTPbQglqhqHr1RWohliDmKSroIil68s42An0fv9sUr0Btf4itKS1gTb4rNiKTZC/
8sLKs+CA5MB+F8lCllGGFfv1RFiUZBQs9+YEE+ru+yJw39lHeZQsEUgHbLjbVHs1
WFqiKTO8VKl1/eGwG0l9dI26qisIAa/I7kLjlqboKycGDmAAarsmcJBLPzS+ytiu
hoxA/fLhSWJvPXbdGemXLWQGf5DLN/8QGB63Rjp9WC3HhwSoU0NvmNmHoh+AdRRT
dYbCU/DMZjsv+Pt9flhj7ELLo+WKHyI767hJSq9A7IT3GzFt8iGiEAt1qj2yS0DX
36hwbfc1Gh/8nKgFeLmPOlBfKncjTjL2FvBNap6a8tVHXO9FvQ==
-----END CERTIFICATE-----`)
			)

			BeforeEach(func() {
				testServer = httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					fmt.Fprint(w, "Hello, client")
				}))

				cert, err := tls.X509KeyPair(localhostCert, localhostKey)
				Expect(err).NotTo(HaveOccurred())

				testServer.TLS = &tls.Config{
					Certificates: []tls.Certificate{cert},
				}
				testServer.StartTLS()
			})

			JustBeforeEach(func() {
				serverUrl, _ = url.Parse(testServer.URL + "/somepath")
				downloadedFile, _, downloadErr = downloader.Download(serverUrl, createDestFile, cacheddownloader.CachingInfoType{}, cacheddownloader.ChecksumInfoType{}, cancelChan)
			})

			AfterEach(func() {
				if downloadedFile != "" {
					os.Remove(downloadedFile)
				}
			})

			Context("and setting the correct CA", func() {
				BeforeEach(func() {
					caCertPool := systemcerts.NewCertPool()
					ok := caCertPool.AppendCertsFromPEM(localhostCert)
					Expect(ok).To(BeTrue())
					downloader = cacheddownloader.NewDownloader(100*time.Millisecond, 10, false, caCertPool)
				})

				It("succeeds the download", func() {
					Expect(downloadedFile).NotTo(BeEmpty())
					Expect(downloadErr).NotTo(HaveOccurred())
				})
			})

			Context("and setting the incorrect CA", func() {
				BeforeEach(func() {
					caCertPool := systemcerts.NewCertPool()
					ok := caCertPool.AppendCertsFromPEM(wrongCA)
					Expect(ok).To(BeTrue())
					downloader = cacheddownloader.NewDownloader(100*time.Millisecond, 10, false, caCertPool)
				})

				It("fails the download", func() {
					Expect(downloadedFile).To(BeEmpty())
					Expect(downloadErr).To(HaveOccurred())
				})
			})

			Context("and setting multiple CAs, including the correct one", func() {
				BeforeEach(func() {
					caCertPool := systemcerts.NewCertPool()
					ok := caCertPool.AppendCertsFromPEM(wrongCA)
					Expect(ok).To(BeTrue())
					ok = caCertPool.AppendCertsFromPEM(localhostCert)
					Expect(ok).To(BeTrue())
					downloader = cacheddownloader.NewDownloader(100*time.Millisecond, 10, false, caCertPool)
				})

				It("succeeds the download", func() {
					Expect(downloadedFile).NotTo(BeEmpty())
					Expect(downloadErr).NotTo(HaveOccurred())
				})
			})

			Context("and skipping certificate verification", func() {
				BeforeEach(func() {
					downloader = cacheddownloader.NewDownloader(100*time.Millisecond, 10, true, nil)
				})

				It("succeeds without doing checking certificate validity", func() {
					Expect(downloadErr).NotTo(HaveOccurred())
					Expect(downloadedFile).NotTo(BeEmpty())
				})
			})

			Context("without any truested CAs", func() {
				BeforeEach(func() {
					downloader = cacheddownloader.NewDownloader(100*time.Millisecond, 10, false, nil)
				})

				It("fails the download", func() {
					Expect(downloadedFile).To(BeEmpty())
					Expect(downloadErr).To(HaveOccurred())
				})
			})
		})

		Context("when checksum info is provided", func() {
			var msg string
			BeforeEach(func() {
				msg = "Hello, client"
				testServer = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					fmt.Fprint(w, msg)
				}))
				serverUrl, _ = url.Parse(testServer.URL + "/somepath")
			})

			Context("when algorithm is invalid", func() {
				It("should return an algorithm invalid error", func() {
					checksum := cacheddownloader.ChecksumInfoType{Algorithm: "wrong alg", Value: "some value"}
					downloadedFile, _, err := downloader.Download(serverUrl, createDestFile, cacheddownloader.CachingInfoType{}, checksum, cancelChan)
					Expect(err).To(HaveOccurred())
					Expect(err.Error()).To(ContainSubstring("algorithm invalid"))
					Expect(downloadedFile).To(BeEmpty())
				})
			})

			Context("when the value is invalid", func() {
				It("should return a checksum invalid error", func() {
					checksum := cacheddownloader.ChecksumInfoType{Algorithm: "md5", Value: "wrong value"}
					downloadedFile, _, err := downloader.Download(serverUrl, createDestFile, cacheddownloader.CachingInfoType{}, checksum, cancelChan)
					Expect(err).To(HaveOccurred())
					Expect(err.Error()).To(ContainSubstring("checksum missing or invalid"))
					Expect(downloadedFile).To(BeEmpty())
				})
			})

			Context("when the checksum algorithm is supported", func() {
				It("with an md5 checksum", func() {
					hexMsg, err := cacheddownloader.HexValue("md5", msg)
					Expect(err).NotTo(HaveOccurred())

					checksum := cacheddownloader.ChecksumInfoType{Algorithm: "md5", Value: hexMsg}
					downloadedFile, _, err := downloader.Download(serverUrl, createDestFile, cacheddownloader.CachingInfoType{}, checksum, cancelChan)
					Expect(err).NotTo(HaveOccurred())
					Expect(downloadedFile).NotTo(BeEmpty())
				})

				It("with a sha1 checksum", func() {
					hexMsg, err := cacheddownloader.HexValue("sha1", msg)
					Expect(err).NotTo(HaveOccurred())

					checksum := cacheddownloader.ChecksumInfoType{Algorithm: "sha1", Value: hexMsg}
					downloadedFile, _, err := downloader.Download(serverUrl, createDestFile, cacheddownloader.CachingInfoType{}, checksum, cancelChan)
					Expect(err).NotTo(HaveOccurred())
					Expect(downloadedFile).NotTo(BeEmpty())
				})

				It("with a sha256 checksum", func() {
					hexMsg, err := cacheddownloader.HexValue("sha256", msg)
					Expect(err).NotTo(HaveOccurred())

					checksum := cacheddownloader.ChecksumInfoType{Algorithm: "sha256", Value: hexMsg}
					downloadedFile, _, err := downloader.Download(serverUrl, createDestFile, cacheddownloader.CachingInfoType{}, checksum, cancelChan)
					Expect(err).NotTo(HaveOccurred())
					Expect(downloadedFile).NotTo(BeEmpty())
				})
			})
		})
	})

	Describe("Concurrent downloads", func() {
		var (
			server    *ghttp.Server
			serverUrl *url.URL
			barrier   chan interface{}
			results   chan bool
			tempDir   string
		)

		BeforeEach(func() {
			barrier = make(chan interface{}, 1)
			results = make(chan bool, 1)

			downloader = cacheddownloader.NewDownloader(1*time.Second, 1, false, nil)

			var err error
			tempDir, err = ioutil.TempDir("", "temp-dl-dir")
			Expect(err).NotTo(HaveOccurred())

			server = ghttp.NewServer()
			server.AppendHandlers(
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("GET", "/the-file"),
					http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
						barrier <- nil
						Consistently(results, .5).ShouldNot(Receive())
					}),
					ghttp.RespondWith(http.StatusOK, "download content", http.Header{}),
				),
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("GET", "/the-file"),
					http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
						results <- true
					}),
					ghttp.RespondWith(http.StatusOK, "download content", http.Header{}),
				),
			)

			serverUrl, _ = url.Parse(server.URL() + "/the-file")
		})

		AfterEach(func() {
			server.Close()
			os.RemoveAll(tempDir)
		})

		downloadTestFile := func(cancelChan <-chan struct{}) (path string, cachingInfoOut cacheddownloader.CachingInfoType, err error) {
			return downloader.Download(
				serverUrl,
				func() (*os.File, error) {
					return ioutil.TempFile(tempDir, "the-file")
				},
				cacheddownloader.CachingInfoType{},
				cacheddownloader.ChecksumInfoType{},
				cancelChan,
			)
		}

		It("only allows n downloads at the same time", func() {
			go func() {
				downloadTestFile(make(chan struct{}, 0))
				barrier <- nil
			}()

			<-barrier
			downloadTestFile(cancelChan)
			<-barrier
		})

		Context("when cancelling", func() {
			It("bails when waiting", func() {
				go func() {
					downloadTestFile(make(chan struct{}, 0))
					barrier <- nil
				}()

				<-barrier
				cancelChan := make(chan struct{}, 0)
				close(cancelChan)
				_, _, err := downloadTestFile(cancelChan)
				Expect(err).To(BeAssignableToTypeOf(cacheddownloader.NewDownloadCancelledError("", 0, cacheddownloader.NoBytesReceived)))
				<-barrier
			})
		})

		Context("Downloading with caching info", func() {
			var (
				server     *ghttp.Server
				cachedInfo cacheddownloader.CachingInfoType
				statusCode int
				serverUrl  *url.URL
				body       string
			)

			BeforeEach(func() {
				cachedInfo = cacheddownloader.CachingInfoType{
					ETag:         "It's Just a Flesh Wound",
					LastModified: "The 60s",
				}

				server = ghttp.NewServer()
				server.AppendHandlers(ghttp.CombineHandlers(
					ghttp.VerifyRequest("GET", "/get-the-file"),
					ghttp.VerifyHeader(http.Header{
						"If-None-Match":     []string{cachedInfo.ETag},
						"If-Modified-Since": []string{cachedInfo.LastModified},
					}),
					ghttp.RespondWithPtr(&statusCode, &body),
				))

				serverUrl, _ = url.Parse(server.URL() + "/get-the-file")
			})

			AfterEach(func() {
				server.Close()
			})

			Context("when the server replies with 304", func() {
				BeforeEach(func() {
					statusCode = http.StatusNotModified
				})

				It("should return that it did not download", func() {
					downloadedFile, _, err := downloader.Download(serverUrl, createDestFile, cachedInfo, cacheddownloader.ChecksumInfoType{}, cancelChan)
					Expect(downloadedFile).To(BeEmpty())
					Expect(err).NotTo(HaveOccurred())
				})
			})

			Context("when the server replies with 200", func() {
				var (
					downloadedFile string
					err            error
				)

				BeforeEach(func() {
					statusCode = http.StatusOK
					body = "quarb!"
				})

				AfterEach(func() {
					if downloadedFile != "" {
						os.Remove(downloadedFile)
					}
				})

				It("should download the file", func() {
					downloadedFile, _, err = downloader.Download(serverUrl, createDestFile, cachedInfo, cacheddownloader.ChecksumInfoType{}, cancelChan)
					Expect(err).NotTo(HaveOccurred())

					info, err := os.Stat(downloadedFile)
					Expect(err).NotTo(HaveOccurred())
					Expect(info.Size()).To(Equal(int64(len(body))))
				})
			})

			Context("for anything else (including a server error)", func() {
				BeforeEach(func() {
					statusCode = http.StatusInternalServerError

					// cope with built in retry
					for i := 0; i < cacheddownloader.MAX_DOWNLOAD_ATTEMPTS; i++ {
						server.AppendHandlers(ghttp.CombineHandlers(
							ghttp.VerifyRequest("GET", "/get-the-file"),
							ghttp.VerifyHeader(http.Header{
								"If-None-Match":     []string{cachedInfo.ETag},
								"If-Modified-Since": []string{cachedInfo.LastModified},
							}),
							ghttp.RespondWithPtr(&statusCode, &body),
						))
					}
				})

				It("should return false with an error", func() {
					downloadedFile, _, err := downloader.Download(serverUrl, createDestFile, cachedInfo, cacheddownloader.ChecksumInfoType{}, cancelChan)
					Expect(downloadedFile).To(BeEmpty())
					Expect(err).To(HaveOccurred())
				})
			})
		})
	})
})
