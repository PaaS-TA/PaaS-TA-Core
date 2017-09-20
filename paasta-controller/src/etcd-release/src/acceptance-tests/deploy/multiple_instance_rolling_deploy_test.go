package deploy_test

import (
	"errors"
	"fmt"
	"strings"
	"time"

	etcdclient "acceptance-tests/testing/etcd"

	"acceptance-tests/testing/helpers"

	"github.com/pivotal-cf-experimental/bosh-test/bosh"
	"github.com/pivotal-cf-experimental/destiny/etcd"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Multiple instance rolling deploys", func() {
	MultipleInstanceRollingDeploy := func(enableSSL bool) {
		var (
			manifest   etcd.Manifest
			etcdClient etcdclient.Client
			spammer    *helpers.Spammer

			testKey   string
			testValue string
		)

		BeforeEach(func() {
			guid, err := helpers.NewGUID()
			Expect(err).NotTo(HaveOccurred())

			testKey = "etcd-key-" + guid
			testValue = "etcd-value-" + guid

			manifest, err = helpers.DeployEtcdWithInstanceCount("multiple_instance_rolling_deploy", 3, client, config, enableSSL)
			Expect(err).NotTo(HaveOccurred())

			Eventually(func() ([]bosh.VM, error) {
				return helpers.DeploymentVMs(client, manifest.Name)
			}, "1m", "10s").Should(ConsistOf(helpers.GetVMsFromManifest(manifest)))

			testConsumerIndex, err := helpers.FindJobIndexByName(manifest, "testconsumer_z1")
			Expect(err).NotTo(HaveOccurred())
			etcdClient = etcdclient.NewClient(fmt.Sprintf("http://%s:6769", manifest.Jobs[testConsumerIndex].Networks[0].StaticIPs[0]))
			spammer = helpers.NewSpammer(etcdClient, 1*time.Second, "multi-instance-rolling-deploy")
		})

		AfterEach(func() {
			if !CurrentGinkgoTestDescription().Failed {
				err := client.DeleteDeployment(manifest.Name)
				Expect(err).NotTo(HaveOccurred())
			}
		})

		It("persists data throughout the rolling deploy", func() {
			By("setting a persistent value", func() {
				err := etcdClient.Set(testKey, testValue)
				Expect(err).ToNot(HaveOccurred())
			})

			By("deploying", func() {
				manifest.Properties.Etcd.HeartbeatIntervalInMilliseconds = 51

				yaml, err := manifest.ToYAML()
				Expect(err).NotTo(HaveOccurred())

				yaml, err = client.ResolveManifestVersions(yaml)
				Expect(err).NotTo(HaveOccurred())

				spammer.Spam()

				_, err = client.Deploy(yaml)
				Expect(err).NotTo(HaveOccurred())

				Eventually(func() ([]bosh.VM, error) {
					return helpers.DeploymentVMs(client, manifest.Name)
				}, "1m", "10s").Should(ConsistOf(helpers.GetVMsFromManifest(manifest)))

				spammer.Stop()
			})

			By("reading the value from etcd", func() {
				value, err := etcdClient.Get(testKey)
				Expect(err).ToNot(HaveOccurred())
				Expect(value).To(Equal(testValue))

				spammerErrs := spammer.Check()
				var errorSet helpers.ErrorSet

				switch spammerErrs.(type) {
				case helpers.ErrorSet:
					errorSet = spammerErrs.(helpers.ErrorSet)
				case nil:
					return
				default:
					Fail(spammerErrs.Error())
				}

				tcpErrCount := 0
				unexpectedErrCount := 0
				testConsumerConnectionResetErrorCount := 0
				otherErrors := helpers.ErrorSet{}

				for err, occurrences := range errorSet {
					switch {
					// This happens when the etcd leader is killed and a request is issued while an election is happening
					case strings.Contains(err, "Unexpected HTTP status code"):
						unexpectedErrCount += occurrences
					// This happens when the consul_agent gets rolled when a request is sent to the testconsumer
					case strings.Contains(err, "dial tcp: lookup etcd.service.cf.internal on"):
						tcpErrCount += occurrences
					// This happens when a connection is severed by the etcd server
					case strings.Contains(err, "EOF"):
						testConsumerConnectionResetErrorCount += occurrences
					default:
						otherErrors.Add(errors.New(err))
					}
				}

				Expect(otherErrors).To(HaveLen(0))
				Expect(unexpectedErrCount).To(BeNumerically("<=", 3))
				Expect(tcpErrCount).To(BeNumerically("<=", 1))
				Expect(testConsumerConnectionResetErrorCount).To(BeNumerically("<=", 1))

			})
		})
	}

	Context("without TLS", func() {
		MultipleInstanceRollingDeploy(false)
	})

	Context("with TLS", func() {
		MultipleInstanceRollingDeploy(true)
	})
})
