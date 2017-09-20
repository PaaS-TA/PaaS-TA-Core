package deploy_test

import (
	"fmt"

	"github.com/cloudfoundry-incubator/consul-release/src/acceptance-tests/testing/helpers"
	testconsumerclient "github.com/cloudfoundry-incubator/consul-release/src/acceptance-tests/testing/testconsumer/client"
	"github.com/pivotal-cf-experimental/bosh-test/bosh"
	"github.com/pivotal-cf-experimental/destiny/consul"
	"github.com/pivotal-cf-experimental/destiny/core"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Single host multiple services", func() {
	var (
		manifest consul.ManifestV2
		tcClient testconsumerclient.Client
	)

	BeforeEach(func() {
		var err error

		manifest, _, err = helpers.DeployConsulWithInstanceCount("single-host-multiple-services", 1, boshClient, config)
		Expect(err).NotTo(HaveOccurred())

		Eventually(func() ([]bosh.VM, error) {
			return helpers.DeploymentVMs(boshClient, manifest.Name)
		}, "1m", "10s").Should(ConsistOf(helpers.GetVMsFromManifest(manifest)))

		tcClient = testconsumerclient.New(fmt.Sprintf("http://%s:6769", manifest.InstanceGroups[1].Networks[0].StaticIPs[0]))
	})

	AfterEach(func() {
		if !CurrentGinkgoTestDescription().Failed {
			err := boshClient.DeleteDeployment(manifest.Name)
			Expect(err).NotTo(HaveOccurred())
		}
	})

	It("discovers multiples services on a single host", func() {
		By("registering services", func() {
			healthCheck := fmt.Sprintf("curl -f http://%s:6769/health_check", manifest.InstanceGroups[1].Networks[0].StaticIPs[0])
			manifest.InstanceGroups[1].Properties = &core.JobProperties{
				Consul: &core.JobPropertiesConsul{
					Agent: core.JobPropertiesConsulAgent{
						Mode: "client",
						Services: core.JobPropertiesConsulAgentServices{
							"consul-test-consumer": core.JobPropertiesConsulAgentService{},
							"some-service": core.JobPropertiesConsulAgentService{
								Check: &core.JobPropertiesConsulAgentServiceCheck{
									Name:     "some-service-check",
									Script:   healthCheck,
									Interval: "1m",
								},
							},
						},
					},
				},
			}
		})

		By("deploying", func() {
			yaml, err := manifest.ToYAML()
			Expect(err).NotTo(HaveOccurred())

			yaml, err = boshClient.ResolveManifestVersions(yaml)
			Expect(err).NotTo(HaveOccurred())

			_, err = boshClient.Deploy(yaml)
			Expect(err).NotTo(HaveOccurred())

			Eventually(func() ([]bosh.VM, error) {
				return helpers.DeploymentVMs(boshClient, manifest.Name)
			}, "1m", "10s").Should(ConsistOf(helpers.GetVMsFromManifest(manifest)))
		})

		By("resolving service addresses", func() {
			Eventually(func() ([]string, error) {
				return tcClient.DNS("consul-test-consumer.service.cf.internal")
			}, "1m", "10s").Should(ConsistOf(manifest.InstanceGroups[1].Networks[0].StaticIPs))

			Eventually(func() ([]string, error) {
				return tcClient.DNS("some-service.service.cf.internal")
			}, "1m", "10s").Should(ConsistOf(manifest.InstanceGroups[1].Networks[0].StaticIPs))
		})
	})
})
