package chaperon_test

import (
	"fmt"
	"io/ioutil"
	"os"
	"runtime"

	"code.cloudfoundry.org/lager"

	"github.com/cloudfoundry-incubator/consul-release/src/confab/chaperon"
	"github.com/cloudfoundry-incubator/consul-release/src/confab/fakes"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	. "github.com/pivotal-cf-experimental/gomegamatchers"
)

var _ = Describe("KeyringRemover", func() {
	Describe("Execute", func() {
		var (
			dataDir string
			keyring string
			logger  *fakes.Logger
			remover chaperon.KeyringRemover
		)

		BeforeEach(func() {
			var err error
			dataDir, err = ioutil.TempDir("", "")
			Expect(err).NotTo(HaveOccurred())

			file, err := ioutil.TempFile(dataDir, "keyring")
			Expect(err).NotTo(HaveOccurred())
			defer file.Close()
			keyring = file.Name()

			logger = &fakes.Logger{}

			remover = chaperon.NewKeyringRemover(keyring, logger)
		})

		It("removes the keyring file", func() {
			err := remover.Execute()
			Expect(err).NotTo(HaveOccurred())

			_, err = os.Stat(keyring)
			Expect(err).To(BeAnOsIsNotExistError())

			Expect(logger.Messages()).To(ContainSequence([]fakes.LoggerMessage{
				{
					Action: "keyring-remover.execute",
					Data: []lager.Data{{
						"keyring": keyring,
					}},
				},
				{
					Action: "keyring-remover.execute.success",
					Data: []lager.Data{{
						"keyring": keyring,
					}},
				},
			}))
		})

		Context("when the file does not exist", func() {
			It("does not error", func() {
				err := os.Remove(keyring)
				Expect(err).NotTo(HaveOccurred())

				err = remover.Execute()
				Expect(err).NotTo(HaveOccurred())
			})
		})

		Context("failure cases", func() {
			Context("when the file cannot be removed", func() {
				It("returns an error", func() {
					if runtime.GOOS == "windows" {
						// file permissions do not prevent file owners from being able to
						// delete files on Windows. Couldn't think of another way to make this
						// test still create an error.
						Skip("Test doesn't work on Windows")
					}
					err := os.Chmod(dataDir, 0000)
					Expect(err).NotTo(HaveOccurred())

					err = remover.Execute()
					Expect(err).To(MatchError(ContainSubstring("permission denied")))

					Expect(logger.Messages()).To(ContainSequence([]fakes.LoggerMessage{
						{
							Action: "keyring-remover.execute",
							Data: []lager.Data{{
								"keyring": keyring,
							}},
						},
						{
							Action: "keyring-remover.execute.failed",
							Error:  fmt.Errorf("remove %s: permission denied", keyring),
							Data: []lager.Data{{
								"keyring": keyring,
							}},
						},
					}))
				})
			})
		})
	})
})
