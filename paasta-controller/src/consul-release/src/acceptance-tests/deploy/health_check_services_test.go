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

var _ = Describe("Health Check", func() {
	var (
		manifest consul.ManifestV2
		tcClient testconsumerclient.Client
	)

	BeforeEach(func() {
		var err error

		manifest, _, err = helpers.DeployConsulWithInstanceCount("health-check", 1, boshClient, config)
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

	Context("with an operator defined check script", func() {
		It("deregisters a service if the health check fails", func() {
			By("registering a service", func() {
				manifest, err := manifest.SetInstanceCount("test_consumer", 3)
				Expect(err).NotTo(HaveOccurred())
				manifest.InstanceGroups[0].Properties.Consul.Agent.Services = core.JobPropertiesConsulAgentServices{
					"some-service": core.JobPropertiesConsulAgentService{
						Name: "some-service-name",
						Check: &core.JobPropertiesConsulAgentServiceCheck{
							Name:     "some-service-check",
							Script:   fmt.Sprintf("curl -f %s", fmt.Sprintf("http://%s:6769/health_check", manifest.InstanceGroups[1].Networks[0].StaticIPs[0])),
							Interval: "10s",
						},
						Tags: []string{"some-service-tag"},
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

			By("resolving the service address", func() {
				Eventually(func() ([]string, error) {
					return tcClient.DNS("some-service-name.service.cf.internal")
				}, "1m", "10s").Should(ConsistOf(manifest.InstanceGroups[0].Networks[0].StaticIPs))
			})

			By("causing the health check to fail", func() {
				err := tcClient.SetHealthCheck(false)
				Expect(err).NotTo(HaveOccurred())
			})

			By("the service should be deregistered", func() {
				Eventually(func() ([]string, error) {
					return tcClient.DNS("some-service-name.service.cf.internal")
				}, "1m", "10s").Should(BeEmpty())
			})

			By("causing the health check to succeed", func() {
				err := tcClient.SetHealthCheck(true)
				Expect(err).NotTo(HaveOccurred())
			})

			By("the service should be alive", func() {
				Eventually(func() ([]string, error) {
					return tcClient.DNS("some-service-name.service.cf.internal")
				}, "1m", "10s").Should(ConsistOf(manifest.InstanceGroups[0].Networks[0].StaticIPs))
			})
		})
	})

	Context("with the default check script", func() {
		It("deregisters a service if the health check fails", func() {
			By("registering a service", func() {
				manifest, err := manifest.SetInstanceCount("test_consumer", 3)
				Expect(err).NotTo(HaveOccurred())
				manifest.InstanceGroups[1].Properties = &core.JobProperties{
					Consul: &core.JobPropertiesConsul{
						Agent: core.JobPropertiesConsulAgent{
							Mode: "client",
							Services: core.JobPropertiesConsulAgentServices{
								"consul-test-consumer": core.JobPropertiesConsulAgentService{},
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

			By("resolving the service address", func() {
				Eventually(func() ([]string, error) {
					return tcClient.DNS("consul-test-consumer.service.cf.internal")
				}, "1m", "10s").Should(ConsistOf(manifest.InstanceGroups[1].Networks[0].StaticIPs))
			})

			By("causing the health check to fail", func() {
				err := tcClient.SetHealthCheck(false)
				Expect(err).NotTo(HaveOccurred())
			})

			By("the service should be deregistered", func() {
				Eventually(func() ([]string, error) {
					return tcClient.DNS("consul-test-consumer.service.cf.internal")
				}, "1m", "10s").Should(HaveLen(2))
			})

			By("causing the health check to succeed", func() {
				err := tcClient.SetHealthCheck(true)
				Expect(err).NotTo(HaveOccurred())
			})

			By("the service should be alive", func() {
				Eventually(func() ([]string, error) {
					return tcClient.DNS("consul-test-consumer.service.cf.internal")
				}, "1m", "10s").Should(ConsistOf(manifest.InstanceGroups[1].Networks[0].StaticIPs))
			})
		})
	})
})
