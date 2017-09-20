package turbulence_test

import (
	"fmt"
	"time"

	"github.com/cloudfoundry-incubator/consul-release/src/acceptance-tests/testing/helpers"
	"github.com/pivotal-cf-experimental/bosh-test/bosh"
	"github.com/pivotal-cf-experimental/destiny/consul"
	"github.com/pivotal-cf-experimental/destiny/turbulence"

	testconsumerclient "github.com/cloudfoundry-incubator/consul-release/src/acceptance-tests/testing/testconsumer/client"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	turbulenceclient "github.com/pivotal-cf-experimental/bosh-test/turbulence"
)

const (
	DELAY   = 10 * time.Second
	TIMEOUT = 30 * time.Second
)

var _ = Describe("recursor timeout", func() {
	var (
		turbulenceClient   turbulenceclient.Client
		turbulenceManifest turbulence.Manifest
		consulManifest     consul.ManifestV2
		delayIncidentID    string
		tcClient           testconsumerclient.Client
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
			config.TurbulenceHost = turbulenceManifest.InstanceGroups[0].Networks[0].StaticIPs[0]

			consulManifest, _, err = helpers.DeployConsulWithTurbulence("recursor-timeout", 1, boshClient, config)
			Expect(err).NotTo(HaveOccurred())

			Eventually(func() ([]bosh.VM, error) {
				return helpers.DeploymentVMs(boshClient, consulManifest.Name)
			}, "1m", "10s").Should(ConsistOf(helpers.GetVMsFromManifest(consulManifest)))

			tcClient = testconsumerclient.New(fmt.Sprintf("http://%s:6769", consulManifest.InstanceGroups[1].Networks[0].StaticIPs[0]))
		})
	})

	AfterEach(func() {
		By("deleting consul deployment", func() {
			if !CurrentGinkgoTestDescription().Failed {
				Eventually(func() string {
					incidentResp, err := turbulenceClient.Incident(delayIncidentID)
					Expect(err).NotTo(HaveOccurred())

					return incidentResp.ExecutionCompletedAt
				}, TIMEOUT.String(), "10s").ShouldNot(BeEmpty())

				// Turbulence API might say that the incident is finished, but it might not be - sanity check
				Eventually(func() (int64, error) {
					var err error
					dnsStartTime := time.Now()
					_, err = tcClient.DNS("my-fake-server.fake.local")
					if err != nil {
						return 0, err
					}
					dnsElapsedTime := time.Since(dnsStartTime)
					return dnsElapsedTime.Nanoseconds(), nil
				}, TIMEOUT.String(), "100ms").Should(BeNumerically("<", 1*time.Second))

				Eventually(func() ([]string, error) {
					return lockedDeployments()
				}, "10m", "30s").ShouldNot(ContainElement(consulManifest.Name))

				err := boshClient.DeleteDeployment(consulManifest.Name)
				Expect(err).NotTo(HaveOccurred())
			}
		})

		By("deleting turbulence", func() {
			err := boshClient.DeleteDeployment(turbulenceManifest.Name)
			Expect(err).NotTo(HaveOccurred())
		})
	})

	It("resolves long running DNS queries utilizing the consul recursor_timeout property", func() {
		By("making sure my-fake-server resolves", func() {
			address, err := tcClient.DNS("my-fake-server.fake.local")
			Expect(err).NotTo(HaveOccurred())

			// miekg/dns implementation responds with A and AAAA records regardless of the type of record requested
			// therefore we're expected 4 IPs here
			Expect(address).To(Equal([]string{"10.2.3.4", "10.2.3.4", "10.2.3.4", "10.2.3.4"}))
		})

		By("delaying DNS queries with a network delay that is greater than the recursor timeout", func() {
			response, err := turbulenceClient.Delay(consulManifest.Name, "fake-dns-server", []int{0}, DELAY, TIMEOUT)
			Expect(err).NotTo(HaveOccurred())
			delayIncidentID = response.ID
		})

		By("making a DNS query which should timeout", func() {
			Eventually(func() ([]string, error) {
				return tcClient.DNS("my-fake-server.fake.local")
			}, "30s", "100ms").Should(BeEmpty())
		})

		By("waiting for the network delay to end", func() {
			Eventually(func() string {
				incidentResp, err := turbulenceClient.Incident(delayIncidentID)
				Expect(err).NotTo(HaveOccurred())

				return incidentResp.ExecutionCompletedAt
			}, TIMEOUT.String(), "10s").ShouldNot(BeEmpty())
		})

		By("redeploying with 30s recursor timeout", func() {
			consulManifest.Properties.Consul.Agent.DNSConfig.RecursorTimeout = "30s"

			yaml, err := consulManifest.ToYAML()
			Expect(err).NotTo(HaveOccurred())

			_, err = boshClient.Deploy(yaml)
			Expect(err).NotTo(HaveOccurred())

			Eventually(func() ([]bosh.VM, error) {
				return helpers.DeploymentVMs(boshClient, consulManifest.Name)
			}, "1m", "10s").Should(ConsistOf(helpers.GetVMsFromManifest(consulManifest)))
		})

		By("delaying DNS queries with a network delay that is less than the recursor timeout", func() {
			response, err := turbulenceClient.Delay(consulManifest.Name, "fake-dns-server", []int{0}, DELAY, TIMEOUT)
			Expect(err).NotTo(HaveOccurred())
			delayIncidentID = response.ID
		})

		By("successfully making a DNS query", func() {
			var addresses []string

			Eventually(func() (int64, error) {
				var err error
				dnsStartTime := time.Now()
				addresses, err = tcClient.DNS("my-fake-server.fake.local")
				if err != nil {
					return 0, err
				}
				dnsElapsedTime := time.Since(dnsStartTime)
				return dnsElapsedTime.Nanoseconds(), nil
			}, "30s", "100ms").Should(BeNumerically(">", DELAY))

			// miekg/dns implementation responds with A and AAAA records regardless of the type of record requested
			// therefore we're expected 4 IPs here
			Expect(addresses).To(Equal([]string{"10.2.3.4", "10.2.3.4", "10.2.3.4", "10.2.3.4"}))
		})
	})
})
