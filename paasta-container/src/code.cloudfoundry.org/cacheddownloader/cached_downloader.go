package cacheddownloader

import (
	"crypto/md5"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/url"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/cloudfoundry/systemcerts"
)

// called after a new object has entered the cache.
// it is assumed that `path` will be removed, if a new path is returned.
// a noop transformer returns the given path and its detected size.
type CacheTransformer func(source, destination string) (newSize int64, err error)

//go:generate counterfeiter -o cacheddownloaderfakes/fake_cached_downloader.go . CachedDownloader

// CachedDownloader is responsible for downloading and caching files and maintaining reference counts for each cache entry.
// Entries in the cache with no active references are ejected from the cache when new space is needed.
type CachedDownloader interface {
	// Fetch downloads the file at the given URL and stores it in the cache with the given cacheKey.
	// If cacheKey is empty, the file will not be saved in the cache.
	//
	// Fetch returns a stream that can be used to read the contents of the downloaded file. While this stream is active (i.e., not yet closed),
	// the associated cache entry will be considered in use and will not be ejected from the cache.
	Fetch(urlToFetch *url.URL, cacheKey string, checksum ChecksumInfoType, cancelChan <-chan struct{}) (stream io.ReadCloser, size int64, err error)

	// FetchAsDirectory downloads the tarfile pointed to by the given URL, expands the tarfile into a directory, and returns the path of that directory as well as the total number of bytes downloaded.
	FetchAsDirectory(urlToFetch *url.URL, cacheKey string, checksum ChecksumInfoType, cancelChan <-chan struct{}) (dirPath string, size int64, err error)

	// CloseDirectory decrements the usage counter for the given cacheKey/directoryPath pair.
	// It should be called when the directory returned by FetchAsDirectory is no longer in use.
	// In this way, FetchAsDirectory and CloseDirectory should be treated as a pair of operations,
	// and a process that calls FetchAsDirectory should make sure a corresponding CloseDirectory is eventually called.
	CloseDirectory(cacheKey, directoryPath string) error

	// SaveState writes the current state of the cache metadata to a file so that it can be recovered
	// later. This should be called on process shutdown.
	SaveState() error

	// RecoverState checks to see if a state file exists (from a previous SaveState call), and restores
	// the cache state from that information if such a file exists. This should be called on startup.
	RecoverState() error
}

func NoopTransform(source, destination string) (int64, error) {
	err := os.Rename(source, destination)
	if err != nil {
		return 0, err
	}

	fi, err := os.Stat(destination)
	if err != nil {
		return 0, err
	}

	return fi.Size(), nil
}

type CachingInfoType struct {
	ETag         string
	LastModified string
}

type ChecksumInfoType struct {
	Algorithm string
	Value     string
}

type cachedDownloader struct {
	downloader    *Downloader
	uncachedPath  string
	cache         *FileCache
	transformer   CacheTransformer
	cacheLocation string

	lock       *sync.Mutex
	inProgress map[string]chan struct{}
}

func (c CachingInfoType) isCacheable() bool {
	return c.ETag != "" || c.LastModified != ""
}

func (c CachingInfoType) Equal(other CachingInfoType) bool {
	return c.ETag == other.ETag && c.LastModified == other.LastModified
}

// A transformer function can be used to do post-download
// processing on the file before it is stored in the cache.
func New(cachedPath string, uncachedPath string, maxSizeInBytes int64, downloadTimeout time.Duration, maxConcurrentDownloads int, skipSSLVerification bool, caCertPool *systemcerts.CertPool, transformer CacheTransformer) *cachedDownloader {
	os.MkdirAll(cachedPath, 0770)
	return &cachedDownloader{
		downloader:    NewDownloader(downloadTimeout, maxConcurrentDownloads, skipSSLVerification, caCertPool),
		uncachedPath:  uncachedPath,
		cache:         NewCache(cachedPath, maxSizeInBytes),
		transformer:   transformer,
		lock:          &sync.Mutex{},
		inProgress:    map[string]chan struct{}{},
		cacheLocation: filepath.Join(cachedPath, "saved_cache.json"),
	}
}

func (c *cachedDownloader) SaveState() error {
	json, err := json.Marshal(c.cache)
	if err != nil {
		return err
	}

	return ioutil.WriteFile(c.cacheLocation, json, 0600)
}

func (c *cachedDownloader) RecoverState() error {
	file, err := os.Open(c.cacheLocation)
	if err != nil && !os.IsNotExist(err) {
		return err
	}

	if err == nil {
		// parse the file only if it exists
		err = json.NewDecoder(file).Decode(c.cache)
		file.Close()
	}

	// set the inuse count to 0 since all containers will be recreated
	for _, entry := range c.cache.Entries {
		// inuseCount starts at 1 (i.e. 1 == no references to the entry)
		entry.inuseCount = 1
	}

	// delete files that aren't in the cache. **note** if there is no
	// saved_cache.json, then all files will be deleted
	trackedFiles := map[string]struct{}{}

	for _, entry := range c.cache.Entries {
		trackedFiles[entry.FilePath] = struct{}{}
		trackedFiles[entry.ExpandedDirectoryPath] = struct{}{}
	}

	files, err := ioutil.ReadDir(c.cache.CachedPath)
	if err != nil && !os.IsNotExist(err) {
		return err
	}

	for _, file := range files {
		path := filepath.Join(c.cache.CachedPath, file.Name())
		if _, ok := trackedFiles[path]; ok {
			continue
		}

		err = os.RemoveAll(path)
		if err != nil {
			return err
		}
	}

	// free some disk space in case the maxSizeInBytes was changed
	c.cache.makeRoom(0, "")
	return err
}

func (c *cachedDownloader) CloseDirectory(cacheKey, directoryPath string) error {
	cacheKey = fmt.Sprintf("%x", md5.Sum([]byte(cacheKey)))
	return c.cache.CloseDirectory(cacheKey, directoryPath)
}

func (c *cachedDownloader) Fetch(url *url.URL, cacheKey string, checksum ChecksumInfoType, cancelChan <-chan struct{}) (io.ReadCloser, int64, error) {
	if cacheKey == "" {
		return c.fetchUncachedFile(url, checksum, cancelChan)
	}

	cacheKey = fmt.Sprintf("%x", md5.Sum([]byte(cacheKey)))
	return c.fetchCachedFile(url, cacheKey, checksum, cancelChan)
}

func (c *cachedDownloader) fetchUncachedFile(url *url.URL, checksum ChecksumInfoType, cancelChan <-chan struct{}) (*CachedFile, int64, error) {
	download, _, size, err := c.populateCache(url, "uncached", CachingInfoType{}, checksum, c.transformer, cancelChan)
	if err != nil {
		return nil, 0, err
	}

	file, err := tempFileRemoveOnClose(download.path)
	return file, size, err
}

func (c *cachedDownloader) fetchCachedFile(url *url.URL, cacheKey string, checksum ChecksumInfoType, cancelChan <-chan struct{}) (*CachedFile, int64, error) {
	rateLimiter, err := c.acquireLimiter(cacheKey, cancelChan)
	if err != nil {
		return nil, 0, err
	}
	defer c.releaseLimiter(cacheKey, rateLimiter)

	// lookup cache entry
	currentReader, currentCachingInfo, getErr := c.cache.Get(cacheKey)

	// download (short circuits if endpoint respects etag/etc.)
	download, cacheIsWarm, size, err := c.populateCache(url, cacheKey, currentCachingInfo, checksum, c.transformer, cancelChan)
	if err != nil {
		if currentReader != nil {
			currentReader.Close()
		}
		return nil, 0, err
	}

	// nothing had to be downloaded; return the cached entry
	if cacheIsWarm {
		return currentReader, 0, getErr
	}

	// current cache is not fresh; disregard it
	if currentReader != nil {
		currentReader.Close()
	}

	// fetch uncached data
	var newReader *CachedFile
	if download.cachingInfo.isCacheable() {
		newReader, err = c.cache.Add(cacheKey, download.path, download.size, download.cachingInfo)
	} else {
		c.cache.Remove(cacheKey)
		newReader, err = tempFileRemoveOnClose(download.path)
	}

	// return newly fetched file
	return newReader, size, err
}

func (c *cachedDownloader) FetchAsDirectory(url *url.URL, cacheKey string, checksum ChecksumInfoType, cancelChan <-chan struct{}) (string, int64, error) {
	if cacheKey == "" {
		return "", 0, NotCacheable
	}

	cacheKey = fmt.Sprintf("%x", md5.Sum([]byte(cacheKey)))
	return c.fetchCachedDirectory(url, cacheKey, checksum, cancelChan)
}

func (c *cachedDownloader) fetchCachedDirectory(url *url.URL, cacheKey string, checksum ChecksumInfoType, cancelChan <-chan struct{}) (string, int64, error) {
	rateLimiter, err := c.acquireLimiter(cacheKey, cancelChan)
	if err != nil {
		return "", 0, err
	}
	defer c.releaseLimiter(cacheKey, rateLimiter)

	// lookup cache entry
	currentDirectory, currentCachingInfo, getErr := c.cache.GetDirectory(cacheKey)

	// download (short circuits if endpoint respects etag/etc.)
	download, cacheIsWarm, size, err := c.populateCache(url, cacheKey, currentCachingInfo, checksum, TarTransform, cancelChan)
	if err != nil {
		if currentDirectory != "" {
			c.cache.CloseDirectory(cacheKey, currentDirectory)
		}
		return "", 0, err
	}

	// nothing had to be downloaded; return the cached entry
	if cacheIsWarm {
		return currentDirectory, 0, getErr
	}

	// current cache is not fresh; disregard it
	if currentDirectory != "" {
		c.cache.CloseDirectory(cacheKey, currentDirectory)
	}

	// fetch uncached data
	var newDirectory string
	if download.cachingInfo.isCacheable() {
		newDirectory, err = c.cache.AddDirectory(cacheKey, download.path, download.size, download.cachingInfo)
		// return newly fetched directory
		return newDirectory, size, err
	} else {
		c.cache.Remove(cacheKey)
	}

	return "", 0, NotCacheable
}

func (c *cachedDownloader) acquireLimiter(cacheKey string, cancelChan <-chan struct{}) (chan struct{}, error) {
	startTime := time.Now()

	for {
		c.lock.Lock()
		rateLimiter := c.inProgress[cacheKey]
		if rateLimiter == nil {
			rateLimiter = make(chan struct{})
			c.inProgress[cacheKey] = rateLimiter
			c.lock.Unlock()
			return rateLimiter, nil
		}
		c.lock.Unlock()

		select {
		case <-rateLimiter:
		case <-cancelChan:
			return nil, NewDownloadCancelledError("acquire-limiter", time.Now().Sub(startTime), NoBytesReceived)
		}
	}
}

func (c *cachedDownloader) releaseLimiter(cacheKey string, limiter chan struct{}) {
	c.lock.Lock()
	delete(c.inProgress, cacheKey)
	close(limiter)
	c.lock.Unlock()
}

func tempFileRemoveOnClose(path string) (*CachedFile, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}

	return NewFileCloser(f, func(path string) {
		os.RemoveAll(path)
	}), nil
}

type download struct {
	path        string
	size        int64
	cachingInfo CachingInfoType
}

// Currently populateCache takes a transformer due to the fact that a fetchCachedDirectory
// uses only a TarTransformer, which overwrites what is currently set. This way one transformer
// can be used to call Fetch and FetchAsDirectory
func (c *cachedDownloader) populateCache(
	url *url.URL,
	name string,
	cachingInfo CachingInfoType,
	checksum ChecksumInfoType,
	transformer CacheTransformer,
	cancelChan <-chan struct{},
) (download, bool, int64, error) {
	filename, cachingInfo, err := c.downloader.Download(url, func() (*os.File, error) {
		return ioutil.TempFile(c.uncachedPath, name+"-")
	}, cachingInfo, checksum, cancelChan)
	if err != nil {
		return download{}, false, 0, err
	}

	if filename == "" {
		return download{}, true, 0, nil
	}

	fileInfo, err := os.Stat(filename)
	if err != nil {
		return download{}, false, 0, err
	}

	cachedFile, err := ioutil.TempFile(c.uncachedPath, "transformed")
	if err != nil {
		return download{}, false, 0, err
	}

	err = cachedFile.Close()
	if err != nil {
		return download{}, false, 0, err
	}

	cachedSize, err := transformer(filename, cachedFile.Name())
	if err != nil {
		// os.Remove(filename)
		return download{}, false, 0, err
	}

	return download{
		path:        cachedFile.Name(),
		size:        cachedSize,
		cachingInfo: cachingInfo,
	}, false, fileInfo.Size(), nil
}
