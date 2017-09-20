package containerstore_test

import (
	"errors"
	"net/url"

	"code.cloudfoundry.org/cacheddownloader"
	"code.cloudfoundry.org/cacheddownloader/cacheddownloaderfakes"
	"code.cloudfoundry.org/executor"
	"code.cloudfoundry.org/executor/depot/containerstore"
	"code.cloudfoundry.org/executor/depot/log_streamer/fake_log_streamer"
	"code.cloudfoundry.org/garden"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
)

var _ = Describe("DependencyManager", func() {
	var (
		dependencyManager   containerstore.DependencyManager
		cache               *cacheddownloaderfakes.FakeCachedDownloader
		dependencies        []executor.CachedDependency
		logStreamer         *fake_log_streamer.FakeLogStreamer
		downloadRateLimiter chan struct{}
	)

	BeforeEach(func() {
		cache = &cacheddownloaderfakes.FakeCachedDownloader{}
		logStreamer = fake_log_streamer.NewFakeLogStreamer()
		downloadRateLimiter = make(chan struct{}, 2)
		dependencyManager = containerstore.NewDependencyManager(cache, downloadRateLimiter)
		dependencies = []executor.CachedDependency{
			{Name: "name-1", CacheKey: "cache-key-1", LogSource: "log-source-1", From: "https://user:pass@example.com:8080/download-1", To: "/var/data/buildpack-1"},
			{CacheKey: "cache-key-2", LogSource: "log-source-2", From: "http://example.com:1515/download-2", To: "/var/data/buildpack-2"},
		}
	})

	Context("when fetching all of the dependencies succeeds", func() {
		var bindMounts containerstore.BindMounts

		BeforeEach(func() {
			cache.FetchAsDirectoryReturns("/tmp/download/dependencies", 123, nil)
			var err error
			bindMounts, err = dependencyManager.DownloadCachedDependencies(logger, dependencies, logStreamer)
			Expect(err).NotTo(HaveOccurred())
		})

		It("uses the log source of the cached dependency", func() {
			Expect(logStreamer.WithSourceCallCount()).To(Equal(2))
			expectedSources := []string{
				"log-source-1",
				"log-source-2",
			}

			Expect(expectedSources).To(ContainElement(logStreamer.WithSourceArgsForCall(0)))
			Expect(expectedSources).To(ContainElement(logStreamer.WithSourceArgsForCall(1)))
		})

		It("emits the download log messages for downloads with names", func() {
			stdout := logStreamer.Stdout().(*gbytes.Buffer)
			Expect(stdout.Contents()).To(ContainSubstring("Downloading name-1..."))
			Expect(stdout.Contents()).To(ContainSubstring("Downloaded name-1 (123B)"))

			Expect(stdout.Contents()).ToNot(ContainSubstring("Downloading ..."))
			Expect(stdout.Contents()).ToNot(ContainSubstring("Downloaded  (123B)"))
		})

		It("returns the expected mount information", func() {
			expectedGardenMounts := []garden.BindMount{
				{SrcPath: "/tmp/download/dependencies", DstPath: "/var/data/buildpack-1", Mode: garden.BindMountModeRO, Origin: garden.BindMountOriginHost},
				{SrcPath: "/tmp/download/dependencies", DstPath: "/var/data/buildpack-2", Mode: garden.BindMountModeRO, Origin: garden.BindMountOriginHost},
			}

			expectedCacheKeys := []containerstore.BindMountCacheKey{
				{CacheKey: "cache-key-1", Dir: "/tmp/download/dependencies"},
				{CacheKey: "cache-key-2", Dir: "/tmp/download/dependencies"},
			}

			Expect(bindMounts.GardenBindMounts).To(ConsistOf(expectedGardenMounts))
			Expect(bindMounts.CacheKeys).To(ConsistOf(expectedCacheKeys))
		})

		It("downloads the directories", func() {
			Expect(cache.FetchAsDirectoryCallCount()).To(Equal(2))
			// Again order here will not necessisarily be preserved!
			expectedUrls := []url.URL{
				{Scheme: "https", Host: "example.com:8080", Path: "/download-1", User: url.UserPassword("user", "pass")},
				{Scheme: "http", Host: "example.com:1515", Path: "/download-2"},
			}
			expectedCacheKeys := []string{
				"cache-key-1",
				"cache-key-2",
			}

			downloadURLs := make([]url.URL, 2)
			cacheKeys := make([]string, 2)
			downloadUrl, cacheKey, _, _ := cache.FetchAsDirectoryArgsForCall(0)
			downloadURLs[0] = *downloadUrl
			cacheKeys[0] = cacheKey
			downloadUrl, cacheKey, _, _ = cache.FetchAsDirectoryArgsForCall(1)
			downloadURLs[1] = *downloadUrl
			cacheKeys[1] = cacheKey
			Expect(downloadURLs).To(ConsistOf(expectedUrls))
			Expect(cacheKeys).To(ConsistOf(expectedCacheKeys))
		})
	})

	Context("When a mount has an invalid 'From' field", func() {
		BeforeEach(func() {
			dependencies = []executor.CachedDependency{
				{Name: "name-1", CacheKey: "cache-key-1", LogSource: "log-source-1", From: "%", To: "/var/data/buildpack-1"},
			}
		})

		It("returns the error", func() {
			_, err := dependencyManager.DownloadCachedDependencies(logger, dependencies, logStreamer)
			Expect(err).To(HaveOccurred())
		})
	})

	Context("When fetching a directory fails", func() {
		BeforeEach(func() {
			cache.FetchAsDirectoryReturns("", 0, errors.New("nope"))
		})

		It("emits the download events", func() {
			_, _ = dependencyManager.DownloadCachedDependencies(logger, dependencies, logStreamer)
			Eventually(func() []byte {
				stdout := logStreamer.Stdout().(*gbytes.Buffer)
				return stdout.Contents()
			}).Should(And(ContainSubstring("Downloading name-1..."), ContainSubstring("Downloading name-1 failed\n")))
		})

		It("returns the error", func() {
			_, err := dependencyManager.DownloadCachedDependencies(logger, dependencies, logStreamer)
			Expect(err).To(HaveOccurred())
		})
	})

	Context("When there are no cached dependencies ", func() {
		It("returns an empty list of bindmounts", func() {
			bindMounts, err := dependencyManager.DownloadCachedDependencies(logger, nil, logStreamer)
			Expect(err).NotTo(HaveOccurred())
			Expect(bindMounts.CacheKeys).To(HaveLen(0))
			Expect(bindMounts.GardenBindMounts).To(HaveLen(0))
		})
	})

	Context("rate limiting", func() {
		var downloadBlocker chan struct{}

		BeforeEach(func() {
			downloadBlocker = make(chan struct{})
			cache.FetchAsDirectoryStub = func(downloadUrl *url.URL, cacheKey string, checksum cacheddownloader.ChecksumInfoType, cancelChan <-chan struct{}) (string, int64, error) {
				<-downloadBlocker
				return cacheKey, 0, nil
			}

			dependencies = append(dependencies, executor.CachedDependency{
				Name:      "name3",
				CacheKey:  "cache-key3",
				LogSource: "log-source3",
				From:      "https://user:pass@example.com:8080/download-1",
				To:        "/var/data/buildpack-1",
			})
		})

		It("limits how many downloads can occur concurrently", func() {
			done := make(chan struct{})

			go func() {
				_, err := dependencyManager.DownloadCachedDependencies(logger, dependencies, logStreamer)
				Expect(err).NotTo(HaveOccurred())
				close(done)
			}()

			Eventually(cache.FetchAsDirectoryCallCount).Should(Equal(2))
			Consistently(cache.FetchAsDirectoryCallCount).Should(Equal(2))

			Eventually(downloadBlocker).Should(BeSent(struct{}{}))

			Eventually(cache.FetchAsDirectoryCallCount).Should(Equal(3))
			Consistently(cache.FetchAsDirectoryCallCount).Should(Equal(3))

			Eventually(downloadBlocker).Should(BeSent(struct{}{}))
			Eventually(downloadBlocker).Should(BeSent(struct{}{}))

			Eventually(done).Should(BeClosed())
		})
	})
})
