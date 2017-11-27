package fixtures

import (
	"io/ioutil"
	"os"

	archive_helper "code.cloudfoundry.org/archiver/extractor/test_helper"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"
)

func GoServerApp() []archive_helper.ArchiveFile {
	originalCGOValue := os.Getenv("CGO_ENABLED")
	os.Setenv("CGO_ENABLED", "0")

	serverPath, err := gexec.Build("code.cloudfoundry.org/inigo/fixtures/go-server")

	os.Setenv("CGO_ENABLED", originalCGOValue)

	Expect(err).NotTo(HaveOccurred())

	contents, err := ioutil.ReadFile(serverPath)
	Expect(err).NotTo(HaveOccurred())
	return []archive_helper.ArchiveFile{
		{
			Name: "go-server",
			Body: string(contents),
		}, {
			Name: "staging_info.yml",
			Body: `detected_buildpack: Doesn't Matter
start_command: go-server`,
		},
	}
}
