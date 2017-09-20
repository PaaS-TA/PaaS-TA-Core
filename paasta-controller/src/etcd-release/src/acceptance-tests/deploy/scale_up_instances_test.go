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

var _ = Describe("Scaling up instances", func() {
	ScaleUpInstances := func(enableSSL bool) {

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

			manifest, err = helpers.DeployEtcdWithInstanceCount("scale_up_instances", 1, client, config, enableSSL)
			Expect(err).NotTo(HaveOccurred())

			Eventually(func() ([]bosh.VM, error) {
				return helpers.DeploymentVMs(client, manifest.Name)
			}, "1m", "10s").Should(ConsistOf(helpers.GetVMsFromManifest(manifest)))

			testConsumerIndex, err := helpers.FindJobIndexByName(manifest, "testconsumer_z1")
			Expect(err).NotTo(HaveOccurred())
			etcdClient = etcdclient.NewClient(fmt.Sprintf("http://%s:6769", manifest.Jobs[testConsumerIndex].Networks[0].StaticIPs[0]))
		})

		AfterEach(func() {
			if !CurrentGinkgoTestDescription().Failed {
				err := client.DeleteDeployment(manifest.Name)
				Expect(err).NotTo(HaveOccurred())
			}
		})

		It("scales from 1 to 3 nodes", func() {
			By("setting a persistent value", func() {
				err := etcdClient.Set(testKey, testValue)
				Expect(err).ToNot(HaveOccurred())
			})

			By("scaling up to 3 nodes", func() {
				var err error
				manifest, err = helpers.SetEtcdInstanceCount(3, manifest)

				members := manifest.EtcdMembers()
				Expect(members).To(HaveLen(3))

				yaml, err := manifest.ToYAML()
				Expect(err).NotTo(HaveOccurred())

				_, err = client.Deploy(yaml)
				Expect(err).NotTo(HaveOccurred())

				Eventually(func() ([]bosh.VM, error) {
					return helpers.DeploymentVMs(client, manifest.Name)
				}, "1m", "10s").Should(ConsistOf(helpers.GetVMsFromManifest(manifest)))
			})

			By("reading the value from each etcd node in the cluster", func() {
				value, err := etcdClient.Get(testKey)
				Expect(err).ToNot(HaveOccurred())
				Expect(value).To(Equal(testValue))
			})
		})
	}

	Context("without TLS", func() {
		ScaleUpInstances(false)
	})

	Context("with TLS", func() {
		ScaleUpInstances(true)
	})
})
