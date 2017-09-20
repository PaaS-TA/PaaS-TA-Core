package deploy_test

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/cloudfoundry-incubator/consul-release/src/acceptance-tests/testing/consulclient"
	"github.com/cloudfoundry-incubator/consul-release/src/acceptance-tests/testing/helpers"
	"github.com/pivotal-cf-experimental/bosh-test/bosh"
	"github.com/pivotal-cf-experimental/destiny/consul"

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
		manifest consul.Manifest
		kv       consulclient.HTTPKV
		spammers []*helpers.Spammer
	)

	AfterEach(func() {
		if !CurrentGinkgoTestDescription().Failed {
			err := boshClient.DeleteDeployment(manifest.Name)
			Expect(err).NotTo(HaveOccurred())
		}
	})

	Describe("when migrating two instance groups from different AZs to a multi-AZ single instance group", func() {
		It("deploys successfully with minimal interruption", func() {
			By("deploying 3 node cluster across two AZs with BOSH 1.0 manifest", func() {
				var err error
				manifest, err = helpers.DeployMultiAZConsul("migrate-instance-group", boshClient, config)
				Expect(err).NotTo(HaveOccurred())

				Eventually(func() ([]bosh.VM, error) {
					return helpers.DeploymentVMs(boshClient, manifest.Name)
				}, "1m", "10s").Should(ConsistOf(helpers.GetVMsFromManifestV1(manifest)))

				for i, ip := range manifest.Jobs[2].Networks[0].StaticIPs {
					kv = consulclient.NewHTTPKV(fmt.Sprintf("http://%s:6769", ip))
					spammers = append(spammers, helpers.NewSpammer(kv, 1*time.Second, fmt.Sprintf("test-consumer-%d", i)))
				}
			})

			By("starting spammer", func() {
				for _, spammer := range spammers {
					spammer.Spam()
				}
			})

			By("deploying 3 node cluster across two AZs with BOSH 2.0 manifest", func() {
				manifestv2, err := helpers.DeployMultiAZConsulMigration(boshClient, config, manifest.Name)
				Expect(err).NotTo(HaveOccurred())

				Eventually(func() ([]bosh.VM, error) {
					return helpers.DeploymentVMs(boshClient, manifestv2.Name)
				}, "1m", "10s").Should(ConsistOf(helpers.GetVMsFromManifest(manifestv2)))
			})

			By("verifying keys are accounted for in cluster", func() {
				for _, spammer := range spammers {
					spammer.Stop()
					spammerErrs := spammer.Check()

					var errorSet helpers.ErrorSet

					switch spammerErrs.(type) {
					case helpers.ErrorSet:
						errorSet = spammerErrs.(helpers.ErrorSet)
					case nil:
						continue
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
