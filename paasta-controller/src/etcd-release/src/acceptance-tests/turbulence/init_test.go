package turbulence_test

import (
	"testing"

	"github.com/cloudfoundry-incubator/etcd-release/src/acceptance-tests/testing/helpers"

	"github.com/pivotal-cf-experimental/bosh-test/bosh"

	turbulenceclient "github.com/pivotal-cf-experimental/bosh-test/turbulence"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var (
	config           helpers.Config
	boshClient       bosh.Client
	turbulenceClient turbulenceclient.Client
)

func TestDeploy(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "turbulence")
}

var _ = BeforeSuite(func() {
	configPath, err := helpers.ConfigPath()
	Expect(err).NotTo(HaveOccurred())

	config, err = helpers.LoadConfig(configPath)
	Expect(err).NotTo(HaveOccurred())

	boshClient = bosh.NewClient(bosh.Config{
		URL:              config.BOSH.Target,
		Host:             config.BOSH.Host,
		DirectorCACert:   config.BOSH.DirectorCACert,
		Username:         config.BOSH.Username,
		Password:         config.BOSH.Password,
		AllowInsecureSSL: true,
	})
})
