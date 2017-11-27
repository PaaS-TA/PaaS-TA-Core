package turbulence_test

import (
	"encoding/json"
	"errors"
	"fmt"
	"math/rand"
	"net/http"
	"strings"
	"time"

	"github.com/cloudfoundry-incubator/consul-release/src/acceptance-tests/testing/consulclient"
	"github.com/cloudfoundry-incubator/consul-release/src/acceptance-tests/testing/helpers"
	"github.com/pivotal-cf-experimental/bosh-test/bosh"
	"github.com/pivotal-cf-experimental/destiny/ops"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	turbulenceclient "github.com/pivotal-cf-experimental/bosh-test/turbulence"
)

var _ = Describe("quorum loss", func() {
	var (
		turbulenceClient       turbulenceclient.Client
		turbulenceManifest     string
		turbulenceManifestName string

		consulManifest     string
		consulManifestName string

		kv consulclient.HTTPKV
	)

	BeforeEach(func() {
		By("deploying turbulence", func() {
			var err error
			turbulenceManifest, err = helpers.DeployTurbulence("quorum-loss", boshClient)
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
			var err error
			consulManifest, err = helpers.DeployConsulWithInstanceCount("quorum-loss", 5, config.WindowsClients, boshClient)
			Expect(err).NotTo(HaveOccurred())

			consulManifestName, err = ops.ManifestName(consulManifest)
			Expect(err).NotTo(HaveOccurred())

			Eventually(func() ([]bosh.VM, error) {
				return helpers.DeploymentVMs(boshClient, consulManifestName)
			}, "1m", "10s").Should(ConsistOf(helpers.GetVMsFromManifest(consulManifest)))

			testConsumerIPs, err := helpers.GetVMIPs(boshClient, consulManifestName, "testconsumer")
			Expect(err).NotTo(HaveOccurred())

			kv = consulclient.NewHTTPKV(fmt.Sprintf("http://%s:6769", testConsumerIPs[0]))
		})
	})

	AfterEach(func() {
		By("deleting the deployment", func() {
			if !CurrentGinkgoTestDescription().Failed {
				for i := 0; i < 5; i++ {
					err := boshClient.SetVMResurrection(consulManifestName, "consul", i, true)
					Expect(err).NotTo(HaveOccurred())
				}

				Eventually(func() error {
					return boshClient.ScanAndFixAll([]byte(consulManifest))
				}, "10m", "30s").ShouldNot(HaveOccurred())

				Eventually(func() ([]bosh.VM, error) {
					return helpers.DeploymentVMs(boshClient, consulManifestName)
				}, "10m", "10s").Should(ConsistOf(helpers.GetVMsFromManifest(consulManifest)))

				Eventually(func() ([]string, error) {
					return lockedDeployments()
				}, "5m", "10s").ShouldNot(ContainElement(consulManifestName))

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
			By("setting and getting a value", func() {
				guid, err := helpers.NewGUID()
				Expect(err).NotTo(HaveOccurred())

				testKey := "consul-key-" + guid
				testValue := "consul-value-" + guid

				err = kv.Set(testKey, testValue)
				Expect(err).NotTo(HaveOccurred())

				value, err := kv.Get(testKey)
				Expect(err).NotTo(HaveOccurred())
				Expect(value).To(Equal(testValue))
			})

			By("killing indices", func() {
				for i := 0; i < 5; i++ {
					err := boshClient.SetVMResurrection(consulManifestName, "consul", i, false)
					Expect(err).NotTo(HaveOccurred())
				}

				leader, err := jobIndexOfLeader(kv, boshClient, consulManifestName)
				Expect(err).ToNot(HaveOccurred())

				rand.Seed(time.Now().Unix())
				startingIndex := rand.Intn(3)
				instances := []int{startingIndex, startingIndex + 1, startingIndex + 2}

				if leader < startingIndex || leader > startingIndex+2 {
					instances[0] = leader
				}

				jobIndexToResurrect := startingIndex + 1

				vmIDs, err := helpers.GetVMIDByIndices(boshClient, consulManifestName, "consul", instances)
				Expect(err).NotTo(HaveOccurred())

				err = turbulenceClient.KillIDs(vmIDs)
				Expect(err).NotTo(HaveOccurred())

				err = boshClient.SetVMResurrection(consulManifestName, "consul", jobIndexToResurrect, true)
				Expect(err).NotTo(HaveOccurred())

				Eventually(func() error {
					return boshClient.ScanAndFix(consulManifestName, "consul", []int{jobIndexToResurrect})
				}, "10m", "1m").ShouldNot(HaveOccurred())

				Eventually(func() ([]bosh.VM, error) {
					return helpers.DeploymentVMs(boshClient, consulManifestName)
				}, "5m", "1m").Should(ContainElement(bosh.VM{JobName: "consul", Index: jobIndexToResurrect, State: "running"}))
			})

			By("setting and getting a new value", func() {
				guid, err := helpers.NewGUID()
				Expect(err).NotTo(HaveOccurred())

				testKey := "consul-key-" + guid
				testValue := "consul-value-" + guid

				err = kv.Set(testKey, testValue)
				Expect(err).NotTo(HaveOccurred())

				value, err := kv.Get(testKey)
				Expect(err).NotTo(HaveOccurred())
				Expect(value).To(Equal(testValue))
			})
		})
	})
})

func jobIndexOfLeader(kv consulclient.HTTPKV, client bosh.Client, deploymentName string) (int, error) {
	resp, err := http.Get(fmt.Sprintf("%s/v1/status/leader", kv.Address()))
	if err != nil {
		return -1, err
	}

	var leader string
	if err := json.NewDecoder(resp.Body).Decode(&leader); err != nil {
		return -1, err
	}

	vms, err := client.DeploymentVMs(deploymentName)
	if err != nil {
		return -1, err
	}

	for _, vm := range vms {
		if len(vm.IPs) > 0 {
			if vm.IPs[0] == strings.Split(leader, ":")[0] {
				return vm.Index, nil
			}
		}
	}

	return -1, errors.New("could not determine leader")
}
