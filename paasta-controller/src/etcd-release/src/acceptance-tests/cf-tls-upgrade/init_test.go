package cf_tls_upgrade_test

import (
	"testing"

	"github.com/cloudfoundry-incubator/etcd-release/src/acceptance-tests/testing/helpers"

	"github.com/pivotal-cf-experimental/bosh-test/bosh"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var (
	config     helpers.Config
	boshClient bosh.Client
)

func TestCFTLSUpgrade(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "cf-tls-upgrade")
}

var _ = BeforeSuite(func() {
	configPath, err := helpers.ConfigPath()
	Expect(err).NotTo(HaveOccurred())

	config, err = helpers.LoadConfig(configPath)
	Expect(err).NotTo(HaveOccurred())

	boshClient = bosh.NewClient(bosh.Config{
		URL:              config.BOSH.Target,
		Host:             config.BOSH.Host,
		Username:         config.BOSH.Username,
		Password:         config.BOSH.Password,
		AllowInsecureSSL: true,
	})
})
