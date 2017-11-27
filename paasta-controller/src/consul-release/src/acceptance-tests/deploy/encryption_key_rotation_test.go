package deploy_test

import (
	"fmt"
	"time"

	"github.com/cloudfoundry-incubator/consul-release/src/acceptance-tests/testing/consulclient"
	"github.com/cloudfoundry-incubator/consul-release/src/acceptance-tests/testing/helpers"
	"github.com/pivotal-cf-experimental/bosh-test/bosh"
	"github.com/pivotal-cf-experimental/destiny/ops"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Encryption key rotation", func() {
	var (
		manifest     string
		manifestName string

		testKey   string
		testValue string

		kv      consulclient.HTTPKV
		spammer *helpers.Spammer
	)

	BeforeEach(func() {
		guid, err := helpers.NewGUID()
		Expect(err).NotTo(HaveOccurred())

		testKey = "consul-key-" + guid
		testValue = "consul-value-" + guid

		manifest, err = helpers.DeployConsulWithInstanceCount("encryption-key-rotation", 3, config.WindowsClients, boshClient)
		Expect(err).NotTo(HaveOccurred())

		manifestName, err = ops.ManifestName(manifest)
		Expect(err).NotTo(HaveOccurred())

		Eventually(func() ([]bosh.VM, error) {
			return helpers.DeploymentVMs(boshClient, manifestName)
		}, "5m", "10s").Should(ConsistOf(helpers.GetVMsFromManifest(manifest)))

		testConsumerIPs, err := helpers.GetVMIPs(boshClient, manifestName, "testconsumer")
		Expect(err).NotTo(HaveOccurred())

		kv = consulclient.NewHTTPKV(fmt.Sprintf("http://%s:6769", testConsumerIPs[0]))

		spammer = helpers.NewSpammer(kv, 1*time.Second, "test-consumer-0")
	})

	AfterEach(func() {
		if !CurrentGinkgoTestDescription().Failed {
			err := boshClient.DeleteDeployment(manifestName)
			Expect(err).NotTo(HaveOccurred())
		}
	})

	It("successfully rolls with a new encryption key", func() {
		By("setting a persistent value", func() {
			err := kv.Set(testKey, testValue)
			Expect(err).NotTo(HaveOccurred())
		})

		By("adding a new primary encryption key", func() {
			oldEncryptionKey, err := ops.FindOp(manifest, "/instance_groups/name=consul/properties/consul/encrypt_keys/0")
			Expect(err).NotTo(HaveOccurred())

			manifest, err = ops.ApplyOps(manifest, []ops.Op{
				{
					Type:  "replace",
					Path:  "/instance_groups/name=consul/properties/consul/encrypt_keys/0",
					Value: "banana",
				},
				{
					Type:  "replace",
					Path:  "/instance_groups/name=consul/properties/consul/encrypt_keys/-",
					Value: oldEncryptionKey,
				},
			})
			Expect(err).NotTo(HaveOccurred())
		})

		By("deploying with the new key", func() {
			spammer.Spam()

			_, err := boshClient.Deploy([]byte(manifest))
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

		By("removing the old encryption key", func() {
			var err error
			manifest, err = ops.ApplyOp(manifest, ops.Op{
				Type: "remove",
				Path: "/instance_groups/name=consul/properties/consul/encrypt_keys/1",
			})
			Expect(err).NotTo(HaveOccurred())
		})

		By("deploying with the original key removed", func() {
			_, err := boshClient.Deploy([]byte(manifest))
			Expect(err).NotTo(HaveOccurred())

			Eventually(func() ([]bosh.VM, error) {
				return helpers.DeploymentVMs(boshClient, manifestName)
			}, "5m", "10s").Should(ConsistOf(helpers.GetVMsFromManifest(manifest)))

			spammer.Stop()
		})

		By("checking if values was persisted", func() {
			value, err := kv.Get(testKey)
			Expect(err).NotTo(HaveOccurred())
			Expect(value).To(Equal(testValue))

			err = spammer.Check()
			Expect(err).NotTo(HaveOccurred())
		})
	})
})
