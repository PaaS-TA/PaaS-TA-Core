package cacheddownloader_test

import (
	"io/ioutil"
	"os"
	"path/filepath"

	"code.cloudfoundry.org/archiver/extractor/test_helper"
	. "code.cloudfoundry.org/cacheddownloader"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("TarTransformer", func() {
	var (
		scratch string

		sourcePath      string
		destinationPath string

		transformedSize int64
		transformErr    error
	)

	archiveFiles := []test_helper.ArchiveFile{
		{Name: "some-file", Body: "some-contents"},
	}

	BeforeEach(func() {
		var err error

		scratch, err = ioutil.TempDir("", "tar-transformer-scratch")
		Expect(err).ShouldNot(HaveOccurred())

		destinationFile, err := ioutil.TempFile("", "destination")
		Expect(err).ShouldNot(HaveOccurred())

		err = destinationFile.Close()
		Expect(err).ShouldNot(HaveOccurred())

		destinationPath = destinationFile.Name()
	})

	AfterEach(func() {
		err := os.RemoveAll(scratch)
		Expect(err).ShouldNot(HaveOccurred())
	})

	JustBeforeEach(func() {
		transformedSize, transformErr = TarTransform(sourcePath, destinationPath)
	})

	Context("when the file is a .zip", func() {
		BeforeEach(func() {
			sourcePath = filepath.Join(scratch, "file.zip")

			test_helper.CreateZipArchive(sourcePath, archiveFiles)
		})

		It("closes the tarfile", func() {
			// On Windows, you can't remove files that are still open.  On Linux, you can.
			err := os.Remove(destinationPath)

			Expect(err).ShouldNot(HaveOccurred())
		})
	})
})
