package cacheddownloader_test

import (
	"crypto/md5"
	"crypto/tls"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	Url "net/url"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"code.cloudfoundry.org/cacheddownloader"
	"code.cloudfoundry.org/lager/lagertest"
	"code.cloudfoundry.org/systemcerts"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/ghttp"
)

const MAX_CONCURRENT_DOWNLOADS = 10

func computeMd5(key string) string {
	return fmt.Sprintf("%x", md5.Sum([]byte(key)))
}

var _ = Describe("File cache", func() {
	var (
		cache            *cacheddownloader.FileCache
		downloader       *cacheddownloader.Downloader
		cachedDownloader cacheddownloader.CachedDownloader
		checksum         cacheddownloader.ChecksumInfoType
		cachedPath       string
		uncachedPath     string
		maxSizeInBytes   int64
		cacheKey         string
		downloadContent  []byte
		url              *Url.URL
		server           *ghttp.Server
		cancelChan       chan struct{}
		transformer      cacheddownloader.CacheTransformer

		logger *lagertest.TestLogger

		file     io.ReadCloser
		fileSize int64
		err      error
		dir      string
	)

	BeforeEach(func() {
		logger = lagertest.NewTestLogger("test")
		var err error
		cachedPath, err = ioutil.TempDir("", "test_file_cached")
		Expect(err).NotTo(HaveOccurred())

		uncachedPath, err = ioutil.TempDir("", "test_file_uncached")
		Expect(err).NotTo(HaveOccurred())

		// This needs to be larger now since when we do the tests with directories
		// the tar file is at least 4000 bytes and we calculate 8000 bytes with the
		// archive and the directory
		maxSizeInBytes = 32000

		cacheKey = "the-cache-key"

		transformer = cacheddownloader.NoopTransform

		cache = cacheddownloader.NewCache(cachedPath, maxSizeInBytes)
		downloader = cacheddownloader.NewDownloader(1*time.Second, MAX_CONCURRENT_DOWNLOADS, nil)
		cachedDownloader = cacheddownloader.New(uncachedPath, downloader, cache, transformer)
		server = ghttp.NewServer()

		url, err = Url.Parse(server.URL() + "/my_file")
		Expect(err).NotTo(HaveOccurred())

		cancelChan = make(chan struct{})
	})

	AfterEach(func() {
		os.RemoveAll(cachedPath)
		os.RemoveAll(uncachedPath)
	})

	Describe("when the cache folder does not exist", func() {
		It("should create it", func() {
			os.RemoveAll(cachedPath)
			cachedDownloader = cacheddownloader.New(uncachedPath, downloader, cache, transformer)
			_, err := os.Stat(cachedPath)
			Expect(err).NotTo(HaveOccurred())
		})
	})

	Describe("when the cache folder has stuff in it", func() {
		It("should not nuke that stuff", func() {
			filename := filepath.Join(cachedPath, "last_nights_dinner")
			ioutil.WriteFile(filename, []byte("leftovers"), 0666)
			cachedDownloader = cacheddownloader.New(uncachedPath, downloader, cache, transformer)
			_, err := os.Stat(filename)
			Expect(err).NotTo(HaveOccurred())
		})
	})

	Describe("When providing a file that should not be cached", func() {
		Context("when the download succeeds", func() {
			BeforeEach(func() {
				downloadContent = []byte("777")

				header := http.Header{}
				header.Set("ETag", "foo")
				server.AppendHandlers(ghttp.CombineHandlers(
					ghttp.VerifyRequest("GET", "/my_file"),
					ghttp.RespondWith(http.StatusOK, string(downloadContent), header),
				))

				file, fileSize, err = cachedDownloader.Fetch(logger, url, "", checksum, cancelChan)
			})

			It("should not error", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			It("should return a readCloser that streams the file", func() {
				Expect(file).NotTo(BeNil())
				Expect(fileSize).To(BeNumerically("==", 3))
				Expect(ioutil.ReadAll(file)).To(Equal(downloadContent))
			})

			It("should delete the file when we close the readCloser", func() {
				err := file.Close()
				Expect(err).NotTo(HaveOccurred())
				Expect(ioutil.ReadDir(uncachedPath)).To(HaveLen(0))
				Expect(ioutil.ReadDir(cachedPath)).To(HaveLen(0))
			})
		})

		Context("when the download fails", func() {
			BeforeEach(func() {
				server.AllowUnhandledRequests = true //will 500 for any attempted requests
				file, fileSize, err = cachedDownloader.Fetch(logger, url, "", checksum, cancelChan)
			})

			It("should return an error and no file", func() {
				Expect(file).To(BeNil())
				Expect(fileSize).To(BeNumerically("==", 0))
				Expect(err).To(HaveOccurred())
			})

			It("should clean up after itself", func() {
				Expect(ioutil.ReadDir(uncachedPath)).To(HaveLen(0))
				Expect(ioutil.ReadDir(cachedPath)).To(HaveLen(0))
			})
		})
	})

	Describe("When providing a file that should be cached", func() {
		var cacheFilePath string
		var returnedHeader http.Header

		BeforeEach(func() {
			cacheFilePath = filepath.Join(cachedPath, computeMd5(cacheKey))
			returnedHeader = http.Header{}
			returnedHeader.Set("ETag", "my-original-etag")
		})

		Context("when the file is not in the cache", func() {
			var (
				fetchedFile     io.ReadCloser
				fetchedFileSize int64
				fetchErr        error
			)

			JustBeforeEach(func() {
				fetchedFile, fetchedFileSize, fetchErr = cachedDownloader.Fetch(logger, url, cacheKey, checksum, cancelChan)
			})

			Context("when the download succeeds", func() {
				BeforeEach(func() {
					downloadContent = []byte(strings.Repeat("7", int(maxSizeInBytes/2)))
					server.AppendHandlers(ghttp.CombineHandlers(
						ghttp.VerifyRequest("GET", "/my_file"),
						http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
							Expect(req.Header.Get("If-None-Match")).To(BeEmpty())
						}),
						ghttp.RespondWith(http.StatusOK, string(downloadContent), returnedHeader),
					))
				})

				It("should not error", func() {
					Expect(fetchErr).NotTo(HaveOccurred())
				})

				It("should return a readCloser that streams the file", func() {
					Expect(fetchedFile).NotTo(BeNil())
					Expect(fetchedFileSize).To(BeNumerically("==", maxSizeInBytes/2))
					Expect(ioutil.ReadAll(fetchedFile)).To(Equal(downloadContent))
				})

				It("should return a file within the cache", func() {
					Expect(ioutil.ReadDir(cachedPath)).To(HaveLen(1))
				})

				It("should remove any temporary assets generated along the way", func() {
					Expect(ioutil.ReadDir(uncachedPath)).To(HaveLen(0))
				})

				Describe("downloading with a transformer", func() {
					BeforeEach(func() {
						transformer = func(source string, destination string) (int64, error) {
							err := ioutil.WriteFile(destination, []byte("hello tmp"), 0644)
							Expect(err).NotTo(HaveOccurred())

							return 100, err
						}
						cachedDownloader = cacheddownloader.New(uncachedPath, downloader, cache, transformer)
					})

					It("passes the download through the transformer", func() {
						Expect(fetchErr).NotTo(HaveOccurred())

						content, err := ioutil.ReadAll(fetchedFile)
						Expect(err).NotTo(HaveOccurred())
						Expect(string(content)).To(Equal("hello tmp"))
					})
				})
			})

			Context("when the download succeeds but does not have an ETag", func() {
				BeforeEach(func() {
					downloadContent = []byte(strings.Repeat("7", int(maxSizeInBytes/2)))
					server.AppendHandlers(ghttp.CombineHandlers(
						ghttp.VerifyRequest("GET", "/my_file"),
						http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
							Expect(req.Header.Get("If-None-Match")).To(BeEmpty())
						}),
						ghttp.RespondWith(http.StatusOK, string(downloadContent)),
					))
				})

				It("should not error", func() {
					Expect(fetchErr).NotTo(HaveOccurred())
				})

				It("should return a readCloser that streams the file", func() {
					Expect(fetchedFile).NotTo(BeNil())
					Expect(ioutil.ReadAll(fetchedFile)).To(Equal(downloadContent))
				})

				It("should not store the file", func() {
					fetchedFile.Close()
					Expect(ioutil.ReadDir(cachedPath)).To(HaveLen(0))
					Expect(ioutil.ReadDir(uncachedPath)).To(HaveLen(0))
				})
			})

			Context("when the download fails", func() {
				BeforeEach(func() {
					server.AllowUnhandledRequests = true //will 500 for any attempted requests
				})

				It("should return an error and no file", func() {
					Expect(fetchedFile).To(BeNil())
					Expect(fetchErr).To(HaveOccurred())
				})

				It("should clean up after itself", func() {
					Expect(ioutil.ReadDir(cachedPath)).To(HaveLen(0))
					Expect(ioutil.ReadDir(uncachedPath)).To(HaveLen(0))
				})
			})
		})

		Context("when the file is already on disk in the cache", func() {
			Context("and it is a directory zip", func() {
				var fileContent []byte
				var status int
				var downloadContent string

				BeforeEach(func() {
					status = http.StatusOK
					fileContent = []byte("now you see it")

					server.AppendHandlers(ghttp.CombineHandlers(
						ghttp.VerifyRequest("GET", "/my_file"),
						ghttp.RespondWith(http.StatusOK, string(fileContent), returnedHeader),
					))

					f, s, _ := cachedDownloader.Fetch(logger, url, cacheKey, checksum, cancelChan)
					defer f.Close()
					Expect(s).To(BeNumerically("==", len(fileContent)))

					downloadContent = "now you don't"

					etag := "second-round-etag"
					returnedHeader.Set("ETag", etag)

					server.AppendHandlers(ghttp.CombineHandlers(
						ghttp.VerifyRequest("GET", "/my_file"),
						http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
							Expect(req.Header.Get("If-None-Match")).To(Equal("my-original-etag"))
						}),
						ghttp.RespondWithPtr(&status, &downloadContent, returnedHeader),
					))
				})

				It("should perform the request with the correct modified headers", func() {
					cachedDownloader.Fetch(logger, url, cacheKey, checksum, cancelChan)
					Expect(server.ReceivedRequests()).To(HaveLen(2))
				})

				Context("if the file has been modified", func() {
					BeforeEach(func() {
						status = http.StatusOK
					})

					It("should redownload the file", func() {
						f, _, _ := cachedDownloader.Fetch(logger, url, cacheKey, checksum, cancelChan)
						defer f.Close()

						paths, _ := filepath.Glob(cacheFilePath + "*")
						Expect(ioutil.ReadFile(paths[0])).To(Equal([]byte(downloadContent)))
					})

					It("should return a readcloser pointing to the file", func() {
						file, fileSize, err := cachedDownloader.Fetch(logger, url, cacheKey, checksum, cancelChan)
						Expect(err).NotTo(HaveOccurred())
						Expect(ioutil.ReadAll(file)).To(Equal([]byte(downloadContent)))
						Expect(fileSize).To(BeNumerically("==", len(downloadContent)))
					})

					It("should have put the file in the cache", func() {
						f, s, err := cachedDownloader.Fetch(logger, url, cacheKey, checksum, cancelChan)
						f.Close()
						Expect(err).NotTo(HaveOccurred())
						Expect(ioutil.ReadDir(cachedPath)).To(HaveLen(1))
						Expect(ioutil.ReadDir(uncachedPath)).To(HaveLen(0))
						Expect(s).To(BeNumerically("==", len(downloadContent)))
					})
				})

				Context("if the file has been modified, but the new file has no etag", func() {
					BeforeEach(func() {
						status = http.StatusOK
						returnedHeader.Del("ETag")
					})

					It("should return a readcloser pointing to the file", func() {
						file, fileSize, err := cachedDownloader.Fetch(logger, url, cacheKey, checksum, cancelChan)
						Expect(err).NotTo(HaveOccurred())
						Expect(ioutil.ReadAll(file)).To(Equal([]byte(downloadContent)))
						Expect(fileSize).To(BeNumerically("==", len(downloadContent)))
					})

					It("should have removed the file from the cache", func() {
						f, s, err := cachedDownloader.Fetch(logger, url, cacheKey, checksum, cancelChan)
						f.Close()
						Expect(err).NotTo(HaveOccurred())
						Expect(ioutil.ReadDir(cachedPath)).To(HaveLen(0))
						Expect(ioutil.ReadDir(uncachedPath)).To(HaveLen(0))
						Expect(s).To(BeNumerically("==", len(downloadContent)))
					})
				})

				Context("if the file has not been modified", func() {
					BeforeEach(func() {
						status = http.StatusNotModified
					})

					It("should not redownload the file", func() {
						f, s, err := cachedDownloader.Fetch(logger, url, cacheKey, checksum, cancelChan)
						Expect(err).NotTo(HaveOccurred())
						defer f.Close()

						paths, _ := filepath.Glob(cacheFilePath + "*")
						Expect(ioutil.ReadFile(paths[0])).To(Equal(fileContent))
						Expect(s).To(BeZero())
					})

					It("should return a readcloser pointing to the file", func() {
						file, _, err := cachedDownloader.Fetch(logger, url, cacheKey, checksum, cancelChan)
						Expect(err).NotTo(HaveOccurred())
						Expect(ioutil.ReadAll(file)).To(Equal(fileContent))
					})
				})
			})
		})

		Context("when the file size exceeds the total available cache size", func() {
			BeforeEach(func() {
				downloadContent = []byte(strings.Repeat("7", int(maxSizeInBytes*3)))

				server.AppendHandlers(ghttp.CombineHandlers(
					ghttp.VerifyRequest("GET", "/my_file"),
					ghttp.RespondWith(http.StatusOK, string(downloadContent), returnedHeader),
				))
				server.AppendHandlers(ghttp.CombineHandlers(
					ghttp.VerifyRequest("GET", "/my_file"),
					ghttp.RespondWith(http.StatusOK, string(downloadContent), returnedHeader),
				))

				file, fileSize, err = cachedDownloader.Fetch(logger, url, cacheKey, checksum, cancelChan)
			})

			It("should not error", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			It("should return a readCloser that streams the file", func() {
				Expect(file).NotTo(BeNil())
				Expect(ioutil.ReadAll(file)).To(Equal(downloadContent))
				Expect(fileSize).To(BeNumerically("==", maxSizeInBytes*3))
			})

			It("should put the file in the uncached path, then delete it", func() {
				err := file.Close()
				Expect(err).NotTo(HaveOccurred())

				_, _, fetchErr := cachedDownloader.Fetch(logger, url, "new-cache-key", checksum, cancelChan)
				Expect(fetchErr).NotTo(HaveOccurred())

				Expect(ioutil.ReadDir(cachedPath)).To(HaveLen(1))
				Expect(ioutil.ReadDir(uncachedPath)).To(HaveLen(0))
			})

			Context("when the file is downloaded the second time", func() {
				BeforeEach(func() {
					err = file.Close()
					Expect(err).NotTo(HaveOccurred())

					server.AppendHandlers(ghttp.CombineHandlers(
						ghttp.VerifyRequest("GET", "/my_file"),
						http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
							Expect(req.Header.Get("If-None-Match")).NotTo(BeEmpty())
						}),
						ghttp.RespondWith(http.StatusOK, string(downloadContent), returnedHeader),
					))

					file, _, err = cachedDownloader.Fetch(logger, url, cacheKey, checksum, cancelChan)
				})

				It("should not error", func() {
					Expect(err).NotTo(HaveOccurred())
				})

				It("should return a readCloser that streams the file", func() {
					Expect(file).NotTo(BeNil())
					Expect(ioutil.ReadAll(file)).To(Equal(downloadContent))
				})
			})
		})

		Context("when the cache is full", func() {
			var closeAfterDownload bool
			var unclosedFiles []io.ReadCloser

			fetchFileOfSize := func(name string, size int) {
				downloadContent = []byte(strings.Repeat("7", size))
				url, _ = Url.Parse(server.URL() + "/" + name)
				server.AppendHandlers(ghttp.CombineHandlers(
					ghttp.VerifyRequest("GET", "/"+name),
					ghttp.RespondWith(http.StatusOK, string(downloadContent), returnedHeader),
				))

				cachedFile, _, err := cachedDownloader.Fetch(logger, url, name, checksum, cancelChan)
				Expect(err).NotTo(HaveOccurred())
				Expect(ioutil.ReadAll(cachedFile)).To(Equal(downloadContent))
				if closeAfterDownload {
					cachedFile.Close()
				} else {
					// *note*: the following is absolutely necessary since the
					// ReadClosers we get back have gc finalizers and could potentially
					// close the file if they are garbage collected and make the test
					// flaky
					unclosedFiles = append(unclosedFiles, cachedFile)
				}
			}

			BeforeEach(func() {
				closeAfterDownload = true
			})

			AfterEach(func() {
				for _, f := range unclosedFiles {
					f.Close()
				}
			})

			JustBeforeEach(func() {
				fetchFileOfSize("A", int(maxSizeInBytes/4))
				fetchFileOfSize("B", int(maxSizeInBytes/4))
				fetchFileOfSize("C", int(maxSizeInBytes/4))
			})

			Context("when the cache entries are not in use", func() {
				BeforeEach(func() {
					closeAfterDownload = true
				})

				JustBeforeEach(func() {
					fetchFileOfSize("D", int(maxSizeInBytes/2)+1)
				})

				It("deletes the oldest cached files until there is space", func() {
					Expect(ioutil.ReadDir(cachedPath)).To(HaveLen(2))

					Expect(filepath.Glob(filepath.Join(cachedPath, computeMd5("A")+"*"))).To(HaveLen(0))
					Expect(filepath.Glob(filepath.Join(cachedPath, computeMd5("B")+"*"))).To(HaveLen(0))
					Expect(filepath.Glob(filepath.Join(cachedPath, computeMd5("C")+"*"))).To(HaveLen(1))
					Expect(filepath.Glob(filepath.Join(cachedPath, computeMd5("D")+"*"))).To(HaveLen(1))
				})
			})

			Context("when the cache entries are still in use", func() {
				BeforeEach(func() {
					closeAfterDownload = false
				})

				JustBeforeEach(func() {
					fetchFileOfSize("D", int(maxSizeInBytes/2)+1)
				})

				It("does not delete the cache entries from disk", func() {
					Expect(ioutil.ReadDir(cachedPath)).To(HaveLen(4))

					Expect(filepath.Glob(filepath.Join(cachedPath, computeMd5("A")+"*"))).To(HaveLen(1))
					Expect(filepath.Glob(filepath.Join(cachedPath, computeMd5("B")+"*"))).To(HaveLen(1))
					Expect(filepath.Glob(filepath.Join(cachedPath, computeMd5("C")+"*"))).To(HaveLen(1))
					Expect(filepath.Glob(filepath.Join(cachedPath, computeMd5("D")+"*"))).To(HaveLen(1))
				})

				Context("and an earlier cache key was fetched", func() {
					var cachedFile io.ReadCloser

					JustBeforeEach(func() {
						name := "A"
						url, _ := Url.Parse(server.URL() + "/" + name)
						server.AppendHandlers(ghttp.CombineHandlers(
							ghttp.VerifyRequest("GET", "/"+name),
							ghttp.VerifyHeader(http.Header{"If-None-Match": []string{"my-original-etag"}}),
							ghttp.RespondWith(http.StatusNotModified, nil),
						))

						var err error
						cachedFile, _, err = cachedDownloader.Fetch(logger, url, name, checksum, cancelChan)
						Expect(err).NotTo(HaveOccurred())
					})

					AfterEach(func() {
						cachedFile.Close()
					})

					It("does not fetch the file again", func() {
						Expect(filepath.Glob(filepath.Join(cachedPath, computeMd5("A")+"*"))).To(HaveLen(1))
					})
				})
			})

			Describe("when one of the files has just been read", func() {
				JustBeforeEach(func() {
					server.AppendHandlers(ghttp.CombineHandlers(
						ghttp.VerifyRequest("GET", "/A"),
						ghttp.RespondWith(http.StatusNotModified, "", returnedHeader),
					))

					url, _ = Url.Parse(server.URL() + "/A")
					// windows timer increments every 15.6ms, without the sleep
					// A, B & C will sometimes have the same timestamp
					time.Sleep(16 * time.Millisecond)
					cachedDownloader.Fetch(logger, url, "A", checksum, cancelChan)
				})

				It("considers that file to be the newest", func() {
					//try to add a file that has size larger
					fetchFileOfSize("D", int(maxSizeInBytes/2)+1)

					Expect(ioutil.ReadDir(cachedPath)).To(HaveLen(2))

					Expect(filepath.Glob(filepath.Join(cachedPath, computeMd5("A")+"*"))).To(HaveLen(1))
					Expect(filepath.Glob(filepath.Join(cachedPath, computeMd5("B")+"*"))).To(HaveLen(0))
					Expect(filepath.Glob(filepath.Join(cachedPath, computeMd5("C")+"*"))).To(HaveLen(0))
					Expect(filepath.Glob(filepath.Join(cachedPath, computeMd5("D")+"*"))).To(HaveLen(1))
				})
			})
		})
	})

	Describe("rate limiting", func() {

		Context("when multiple, concurrent requests occur", func() {
			var barrier chan interface{}
			var results chan bool

			BeforeEach(func() {
				barrier = make(chan interface{}, 1)
				results = make(chan bool, 1)

				downloadContent = []byte(strings.Repeat("7", int(maxSizeInBytes/2)))
				server.AppendHandlers(
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("GET", "/my_file"),
						http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
							barrier <- nil
							Consistently(results, .5).ShouldNot(Receive())
						}),
						ghttp.RespondWith(http.StatusOK, string(downloadContent)),
					),
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("GET", "/my_file"),
						http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
							results <- true
						}),
						ghttp.RespondWith(http.StatusOK, string(downloadContent)),
					),
				)
			})

			It("processes one at a time", func() {
				go func() {
					cachedDownloader.Fetch(logger, url, cacheKey, checksum, cancelChan)
					barrier <- nil
				}()
				<-barrier
				cachedDownloader.Fetch(logger, url, cacheKey, checksum, cancelChan)
				<-barrier
			})
		})

		Context("when cancelling", func() {
			var requestInitiated chan struct{}
			var completeRequest chan struct{}

			BeforeEach(func() {
				requestInitiated = make(chan struct{})
				completeRequest = make(chan struct{})

				server.AllowUnhandledRequests = true
				server.AppendHandlers(
					http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
						requestInitiated <- struct{}{}
						<-completeRequest
						w.Write([]byte("response data..."))
					}),
				)

				serverUrl := server.URL() + "/somepath"
				url, _ = url.Parse(serverUrl)
			})

			It("stops waiting", func() {
				errs := make(chan error)

				go func() {
					_, _, err := cachedDownloader.Fetch(logger, url, cacheKey, checksum, make(chan struct{}))
					errs <- err
				}()
				Eventually(requestInitiated).Should(Receive())

				close(cancelChan)

				_, _, err := cachedDownloader.Fetch(logger, url, cacheKey, checksum, cancelChan)
				Expect(err).To(BeAssignableToTypeOf(cacheddownloader.NewDownloadCancelledError("", 0, cacheddownloader.NoBytesReceived)))

				close(completeRequest)
				Eventually(errs).Should(Receive(BeNil()))
			})
		})
	})

	Describe("FetchAsDirectory", func() {
		var returnedHeader http.Header

		BeforeEach(func() {
			returnedHeader = http.Header{}
			returnedHeader.Set("ETag", "my-original-etag")
		})

		It("returns a NotCacheable error if the cache key is empty", func() {
			_, _, fetchErr := cachedDownloader.FetchAsDirectory(logger, url, "", checksum, cancelChan)
			Expect(fetchErr).To(Equal(cacheddownloader.NotCacheable))
		})

		Context("when the file is not in the cache", func() {
			var (
				fetchedDir     string
				fetchedDirSize int64
				fetchErr       error
			)

			JustBeforeEach(func() {
				fetchedDir, fetchedDirSize, fetchErr = cachedDownloader.FetchAsDirectory(logger, url, cacheKey, checksum, cancelChan)
			})

			Context("when the download succeeds", func() {
				BeforeEach(func() {
					downloadContent = createTarBuffer("test content", 0).Bytes()
					server.AppendHandlers(ghttp.CombineHandlers(
						ghttp.VerifyRequest("GET", "/my_file"),
						http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
							Expect(req.Header.Get("If-None-Match")).To(BeEmpty())
						}),
						ghttp.RespondWith(http.StatusOK, string(downloadContent), returnedHeader),
					))
				})

				It("should not error", func() {
					Expect(fetchErr).NotTo(HaveOccurred())
				})

				It("should return a directory", func() {
					Expect(fetchedDir).NotTo(BeEmpty())
					fileInfo, err := os.Stat(fetchedDir)
					Expect(err).NotTo(HaveOccurred())
					Expect(fileInfo.IsDir()).To(BeTrue())
				})

				It("returns the total number of bytes downloaded", func() {
					Expect(fetchedDirSize).To(Equal(int64(len(downloadContent))))
				})

				It("should store the directory in the cache", func() {
					Expect(ioutil.ReadDir(cachedPath)).To(HaveLen(1))
				})

				It("should remove any temporary assets generated along the way", func() {
					Expect(ioutil.ReadDir(uncachedPath)).To(HaveLen(0))
				})
			})

			Context("when the download succeeds but does not have an ETag", func() {
				BeforeEach(func() {
					downloadContent = createTarBuffer("test content", 0).Bytes()
					server.AppendHandlers(ghttp.CombineHandlers(
						ghttp.VerifyRequest("GET", "/my_file"),
						http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
							Expect(req.Header.Get("If-None-Match")).To(BeEmpty())
						}),
						ghttp.RespondWith(http.StatusOK, string(downloadContent)),
					))
				})

				It("should error", func() {
					Expect(fetchErr).To(HaveOccurred())
				})
			})

			Context("when the download fails", func() {
				BeforeEach(func() {
					server.AllowUnhandledRequests = true //will 500 for any attempted requests
				})

				It("should return an error and no file", func() {
					Expect(fetchedDir).To(BeEmpty())
					Expect(fetchErr).To(HaveOccurred())
				})

				It("should clean up after itself", func() {
					Expect(ioutil.ReadDir(cachedPath)).To(HaveLen(0))
					Expect(ioutil.ReadDir(uncachedPath)).To(HaveLen(0))
				})
			})
		})

		Context("when the file is already on disk in the cache", func() {
			var fileContent []byte
			var status int
			var downloadContent []byte

			BeforeEach(func() {
				status = http.StatusOK
				fileContent = createTarBuffer("now you see it", 0).Bytes()

				server.AppendHandlers(ghttp.CombineHandlers(
					ghttp.VerifyRequest("GET", "/my_file"),
					ghttp.RespondWith(http.StatusOK, string(fileContent), returnedHeader),
				))

				dir, _, _ := cachedDownloader.FetchAsDirectory(logger, url, cacheKey, checksum, cancelChan)
				err := cachedDownloader.CloseDirectory(logger, cacheKey, dir)
				Expect(err).ToNot(HaveOccurred())

				downloadContent = createTarBuffer("now you don't", 0).Bytes()

				etag := "second-round-etag"
				returnedHeader.Set("ETag", etag)

				server.AppendHandlers(ghttp.CombineHandlers(
					ghttp.VerifyRequest("GET", "/my_file"),
					http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
						Expect(req.Header.Get("If-None-Match")).To(Equal("my-original-etag"))
					}),
					ghttp.RespondWithPtr(&status, &downloadContent, returnedHeader),
				))
			})

			It("should perform the request with the correct modified headers", func() {
				dir, _, _ := cachedDownloader.FetchAsDirectory(logger, url, cacheKey, checksum, cancelChan)
				err := cachedDownloader.CloseDirectory(logger, cacheKey, dir)
				Expect(err).ToNot(HaveOccurred())
				Expect(server.ReceivedRequests()).To(HaveLen(2))
			})

			Context("if the file has been modified", func() {
				BeforeEach(func() {
					status = http.StatusOK
				})

				It("should redownload the file", func() {
					dir, _, _ := cachedDownloader.FetchAsDirectory(logger, url, cacheKey, checksum, cancelChan)
					err := cachedDownloader.CloseDirectory(logger, cacheKey, dir)
					Expect(err).ToNot(HaveOccurred())
				})

				It("should return a string pointing to the file", func() {
					dir, _, err := cachedDownloader.FetchAsDirectory(logger, url, cacheKey, checksum, cancelChan)
					Expect(err).NotTo(HaveOccurred())
					fileInfo, err := os.Stat(dir)
					Expect(err).NotTo(HaveOccurred())
					Expect(fileInfo.IsDir()).To(BeTrue())
				})

				It("should have put the file in the cache", func() {
					dir, _, err := cachedDownloader.FetchAsDirectory(logger, url, cacheKey, checksum, cancelChan)
					Expect(err).NotTo(HaveOccurred())
					err = cachedDownloader.CloseDirectory(logger, cacheKey, dir)
					Expect(err).ToNot(HaveOccurred())
					Expect(ioutil.ReadDir(cachedPath)).To(HaveLen(1))
					Expect(ioutil.ReadDir(uncachedPath)).To(HaveLen(0))
				})
			})

			Context("if the file has been modified, but the new file has no etag", func() {
				var fetchErr error
				var fetchDir string

				BeforeEach(func() {
					status = http.StatusOK
					returnedHeader.Del("ETag")
					fetchDir, _, fetchErr = cachedDownloader.FetchAsDirectory(logger, url, cacheKey, checksum, cancelChan)
				})

				It("should error", func() {
					Expect(fetchErr).To(HaveOccurred())
				})

				It("should have removed the file from the cache", func() {
					cachedDownloader.CloseDirectory(logger, cacheKey, fetchDir)
					Expect(fetchDir).NotTo(BeADirectory())
				})
			})

			Context("if the file has not been modified", func() {
				BeforeEach(func() {
					status = http.StatusNotModified
				})

				It("should not redownload the file", func() {
					dir, _, err := cachedDownloader.FetchAsDirectory(logger, url, cacheKey, checksum, cancelChan)
					Expect(err).NotTo(HaveOccurred())
					Expect(ioutil.ReadDir(cachedPath)).To(HaveLen(1))
					cachedDownloader.CloseDirectory(logger, cacheKey, dir)
				})

				It("should return a directory pointing to the file", func() {
					dir, _, err := cachedDownloader.FetchAsDirectory(logger, url, cacheKey, checksum, cancelChan)
					Expect(err).NotTo(HaveOccurred())
					fileInfo, err := os.Stat(dir)
					Expect(err).NotTo(HaveOccurred())
					Expect(fileInfo.IsDir()).To(BeTrue())
				})
			})

		})

		Context("when the file size exceeds the total available cache size", func() {
			var fileContent []byte
			BeforeEach(func() {
				fileContent = createTarBuffer("Test small string", 100).Bytes()

				server.AppendHandlers(ghttp.CombineHandlers(
					ghttp.VerifyRequest("GET", "/my_file"),
					ghttp.RespondWith(http.StatusOK, string(fileContent), returnedHeader),
				))
				server.AppendHandlers(ghttp.CombineHandlers(
					ghttp.VerifyRequest("GET", "/my_file"),
					ghttp.RespondWith(http.StatusOK, string(fileContent), returnedHeader),
				))

				dir, _, err = cachedDownloader.FetchAsDirectory(logger, url, cacheKey, checksum, cancelChan)
			})

			It("should successfully download", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			It("should have made room when adding to the cache", func() {
				cachedDownloader.CloseDirectory(logger, cacheKey, dir)
				_, _, fetchErr := cachedDownloader.FetchAsDirectory(logger, url, "new-cache-key", checksum, cancelChan)
				Expect(fetchErr).NotTo(HaveOccurred())
				Expect(dir).NotTo(BeADirectory())
			})
		})

		Context("when the cache is full", func() {
			var fileContent []byte

			fetchDir := func(name string, numFiles int) (string, int64, error) {
				fileContent = createTarBuffer("Test small string", numFiles).Bytes()

				url, _ = Url.Parse(server.URL() + "/" + name)
				server.AppendHandlers(ghttp.CombineHandlers(
					ghttp.VerifyRequest("GET", "/"+name),
					ghttp.RespondWith(http.StatusOK, string(fileContent), returnedHeader),
				))

				return cachedDownloader.FetchAsDirectory(logger, url, name, checksum, cancelChan)
			}

			fetchDirOfSize := func(name string, numFiles int) {
				dir, _, err = fetchDir(name, numFiles)

				Expect(err).NotTo(HaveOccurred())
				fileInfo, err := os.Stat(dir)
				Expect(err).NotTo(HaveOccurred())
				Expect(fileInfo.IsDir()).To(BeTrue())
				cachedDownloader.CloseDirectory(logger, name, dir)
			}

			BeforeEach(func() {
				fetchDirOfSize("A", 0)
				fetchDirOfSize("B", 0)
				fetchDirOfSize("C", 0)
			})

			It("should have all entries in the cache", func() {
				Expect(ioutil.ReadDir(cachedPath)).To(HaveLen(3))
			})

			It("deletes the oldest cached files until there is space", func() {
				//try to add a file that has size larger (this creates a file that is 23 GB...)
				fetchDirOfSize("D", 18)

				Expect(ioutil.ReadDir(cachedPath)).To(HaveLen(2))

				Expect(filepath.Glob(filepath.Join(cachedPath, computeMd5("A")+"*"))).To(HaveLen(0))
				Expect(filepath.Glob(filepath.Join(cachedPath, computeMd5("B")+"*"))).To(HaveLen(0))
				Expect(filepath.Glob(filepath.Join(cachedPath, computeMd5("C")+"*"))).To(HaveLen(1))
				Expect(filepath.Glob(filepath.Join(cachedPath, computeMd5("D")+"*"))).To(HaveLen(1))
			})

			Context("and cannot delete any items", func() {
				It("succeeds even if the downloaded file is larger than the resources left", func() {
					_, downloadedBytes, err := fetchDir("test", 30)
					Expect(err).NotTo(HaveOccurred())
					Expect(downloadedBytes).To(Equal(int64(len(fileContent))))
				})
			})

			Describe("when one of the files has just been read", func() {
				BeforeEach(func() {
					server.AppendHandlers(ghttp.CombineHandlers(
						ghttp.VerifyRequest("GET", "/A"),
						ghttp.RespondWith(http.StatusNotModified, "", returnedHeader),
					))

					url, _ = Url.Parse(server.URL() + "/A")
					// windows timer increments every 15.6ms, without the sleep
					// A, B & C will sometimes have the same timestamp
					time.Sleep(16 * time.Millisecond)
					cachedDownloader.FetchAsDirectory(logger, url, "A", checksum, cancelChan)
				})

				It("considers that file to be the newest", func() {
					//try to add a file that has size larger
					fetchDirOfSize("D", 18)

					Expect(ioutil.ReadDir(cachedPath)).To(HaveLen(2))

					Expect(filepath.Glob(filepath.Join(cachedPath, computeMd5("A")+"*"))).To(HaveLen(1))
					Expect(filepath.Glob(filepath.Join(cachedPath, computeMd5("B")+"*"))).To(HaveLen(0))
					Expect(filepath.Glob(filepath.Join(cachedPath, computeMd5("C")+"*"))).To(HaveLen(0))
					Expect(filepath.Glob(filepath.Join(cachedPath, computeMd5("D")+"*"))).To(HaveLen(1))
				})
			})
		})
	})

	Describe("When doing a Fetch and then a FetchAsDirectory", func() {
		var returnedHeader http.Header

		BeforeEach(func() {
			returnedHeader = http.Header{}
			returnedHeader.Set("ETag", "my-original-etag")
		})

		Context("when the archive is fetched", func() {
			var (
				fetchedFile io.ReadCloser
				fetchErr    error
				status      int
			)

			JustBeforeEach(func() {
				status = http.StatusOK

				server.AppendHandlers(ghttp.CombineHandlers(
					ghttp.VerifyRequest("GET", "/my_file"),
					ghttp.RespondWith(http.StatusOK, string(downloadContent), returnedHeader),
				))

				cache = cacheddownloader.NewCache(cachedPath, maxSizeInBytes)
				cachedDownloader = cacheddownloader.New(uncachedPath, downloader, cache, cacheddownloader.TarTransform)

				fetchedFile, _, fetchErr = cachedDownloader.Fetch(logger, url, cacheKey, checksum, cancelChan)
				Expect(fetchErr).NotTo(HaveOccurred())
				Expect(fetchedFile).NotTo(BeNil())
				Expect(ioutil.ReadAll(fetchedFile)).To(Equal(downloadContent))
				fetchedFile.Close()
			})

			Context("then is fetched with FetchAsDirecory", func() {
				var (
					fetchedDir  string
					fetchDirErr error
				)

				JustBeforeEach(func() {
					server.AppendHandlers(ghttp.CombineHandlers(
						ghttp.VerifyRequest("GET", "/my_file"),
						http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
							Expect(req.Header.Get("If-None-Match")).To(Equal("my-original-etag"))
						}),
						ghttp.RespondWithPtr(&status, &downloadContent, returnedHeader),
					))
					fetchedDir, _, fetchDirErr = cachedDownloader.FetchAsDirectory(logger, url, cacheKey, checksum, cancelChan)
				})

				Context("the content will fit in the cache on expansion", func() {
					BeforeEach(func() {
						downloadContent = createTarBuffer("test content", 0).Bytes()
					})

					It("should not error", func() {
						Expect(fetchDirErr).NotTo(HaveOccurred())
						Expect(fetchedDir).NotTo(BeEmpty())
						fileInfo, err := os.Stat(fetchedDir)
						Expect(err).NotTo(HaveOccurred())
						Expect(fileInfo.IsDir()).To(BeTrue())
						Expect(ioutil.ReadDir(cachedPath)).To(HaveLen(1))
						Expect(ioutil.ReadDir(uncachedPath)).To(HaveLen(0))
					})
				})

				Context("the content will NOT fit in the cache on expansion", func() {
					BeforeEach(func() {
						downloadContent = createTarBuffer("test content", 12).Bytes()
					})

					It("should not error", func() {
						Expect(fetchDirErr).NotTo(HaveOccurred())
						Expect(fetchedDir).To(BeADirectory())
					})
				})

				Context("the content will fit in the cache on expansion if we make room", func() {
					BeforeEach(func() {
						fillerContent := []byte(strings.Repeat("7", int(maxSizeInBytes/2)))
						urlFiller, _ := Url.Parse(server.URL() + "/filler")
						server.AppendHandlers(ghttp.CombineHandlers(
							ghttp.VerifyRequest("GET", "/filler"),
							ghttp.RespondWith(http.StatusOK, string(fillerContent), returnedHeader),
						))

						cachedFile, _, err := cachedDownloader.Fetch(logger, urlFiller, "filler", checksum, cancelChan)
						Expect(err).NotTo(HaveOccurred())
						Expect(ioutil.ReadAll(cachedFile)).To(Equal(fillerContent))
						cachedFile.Close()

						downloadContent = createTarBuffer("test content", 10).Bytes()
					})

					It("should not error", func() {
						Expect(fetchDirErr).NotTo(HaveOccurred())
						Expect(fetchedDir).NotTo(BeEmpty())
						fileInfo, err := os.Stat(fetchedDir)
						Expect(err).NotTo(HaveOccurred())
						Expect(fileInfo.IsDir()).To(BeTrue())
						Expect(ioutil.ReadDir(cachedPath)).To(HaveLen(2))
						Expect(ioutil.ReadDir(uncachedPath)).To(HaveLen(0))
					})
				})
			})
		})
	})

	Describe("When doing a FetchAsDirectory and then a Fetch", func() {
		var returnedHeader http.Header

		BeforeEach(func() {
			returnedHeader = http.Header{}
			returnedHeader.Set("ETag", "my-original-etag")
		})

		Context("when the archive is fetched", func() {
			var (
				fetchedFile io.ReadCloser
				fetchErr    error
				status      int
			)

			JustBeforeEach(func() {
				status = http.StatusOK

				server.AppendHandlers(ghttp.CombineHandlers(
					ghttp.VerifyRequest("GET", "/my_file"),
					ghttp.RespondWith(http.StatusOK, string(downloadContent), returnedHeader),
				))

				dir, _, err := cachedDownloader.FetchAsDirectory(logger, url, cacheKey, checksum, cancelChan)
				Expect(err).NotTo(HaveOccurred())
				Expect(dir).NotTo(BeEmpty())
				fileInfo, err := os.Stat(dir)
				Expect(err).NotTo(HaveOccurred())
				Expect(fileInfo.IsDir()).To(BeTrue())
			})

			Context("then is fetched with Fetch", func() {
				BeforeEach(func() {
					cachedDownloader = cacheddownloader.New(uncachedPath, downloader, cache, cacheddownloader.TarTransform)
				})

				JustBeforeEach(func() {
					server.AppendHandlers(ghttp.CombineHandlers(
						ghttp.VerifyRequest("GET", "/my_file"),
						http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
							Expect(req.Header.Get("If-None-Match")).To(Equal("my-original-etag"))
						}),
						ghttp.RespondWithPtr(&status, &downloadContent, returnedHeader),
					))

					fetchedFile, _, fetchErr = cachedDownloader.Fetch(logger, url, cacheKey, checksum, cancelChan)
				})

				BeforeEach(func() {
					downloadContent = createTarBuffer("test content", 0).Bytes()
				})

				It("should not error", func() {
					Expect(fetchErr).NotTo(HaveOccurred())
					Expect(fetchedFile).NotTo(BeNil())
					Expect(ioutil.ReadAll(fetchedFile)).To(Equal(downloadContent))
					fetchedFile.Close()
				})
			})
		})
	})

	Describe("SaveState", func() {
		It("writes the cache to the persistent disk", func() {
			err := cachedDownloader.SaveState(logger)
			Expect(err).NotTo(HaveOccurred())
			saveStateFile := filepath.Join(cachedPath, "saved_cache.json")
			Expect(saveStateFile).To(BeARegularFile())
		})
	})

	Describe("RecoverState", func() {
		BeforeEach(func() {
			fileContent := []byte("now you see it")
			returnedHeader := http.Header{}
			returnedHeader.Set("ETag", "my-original-etag")

			server.AppendHandlers(ghttp.CombineHandlers(
				ghttp.VerifyRequest("GET", "/my_file"),
				ghttp.RespondWith(http.StatusOK, string(fileContent), returnedHeader),
			))

			file, size, err := cachedDownloader.Fetch(logger, url, cacheKey, checksum, cancelChan)
			defer file.Close()
			Expect(err).NotTo(HaveOccurred())
			Expect(size).NotTo(BeZero())

			err = cachedDownloader.SaveState(logger)
			Expect(err).NotTo(HaveOccurred())
		})

		Context("when the state file does not exist", func() {
			BeforeEach(func() {
				Expect(os.RemoveAll(cachedPath)).To(Succeed())
			})

			It("does not return an error", func() {
				cachedDownloader = cacheddownloader.New(uncachedPath, downloader, cache, transformer)

				err := cachedDownloader.RecoverState(logger)
				Expect(err).NotTo(HaveOccurred())
			})
		})

		Context("when the state file is corrupted", func() {
			BeforeEach(func() {
				saveStateFile := filepath.Join(cachedPath, "saved_cache.json")
				err := ioutil.WriteFile(saveStateFile, []byte("{\"foo\""), 0755)
				Expect(err).NotTo(HaveOccurred())
			})

			It("does not return an error", func() {
				cachedDownloader = cacheddownloader.New(uncachedPath, downloader, cache, transformer)

				err := cachedDownloader.RecoverState(logger)
				Expect(err).NotTo(HaveOccurred())
			})
		})

		Context("when additional files beyond the saved cache exist on disk", func() {
			var (
				extraFile string
				extraDir  string
			)

			BeforeEach(func() {
				extraFile = filepath.Join(cachedPath, "dummy_file")
				err := ioutil.WriteFile(extraFile, []byte("foo"), 0755)
				Expect(err).NotTo(HaveOccurred())

				extraDir = filepath.Join(cachedPath, "dummy_dir/nested_dummy_dir")
				err = os.MkdirAll(extraDir, 0755)
				Expect(err).NotTo(HaveOccurred())
				err = ioutil.WriteFile(filepath.Join(extraDir, "file"), []byte("foo"), 0755)
				Expect(err).NotTo(HaveOccurred())
			})

			It("should remove regular files", func() {
				cachedDownloader = cacheddownloader.New(uncachedPath, downloader, cache, transformer)
				Expect(cachedDownloader.RecoverState(logger)).To(Succeed())
				Expect(extraFile).NotTo(BeAnExistingFile())
			})

			It("should remove directories", func() {
				cachedDownloader = cacheddownloader.New(uncachedPath, downloader, cache, transformer)
				Expect(cachedDownloader.RecoverState(logger)).To(Succeed())
				Expect(extraDir).NotTo(BeADirectory())
			})

			Context("and saved_cache.json is missing", func() {
				BeforeEach(func() {
					err := os.Remove(path.Join(cachedPath, "saved_cache.json"))
					Expect(err).NotTo(HaveOccurred())
				})

				It("removes all entries in the cached path", func() {
					Expect(cachedDownloader.RecoverState(logger)).To(Succeed())
					files, err := ioutil.ReadDir(cachedPath)
					Expect(err).NotTo(HaveOccurred())
					Expect(files).To(HaveLen(1))
				})
			})
		})

		Context("when the cache size changes", func() {
			BeforeEach(func() {
				returnedHeader := http.Header{}
				returnedHeader.Set("ETag", "another-file-etag")
				fileContent := "a file with lots of content"

				server.AppendHandlers(ghttp.CombineHandlers(
					ghttp.VerifyRequest("GET", "/another_file"),
					ghttp.RespondWith(http.StatusOK, string(fileContent), returnedHeader),
				))

				anotherFileUrl, err := Url.Parse(server.URL() + "/another_file")
				Expect(err).NotTo(HaveOccurred())
				file, _, err := cachedDownloader.Fetch(logger, anotherFileUrl, "another-file-cache-key", checksum, cancelChan)
				Expect(err).NotTo(HaveOccurred())
				defer file.Close()

				Expect(cachedDownloader.SaveState(logger)).To(Succeed())

				maxSizeInBytes = 32
				cache = cacheddownloader.NewCache(cachedPath, maxSizeInBytes)
			})

			It("should evict old entries from the cache", func() {
				files, err := ioutil.ReadDir(cachedPath)
				Expect(err).NotTo(HaveOccurred())
				Expect(files).To(HaveLen(3))

				cachedDownloader = cacheddownloader.New(uncachedPath, downloader, cache, transformer)
				Expect(cachedDownloader.RecoverState(logger)).To(Succeed())

				files, err = ioutil.ReadDir(cachedPath)
				Expect(err).NotTo(HaveOccurred())
				Expect(files).To(HaveLen(1))
			})
		})

		It("recovers the cache from a saved state file", func() {
			server.AppendHandlers(ghttp.CombineHandlers(
				ghttp.VerifyRequest("GET", "/my_file"),
				ghttp.RespondWith(http.StatusNotModified, nil),
			))

			cachedDownloader = cacheddownloader.New(uncachedPath, downloader, cache, transformer)

			err := cachedDownloader.RecoverState(logger)
			Expect(err).NotTo(HaveOccurred())

			file, downloadSize, err := cachedDownloader.Fetch(logger, url, cacheKey, checksum, cancelChan)
			defer file.Close()
			Expect(err).NotTo(HaveOccurred())

			Expect(downloadSize).To(BeEquivalentTo(0))
		})

		Context("when an expanded directory is fetched", func() {
			var (
				path           string
				size           int64
				returnedHeader = http.Header{}
			)

			BeforeEach(func() {
				downloadContent = createTarBuffer("test content", 0).Bytes()
				url, err = Url.Parse(server.URL() + "/my_tar_file")
				Expect(err).NotTo(HaveOccurred())
				cacheKey = "my-tar-file-cache-key"

				returnedHeader.Set("ETag", "tar-file-etag")
				server.AppendHandlers(ghttp.CombineHandlers(
					ghttp.VerifyRequest("GET", "/my_tar_file"),
					ghttp.RespondWith(http.StatusOK, string(downloadContent), returnedHeader),
				))
				var err error
				path, size, err = cachedDownloader.FetchAsDirectory(logger, url, cacheKey, checksum, cancelChan)
				Expect(err).NotTo(HaveOccurred())
			})

			Context("and cacheddownloader restarted", func() {
				JustBeforeEach(func() {
					Expect(cachedDownloader.SaveState(logger)).To(Succeed())
					cachedDownloader = cacheddownloader.New(uncachedPath, downloader, cache, transformer)
					Expect(cachedDownloader.RecoverState(logger)).To(Succeed())
				})

				It("leaves cache entry directories in the cache", func() {
					Expect(path).To(BeADirectory())
				})

				Context("and there isn't enough space for another directory to be fetched", func() {
					BeforeEach(func() {
						maxSizeInBytes = size * 3 // just enough space for the tar file currently fetched
						server.AppendHandlers(ghttp.CombineHandlers(
							ghttp.VerifyRequest("GET", "/my_tar_file"),
							ghttp.RespondWith(http.StatusNotModified, "", returnedHeader),
						))

						server.AppendHandlers(ghttp.CombineHandlers(
							ghttp.VerifyRequest("GET", "/my_tar_file"),
							ghttp.RespondWith(http.StatusOK, string(downloadContent), returnedHeader),
						))
					})

					Context("and another directory is fetched", func() {
						JustBeforeEach(func() {
							path, _, err = cachedDownloader.FetchAsDirectory(logger, url, cacheKey, checksum, cancelChan)
							Expect(err).NotTo(HaveOccurred())
							Expect(path).To(BeADirectory())
						})

						It("does not delete the directory that is referenced", func() {
							_, _, err := cachedDownloader.FetchAsDirectory(logger, url, "a-totally-different-cache-key", checksum, cancelChan)
							Expect(err).NotTo(HaveOccurred())
							Expect(path).To(BeADirectory())
						})
					})
				})
			})
		})
	})

	Context("when passing a CA cert pool", func() {
		It("uses it", func() {
			server.Close()
			server = ghttp.NewUnstartedServer()
			cert, err := tls.LoadX509KeyPair("fixtures/localhost.crt", "fixtures/localhost.key")
			Expect(err).NotTo(HaveOccurred())

			server.HTTPTestServer.TLS = &tls.Config{
				Certificates: []tls.Certificate{cert},
			}
			server.HTTPTestServer.StartTLS()

			header := http.Header{}
			header.Set("ETag", "foo")
			server.AppendHandlers(ghttp.CombineHandlers(
				ghttp.VerifyRequest("GET", "/my_file"),
				ghttp.RespondWith(http.StatusOK, "content", header),
			))

			caCertPool := systemcerts.NewCertPool()
			goodCACert, err := ioutil.ReadFile("fixtures/goodCA.crt")
			Expect(err).NotTo(HaveOccurred())
			ok := caCertPool.AppendCertsFromPEM(goodCACert)
			Expect(ok).To(BeTrue())

			url, err = Url.Parse(server.URL() + "/my_file")
			Expect(err).NotTo(HaveOccurred())

			tlsConfig := &tls.Config{
				RootCAs: caCertPool.AsX509CertPool(),
			}

			downloader = cacheddownloader.NewDownloader(time.Second, MAX_CONCURRENT_DOWNLOADS, tlsConfig)
			cachedDownloader = cacheddownloader.New(uncachedPath, downloader, cache, transformer)
			_, _, err = cachedDownloader.Fetch(logger, url, "", checksum, cancelChan)
			Expect(err).NotTo(HaveOccurred())
		})
	})
})

type constTransformer struct {
	file string
	size int64
	err  error
}

func (t constTransformer) ConstTransform(path string) (string, int64, error) {
	return t.file, t.size, t.err
}
