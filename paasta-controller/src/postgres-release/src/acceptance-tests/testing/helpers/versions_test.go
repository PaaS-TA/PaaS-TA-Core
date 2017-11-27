package helpers_test

import (
	"io/ioutil"
	"os"

	"github.com/cloudfoundry/postgres-release/src/acceptance-tests/testing/helpers"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func writeVersionsFile(data string) (string, error) {
	tempFile, err := ioutil.TempFile("", "versions")
	if err != nil {
		return "", err
	}

	err = ioutil.WriteFile(tempFile.Name(), []byte(data), os.ModePerm)
	if err != nil {
		return "", err
	}

	return tempFile.Name(), nil
}

var _ = Describe("Postgres-release versions", func() {
	Context("With a valid yaml file", func() {
		var versionsFilePath string
		var pgVersions helpers.PostgresReleaseVersions

		BeforeEach(func() {
			var err error
			var data = `
versions:
  1: "PostgreSQL 9.4.6"
  3: "PostgreSQL 9.4.9"
  2: "PostgreSQL 9.4.6"
old: 3
older: 1
`
			versionsFilePath, err = writeVersionsFile(data)
			Expect(err).NotTo(HaveOccurred())
			pgVersions, err = helpers.NewPostgresReleaseVersions(versionsFilePath)
			Expect(err).NotTo(HaveOccurred())
		})
		AfterEach(func() {
			err := os.Remove(versionsFilePath)
			Expect(err).NotTo(HaveOccurred())
		})

		It("Get the latest postgres-release version", func() {
			latestVersion := pgVersions.GetLatestVersion()
			Expect(latestVersion).To(Equal(3))
		})
		It("Get the proper postgres-release old version", func() {
			oldVersion := pgVersions.GetOldVersion()
			Expect(oldVersion).To(Equal(3))
		})
		It("Get the proper postgres-release old version", func() {
			olderVersion := pgVersions.GetOlderVersion()
			Expect(olderVersion).To(Equal(1))
		})
		It("Get the proper PostgreSQL version", func() {
			pgVersion := pgVersions.GetPostgreSQLVersion(1)
			Expect(pgVersion).To(Equal("PostgreSQL 9.4.6"))
			pgVersion = pgVersions.GetPostgreSQLVersion(2)
			Expect(pgVersion).To(Equal("PostgreSQL 9.4.6"))
			pgVersion = pgVersions.GetPostgreSQLVersion(3)
			Expect(pgVersion).To(Equal("PostgreSQL 9.4.9"))
		})
	})
	Context("With an invalid config yaml location", func() {
		It("Should return an error that the file does not exist", func() {
			_, err := helpers.LoadConfig("notExistentPath")
			Expect(err).To(MatchError(ContainSubstring("no such file or directory")))
		})
	})

	Context("With an incorrect config yaml content", func() {
		var versionsFilePath string

		AfterEach(func() {
			err := os.Remove(versionsFilePath)
			Expect(err).NotTo(HaveOccurred())
		})

		It("Should return an error if not a valid yaml", func() {
			var err error
			versionsFilePath, err = writeVersionsFile("%%%")
			Expect(err).NotTo(HaveOccurred())

			_, err = helpers.NewPostgresReleaseVersions(versionsFilePath)
			Expect(err).To(MatchError(ContainSubstring("yaml: could not find expected directive name")))
		})
	})
})
