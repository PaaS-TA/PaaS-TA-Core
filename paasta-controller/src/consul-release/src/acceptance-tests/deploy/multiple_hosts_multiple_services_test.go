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

var _ = Describe("Multiple hosts multiple services", func() {
	var (
		manifest consul.ManifestV2
		tcClient testconsumerclient.Client
	)

	BeforeEach(func() {
		var err error

		manifest, _, err = helpers.DeployConsulWithInstanceCount("multiple-host-multiple-services", 3, boshClient, config)
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

	It("discovers multiples services on multiple hosts", func() {
		By("registering services", func() {
			healthCheck := fmt.Sprintf("curl -f http://%s:6769/health_check", manifest.InstanceGroups[1].Networks[0].StaticIPs[0])
			manifest.InstanceGroups[0].Properties.Consul.Agent.Services = core.JobPropertiesConsulAgentServices{
				"some-service": core.JobPropertiesConsulAgentService{
					Check: &core.JobPropertiesConsulAgentServiceCheck{
						Name:     "some-service-check",
						Script:   healthCheck,
						Interval: "30s",
					},
				},
				"some-other-service": core.JobPropertiesConsulAgentService{
					Check: &core.JobPropertiesConsulAgentServiceCheck{
						Name:     "some-other-service-check",
						Script:   healthCheck,
						Interval: "30s",
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
				return tcClient.DNS("some-service.service.cf.internal")
			}, "2m", "10s").Should(ConsistOf(manifest.InstanceGroups[0].Networks[0].StaticIPs))

			Eventually(func() ([]string, error) {
				return tcClient.DNS("consul-0.some-service.service.cf.internal")
			}, "2m", "10s").Should(ConsistOf(manifest.InstanceGroups[0].Networks[0].StaticIPs[0]))

			Eventually(func() ([]string, error) {
				return tcClient.DNS("consul-1.some-service.service.cf.internal")
			}, "2m", "10s").Should(ConsistOf(manifest.InstanceGroups[0].Networks[0].StaticIPs[1]))

			Eventually(func() ([]string, error) {
				return tcClient.DNS("consul-2.some-service.service.cf.internal")
			}, "2m", "10s").Should(ConsistOf(manifest.InstanceGroups[0].Networks[0].StaticIPs[2]))

			Eventually(func() ([]string, error) {
				return tcClient.DNS("some-other-service.service.cf.internal")
			}, "2m", "10s").Should(ConsistOf(manifest.InstanceGroups[0].Networks[0].StaticIPs))

			Eventually(func() ([]string, error) {
				return tcClient.DNS("consul-0.some-other-service.service.cf.internal")
			}, "2m", "10s").Should(ConsistOf(manifest.InstanceGroups[0].Networks[0].StaticIPs[0]))

			Eventually(func() ([]string, error) {
				return tcClient.DNS("consul-1.some-other-service.service.cf.internal")
			}, "2m", "10s").Should(ConsistOf(manifest.InstanceGroups[0].Networks[0].StaticIPs[1]))

			Eventually(func() ([]string, error) {
				return tcClient.DNS("consul-2.some-other-service.service.cf.internal")
			}, "2m", "10s").Should(ConsistOf(manifest.InstanceGroups[0].Networks[0].StaticIPs[2]))
		})
	})
})
