package bosh_test

import (
	"bytes"
	"errors"
	"os"
	"strings"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/pivotal-cf-experimental/destiny/bosh"
)

const (
	manifest = `name: hello`

	opsYAML = `
- type: replace
  path: /name
  value: world
`
)

var _ = Describe("CLI", func() {
	Describe("Interpolate", func() {
		var (
			boshCLI bosh.CLI
		)

		It("returns a manifest given a manifest and ops yaml", func() {
			newManifest, err := boshCLI.Interpolate(manifest, opsYAML)
			Expect(err).NotTo(HaveOccurred())
			Expect(newManifest).To(Equal("name: world"))
		})

		Context("failure cases", func() {
			It("returns an error when the temp dir cannot be created", func() {
				bosh.SetTempDir(func(dir, prefix string) (string, error) {
					return "", errors.New("failed to make temp dir")
				})

				_, err := boshCLI.Interpolate(manifest, opsYAML)
				Expect(err).To(MatchError("failed to make temp dir"))

				bosh.ResetTempDir()
			})

			It("returns an error when it can't write the manifest", func() {
				bosh.SetWriteFile(func(string, []byte, os.FileMode) error {
					return errors.New("failed to write manifest")
				})

				_, err := boshCLI.Interpolate(manifest, opsYAML)
				Expect(err).To(MatchError("failed to write manifest"))

				bosh.ResetWriteFile()
			})

			It("returns an error when it can't write the ops file", func() {
				bosh.SetWriteFile(func(path string, contents []byte, mode os.FileMode) error {
					if strings.Contains(path, "ops") {
						return errors.New("failed to write ops file")
					}
					return nil
				})

				_, err := boshCLI.Interpolate(manifest, opsYAML)
				Expect(err).To(MatchError("failed to write ops file"))

				bosh.ResetWriteFile()
			})

			It("returns an error when the command fails to run", func() {
				stderr := bytes.NewBuffer([]byte{})
				bosh.SetStderr(stderr)

				_, err := boshCLI.Interpolate("", opsYAML)
				Expect(err).To(MatchError("exit status 1"))

				Expect(stderr.String()).To(ContainSubstring("Expected to find a map at path"))

				bosh.ResetStderr()
			})
		})
	})
})
