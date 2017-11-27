package deploy_test

import (
	"fmt"

	"github.com/cloudfoundry-incubator/consul-release/src/acceptance-tests/testing/helpers"
	testconsumerclient "github.com/cloudfoundry-incubator/consul-release/src/acceptance-tests/testing/testconsumer/client"
	"github.com/pivotal-cf-experimental/bosh-test/bosh"
	"github.com/pivotal-cf-experimental/destiny/ops"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Multiple hosts multiple services", func() {
	var (
		manifest     string
		manifestName string

		testConsumerIP string
		tcClient       testconsumerclient.Client
	)

	BeforeEach(func() {
		var err error
		manifest, err = helpers.DeployConsulWithInstanceCount("multiple-host-multiple-services", 3, config.WindowsClients, boshClient)
		Expect(err).NotTo(HaveOccurred())

		manifestName, err = ops.ManifestName(manifest)
		Expect(err).NotTo(HaveOccurred())

		Eventually(func() ([]bosh.VM, error) {
			return helpers.DeploymentVMs(boshClient, manifestName)
		}, "5m", "10s").Should(ConsistOf(helpers.GetVMsFromManifest(manifest)))

		testConsumerIPs, err := helpers.GetVMIPs(boshClient, manifestName, "testconsumer")
		Expect(err).NotTo(HaveOccurred())

		testConsumerIP = testConsumerIPs[0]

		tcClient = testconsumerclient.New(fmt.Sprintf("http://%s:6769", testConsumerIP))
	})

	AfterEach(func() {
		if !CurrentGinkgoTestDescription().Failed {
			err := boshClient.DeleteDeployment(manifestName)
			Expect(err).NotTo(HaveOccurred())
		}
	})

	It("discovers multiples services on multiple hosts", func() {
		By("registering services", func() {
			healthCheck := fmt.Sprintf("curl -f http://%s:6769/health_check", testConsumerIP)

			var err error
			manifest, err = ops.ApplyOp(manifest, ops.Op{
				Type: "replace",
				Path: "/instance_groups/name=consul/properties/consul/agent/services",
				Value: map[string]service{
					"some-service": service{
						Name: "some-service-name",
						Check: serviceCheck{
							Name:     "some-service-check",
							Script:   healthCheck,
							Interval: "10s",
						},
					},
					"some-other-service": service{
						Name: "some-other-service-name",
						Check: serviceCheck{
							Name:     "some-other-service-check",
							Script:   healthCheck,
							Interval: "10s",
						},
					},
				},
			})
			Expect(err).NotTo(HaveOccurred())
		})

		By("deploying", func() {
			_, err := boshClient.Deploy([]byte(manifest))
			Expect(err).NotTo(HaveOccurred())

			Eventually(func() ([]bosh.VM, error) {
				return helpers.DeploymentVMs(boshClient, manifestName)
			}, "5m", "10s").Should(ConsistOf(helpers.GetVMsFromManifest(manifest)))
		})

		By("resolving service addresses", func() {
			consulIPs, err := helpers.GetVMIPs(boshClient, manifestName, "consul")
			Expect(err).NotTo(HaveOccurred())

			deploymentVMs, err := boshClient.DeploymentVMs(manifestName)
			Expect(err).NotTo(HaveOccurred())

			var consulVM0 bosh.VM
			var consulVM1 bosh.VM
			var consulVM2 bosh.VM

			for _, vm := range deploymentVMs {
				if vm.JobName == "consul" {
					switch vm.Index {
					case 0:
						consulVM0 = vm
					case 1:
						consulVM1 = vm
					case 2:
						consulVM2 = vm
					}
				}
			}

			Eventually(func() ([]string, error) {
				return tcClient.DNS("some-service-name.service.cf.internal")
			}, "5m", "10s").Should(ConsistOf(consulIPs))

			Eventually(func() ([]string, error) {
				return tcClient.DNS("consul-0.some-service-name.service.cf.internal")
			}, "5m", "10s").Should(ConsistOf(consulVM0.IPs))

			Eventually(func() ([]string, error) {
				return tcClient.DNS("consul-1.some-service-name.service.cf.internal")
			}, "5m", "10s").Should(ConsistOf(consulVM1.IPs))

			Eventually(func() ([]string, error) {
				return tcClient.DNS("consul-2.some-service-name.service.cf.internal")
			}, "5m", "10s").Should(ConsistOf(consulVM2.IPs))

			Eventually(func() ([]string, error) {
				return tcClient.DNS("some-other-service-name.service.cf.internal")
			}, "5m", "10s").Should(ConsistOf(consulIPs))

			Eventually(func() ([]string, error) {
				return tcClient.DNS("consul-0.some-other-service-name.service.cf.internal")
			}, "5m", "10s").Should(ConsistOf(consulVM0.IPs))

			Eventually(func() ([]string, error) {
				return tcClient.DNS("consul-1.some-other-service-name.service.cf.internal")
			}, "5m", "10s").Should(ConsistOf(consulVM1.IPs))

			Eventually(func() ([]string, error) {
				return tcClient.DNS("consul-2.some-other-service-name.service.cf.internal")
			}, "5m", "10s").Should(ConsistOf(consulVM2.IPs))
		})
	})
})
