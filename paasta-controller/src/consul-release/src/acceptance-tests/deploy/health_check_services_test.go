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

type service struct {
	Name  string       `yaml:"name,omitempty"`
	Check serviceCheck `yaml:"check,omitempty"`
	Tags  []string     `yaml:"tags,omitempty"`
}

type serviceCheck struct {
	Name     string
	Script   string
	Interval string
}

var _ = Describe("Health Check", func() {
	Context("with an operator defined check script", func() {
		var (
			manifest       string
			manifestName   string
			testConsumerIP string

			tcClient testconsumerclient.Client
		)

		BeforeEach(func() {
			var err error
			manifest, err = helpers.DeployConsulWithInstanceCount("health-check-custom-script", 3, config.WindowsClients, boshClient)
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

		It("deregisters a service if the health check fails", func() {
			By("registering a service", func() {
				var err error
				manifest, err = ops.ApplyOp(manifest, ops.Op{
					Type: "replace",
					Path: "/instance_groups/name=consul/properties/consul/agent/services",
					Value: map[string]service{
						"some-service": service{
							Name: "some-service-name",
							Check: serviceCheck{
								Name:     "some-service-check",
								Script:   fmt.Sprintf("curl -f %s", fmt.Sprintf("http://%s:6769/health_check", testConsumerIP)),
								Interval: "10s",
							},
							Tags: []string{"some-service-tag"},
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

			By("resolving the service address", func() {
				consulIPs, err := helpers.GetVMIPs(boshClient, manifestName, "consul")
				Expect(err).NotTo(HaveOccurred())

				Eventually(func() ([]string, error) {
					return tcClient.DNS("some-service-name.service.cf.internal")
				}, "5m", "10s").Should(ConsistOf(consulIPs))
			})

			By("causing the health check to fail", func() {
				err := tcClient.SetHealthCheck(false)
				Expect(err).NotTo(HaveOccurred())
			})

			By("the service should be deregistered", func() {
				Eventually(func() ([]string, error) {
					return tcClient.DNS("some-service-name.service.cf.internal")
				}, "5m", "10s").Should(BeEmpty())
			})

			By("causing the health check to succeed", func() {
				err := tcClient.SetHealthCheck(true)
				Expect(err).NotTo(HaveOccurred())
			})

			By("the service should be alive", func() {
				consulIPs, err := helpers.GetVMIPs(boshClient, manifestName, "consul")
				Expect(err).NotTo(HaveOccurred())

				Eventually(func() ([]string, error) {
					return tcClient.DNS("some-service-name.service.cf.internal")
				}, "5m", "10s").Should(ConsistOf(consulIPs))
			})
		})
	})

	Context("with the default check script", func() {
		var (
			manifest       string
			manifestName   string
			testConsumerIP string
			serviceName    string

			tcClient testconsumerclient.Client
		)

		BeforeEach(func() {
			var err error
			manifest, err = helpers.DeployConsulWithInstanceCount("health-check-default-script", 3, config.WindowsClients, boshClient)
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
			serviceName = "consul-test-consumer"
			if config.WindowsClients {
				serviceName = "consul-test-consumer-windows"
			}
		})

		AfterEach(func() {
			if !CurrentGinkgoTestDescription().Failed {
				err := boshClient.DeleteDeployment(manifestName)
				Expect(err).NotTo(HaveOccurred())
			}
		})

		It("deregisters a service if the health check fails", func() {
			By("registering a service", func() {
				var err error
				var emptyObject struct{}
				manifest, err = ops.ApplyOps(manifest, []ops.Op{
					{
						Type:  "replace",
						Path:  "/instance_groups/name=testconsumer/instances",
						Value: 3,
					},
					{
						Type:  "replace",
						Path:  fmt.Sprintf("/instance_groups/name=testconsumer/properties?/consul/agent/services/%s", serviceName),
						Value: emptyObject,
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

			By("resolving the service address", func() {
				testConsumerIPs, err := helpers.GetVMIPs(boshClient, manifestName, "testconsumer")
				Expect(err).NotTo(HaveOccurred())

				Eventually(func() ([]string, error) {
					return tcClient.DNS(fmt.Sprintf("%s.service.cf.internal", serviceName))
				}, "5m", "10s").Should(ConsistOf(testConsumerIPs))
			})

			By("causing the health check to fail", func() {
				err := tcClient.SetHealthCheck(false)
				Expect(err).NotTo(HaveOccurred())
			})

			By("the service should be deregistered", func() {
				Eventually(func() ([]string, error) {
					return tcClient.DNS(fmt.Sprintf("%s.service.cf.internal", serviceName))
				}, "5m", "10s").Should(HaveLen(2))
			})

			By("causing the health check to succeed", func() {
				err := tcClient.SetHealthCheck(true)
				Expect(err).NotTo(HaveOccurred())
			})

			By("the service should be alive", func() {
				testConsumerIPs, err := helpers.GetVMIPs(boshClient, manifestName, "testconsumer")
				Expect(err).NotTo(HaveOccurred())

				Eventually(func() ([]string, error) {
					return tcClient.DNS(fmt.Sprintf("%s.service.cf.internal", serviceName))
				}, "5m", "10s").Should(ConsistOf(testConsumerIPs))
			})
		})
	})
})
