package turbulence_test

import (
	"fmt"
	"math/rand"
	"time"

	etcdclient "github.com/cloudfoundry-incubator/etcd-release/src/acceptance-tests/testing/etcd"
	turbulenceclient "github.com/pivotal-cf-experimental/bosh-test/turbulence"

	"github.com/cloudfoundry-incubator/etcd-release/src/acceptance-tests/testing/helpers"

	"github.com/pivotal-cf-experimental/bosh-test/bosh"
	"github.com/pivotal-cf-experimental/destiny/ops"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("KillVm", func() {
	KillVMTest := func(enableSSL bool) {
		var (
			turbulenceManifest     string
			turbulenceManifestName string

			etcdManifest     string
			etcdManifestName string

			etcdClient etcdclient.Client

			testKey1   string
			testValue1 string

			testKey2   string
			testValue2 string
		)

		BeforeEach(func() {
			guid, err := helpers.NewGUID()
			Expect(err).NotTo(HaveOccurred())

			testKey1 = "etcd-key-1-" + guid
			testValue1 = "etcd-value-1-" + guid

			testKey2 = "etcd-key-2-" + guid
			testValue2 = "etcd-value-2-" + guid

			deploymentSuffix := "kill-vm-non-tls"
			if enableSSL {
				deploymentSuffix = "kill-vm-tls"
			}

			By("deploying turbulence", func() {
				var err error
				turbulenceManifest, err = helpers.DeployTurbulence(deploymentSuffix, boshClient)
				Expect(err).NotTo(HaveOccurred())

				turbulenceManifestName, err = ops.ManifestName(turbulenceManifest)
				Expect(err).NotTo(HaveOccurred())

				Eventually(func() ([]bosh.VM, error) {
					return helpers.DeploymentVMs(boshClient, turbulenceManifestName)
				}, "1m", "10s").Should(ConsistOf(helpers.GetVMsFromManifest(turbulenceManifest)))

				turbulencePassword, err := ops.FindOp(turbulenceManifest, "/instance_groups/name=api/properties/password")
				Expect(err).NotTo(HaveOccurred())

				turbulenceIPs, err := helpers.GetVMIPs(boshClient, turbulenceManifestName, "api")
				Expect(err).NotTo(HaveOccurred())

				turbulenceClient = turbulenceclient.NewClient(fmt.Sprintf("https://turbulence:%s@%s:8080", turbulencePassword, turbulenceIPs[0]), 5*time.Minute, 2*time.Second)
			})

			By("deploying 3 node etcd", func() {
				etcdManifest, err = helpers.DeployEtcdWithInstanceCount(deploymentSuffix, 3, enableSSL, boshClient)
				Expect(err).NotTo(HaveOccurred())

				etcdManifestName, err = ops.ManifestName(etcdManifest)
				Expect(err).NotTo(HaveOccurred())

				Eventually(func() ([]bosh.VM, error) {
					return helpers.DeploymentVMs(boshClient, etcdManifestName)
				}, "1m", "10s").Should(ConsistOf(helpers.GetVMsFromManifest(etcdManifest)))

				testConsumerIPs, err := helpers.GetVMIPs(boshClient, etcdManifestName, "testconsumer")
				Expect(err).NotTo(HaveOccurred())

				etcdClient = etcdclient.NewClient(fmt.Sprintf("http://%s:6769", testConsumerIPs[0]))
			})
		})

		AfterEach(func() {
			By("deleting the deployment", func() {
				if !CurrentGinkgoTestDescription().Failed {
					Eventually(func() ([]string, error) {
						return lockedDeployments(boshClient)
					}, "12m", "1m").ShouldNot(ContainElement(etcdManifestName))

					Eventually(func() error {
						return boshClient.ScanAndFixAll([]byte(etcdManifest))
					}, "12m", "1m").ShouldNot(HaveOccurred())

					Eventually(func() ([]bosh.VM, error) {
						return helpers.DeploymentVMs(boshClient, etcdManifestName)
					}, "10m", "10s").Should(ConsistOf(helpers.GetVMsFromManifest(etcdManifest)))

					err := boshClient.DeleteDeployment(etcdManifestName)
					Expect(err).NotTo(HaveOccurred())
				}
			})

			By("deleting turbulence", func() {
				if !CurrentGinkgoTestDescription().Failed {
					err := boshClient.DeleteDeployment(turbulenceManifestName)
					Expect(err).NotTo(HaveOccurred())
				}
			})
		})

		Context("when a etcd node is killed", func() {
			It("is still able to function on healthy vms and recover", func() {
				By("setting a persistent value", func() {
					err := etcdClient.Set(testKey1, testValue1)
					Expect(err).ToNot(HaveOccurred())
				})

				By("killing indices", func() {
					vmIDs, err := helpers.GetVMIDByIndices(boshClient, etcdManifestName, "etcd", []int{rand.Intn(3)})
					Expect(err).NotTo(HaveOccurred())

					err = turbulenceClient.KillIDs(vmIDs)
					Expect(err).ToNot(HaveOccurred())
				})

				By("reading the value from etcd", func() {
					Eventually(func() (string, error) {
						return etcdClient.Get(testKey1)
					}, "10s", "1s").Should(Equal(testValue1))
				})

				By("setting a new persistent value", func() {
					Eventually(func() error {
						return etcdClient.Set(testKey2, testValue2)
					}, "10s", "1s").Should(Succeed())
				})

				By("fixing the deployment", func() {
					Eventually(func() ([]string, error) {
						return lockedDeployments(boshClient)
					}, "12m", "1m").ShouldNot(ContainElement(etcdManifestName))

					Eventually(func() error {
						return boshClient.ScanAndFixAll([]byte(etcdManifest))
					}, "12m", "1m").ShouldNot(HaveOccurred())

					Eventually(func() ([]bosh.VM, error) {
						return helpers.DeploymentVMs(boshClient, etcdManifestName)
					}, "10m", "10s").Should(ConsistOf(helpers.GetVMsFromManifest(etcdManifest)))
				})

				By("reading each value from the resurrected VM", func() {
					value, err := etcdClient.Get(testKey1)
					Expect(err).ToNot(HaveOccurred())
					Expect(value).To(Equal(testValue1))

					value, err = etcdClient.Get(testKey2)
					Expect(err).ToNot(HaveOccurred())
					Expect(value).To(Equal(testValue2))
				})
			})
		})
	}

	Context("without TLS", func() {
		KillVMTest(false)
	})

	Context("with TLS", func() {
		KillVMTest(true)
	})
})
