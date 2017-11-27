package cacheddownloader_test

import (
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"time"

	"code.cloudfoundry.org/cacheddownloader"
	"code.cloudfoundry.org/lager/lagertest"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Integration", func() {
	var (
		cache               *cacheddownloader.FileCache
		downloader          *cacheddownloader.Downloader
		cachedDownloader    cacheddownloader.CachedDownloader
		server              *httptest.Server
		serverPath          string
		cachedPath          string
		uncachedPath        string
		cacheMaxSizeInBytes int64         = 32000
		downloadTimeout     time.Duration = time.Second
		checksum            cacheddownloader.ChecksumInfoType
		url                 *url.URL
		logger              *lagertest.TestLogger
	)

	BeforeEach(func() {
		logger = lagertest.NewTestLogger("test")
		var err error

		serverPath, err = ioutil.TempDir("", "cached_downloader_integration_server")
		Expect(err).NotTo(HaveOccurred())

		cachedPath, err = ioutil.TempDir("", "cached_downloader_integration_cache")
		Expect(err).NotTo(HaveOccurred())

		uncachedPath, err = ioutil.TempDir("", "cached_downloader_integration_uncached")
		Expect(err).NotTo(HaveOccurred())

		handler := http.FileServer(http.Dir(serverPath))
		server = httptest.NewServer(handler)

		url, err = url.Parse(server.URL + "/file")
		Expect(err).NotTo(HaveOccurred())

		cache = cacheddownloader.NewCache(cachedPath, cacheMaxSizeInBytes)
		downloader = cacheddownloader.NewDownloader(downloadTimeout, 10, nil)
		cachedDownloader = cacheddownloader.New(uncachedPath, downloader, cache, cacheddownloader.NoopTransform)
	})

	AfterEach(func() {
		os.RemoveAll(serverPath)
		os.RemoveAll(cachedPath)
		os.RemoveAll(uncachedPath)
		server.Close()
	})

	fetch := func(fileToFetch string) ([]byte, time.Time) {
		url, err := url.Parse(server.URL + "/" + fileToFetch)
		Expect(err).NotTo(HaveOccurred())

		reader, _, err := cachedDownloader.Fetch(logger, url, "the-cache-key", checksum, make(chan struct{}))
		Expect(err).NotTo(HaveOccurred())
		defer reader.Close()

		readData, err := ioutil.ReadAll(reader)
		Expect(err).NotTo(HaveOccurred())

		cacheContents, err := ioutil.ReadDir(cachedPath)
		Expect(cacheContents).To(HaveLen(1))
		Expect(err).NotTo(HaveOccurred())

		content, err := ioutil.ReadFile(filepath.Join(cachedPath, cacheContents[0].Name()))
		Expect(err).NotTo(HaveOccurred())

		Expect(readData).To(Equal(content))

		return content, cacheContents[0].ModTime()
	}

	fetchAsDirectory := func(fileToFetch string) (string, time.Time) {
		url, err := url.Parse(server.URL + "/" + fileToFetch)
		Expect(err).NotTo(HaveOccurred())

		dirPath, _, err := cachedDownloader.FetchAsDirectory(logger, url, "tar-file-cache-key", checksum, make(chan struct{}))
		Expect(err).NotTo(HaveOccurred())
		defer func() {
			err := cachedDownloader.CloseDirectory(logger, "tar-file-cache-key", dirPath)
			Expect(err).NotTo(HaveOccurred())
		}()

		// For some reason the first stat changes the mod time on Windows.
		// Call this so that we have predictable mod times on Windows
		os.Stat(dirPath)

		cacheContents, err := ioutil.ReadDir(cachedPath)
		Expect(err).NotTo(HaveOccurred())
		Expect(cacheContents).To(HaveLen(1))

		// ReadDir sorts by file name, so the tarfile should come before the directory
		Expect(cacheContents[0].Mode().IsDir()).To(BeTrue())

		dirPathInCache := filepath.Join(cachedPath, cacheContents[0].Name())
		Expect(dirPath).To(Equal(dirPathInCache))

		return dirPath, cacheContents[0].ModTime()
	}

	Describe("Fetch", func() {
		It("caches downloads", func() {
			// touch a file on disk
			err := ioutil.WriteFile(filepath.Join(serverPath, "file"), []byte("a"), 0666)
			Expect(err).NotTo(HaveOccurred())

			// download file once
			content, modTimeBefore := fetch("file")
			Expect(content).To(Equal([]byte("a")))

			time.Sleep(time.Second)

			// download again should be cached
			content, modTimeAfter := fetch("file")
			Expect(content).To(Equal([]byte("a")))
			Expect(modTimeBefore).To(Equal(modTimeAfter))

			time.Sleep(time.Second)

			// touch file again
			err = ioutil.WriteFile(filepath.Join(serverPath, "file"), []byte("b"), 0666)
			Expect(err).NotTo(HaveOccurred())

			// download again and we should get a file containing "b"
			content, _ = fetch("file")
			Expect(content).To(Equal([]byte("b")))
		})
	})

	Describe("FetchAsDirectory", func() {
		It("caches downloads", func() {
			// create a valid tar file
			tarByteBuffer := createTarBuffer("original", 0)
			file, err := os.Create(filepath.Join(serverPath, "tarfile"))
			Expect(err).NotTo(HaveOccurred())
			_, err = tarByteBuffer.WriteTo(file)
			Expect(err).NotTo(HaveOccurred())

			// fetch directory once
			dirPath, modTimeBefore := fetchAsDirectory("tarfile")
			Expect(ioutil.ReadFile(filepath.Join(dirPath, "testdir/file.txt"))).To(Equal([]byte("original")))

			time.Sleep(time.Second)

			// download again should be cached
			dirPath, modTimeAfter := fetchAsDirectory("tarfile")
			Expect(ioutil.ReadFile(filepath.Join(dirPath, "testdir/file.txt"))).To(Equal([]byte("original")))
			Expect(modTimeBefore).To(Equal(modTimeAfter))

			time.Sleep(time.Second)

			// touch file again
			tarByteBuffer = createTarBuffer("modified", 0)
			file, err = os.Create(filepath.Join(serverPath, "tarfile"))
			Expect(err).NotTo(HaveOccurred())
			_, err = tarByteBuffer.WriteTo(file)
			Expect(err).NotTo(HaveOccurred())

			// download again and we should get an untarred file with modified contents
			dirPath, _ = fetchAsDirectory("tarfile")
			Expect(ioutil.ReadFile(filepath.Join(dirPath, "testdir/file.txt"))).To(Equal([]byte("modified")))
		})
	})
})
