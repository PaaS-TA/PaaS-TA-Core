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

var _ = Describe("Single host multiple services", func() {
	var (
		manifest     string
		manifestName string

		testConsumerIP string
		tcClient       testconsumerclient.Client
	)

	BeforeEach(func() {
		var err error
		manifest, err = helpers.DeployConsulWithInstanceCount("single-host-multiple-services", 1, config.WindowsClients, boshClient)
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

	It("discovers multiples services on a single host", func() {
		By("registering services", func() {
			healthCheck := fmt.Sprintf("curl -f http://%s:6769/health_check", testConsumerIP)

			var err error
			manifest, err = ops.ApplyOp(manifest, ops.Op{
				Type: "replace",
				Path: "/instance_groups/name=testconsumer/properties?/consul/agent/services",
				Value: map[string]service{
					"some-service": service{
						Name: "some-service-name",
						Check: serviceCheck{
							Name:     "some-service-check",
							Script:   healthCheck,
							Interval: "1m",
						},
					},
					"consul-test-consumer": service{},
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
			Eventually(func() ([]string, error) {
				return tcClient.DNS("consul-test-consumer.service.cf.internal")
			}, "5m", "10s").Should(ConsistOf(testConsumerIP))

			Eventually(func() ([]string, error) {
				return tcClient.DNS("some-service-name.service.cf.internal")
			}, "5m", "10s").Should(ConsistOf(testConsumerIP))
		})
	})
})
