package deploy_test

import (
	"fmt"

	"github.com/cloudfoundry-incubator/consul-release/src/acceptance-tests/testing/helpers"
	"github.com/pivotal-cf-experimental/bosh-test/bosh"
	"github.com/pivotal-cf-experimental/destiny/ops"

	testconsumerclient "github.com/cloudfoundry-incubator/consul-release/src/acceptance-tests/testing/testconsumer/client"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

type instanceGroup struct {
	Name               string
	Instances          int
	AZs                []string
	VMType             string `yaml:"vm_type"`
	Stemcell           string
	PersistentDiskType string `yaml:"persistent_disk_type"`
	Networks           []instanceGroupNetwork
	Jobs               []instanceGroupJob
}

type instanceGroupNetwork struct {
	Name string
}

type instanceGroupJob struct {
	Name     string
	Release  string
	Provides instanceGroupJobLink
}

type instanceGroupJobLink struct {
	DNS map[string]string `yaml:"dns"`
}

var _ = Describe("given large DNS response", func() {
	var (
		manifest     string
		manifestName string

		testConsumerClient testconsumerclient.Client
	)

	BeforeEach(func() {
		var err error
		manifest, err = helpers.NewConsulManifestWithInstanceCount("large-dns-response", 1, config.WindowsClients, boshClient)
		Expect(err).NotTo(HaveOccurred())

		manifestName, err = ops.ManifestName(manifest)
		Expect(err).NotTo(HaveOccurred())

		testConsumerJobName := "consul-test-consumer"

		if config.WindowsClients {
			testConsumerJobName = "consul-test-consumer-windows"
		}

		manifest, err = ops.ApplyOps(manifest, []ops.Op{
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

		_, err = boshClient.Deploy([]byte(manifest))
		Expect(err).NotTo(HaveOccurred())

		Eventually(func() ([]bosh.VM, error) {
			return helpers.DeploymentVMs(boshClient, manifestName)
		}, "5m", "10s").Should(ConsistOf(helpers.GetVMsFromManifest(manifest)))

		testConsumerIPs, err := helpers.GetVMIPs(boshClient, manifestName, "testconsumer")
		Expect(err).NotTo(HaveOccurred())

		testConsumerClient = testconsumerclient.New(fmt.Sprintf("http://%s:6769", testConsumerIPs[0]))
	})

	AfterEach(func() {
		By("deleting consul deployment", func() {
			if !CurrentGinkgoTestDescription().Failed {
				Eventually(func() ([]string, error) {
					return lockedDeployments()
				}, "10m", "30s").ShouldNot(ContainElement(manifestName))

				err := boshClient.DeleteDeployment(manifestName)
				Expect(err).NotTo(HaveOccurred())
			}
		})
	})

	It("does not error out", func() {
		addresses, err := testConsumerClient.DNS("large-dns-response.fake.local")
		Expect(err).NotTo(HaveOccurred())

		if config.WindowsClients {
			Expect(addresses).To(ConsistOf([]string{
				"1.2.3.0", "1.2.3.0",
				"1.2.3.1", "1.2.3.1",
				"1.2.3.2", "1.2.3.2",
				"1.2.3.3", "1.2.3.3",
			}))
		} else {
			Expect(addresses).To(ConsistOf([]string{
				"1.2.3.0", "1.2.3.0",
				"1.2.3.1", "1.2.3.1",
				"1.2.3.2", "1.2.3.2",
				"1.2.3.3", "1.2.3.3",
				"1.2.3.0", "1.2.3.0",
				"1.2.3.1", "1.2.3.1",
				"1.2.3.2", "1.2.3.2",
				"1.2.3.3", "1.2.3.3",
			}))
		}
	})
})
