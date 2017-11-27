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

var _ = Describe("Scaling down instances", func() {
	var (
		manifest     string
		manifestName string

		testKey   string
		testValue string

		kv      consulclient.HTTPKV
		spammer *helpers.Spammer
	)

	Describe("scaling from 3 nodes to 1", func() {
		BeforeEach(func() {
			var err error
			guid, err := helpers.NewGUID()
			Expect(err).NotTo(HaveOccurred())

			testKey = "consul-key-" + guid
			testValue = "consul-value-" + guid

			manifest, err = helpers.DeployConsulWithInstanceCount("scale-down-3-to-1", 3, config.WindowsClients, boshClient)
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

		It("provides a functioning server after the scale down", func() {
			By("setting a persistent value to check the cluster is up", func() {
				err := kv.Set(testKey, testValue)
				Expect(err).NotTo(HaveOccurred())
			})

			By("scaling from 3 nodes to 1", func() {
				var err error
				manifest, err = ops.ApplyOp(manifest, ops.Op{
					Type:  "replace",
					Path:  "/instance_groups/name=consul/instances",
					Value: 1,
				})
				Expect(err).NotTo(HaveOccurred())

				_, err = boshClient.Deploy([]byte(manifest))
				Expect(err).NotTo(HaveOccurred())

				Eventually(func() ([]bosh.VM, error) {
					return helpers.DeploymentVMs(boshClient, manifestName)
				}, "5m", "10s").Should(ConsistOf(helpers.GetVMsFromManifest(manifest)))
			})

			By("checking if value was persisted", func() {
				actualVal, err := kv.Get(testKey)
				Expect(err).NotTo(HaveOccurred())
				Expect(actualVal).To(Equal(testValue))
			})

			By("setting a persistent value to check the cluster is up after the scale down", func() {
				err := kv.Set(testKey, testValue)
				Expect(err).NotTo(HaveOccurred())
			})
		})
	})

	Describe("scaling from 5 nodes to 3", func() {
		BeforeEach(func() {
			var err error
			guid, err := helpers.NewGUID()
			Expect(err).NotTo(HaveOccurred())

			testKey = "consul-key-" + guid
			testValue = "consul-value-" + guid

			manifest, err = helpers.DeployConsulWithInstanceCount("scale-down-5-to-3", 5, config.WindowsClients, boshClient)
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

		It("persists data throughout the scale down", func() {
			By("setting a persistent value", func() {
				err := kv.Set(testKey, testValue)
				Expect(err).NotTo(HaveOccurred())
			})

			By("scaling from 5 nodes to 3", func() {
				var err error
				manifest, err = ops.ApplyOp(manifest, ops.Op{
					Type:  "replace",
					Path:  "/instance_groups/name=consul/instances",
					Value: 3,
				})
				Expect(err).NotTo(HaveOccurred())

				spammer.Spam()

				_, err = boshClient.Deploy([]byte(manifest))
				Expect(err).NotTo(HaveOccurred())

				Eventually(func() ([]bosh.VM, error) {
					return helpers.DeploymentVMs(boshClient, manifestName)
				}, "5m", "10s").Should(ConsistOf(helpers.GetVMsFromManifest(manifest)))

				spammer.Stop()
			})

			By("reading the value from consul", func() {
				value, err := kv.Get(testKey)
				Expect(err).NotTo(HaveOccurred())
				Expect(value).To(Equal(testValue))
			})

			By("checking the spammer for errors during the deploy", func() {
				err := spammer.Check()
				Expect(err).NotTo(HaveOccurred())
			})
		})
	})
})
