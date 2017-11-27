package deploy_test

import (
	"fmt"
	"os"
	"time"

	"github.com/cloudfoundry-incubator/consul-release/src/acceptance-tests/testing/consulclient"
	"github.com/cloudfoundry-incubator/consul-release/src/acceptance-tests/testing/helpers"
	"github.com/pivotal-cf-experimental/bosh-test/bosh"
	"github.com/pivotal-cf-experimental/destiny/ops"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = PDescribe("Multiple instance rolling upgrade", func() {
	var (
		manifest     string
		manifestName string

		kv      consulclient.HTTPKV
		spammer *helpers.Spammer

		testKey   string
		testValue string
	)

	BeforeEach(func() {
		guid, err := helpers.NewGUID()
		Expect(err).NotTo(HaveOccurred())

		testKey = "consul-key-" + guid
		testValue = "consul-value-" + guid
	})

	AfterEach(func() {
		if !CurrentGinkgoTestDescription().Failed {
			err := boshClient.DeleteDeployment(manifestName)
			Expect(err).NotTo(HaveOccurred())
		}
	})

	It("persists data throughout the rolling upgrade", func() {
		By("deploying the previous version of consul-release", func() {
			releaseNumber := os.Getenv("LATEST_CONSUL_RELEASE_VERSION")

			var err error
			manifest, err = helpers.NewConsulManifestWithInstanceCountAndReleaseVersion("multiple-instance-rolling-upgrade", 3, config.WindowsClients, boshClient, releaseNumber)
			Expect(err).NotTo(HaveOccurred())

			manifestName, err = ops.ManifestName(manifest)
			Expect(err).NotTo(HaveOccurred())

			_, err = boshClient.Deploy([]byte(manifest))
			Expect(err).NotTo(HaveOccurred())

			Eventually(func() ([]bosh.VM, error) {
				return helpers.DeploymentVMs(boshClient, manifestName)
			}, "5m", "10s").Should(ConsistOf(helpers.GetVMsFromManifest(manifest)))

			testConsumerIPs, err := helpers.GetVMIPs(boshClient, manifestName, "testconsumer")
			Expect(err).NotTo(HaveOccurred())

			kv = consulclient.NewHTTPKV(fmt.Sprintf("http://%s:6769", testConsumerIPs[0]))

			spammer = helpers.NewSpammer(kv, 1*time.Second, "testconsumer")
		})

		By("setting a persistent value", func() {
			err := kv.Set(testKey, testValue)
			Expect(err).NotTo(HaveOccurred())
		})

		By("deploying with the new consul-release", func() {
			spammer.Spam()

			var err error
			manifest, err = helpers.DeployConsulWithInstanceCount("multiple-instance-rolling-upgrade", 3, config.WindowsClients, boshClient)
			Expect(err).NotTo(HaveOccurred())

			Eventually(func() ([]bosh.VM, error) {
				return helpers.DeploymentVMs(boshClient, manifestName)
			}, "5m", "10s").Should(ConsistOf(helpers.GetVMsFromManifest(manifest)))

			err = helpers.VerifyDeploymentRelease(boshClient, manifestName, helpers.ConsulReleaseVersion())
			Expect(err).NotTo(HaveOccurred())

			spammer.Stop()
		})

		By("reading the values from consul", func() {
			value, err := kv.Get(testKey)
			Expect(err).NotTo(HaveOccurred())
			Expect(value).To(Equal(testValue))

			err = spammer.Check()
			Expect(err).NotTo(HaveOccurred())
		})
	})
})
