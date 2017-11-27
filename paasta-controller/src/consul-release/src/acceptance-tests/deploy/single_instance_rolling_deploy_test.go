package deploy_test

import (
	"fmt"

	"github.com/cloudfoundry-incubator/consul-release/src/acceptance-tests/testing/consulclient"
	"github.com/cloudfoundry-incubator/consul-release/src/acceptance-tests/testing/helpers"
	"github.com/pivotal-cf-experimental/bosh-test/bosh"
	"github.com/pivotal-cf-experimental/destiny/ops"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Single instance rolling deploys", func() {
	var (
		manifest     string
		manifestName string

		kv consulclient.HTTPKV

		testKey   string
		testValue string
	)

	BeforeEach(func() {
		guid, err := helpers.NewGUID()
		Expect(err).NotTo(HaveOccurred())

		testKey = "consul-key-" + guid
		testValue = "consul-value-" + guid

		manifest, err = helpers.DeployConsulWithInstanceCount("single-instance-rolling-deploy", 1, config.WindowsClients, boshClient)
		Expect(err).NotTo(HaveOccurred())

		manifestName, err = ops.ManifestName(manifest)
		Expect(err).NotTo(HaveOccurred())

		Eventually(func() ([]bosh.VM, error) {
			return helpers.DeploymentVMs(boshClient, manifestName)
		}, "5m", "10s").Should(ConsistOf(helpers.GetVMsFromManifest(manifest)))

		testConsumerIPs, err := helpers.GetVMIPs(boshClient, manifestName, "testconsumer")
		Expect(err).NotTo(HaveOccurred())

		kv = consulclient.NewHTTPKV(fmt.Sprintf("http://%s:6769", testConsumerIPs[0]))
	})

	AfterEach(func() {
		if !CurrentGinkgoTestDescription().Failed {
			err := boshClient.DeleteDeployment(manifestName)
			Expect(err).NotTo(HaveOccurred())
		}
	})

	It("persists data throughout the rolling deploy", func() {
		By("setting a persistent value", func() {
			err := kv.Set(testKey, testValue)
			Expect(err).NotTo(HaveOccurred())
		})

		By("deploying", func() {
			var err error
			manifest, err = ops.ApplyOp(manifest, ops.Op{
				Type:  "replace",
				Path:  "/instance_groups/name=consul/properties/consul/agent/log_level?",
				Value: "trace",
			})
			Expect(err).NotTo(HaveOccurred())

			_, err = boshClient.Deploy([]byte(manifest))
			Expect(err).NotTo(HaveOccurred())

			Eventually(func() ([]bosh.VM, error) {
				return helpers.DeploymentVMs(boshClient, manifestName)
			}, "5m", "10s").Should(ConsistOf(helpers.GetVMsFromManifest(manifest)))
		})

		By("reading the value from consul", func() {
			value, err := kv.Get(testKey)
			Expect(err).NotTo(HaveOccurred())
			Expect(value).To(Equal(testValue))
		})
	})
})
