package deploy_test

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/cloudfoundry-incubator/consul-release/src/acceptance-tests/testing/consulclient"
	"github.com/cloudfoundry-incubator/consul-release/src/acceptance-tests/testing/helpers"
	"github.com/pivotal-cf-experimental/bosh-test/bosh"
	"github.com/pivotal-cf-experimental/destiny/ops"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

const (
	OPERATION_TIMEOUT_ERROR_COUNT_THRESHOLD = 1
	RESET_ERROR_COUNT_THRESHOLD             = 3
	IO_TIMEOUT_ERROR_COUNT_THRESHOLD        = 25
	NO_ROUTE_ERROR_COUNT_THRESHOLD          = 10
)

var _ = Describe("Migrate instance groups", func() {
	var (
		manifest     string
		manifestName string

		kv      consulclient.HTTPKV
		spammer *helpers.Spammer
	)

	AfterEach(func() {
		if !CurrentGinkgoTestDescription().Failed {
			err := boshClient.DeleteDeployment(manifestName)
			Expect(err).NotTo(HaveOccurred())
		}
	})

	Describe("when migrating an instance group from one name to another", func() {
		It("deploys successfully with minimal interruption", func() {
			By("deploying 3 node cluster across two AZs with name consul", func() {
				var err error
				manifest, err = helpers.DeployConsulWithInstanceCount("migrate-instance-group", 3, config.WindowsClients, boshClient)
				Expect(err).NotTo(HaveOccurred())

				manifestName, err = ops.ManifestName(manifest)
				Expect(err).NotTo(HaveOccurred())

				Eventually(func() ([]bosh.VM, error) {
					return helpers.DeploymentVMs(boshClient, manifestName)
				}, "5m", "10s").Should(ConsistOf(helpers.GetVMsFromManifest(manifest)))

				testConsumerIPs, err := helpers.GetVMIPs(boshClient, manifestName, "testconsumer")
				Expect(err).NotTo(HaveOccurred())

				kv = consulclient.NewHTTPKV(fmt.Sprintf("http://%s:6769", testConsumerIPs[0]))

				spammer = helpers.NewSpammer(kv, 1*time.Second, "testconsumer")

				spammer.Spam()
			})

			By("deploying 3 node cluster across two AZs with name consul_new", func() {
				var err error
				manifest, err = ops.ApplyOps(manifest, []ops.Op{
					{
						Type:  "replace",
						Path:  "/instance_groups/name=consul/name",
						Value: "consul_new",
					},
					{
						Type: "replace",
						Path: "/instance_groups/name=consul_new/migrated_from?/-",
						Value: map[string]string{
							"name": "consul",
						},
					},
				})
				Expect(err).NotTo(HaveOccurred())

				_, err = boshClient.Deploy([]byte(manifest))
				Expect(err).NotTo(HaveOccurred())

				Eventually(func() ([]bosh.VM, error) {
					return helpers.DeploymentVMs(boshClient, manifestName)
				}, "5m", "10s").Should(ConsistOf(helpers.GetVMsFromManifest(manifest)))
			})

			By("verifying keys are accounted for in cluster", func() {
				spammer.Stop()
				spammerErrs := spammer.Check()

				var errorSet helpers.ErrorSet

				if spammerErrs != nil {
					switch spammerErrs.(type) {
					case helpers.ErrorSet:
						errorSet = spammerErrs.(helpers.ErrorSet)
					default:
						Fail(spammerErrs.Error())
					}

					timeoutErrCount := 0
					norouteErrCount := 0
					ioErrCount := 0
					resetErrCount := 0
					otherErrors := helpers.ErrorSet{}

					for err, occurrences := range errorSet {
						switch {
						// This happens when the testconsumer gets rolled when a connection is alive
						case strings.Contains(err, "getsockopt: operation timed out"):
							timeoutErrCount += occurrences
						// This happens when the vm has been destroyed and has not been recreated yet
						case strings.Contains(err, "getsockopt: no route to host"):
							norouteErrCount += occurrences
						// This happens when the vm is being recreated
						case strings.Contains(err, "i/o timeout"):
							ioErrCount += occurrences
						// This happens when the testconsumer gets destroyed during a migration
						case strings.Contains(err, "read: connection reset by peer"):
							resetErrCount += occurrences
						default:
							otherErrors.Add(errors.New(err))
						}
					}

					Expect(otherErrors).To(HaveLen(0))
					Expect(timeoutErrCount).To(BeNumerically("<=", OPERATION_TIMEOUT_ERROR_COUNT_THRESHOLD))
					Expect(norouteErrCount).To(BeNumerically("<=", NO_ROUTE_ERROR_COUNT_THRESHOLD))
					Expect(ioErrCount).To(BeNumerically("<=", IO_TIMEOUT_ERROR_COUNT_THRESHOLD))
					Expect(resetErrCount).To(BeNumerically("<=", RESET_ERROR_COUNT_THRESHOLD))
				}
			})
		})
	})
})
