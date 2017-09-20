package deploy_test

import (
	"fmt"

	etcdclient "acceptance-tests/testing/etcd"
	"acceptance-tests/testing/helpers"

	"github.com/pivotal-cf-experimental/bosh-test/bosh"
	"github.com/pivotal-cf-experimental/destiny/etcd"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Single instance rolling deploys", func() {
	SingleInstanceRollingDeploy := func(enableSSL bool) {
		var (
			manifest   etcd.Manifest
			etcdClient etcdclient.Client

			testKey   string
			testValue string
		)

		BeforeEach(func() {
			guid, err := helpers.NewGUID()
			Expect(err).NotTo(HaveOccurred())

			testKey = "etcd-key-" + guid
			testValue = "etcd-value-" + guid

			manifest, err = helpers.DeployEtcdWithInstanceCount("single_instance_rolling_deploy", 1, client, config, enableSSL)
			Expect(err).NotTo(HaveOccurred())

			Eventually(func() ([]bosh.VM, error) {
				return helpers.DeploymentVMs(client, manifest.Name)
			}, "1m", "10s").Should(ConsistOf(helpers.GetVMsFromManifest(manifest)))
		})

		AfterEach(func() {
			if !CurrentGinkgoTestDescription().Failed {
				err := client.DeleteDeployment(manifest.Name)
				Expect(err).NotTo(HaveOccurred())
			}
		})

		It("persists data throughout the rolling deploy", func() {
			By("setting a persistent value", func() {
				testConsumerIndex, err := helpers.FindJobIndexByName(manifest, "testconsumer_z1")
				Expect(err).NotTo(HaveOccurred())
				etcdClient = etcdclient.NewClient(fmt.Sprintf("http://%s:6769", manifest.Jobs[testConsumerIndex].Networks[0].StaticIPs[0]))

				err = etcdClient.Set(testKey, testValue)
				Expect(err).ToNot(HaveOccurred())
			})

			By("deploying", func() {
				manifest.Properties.Etcd.HeartbeatIntervalInMilliseconds = 51

				yaml, err := manifest.ToYAML()
				Expect(err).NotTo(HaveOccurred())

				yaml, err = client.ResolveManifestVersions(yaml)
				Expect(err).NotTo(HaveOccurred())

				_, err = client.Deploy(yaml)
				Expect(err).NotTo(HaveOccurred())

				Eventually(func() ([]bosh.VM, error) {
					return helpers.DeploymentVMs(client, manifest.Name)
				}, "1m", "10s").Should(ConsistOf(helpers.GetVMsFromManifest(manifest)))
			})

			By("reading the value from etcd", func() {
				value, err := etcdClient.Get(testKey)
				Expect(err).ToNot(HaveOccurred())
				Expect(value).To(Equal(testValue))
			})
		})
	}

	Context("without TLS", func() {
		SingleInstanceRollingDeploy(false)
	})

	Context("with TLS", func() {
		SingleInstanceRollingDeploy(true)
	})
})
