package turbulence_test

import (
	"fmt"
	"time"

	"github.com/cloudfoundry-incubator/consul-release/src/acceptance-tests/testing/helpers"
	"github.com/pivotal-cf-experimental/bosh-test/bosh"
	"github.com/pivotal-cf-experimental/destiny/ops"

	testconsumerclient "github.com/cloudfoundry-incubator/consul-release/src/acceptance-tests/testing/testconsumer/client"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	turbulenceclient "github.com/pivotal-cf-experimental/bosh-test/turbulence"
)

const (
	DELAY   = 10 * time.Second
	TIMEOUT = 30 * time.Second
)

type dnsCallResponse struct {
	addresses          []string
	isGreaterThanDelay bool
}

type release struct {
	Name    string
	Version string
}

type instanceGroup struct {
	Name               string
	Instances          int
	AZs                []string
	VMType             string `yaml:"vm_type"`
	Stemcell           string
	PersistentDiskType string `yaml:"persistent_disk_type"`
	Networks           []instanceGroupNetwork
	Jobs               []instanceGroupJob
	Properties         instanceGroupProperties `yaml:",omitempty"`
}

type instanceGroupNetwork struct {
	Name string
}

type instanceGroupJob struct {
	Name     string
	Release  string
	Provides instanceGroupJobLink `yaml:",omitempty"`
	Consumes instanceGroupJobLink `yaml:",omitempty"`
}

type instanceGroupProperties struct {
	FakeDNSServer instanceGroupPropertiesFakeDNSServer `yaml:"fake_dns_server,omitempty"`
}

type instanceGroupPropertiesFakeDNSServer struct {
	HostToAdd instanceGroupPropertiesFakeDNSServerHostToAdd `yaml:"host_to_add,omitempty"`
}

type instanceGroupPropertiesFakeDNSServerHostToAdd struct {
	Name    string `yaml:",omitempty"`
	Address string `yaml:",omitempty"`
}

type instanceGroupJobLink struct {
	DNS map[string]string `yaml:"dns,omitempty"`
	API map[string]string `yaml:"api,omitempty"`
}

var _ = Describe("recursor timeout", func() {
	var (
		turbulenceClient       turbulenceclient.Client
		turbulenceManifest     string
		turbulenceManifestName string
		turbulencePassword     interface{}
		turbulenceIPs          []string

		consulManifest     string
		consulManifestName string

		delayIncidentID string
		tcClient        testconsumerclient.Client
	)

	BeforeEach(func() {
		By("deploying turbulence", func() {
			var err error
			turbulenceManifest, err = helpers.DeployTurbulence("recursor-timeout", boshClient)
			Expect(err).NotTo(HaveOccurred())

			turbulenceManifestName, err = ops.ManifestName(turbulenceManifest)
			Expect(err).NotTo(HaveOccurred())

			Eventually(func() ([]bosh.VM, error) {
				return helpers.DeploymentVMs(boshClient, turbulenceManifestName)
			}, "1m", "10s").Should(ConsistOf(helpers.GetVMsFromManifest(turbulenceManifest)))

			turbulencePassword, err = ops.FindOp(turbulenceManifest, "/instance_groups/name=api/properties/password")
			Expect(err).NotTo(HaveOccurred())

			turbulenceIPs, err = helpers.GetVMIPs(boshClient, turbulenceManifestName, "api")
			Expect(err).NotTo(HaveOccurred())

			turbulenceClient = turbulenceclient.NewClient(fmt.Sprintf("https://turbulence:%s@%s:8080", turbulencePassword.(string), turbulenceIPs[0]), 5*time.Minute, 2*time.Second)
		})

		By("deploying consul", func() {
			var err error
			consulManifest, err = helpers.NewConsulManifestWithInstanceCount("recursor-timeout", 1, config.WindowsClients, boshClient)
			Expect(err).NotTo(HaveOccurred())

			consulManifestName, err = ops.ManifestName(consulManifest)
			Expect(err).NotTo(HaveOccurred())

			testConsumerJobName := "consul-test-consumer"

			if config.WindowsClients {
				testConsumerJobName = "consul-test-consumer-windows"
			}

			consulManifest, err = ops.ApplyOps(consulManifest, []ops.Op{
				{
					Type: "replace",
					Path: "/releases/name=turbulence?",
					Value: release{
						Name:    "turbulence",
						Version: "latest",
					},
				},
				{
					Type: "replace",
					Path: "/instance_groups/name=fake-dns-server?",
					Value: instanceGroup{
						Name:               "fake-dns-server",
						Instances:          1,
						AZs:                []string{"z1"},
						VMType:             "default",
						Stemcell:           "default",
						PersistentDiskType: "1GB",
						Networks: []instanceGroupNetwork{
							{
								Name: "private",
							},
						},
						Jobs: []instanceGroupJob{
							{
								Name:    "fake-dns-server",
								Release: "consul",
								Provides: instanceGroupJobLink{
									DNS: map[string]string{
										"as": "fake-dns-server",
									},
								},
							},
							{
								Name:    "turbulence_agent",
								Release: "turbulence",
								Consumes: instanceGroupJobLink{
									API: map[string]string{
										"from":       "api",
										"deployment": turbulenceManifestName,
									},
								},
							},
						},
						Properties: instanceGroupProperties{
							FakeDNSServer: instanceGroupPropertiesFakeDNSServer{
								HostToAdd: instanceGroupPropertiesFakeDNSServerHostToAdd{
									Name:    "turbulence.local",
									Address: turbulenceIPs[0],
								},
							},
						},
					},
				},
				{
					Type: "replace",
					Path: fmt.Sprintf("/instance_groups/name=testconsumer/jobs/name=%s/consumes?", testConsumerJobName),
					Value: instanceGroupJobLink{
						DNS: map[string]string{
							"from": "fake-dns-server",
						},
					},
				},
			})
			Expect(err).NotTo(HaveOccurred())

			_, err = boshClient.Deploy([]byte(consulManifest))
			Expect(err).NotTo(HaveOccurred())

			Eventually(func() ([]bosh.VM, error) {
				return helpers.DeploymentVMs(boshClient, consulManifestName)
			}, "1m", "10s").Should(ConsistOf(helpers.GetVMsFromManifest(consulManifest)))

			testConsumerIPs, err := helpers.GetVMIPs(boshClient, consulManifestName, "testconsumer")
			Expect(err).NotTo(HaveOccurred())

			tcClient = testconsumerclient.New(fmt.Sprintf("http://%s:6769", testConsumerIPs[0]))
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
				}, "10m", "30s").ShouldNot(ContainElement(consulManifestName))

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

	It("resolves long running DNS queries utilizing the consul recursor_timeout property", func() {
		By("making sure my-fake-server resolves", func() {
			addresses, err := tcClient.DNS("my-fake-server.fake.local")
			Expect(err).NotTo(HaveOccurred())

			// miekg/dns implementation responds with A and AAAA records regardless of the type of record requested
			// therefore we're expected 4 IPs here
			if config.WindowsClients {
				Expect(addresses).To(Equal([]string{"10.2.3.4", "10.2.3.4"}))
			} else {
				Expect(addresses).To(Equal([]string{"10.2.3.4", "10.2.3.4", "10.2.3.4", "10.2.3.4"}))
			}
		})

		By("delaying DNS queries with a network delay that is greater than the recursor timeout", func() {
			vmIDs, err := helpers.GetVMIDByIndices(boshClient, consulManifestName, "fake-dns-server", []int{0})
			Expect(err).NotTo(HaveOccurred())

			response, err := turbulenceClient.Delay(vmIDs, DELAY, TIMEOUT)
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
			var err error
			consulManifest, err = ops.ApplyOp(consulManifest, ops.Op{
				Type:  "replace",
				Path:  "/instance_groups/name=consul/properties/consul/agent/dns_config?/recursor_timeout",
				Value: "30s",
			})
			Expect(err).NotTo(HaveOccurred())

			_, err = boshClient.Deploy([]byte(consulManifest))
			Expect(err).NotTo(HaveOccurred())

			Eventually(func() ([]bosh.VM, error) {
				return helpers.DeploymentVMs(boshClient, consulManifestName)
			}, "1m", "10s").Should(ConsistOf(helpers.GetVMsFromManifest(consulManifest)))
		})

		By("delaying DNS queries with a network delay that is less than the recursor timeout", func() {
			vmIDs, err := helpers.GetVMIDByIndices(boshClient, consulManifestName, "fake-dns-server", []int{0})
			Expect(err).NotTo(HaveOccurred())

			response, err := turbulenceClient.Delay(vmIDs, DELAY, TIMEOUT)
			Expect(err).NotTo(HaveOccurred())

			delayIncidentID = response.ID
		})

		By("successfully making a DNS query", func() {
			var expectedAddresses []string

			// miekg/dns implementation responds with A and AAAA records regardless of the type of record requested
			// therefore we're expected 4 IPs here
			if config.WindowsClients {
				expectedAddresses = []string{"10.2.3.4", "10.2.3.4"}
			} else {
				expectedAddresses = []string{"10.2.3.4", "10.2.3.4", "10.2.3.4", "10.2.3.4"}
			}

			Eventually(func() (dnsCallResponse, error) {
				var err error
				dnsStartTime := time.Now()
				addresses, err := tcClient.DNS("my-fake-server.fake.local")

				if err != nil {
					return dnsCallResponse{}, err
				}

				dnsElapsedTime := time.Since(dnsStartTime)
				return dnsCallResponse{
					addresses:          addresses,
					isGreaterThanDelay: dnsElapsedTime.Nanoseconds() > int64(DELAY),
				}, nil
			}, "1m", "100ms").Should(Equal(dnsCallResponse{
				addresses:          expectedAddresses,
				isGreaterThanDelay: true,
			}))
		})
	})
})
