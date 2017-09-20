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
	"github.com/pivotal-cf-experimental/destiny/consul"
	"github.com/pivotal-cf-experimental/destiny/turbulence"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	turbulenceclient "github.com/pivotal-cf-experimental/bosh-test/turbulence"
)

var _ = PDescribe("quorum loss", func() {
	var (
		turbulenceClient   turbulenceclient.Client
		turbulenceManifest turbulence.Manifest
		consulManifest     consul.ManifestV2
		kv                 consulclient.HTTPKV
	)

	BeforeEach(func() {
		By("deploying turbulence", func() {
			var err error
			turbulenceManifest, err = helpers.DeployTurbulence(boshClient, config)
			Expect(err).NotTo(HaveOccurred())

			Eventually(func() ([]bosh.VM, error) {
				return helpers.DeploymentVMs(boshClient, turbulenceManifest.Name)
			}, "1m", "10s").Should(ConsistOf(helpers.GetTurbulenceVMsFromManifest(turbulenceManifest)))

			turbulenceClient = helpers.NewTurbulenceClient(turbulenceManifest)
		})

		By("deploying consul", func() {
			var err error
			consulManifest, kv, err = helpers.DeployConsulWithInstanceCount("quorum-loss", 5, boshClient, config)
			Expect(err).NotTo(HaveOccurred())

			Eventually(func() ([]bosh.VM, error) {
				return helpers.DeploymentVMs(boshClient, consulManifest.Name)
			}, "1m", "10s").Should(ConsistOf(helpers.GetVMsFromManifest(consulManifest)))
		})
	})

	AfterEach(func() {
		By("deleting the deployment", func() {
			if !CurrentGinkgoTestDescription().Failed {
				for i := 0; i < 5; i++ {
					err := boshClient.SetVMResurrection(consulManifest.Name, "consul", i, true)
					Expect(err).NotTo(HaveOccurred())
				}

				yaml, err := consulManifest.ToYAML()
				Expect(err).NotTo(HaveOccurred())

				Eventually(func() ([]string, error) {
					return lockedDeployments()
				}, "10m", "30s").ShouldNot(ContainElement(consulManifest.Name))

				err = boshClient.ScanAndFixAll(yaml)
				Expect(err).NotTo(HaveOccurred())

				Eventually(func() ([]bosh.VM, error) {
					return helpers.DeploymentVMs(boshClient, consulManifest.Name)
				}, "10m", "10s").Should(ConsistOf(helpers.GetVMsFromManifest(consulManifest)))

				Eventually(func() ([]string, error) {
					return lockedDeployments()
				}, "1m", "10s").ShouldNot(ContainElement(consulManifest.Name))

				err = boshClient.DeleteDeployment(consulManifest.Name)
				Expect(err).NotTo(HaveOccurred())
			}
		})

		By("deleting turbulence", func() {
			err := boshClient.DeleteDeployment(turbulenceManifest.Name)
			Expect(err).NotTo(HaveOccurred())
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
					err := boshClient.SetVMResurrection(consulManifest.Name, "consul", i, false)
					Expect(err).NotTo(HaveOccurred())
				}

				leader, err := jobIndexOfLeader(kv, boshClient, consulManifest.Name)
				Expect(err).ToNot(HaveOccurred())

				rand.Seed(time.Now().Unix())
				startingIndex := rand.Intn(3)
				instances := []int{startingIndex, startingIndex + 1, startingIndex + 2}

				if leader < startingIndex || leader > startingIndex+2 {
					instances[0] = leader
				}

				jobIndexToResurrect := startingIndex + 1

				err = turbulenceClient.KillIndices(consulManifest.Name, "consul", instances)
				Expect(err).NotTo(HaveOccurred())

				err = boshClient.SetVMResurrection(consulManifest.Name, "consul", jobIndexToResurrect, true)
				Expect(err).NotTo(HaveOccurred())

				Eventually(func() error {
					return boshClient.ScanAndFix(consulManifest.Name, "consul", []int{jobIndexToResurrect})
				}, "5m", "1m").ShouldNot(HaveOccurred())

				Eventually(func() ([]bosh.VM, error) {
					return helpers.DeploymentVMs(boshClient, consulManifest.Name)
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
