package cacheddownloader

import (
	"archive/tar"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
	"syscall"
	"time"
)

var (
	lock          = &sync.Mutex{}
	EntryNotFound = errors.New("Entry Not Found")
	AlreadyClosed = errors.New("Already closed directory")
	NotCacheable  = errors.New("Not cacheable directory")
)

type FileCache struct {
	CachedPath     string
	maxSizeInBytes int64
	Entries        map[string]*FileCacheEntry
	OldEntries     map[string]*FileCacheEntry
	Seq            uint64
}

type FileCacheEntry struct {
	Size                  int64
	Access                time.Time
	CachingInfo           CachingInfoType
	FilePath              string
	ExpandedDirectoryPath string
	inuseCount            int
}

func NewCache(dir string, maxSizeInBytes int64) *FileCache {
	return &FileCache{
		CachedPath:     dir,
		maxSizeInBytes: maxSizeInBytes,
		Entries:        map[string]*FileCacheEntry{},
		OldEntries:     map[string]*FileCacheEntry{},
		Seq:            0,
	}
}

func newFileCacheEntry(cachePath string, size int64, cachingInfo CachingInfoType) *FileCacheEntry {
	return &FileCacheEntry{
		Size:                  size,
		FilePath:              cachePath,
		Access:                time.Now(),
		CachingInfo:           cachingInfo,
		ExpandedDirectoryPath: "",
		inuseCount:            1,
	}
}

func (e *FileCacheEntry) incrementUse() {
	e.inuseCount++
}

func (e *FileCacheEntry) decrementUse() {
	if e.inuseCount > 0 {
		e.inuseCount--
	}

	count := e.inuseCount

	if count == 0 {
		err := os.RemoveAll(e.FilePath)
		if err != nil {
			fmt.Fprintln(os.Stderr, "Unable to delete cached file", err)
		}

		// if there is a directory we need to remove it as well
		err = os.RemoveAll(e.ExpandedDirectoryPath)
		if err != nil {
			fmt.Fprintln(os.Stderr, "Unable to delete cached file", err)
		}
	}
}

// Can we change this to be an io.ReadCloser return
func (e *FileCacheEntry) readCloser() (*CachedFile, error) {
	f, err := os.Open(e.FilePath)
	if err != nil {
		return nil, err
	}

	e.incrementUse()
	readCloser := NewFileCloser(f, func(filePath string) {
		lock.Lock()
		e.decrementUse()
		lock.Unlock()
	})

	return readCloser, nil
}

func (e *FileCacheEntry) expandedDirectory() (string, error) {
	e.incrementUse()

	// if it has not been extracted before expand it!
	if e.ExpandedDirectoryPath == "" {
		e.ExpandedDirectoryPath = e.FilePath + ".d"
		err := extractTarToDirectory(e.FilePath, e.ExpandedDirectoryPath)
		if err != nil {
			return "", err
		}
	}

	return e.ExpandedDirectoryPath, nil
}

func (c *FileCache) CloseDirectory(cacheKey, dirPath string) error {
	lock.Lock()
	defer lock.Unlock()

	entry := c.Entries[cacheKey]
	if entry != nil && entry.ExpandedDirectoryPath == dirPath {
		if entry.inuseCount == 1 {
			// We don't think anybody is using this so throw an error
			return AlreadyClosed
		}

		entry.decrementUse()
		return nil
	}

	// Key didn't match anything in the current cache, so
	// check and clean up old entries
	entry = c.OldEntries[cacheKey+dirPath]
	if entry == nil {
		return EntryNotFound
	}

	entry.decrementUse()
	if entry.inuseCount == 0 {
		// done with this old entry, so clean it up
		delete(c.OldEntries, cacheKey+dirPath)
	}
	return nil
}

func (c *FileCache) Add(cacheKey, sourcePath string, size int64, cachingInfo CachingInfoType) (*CachedFile, error) {
	lock.Lock()
	defer lock.Unlock()

	oldEntry := c.Entries[cacheKey]

	c.makeRoom(size, "")

	c.Seq++
	uniqueName := fmt.Sprintf("%s-%d-%d", cacheKey, time.Now().UnixNano(), c.Seq)
	cachePath := filepath.Join(c.CachedPath, uniqueName)

	err := os.Rename(sourcePath, cachePath)
	if err != nil {
		return nil, err
	}

	newEntry := newFileCacheEntry(cachePath, size, cachingInfo)
	c.Entries[cacheKey] = newEntry
	if oldEntry != nil {
		oldEntry.decrementUse()
		c.updateOldEntries(cacheKey, oldEntry)
	}
	return newEntry.readCloser()
}

func (c *FileCache) AddDirectory(cacheKey, sourcePath string, size int64, cachingInfo CachingInfoType) (string, error) {
	lock.Lock()
	defer lock.Unlock()

	// double the size when expanding to directories
	newSize := 2 * size

	oldEntry := c.Entries[cacheKey]

	c.makeRoom(newSize, "")

	c.Seq++
	uniqueName := fmt.Sprintf("%s-%d-%d", cacheKey, time.Now().UnixNano(), c.Seq)
	cachePath := filepath.Join(c.CachedPath, uniqueName)

	err := os.Rename(sourcePath, cachePath)
	if err != nil {
		return "", err
	}
	newEntry := newFileCacheEntry(cachePath, newSize, cachingInfo)
	c.Entries[cacheKey] = newEntry
	if oldEntry != nil {
		oldEntry.decrementUse()
		c.updateOldEntries(cacheKey, oldEntry)
	}
	return newEntry.expandedDirectory()
}

func (c *FileCache) Get(cacheKey string) (*CachedFile, CachingInfoType, error) {
	lock.Lock()
	defer lock.Unlock()

	entry := c.Entries[cacheKey]
	if entry == nil {
		return nil, CachingInfoType{}, EntryNotFound
	}

	entry.Access = time.Now()
	readCloser, err := entry.readCloser()
	if err != nil {
		return nil, CachingInfoType{}, err
	}

	return readCloser, entry.CachingInfo, nil
}

func (c *FileCache) GetDirectory(cacheKey string) (string, CachingInfoType, error) {
	lock.Lock()
	defer lock.Unlock()

	entry := c.Entries[cacheKey]
	if entry == nil {
		return "", CachingInfoType{}, EntryNotFound
	}

	// Was it expanded before
	if entry.ExpandedDirectoryPath == "" {
		// Do we have enough room to double the size?
		c.makeRoom(entry.Size, cacheKey)
		entry.Size = entry.Size * 2
	}

	entry.Access = time.Now()
	dir, err := entry.expandedDirectory()
	if err != nil {
		return "", CachingInfoType{}, err
	}

	return dir, entry.CachingInfo, nil
}

func (c *FileCache) Remove(cacheKey string) {
	lock.Lock()
	c.remove(cacheKey)
	lock.Unlock()
}

func (c *FileCache) remove(cacheKey string) {
	entry := c.Entries[cacheKey]
	if entry != nil {
		entry.decrementUse()
		c.updateOldEntries(cacheKey, entry)
		delete(c.Entries, cacheKey)
	}
}

func (c *FileCache) updateOldEntries(cacheKey string, entry *FileCacheEntry) {
	if entry != nil {
		if entry.inuseCount > 0 && entry.ExpandedDirectoryPath != "" {
			// put it in the oldEntries Cache since somebody may still be using the directory
			c.OldEntries[cacheKey+entry.ExpandedDirectoryPath] = entry
		} else {
			// We need to remove it from oldEntries
			delete(c.OldEntries, cacheKey+entry.ExpandedDirectoryPath)
		}
	}
}

func (c *FileCache) makeRoom(size int64, excludedCacheKey string) {
	usedSpace := c.usedSpace()
	for c.maxSizeInBytes < usedSpace+size {
		var oldestEntry *FileCacheEntry
		oldestAccessTime, oldestCacheKey := time.Now(), ""
		for ck, f := range c.Entries {
			if f.Access.Before(oldestAccessTime) && ck != excludedCacheKey && f.inuseCount <= 1 {
				oldestAccessTime = f.Access
				oldestEntry = f
				oldestCacheKey = ck
			}
		}

		if oldestEntry == nil {
			// could not find anything we could remove
			return
		}

		usedSpace -= oldestEntry.Size
		c.remove(oldestCacheKey)
	}

	return
}

func (c *FileCache) usedSpace() int64 {
	space := int64(0)
	for _, f := range c.Entries {
		space += f.Size
	}
	return space
}

func extractTarToDirectory(sourcePath, destinationDir string) error {
	_, err := os.Stat(destinationDir)
	if err != nil && err.(*os.PathError).Err != syscall.ENOENT {
		return err
	}

	file, err := os.Open(sourcePath)
	if err != nil {
		return err
	}

	defer file.Close()

	var fileReader io.ReadCloser = file

	// Make the target directory
	err = os.MkdirAll(destinationDir, 0777)
	if err != nil {
		return err
	}

	tarBallReader := tar.NewReader(fileReader)
	// Extracting tarred files
	for {
		header, err := tarBallReader.Next()
		if err != nil {
			if err == io.EOF {
				break
			}
			return err
		}

		// get the individual filename and extract to the current directory
		filename := header.Name

		switch header.Typeflag {
		case tar.TypeDir:
			// handle directory
			fullpath := filepath.Join(destinationDir, filename)
			err = os.MkdirAll(fullpath, os.FileMode(header.Mode))

			if err != nil {
				return err
			}

		default:
			// handle normal file
			fullpath := filepath.Join(destinationDir, filename)

			err := os.MkdirAll(filepath.Dir(fullpath), 0777)
			if err != nil {
				return err
			}

			writer, err := os.Create(fullpath)

			if err != nil {
				return err
			}

			io.Copy(writer, tarBallReader)

			err = os.Chmod(fullpath, os.FileMode(header.Mode))

			if err != nil {
				return err
			}

			writer.Close()

		}
	}
	return nil
}
