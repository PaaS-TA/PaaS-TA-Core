package utils_test

import (
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/cloudfoundry-incubator/etcd-release/src/acceptance-tests/testing/monit_agent/utils"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("RemoveStore", func() {
	Describe("Delete", func() {
		var (
			removeStore utils.RemoveStore

			tempDir string
		)

		BeforeEach(func() {
			var err error
			tempDir, err = ioutil.TempDir("", "")
			Expect(err).NotTo(HaveOccurred())

			err = ioutil.WriteFile(filepath.Join(tempDir, "some_file"), []byte(""), os.ModePerm)
			Expect(err).NotTo(HaveOccurred())

			err = os.Mkdir(filepath.Join(tempDir, "nested"), os.ModePerm)
			Expect(err).NotTo(HaveOccurred())

			err = ioutil.WriteFile(filepath.Join(tempDir, "nested", "some_file"), []byte(""), os.ModePerm)
			Expect(err).NotTo(HaveOccurred())

			removeStore = utils.NewRemoveStore()
		})

		It("deletes the contents of the provided directory", func() {
			err := removeStore.DeleteContents(tempDir)
			Expect(err).NotTo(HaveOccurred())

			directoryContents, err := ioutil.ReadDir(tempDir)
			Expect(err).NotTo(HaveOccurred())

			Expect(len(directoryContents)).To(Equal(0))
		})

		Context("failure cases", func() {
			It("bubbles up errors returned by ioutil.ReadDir", func() {
				err := removeStore.DeleteContents("nonexistent_dir")
				Expect(err).To(MatchError("open nonexistent_dir: no such file or directory"))
			})
		})
	})
})
