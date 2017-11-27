package deploy_test

import (
	"testing"

	"github.com/cloudfoundry/postgres-release/src/acceptance-tests/testing/helpers"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var (
	director     helpers.BOSHDirector
	configParams helpers.PgatsConfig
	versions     helpers.PostgresReleaseVersions
)

func TestDeploy(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "deploy")
}

var _ = BeforeSuite(func() {
	configPath, err := helpers.ConfigPath()
	Expect(err).NotTo(HaveOccurred())

	configParams, err = helpers.LoadConfig(configPath)
	Expect(err).NotTo(HaveOccurred())

	releases := make(map[string]string)
	releases["postgres"] = configParams.PGReleaseVersion

	director, err = helpers.NewBOSHDirector(configParams.Bosh, configParams.BoshCC, releases)
	Expect(err).NotTo(HaveOccurred())

	versions, err = helpers.NewPostgresReleaseVersions(configParams.VersionsFile)
	Expect(err).NotTo(HaveOccurred())
})
