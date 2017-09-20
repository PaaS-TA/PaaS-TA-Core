package cacheddownloader_test

import (
	"archive/tar"
	"io"
	"io/ioutil"
	"os"

	"code.cloudfoundry.org/cacheddownloader"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("FileCache", func() {
	var (
		cache                     *cacheddownloader.FileCache
		cacheDir                  string
		err                       error
		sourceFile, sourceArchive *os.File
		maxSizeInBytes            int64
	)

	BeforeEach(func() {
		cacheDir, err = ioutil.TempDir("", "cache-test")
		Expect(err).NotTo(HaveOccurred())

		sourceFile = createFile("cache-test-file", "the-file-content")
		sourceArchive = createArchive("cache-test-archive", "Data in Test File")

		maxSizeInBytes = 123424
	})

	JustBeforeEach(func() {
		cache = cacheddownloader.NewCache(cacheDir, maxSizeInBytes)
	})

	AfterEach(func() {
		os.RemoveAll(sourceFile.Name())
		os.RemoveAll(sourceArchive.Name())
		os.RemoveAll(cacheDir)
	})

	Describe("Add", func() {
		var cacheKey string
		var fileSize int64
		var cacheInfo cacheddownloader.CachingInfoType
		var readCloser *cacheddownloader.CachedFile

		BeforeEach(func() {
			cacheKey = "the-cache-key"
			fileSize = 100
			cacheInfo = cacheddownloader.CachingInfoType{}
		})

		It("succeeds even if room cannot be allocated", func() {
			var err error
			readCloser, err = cache.Add(cacheKey, sourceFile.Name(), 250000, cacheInfo)
			Expect(err).NotTo(HaveOccurred())
			Expect(readCloser).NotTo(BeNil())
		})

		Context("when closed is called", func() {
			JustBeforeEach(func() {
				var err error
				readCloser, err = cache.Add(cacheKey, sourceFile.Name(), fileSize, cacheInfo)
				Expect(err).NotTo(HaveOccurred())
				Expect(readCloser).NotTo(BeNil())
			})

			Context("once", func() {
				It("succeeds and has 1 file in the cache", func() {
					Expect(readCloser.Close()).NotTo(HaveOccurred())
					Expect(filenamesInDir(cacheDir)).To(HaveLen(1))
				})
			})

			Context("more than once", func() {
				It("fails", func() {
					Expect(readCloser.Close()).NotTo(HaveOccurred())
					Expect(readCloser.Close()).To(HaveOccurred())
				})
			})
		})

		Context("when the cache is empty", func() {
			JustBeforeEach(func() {
				var err error
				readCloser, err = cache.Add(cacheKey, sourceFile.Name(), fileSize, cacheInfo)
				Expect(err).NotTo(HaveOccurred())
				Expect(readCloser).NotTo(BeNil())
			})

			AfterEach(func() {
				readCloser.Close()
			})

			It("returns a reader", func() {
				content, err := ioutil.ReadAll(readCloser)
				Expect(err).NotTo(HaveOccurred())
				Expect(string(content)).To(Equal("the-file-content"))
			})

			It("has 1 file in the cache", func() {
				Expect(filenamesInDir(cacheDir)).To(HaveLen(1))
			})
		})

		Context("when a cachekey exists", func() {
			var newSourceFile *os.File
			var newFileSize int64
			var newCacheInfo cacheddownloader.CachingInfoType
			var newReader io.ReadCloser

			BeforeEach(func() {
				newSourceFile = createFile("cache-test-file", "new-file-content")
				newFileSize = fileSize
				newCacheInfo = cacheInfo
			})

			JustBeforeEach(func() {
				readCloser, err = cache.Add(cacheKey, sourceFile.Name(), fileSize, cacheInfo)
				Expect(err).NotTo(HaveOccurred())
				Expect(readCloser).NotTo(BeNil())
			})

			AfterEach(func() {
				readCloser.Close()
				os.RemoveAll(newSourceFile.Name())
			})

			Context("when adding the same cache key with identical info", func() {
				It("ignores the add", func() {
					reader, err := cache.Add(cacheKey, newSourceFile.Name(), fileSize, cacheInfo)
					Expect(err).NotTo(HaveOccurred())
					Expect(reader).NotTo(BeNil())
				})
			})

			Context("when a adding the same cache key and different info", func() {
				JustBeforeEach(func() {
					var err error
					newReader, err = cache.Add(cacheKey, newSourceFile.Name(), newFileSize, newCacheInfo)
					Expect(err).NotTo(HaveOccurred())
					Expect(newReader).NotTo(BeNil())
				})

				AfterEach(func() {
					newReader.Close()
				})

				Context("different file size", func() {
					BeforeEach(func() {
						newFileSize = fileSize - 1
					})

					It("returns a reader for the new content", func() {
						content, err := ioutil.ReadAll(newReader)
						Expect(err).NotTo(HaveOccurred())
						Expect(string(content)).To(Equal("new-file-content"))
					})

					It("has files in the cache", func() {
						Expect(filenamesInDir(cacheDir)).To(HaveLen(2))
						Expect(readCloser.Close()).NotTo(HaveOccurred())
						Expect(filenamesInDir(cacheDir)).To(HaveLen(1))
					})

				})

				Context("different caching info", func() {
					BeforeEach(func() {
						newCacheInfo = cacheddownloader.CachingInfoType{
							LastModified: "1234",
						}
					})

					It("returns a reader for the new content", func() {
						content, err := ioutil.ReadAll(newReader)
						Expect(err).NotTo(HaveOccurred())
						Expect(string(content)).To(Equal("new-file-content"))
					})

					It("has files in the cache", func() {
						Expect(filenamesInDir(cacheDir)).To(HaveLen(2))
						Expect(readCloser.Close()).NotTo(HaveOccurred())
						Expect(filenamesInDir(cacheDir)).To(HaveLen(1))
					})

					It("still allows the previous reader to read", func() {
						content, err := ioutil.ReadAll(readCloser)
						Expect(err).NotTo(HaveOccurred())
						Expect(string(content)).To(Equal("the-file-content"))
					})
				})
			})
		})
	})

	Describe("AddDirectory", func() {
		var cacheKey string
		var fileSize int64
		var cacheInfo cacheddownloader.CachingInfoType
		var directoryPath string

		BeforeEach(func() {
			cacheKey = "the-cache-key"
			fileSize = 100
			cacheInfo = cacheddownloader.CachingInfoType{}
		})

		It("succeeds even if room cannot be allocated", func() {
			var err error
			directoryPath, err = cache.AddDirectory(cacheKey, sourceArchive.Name(), 250000, cacheInfo)
			Expect(err).NotTo(HaveOccurred())
			Expect(directoryPath).NotTo(BeEmpty())
		})

		Context("when closed is called", func() {
			JustBeforeEach(func() {
				var err error
				directoryPath, err = cache.AddDirectory(cacheKey, sourceArchive.Name(), fileSize, cacheInfo)
				Expect(err).NotTo(HaveOccurred())
				Expect(directoryPath).NotTo(BeEmpty())
			})

			Context("once", func() {
				It("succeeds and has 1 file in the cache", func() {
					err = cache.CloseDirectory(cacheKey, directoryPath)
					Expect(err).NotTo(HaveOccurred())
					Expect(filenamesInDir(cacheDir)).To(HaveLen(2))
				})
			})

			Context("more than once", func() {
				It("fails", func() {
					err := cache.CloseDirectory(cacheKey, directoryPath)
					Expect(err).NotTo(HaveOccurred())
					err = cache.CloseDirectory(cacheKey, directoryPath)
					Expect(err).To(HaveOccurred())
				})
			})
		})

		Context("when the cache is empty", func() {
			JustBeforeEach(func() {
				var err error
				directoryPath, err = cache.AddDirectory(cacheKey, sourceArchive.Name(), fileSize, cacheInfo)
				Expect(err).NotTo(HaveOccurred())
				Expect(directoryPath).NotTo(BeEmpty())
			})

			AfterEach(func() {
				cache.CloseDirectory(cacheKey, directoryPath)
			})

			It("returns an existing directory", func() {
				fileInfo, err := os.Stat(directoryPath)
				Expect(err).NotTo(HaveOccurred())
				Expect(fileInfo.IsDir()).To(BeTrue())
			})

			It("has 1 file in the cache", func() {
				Expect(filenamesInDir(cacheDir)).To(HaveLen(2))
			})
		})

		Context("when a cachekey exists", func() {
			var newSourceArchive *os.File
			var newFileSize int64
			var newCacheInfo cacheddownloader.CachingInfoType
			var newDirectoryPath string

			BeforeEach(func() {
				newSourceArchive = createArchive("cache-test-file", "new-file-content")
				newFileSize = fileSize
				newCacheInfo = cacheInfo
			})

			JustBeforeEach(func() {
				directoryPath, err = cache.AddDirectory(cacheKey, sourceArchive.Name(), fileSize, cacheInfo)
				Expect(err).NotTo(HaveOccurred())
				Expect(directoryPath).NotTo(BeEmpty())
			})

			AfterEach(func() {
				cache.CloseDirectory(cacheKey, directoryPath)
				os.RemoveAll(newSourceArchive.Name())
			})

			Context("when adding the same cache key with identical info", func() {
				It("ignores the add", func() {
					directoryPath, err := cache.AddDirectory(cacheKey, newSourceArchive.Name(), fileSize, cacheInfo)
					Expect(err).NotTo(HaveOccurred())
					Expect(directoryPath).NotTo(BeEmpty())
				})
			})

			Context("when a adding the same cache key and different info", func() {
				JustBeforeEach(func() {
					var err error
					newDirectoryPath, err = cache.AddDirectory(cacheKey, newSourceArchive.Name(), newFileSize, newCacheInfo)
					Expect(err).NotTo(HaveOccurred())
					Expect(newDirectoryPath).NotTo(BeEmpty())
				})

				AfterEach(func() {
					cache.CloseDirectory(cacheKey, newDirectoryPath)
				})

				Context("different file size", func() {
					BeforeEach(func() {
						newFileSize = fileSize - 1
					})

					It("returns an existing directory", func() {
						fileInfo, err := os.Stat(newDirectoryPath)
						Expect(err).NotTo(HaveOccurred())
						Expect(fileInfo.IsDir()).To(BeTrue())
					})

					It("has files in the cache", func() {
						Expect(filenamesInDir(cacheDir)).To(HaveLen(4))
						cache.CloseDirectory(cacheKey, directoryPath)
						Expect(filenamesInDir(cacheDir)).To(HaveLen(2))
					})

				})

				Context("different caching info", func() {
					BeforeEach(func() {
						newCacheInfo = cacheddownloader.CachingInfoType{
							LastModified: "1234",
						}
					})

					It("returns an existing directory", func() {
						fileInfo, err := os.Stat(newDirectoryPath)
						Expect(err).NotTo(HaveOccurred())
						Expect(fileInfo.IsDir()).To(BeTrue())
					})

					It("has files in the cache", func() {
						Expect(filenamesInDir(cacheDir)).To(HaveLen(4))
						cache.CloseDirectory(cacheKey, directoryPath)
						Expect(filenamesInDir(cacheDir)).To(HaveLen(2))
					})

					It("still allows the previous directory to exist", func() {
						fileInfo, err := os.Stat(directoryPath)
						Expect(err).NotTo(HaveOccurred())
						Expect(fileInfo.IsDir()).To(BeTrue())
					})
				})
			})
		})
	})

	Describe("Get", func() {
		var cacheKey string
		var fileSize int64
		var cacheInfo cacheddownloader.CachingInfoType

		BeforeEach(func() {
			cacheKey = "key"
			fileSize = 100
			cacheInfo = cacheddownloader.CachingInfoType{}
		})

		Context("when there is nothing", func() {
			It("returns nothing", func() {
				reader, ci, err := cache.Get(cacheKey)
				Expect(err).To(Equal(cacheddownloader.EntryNotFound))
				Expect(reader).To(BeNil())
				Expect(ci).To(Equal(cacheInfo))
			})
		})

		Context("when there is an item", func() {
			JustBeforeEach(func() {
				cacheInfo.LastModified = "1234"
				reader, err := cache.Add(cacheKey, sourceFile.Name(), fileSize, cacheInfo)
				Expect(err).NotTo(HaveOccurred())
				reader.Close()
			})

			It("returns a reader for the item", func() {
				reader, ci, err := cache.Get(cacheKey)
				Expect(err).NotTo(HaveOccurred())
				Expect(reader).NotTo(BeNil())
				Expect(ci).To(Equal(cacheInfo))

				content, err := ioutil.ReadAll(reader)
				Expect(err).NotTo(HaveOccurred())
				Expect(string(content)).To(Equal("the-file-content"))
			})

			Context("when the item is replaced", func() {
				var newSourceFile *os.File

				JustBeforeEach(func() {
					newSourceFile = createFile("cache-test-file", "new-file-content")

					cacheInfo.LastModified = "123"
					reader, err := cache.Add(cacheKey, newSourceFile.Name(), fileSize, cacheInfo)
					Expect(err).NotTo(HaveOccurred())
					reader.Close()
				})

				AfterEach(func() {
					os.RemoveAll(newSourceFile.Name())
				})

				It("gets the new item", func() {
					reader, ci, err := cache.Get(cacheKey)
					Expect(err).NotTo(HaveOccurred())
					Expect(reader).NotTo(BeNil())
					Expect(ci).To(Equal(cacheInfo))

					content, err := ioutil.ReadAll(reader)
					Expect(err).NotTo(HaveOccurred())
					Expect(string(content)).To(Equal("new-file-content"))
				})

				Context("when a get is issued before a replace", func() {
					var reader io.ReadCloser
					JustBeforeEach(func() {
						var err error
						reader, _, err = cache.Get(cacheKey)
						Expect(err).NotTo(HaveOccurred())
						Expect(reader).NotTo(BeNil())

						Expect(filenamesInDir(cacheDir)).To(HaveLen(1))
					})

					It("the old file is removed when closed", func() {
						reader.Close()
						Expect(filenamesInDir(cacheDir)).To(HaveLen(1))
					})
				})
			})
		})

		Context("when there is an item added with AddDirectory", func() {
			JustBeforeEach(func() {
				cacheInfo.LastModified = "1234"
				dir, err := cache.AddDirectory(cacheKey, sourceArchive.Name(), fileSize, cacheInfo)
				Expect(err).NotTo(HaveOccurred())
				cache.CloseDirectory(cacheKey, dir)
			})

			It("returns a reader for the item", func() {
				reader, ci, err := cache.Get(cacheKey)
				Expect(err).NotTo(HaveOccurred())
				Expect(reader).NotTo(BeNil())
				Expect(ci).To(Equal(cacheInfo))
			})
		})
	})

	Describe("GetDirectory", func() {
		var cacheKey string
		var fileSize int64
		var cacheInfo cacheddownloader.CachingInfoType

		BeforeEach(func() {
			cacheKey = "key"
			fileSize = 100
			cacheInfo = cacheddownloader.CachingInfoType{}
		})

		Context("when there is nothing", func() {
			It("returns nothing", func() {
				dir, ci, err := cache.GetDirectory(cacheKey)
				Expect(err).To(Equal(cacheddownloader.EntryNotFound))
				Expect(dir).To(BeEmpty())
				Expect(ci).To(Equal(cacheInfo))
			})
		})

		Context("when there is an added directory", func() {
			JustBeforeEach(func() {
				cacheInfo.LastModified = "1234"
				dir, err := cache.AddDirectory(cacheKey, sourceArchive.Name(), fileSize, cacheInfo)
				Expect(err).NotTo(HaveOccurred())
				cache.CloseDirectory(cacheKey, dir)
			})

			It("returns a directory path for the item", func() {
				dir, ci, err := cache.GetDirectory(cacheKey)
				Expect(err).NotTo(HaveOccurred())
				Expect(dir).NotTo(BeEmpty())
				Expect(ci).To(Equal(cacheInfo))

				fileInfo, err := os.Stat(dir)
				Expect(err).NotTo(HaveOccurred())
				Expect(fileInfo.IsDir()).To(BeTrue())
				cache.CloseDirectory(cacheKey, dir)
			})

			Context("when the item is replaced", func() {
				var newSourceArchive *os.File

				JustBeforeEach(func() {
					newSourceArchive = createArchive("cache-test-file", "new-file-content")

					cacheInfo.LastModified = "123"
					dir, err := cache.AddDirectory(cacheKey, newSourceArchive.Name(), fileSize, cacheInfo)
					Expect(err).NotTo(HaveOccurred())
					Expect(dir).ToNot(BeEmpty())
					cache.CloseDirectory(cacheKey, dir)
				})

				AfterEach(func() {
					os.RemoveAll(newSourceArchive.Name())
				})

				It("gets the new item", func() {
					dir, ci, err := cache.GetDirectory(cacheKey)
					Expect(err).NotTo(HaveOccurred())
					Expect(dir).NotTo(BeEmpty())
					Expect(ci).To(Equal(cacheInfo))

					fileInfo, err := os.Stat(dir)
					Expect(err).NotTo(HaveOccurred())
					Expect(fileInfo.IsDir()).To(BeTrue())
					cache.CloseDirectory(cacheKey, dir)
				})

				Context("when a get is issued before a replace", func() {
					var dirPath string
					JustBeforeEach(func() {
						var err error
						dirPath, _, err = cache.GetDirectory(cacheKey)
						Expect(err).NotTo(HaveOccurred())
						Expect(dirPath).NotTo(BeEmpty())

						// Now we need 2 one for the archive and another for the dir
						Expect(filenamesInDir(cacheDir)).To(HaveLen(2))
					})

					It("the old file is removed when closed", func() {
						cache.CloseDirectory(cacheKey, dirPath)
						Expect(filenamesInDir(cacheDir)).To(HaveLen(2))
					})
				})
			})
		})

		Context("when there is an added archive only", func() {
			var (
				dir           string
				file          *cacheddownloader.CachedFile
				cacheInfoType cacheddownloader.CachingInfoType
				getErr        error
			)

			JustBeforeEach(func() {
				cacheInfo.LastModified = "1234"
				var err error
				file, err = cache.Add(cacheKey, sourceArchive.Name(), fileSize, cacheInfo)
				Expect(err).NotTo(HaveOccurred())
				dir, cacheInfoType, getErr = cache.GetDirectory(cacheKey)
			})

			AfterEach(func() {
				cache.CloseDirectory(cacheKey, dir)
			})

			It("returns a directory path for the item", func() {
				Expect(getErr).NotTo(HaveOccurred())
				Expect(dir).NotTo(BeEmpty())
				Expect(cacheInfoType).To(Equal(cacheInfo))

				Expect(dir).To(BeADirectory())
			})

			Context("when there isn't enough space", func() {
				BeforeEach(func() {
					maxSizeInBytes = 10
				})

				It("should not return an error", func() {
					Expect(getErr).NotTo(HaveOccurred())
				})

				Context("and all references to the file are closed", func() {
					JustBeforeEach(func() {
						file.Close()
						Expect(cache.CloseDirectory(cacheKey, dir)).To(Succeed())
					})

					It("should delete the directory as soon as we add another file to the cache", func() {
						newArchive := createArchive("cache-test-file", "new-file-content")

						_, err := cache.Add("new-cache-key", newArchive.Name(), fileSize, cacheInfo)
						Expect(err).NotTo(HaveOccurred())

						Expect(dir).NotTo(BeADirectory())
					})
				})
			})
		})
	})

	Describe("Remove", func() {
		var cacheKey string
		var cacheInfo cacheddownloader.CachingInfoType

		Context("when doing Add", func() {
			JustBeforeEach(func() {
				cacheKey = "key"

				cacheInfo.LastModified = "1234"
				reader, err := cache.Add(cacheKey, sourceFile.Name(), 100, cacheInfo)
				Expect(err).NotTo(HaveOccurred())
				reader.Close()
			})

			Context("when the key does not exist", func() {
				It("does not fail", func() {
					Expect(func() { cache.Remove("bogus") }).NotTo(Panic())
				})
			})

			Context("when the key exists", func() {
				It("removes the file in the cache", func() {
					Expect(filenamesInDir(cacheDir)).To(HaveLen(1))
					cache.Remove(cacheKey)
					Expect(filenamesInDir(cacheDir)).To(HaveLen(0))
				})
			})

			Context("when a get is issued first", func() {
				It("removes the file after a close", func() {
					reader, _, err := cache.Get(cacheKey)
					Expect(err).NotTo(HaveOccurred())
					Expect(reader).NotTo(BeNil())
					Expect(filenamesInDir(cacheDir)).To(HaveLen(1))

					cache.Remove(cacheKey)
					Expect(filenamesInDir(cacheDir)).To(HaveLen(1))

					reader.Close()
					Expect(filenamesInDir(cacheDir)).To(HaveLen(0))
				})
			})
		})

		Context("when doing AddDirectory add", func() {
			JustBeforeEach(func() {
				cacheKey = "key"

				cacheInfo.LastModified = "1234"
				dir, err := cache.AddDirectory(cacheKey, sourceArchive.Name(), 100, cacheInfo)
				Expect(err).NotTo(HaveOccurred())
				cache.CloseDirectory(cacheKey, dir)
			})

			Context("when the key does not exist", func() {
				It("does not fail", func() {
					Expect(func() { cache.Remove("bogus") }).NotTo(Panic())
				})
			})

			Context("when the key exists", func() {
				It("removes the file in the cache", func() {
					Expect(filenamesInDir(cacheDir)).To(HaveLen(2))
					cache.Remove(cacheKey)
					Expect(filenamesInDir(cacheDir)).To(HaveLen(0))
				})
			})

			Context("when a get is issued first", func() {
				It("removes the file after a close", func() {
					dir, _, err := cache.GetDirectory(cacheKey)
					Expect(err).NotTo(HaveOccurred())
					Expect(dir).NotTo(BeEmpty())
					Expect(filenamesInDir(cacheDir)).To(HaveLen(2))

					cache.Remove(cacheKey)
					Expect(filenamesInDir(cacheDir)).To(HaveLen(2))

					cache.CloseDirectory(cacheKey, dir)
					Expect(filenamesInDir(cacheDir)).To(HaveLen(0))
				})
			})
		})
	})
})

func createFile(filename string, content string) *os.File {
	sourceFile, err := ioutil.TempFile("", filename)
	Expect(err).NotTo(HaveOccurred())
	sourceFile.WriteString(content)
	sourceFile.Close()

	return sourceFile
}

func createArchive(filename, sampleData string) *os.File {
	sourceFile, err := ioutil.TempFile("", filename)

	Expect(err).NotTo(HaveOccurred())

	// Create a new tar archive.
	tw := tar.NewWriter(sourceFile)

	// Add some files to the archive.
	var files = []struct {
		Name, Body string
		Type       byte
		Mode       int64
	}{
		{"bin/readme.txt", "This archive contains some text files.", tar.TypeReg, 0600},
		{"diego.txt", "Diego names:\nVizzini\nGeoffrey\nPrincess Buttercup\n", tar.TypeReg, 0600},
		{"testdir", "", tar.TypeDir, 0766},
		{"testdir/file.txt", sampleData, tar.TypeReg, 0600},
	}
	for _, file := range files {
		hdr := &tar.Header{
			Name:     file.Name,
			Typeflag: file.Type,
			Mode:     file.Mode,
			Size:     int64(len(file.Body)),
		}
		err := tw.WriteHeader(hdr)
		Expect(err).NotTo(HaveOccurred())
		_, err = tw.Write([]byte(file.Body))
		Expect(err).NotTo(HaveOccurred())
	}
	// Make sure to check the error on Close.
	err = tw.Close()
	Expect(err).NotTo(HaveOccurred())
	err = sourceFile.Close()
	Expect(err).NotTo(HaveOccurred())
	return sourceFile
}

func filenamesInDir(dir string) []string {
	entries, err := ioutil.ReadDir(dir)
	Expect(err).NotTo(HaveOccurred())

	result := []string{}
	for _, entry := range entries {
		result = append(result, entry.Name())
	}

	return result
}
