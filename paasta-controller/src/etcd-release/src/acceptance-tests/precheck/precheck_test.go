package precheck_test

import (
	"fmt"
	"testing"

	"acceptance-tests/testing/helpers"

	"github.com/pivotal-cf-experimental/bosh-test/bosh"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var (
	config helpers.Config
	client bosh.Client
)

func TestDeploy(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "precheck")
}

var _ = BeforeSuite(func() {
	configPath, err := helpers.ConfigPath()
	Expect(err).NotTo(HaveOccurred())

	config, err = helpers.LoadConfig(configPath)
	Expect(err).NotTo(HaveOccurred())

	client = bosh.NewClient(bosh.Config{
		URL:              fmt.Sprintf("https://%s:25555", config.BOSH.Target),
		Username:         config.BOSH.Username,
		Password:         config.BOSH.Password,
		AllowInsecureSSL: true,
	})
})

var _ = Describe("precheck", func() {
	It("confirms that there are no conflicting deployments", func() {
		deployments, err := client.Deployments()
		Expect(err).NotTo(HaveOccurred())

		for _, deployment := range deployments {
			for _, name := range []string{"etcd", "turbulence-etcd"} {
				ok, err := MatchRegexp(fmt.Sprintf("%s-\\w{8}-\\w{4}-\\w{4}-\\w{4}-\\w{12}", name)).Match(deployment.Name)

				if err != nil {
					Fail(err.Error())
				}
				if ok {
					Fail(fmt.Sprintf("existing deployment %s", deployment.Name))
				}
			}
		}
	})
})
