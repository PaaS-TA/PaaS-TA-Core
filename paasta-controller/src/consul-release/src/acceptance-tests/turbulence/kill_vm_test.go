package turbulence_test

import (
	"fmt"
	"math/rand"
	"time"

	"github.com/cloudfoundry-incubator/consul-release/src/acceptance-tests/testing/consulclient"
	"github.com/cloudfoundry-incubator/consul-release/src/acceptance-tests/testing/helpers"
	"github.com/pivotal-cf-experimental/bosh-test/bosh"
	"github.com/pivotal-cf-experimental/destiny/ops"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	turbulenceclient "github.com/pivotal-cf-experimental/bosh-test/turbulence"
)

var _ = Describe("KillVm", func() {
	var (
		turbulenceClient       turbulenceclient.Client
		turbulenceManifest     string
		turbulenceManifestName string

		consulManifest     string
		consulManifestName string

		kv        consulclient.HTTPKV
		spammer   *helpers.Spammer
		testKey   string
		testValue string
	)

	BeforeEach(func() {
		By("deploying turbulence", func() {
			var err error
			turbulenceManifest, err = helpers.DeployTurbulence("kill-vm", boshClient)
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

		By("deploying consul", func() {
			guid, err := helpers.NewGUID()
			Expect(err).NotTo(HaveOccurred())

			testKey = "consul-key-" + guid
			testValue = "consul-value-" + guid

			consulManifest, err = helpers.DeployConsulWithInstanceCount("kill-vm", 3, config.WindowsClients, boshClient)
			Expect(err).NotTo(HaveOccurred())

			consulManifestName, err = ops.ManifestName(consulManifest)
			Expect(err).NotTo(HaveOccurred())

			Eventually(func() ([]bosh.VM, error) {
				return helpers.DeploymentVMs(boshClient, consulManifestName)
			}, "1m", "10s").Should(ConsistOf(helpers.GetVMsFromManifest(consulManifest)))

			testConsumerIPs, err := helpers.GetVMIPs(boshClient, consulManifestName, "testconsumer")
			Expect(err).NotTo(HaveOccurred())

			kv = consulclient.NewHTTPKV(fmt.Sprintf("http://%s:6769", testConsumerIPs[0]))

			spammer = helpers.NewSpammer(kv, 1*time.Second, "test-consumer-0")
		})
	})

	AfterEach(func() {
		By("fixing the deployment", func() {
			Eventually(func() ([]string, error) {
				return lockedDeployments()
			}, "5m", "30s").ShouldNot(ContainElement(consulManifestName))

			err := boshClient.ScanAndFixAll([]byte(consulManifest))
			Expect(err).NotTo(HaveOccurred())

			Eventually(func() ([]bosh.VM, error) {
				return helpers.DeploymentVMs(boshClient, consulManifestName)
			}, "10m", "10s").Should(ConsistOf(helpers.GetVMsFromManifest(consulManifest)))
		})

		By("deleting the deployment", func() {
			if !CurrentGinkgoTestDescription().Failed {
				err := boshClient.DeleteDeployment(consulManifestName)
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

	Context("when a consul node is killed", func() {
		It("is still able to function on healthy vms", func() {
			By("setting a persistent value", func() {
				err := kv.Set(testKey, testValue)
				Expect(err).NotTo(HaveOccurred())
			})

			By("killing indices", func() {
				spammer.Spam()

				vmIDs, err := helpers.GetVMIDByIndices(boshClient, consulManifestName, "consul", []int{rand.Intn(3)})
				Expect(err).NotTo(HaveOccurred())

				err = turbulenceClient.KillIDs(vmIDs)
				Expect(err).ToNot(HaveOccurred())

				Eventually(func() error {
					return boshClient.ScanAndFixAll([]byte(consulManifest))
				}, "5m", "1m").ShouldNot(HaveOccurred())

				Eventually(func() ([]bosh.VM, error) {
					return helpers.DeploymentVMs(boshClient, consulManifestName)
				}, "5m", "10s").Should(ConsistOf(helpers.GetVMsFromManifest(consulManifest)))

				spammer.Stop()
			})

			By("reading the value from consul", func() {
				value, err := kv.Get(testKey)
				Expect(err).NotTo(HaveOccurred())
				Expect(value).To(Equal(testValue))
			})

			By("checking the spammer for errors", func() {
				err := spammer.Check()
				Expect(err).NotTo(HaveOccurred())
			})
		})
	})
})
