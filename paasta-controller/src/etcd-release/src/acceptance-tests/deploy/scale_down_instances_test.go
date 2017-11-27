package deploy_test

import (
	"fmt"

	etcdclient "github.com/cloudfoundry-incubator/etcd-release/src/acceptance-tests/testing/etcd"

	"github.com/cloudfoundry-incubator/etcd-release/src/acceptance-tests/testing/helpers"

	"github.com/pivotal-cf-experimental/bosh-test/bosh"
	"github.com/pivotal-cf-experimental/destiny/ops"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Scaling down instances", func() {
	ScaleDownInstances := func(enableSSL bool) {

		var (
			manifest     string
			manifestName string

			testKey   string
			testValue string

			etcdClient etcdclient.Client
		)

		BeforeEach(func() {
			guid, err := helpers.NewGUID()
			Expect(err).NotTo(HaveOccurred())

			testKey = "etcd-key-" + guid
			testValue = "etcd-value-" + guid

			deploymentName := "scale-down-5-to-3-non-tls"
			if enableSSL {
				deploymentName = "scale-down-5-to-3-tls"
			}
			manifest, err = helpers.DeployEtcdWithInstanceCount(deploymentName, 5, enableSSL, boshClient)
			Expect(err).NotTo(HaveOccurred())

			manifestName, err = ops.ManifestName(manifest)
			Expect(err).NotTo(HaveOccurred())

			Eventually(func() ([]bosh.VM, error) {
				return helpers.DeploymentVMs(boshClient, manifestName)
			}, "1m", "10s").Should(ConsistOf(helpers.GetVMsFromManifest(manifest)))

			testConsumerIPs, err := helpers.GetVMIPs(boshClient, manifestName, "testconsumer")
			Expect(err).NotTo(HaveOccurred())

			etcdClient = etcdclient.NewClient(fmt.Sprintf("http://%s:6769", testConsumerIPs[0]))
		})

		AfterEach(func() {
			if !CurrentGinkgoTestDescription().Failed {
				err := boshClient.DeleteDeployment(manifestName)
				Expect(err).NotTo(HaveOccurred())
			}
		})

		It("scales from 5 to 3 nodes", func() {
			By("setting a persistent value", func() {
				err := etcdClient.Set(testKey, testValue)
				Expect(err).ToNot(HaveOccurred())
			})

			By("scaling down to 3 node", func() {
				var err error
				manifest, err = ops.ApplyOp(manifest, ops.Op{
					Type:  "replace",
					Path:  "/instance_groups/name=etcd/instances",
					Value: 3,
				})
				Expect(err).NotTo(HaveOccurred())

				_, err = boshClient.Deploy([]byte(manifest))
				Expect(err).NotTo(HaveOccurred())

				Eventually(func() ([]bosh.VM, error) {
					return helpers.DeploymentVMs(boshClient, manifestName)
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
		ScaleDownInstances(false)
	})

	Context("with TLS", func() {
		ScaleDownInstances(true)
	})
})
