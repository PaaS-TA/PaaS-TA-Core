package deploy_test

import (
	etcdclient "acceptance-tests/testing/etcd"
	"acceptance-tests/testing/helpers"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/pivotal-cf-experimental/bosh-test/bosh"
	"github.com/pivotal-cf-experimental/destiny/etcd"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

const (
	PUT_ERROR_COUNT_THRESHOLD                  = 8
	TEST_CONSUMER_CONNECTION_RESET_ERROR_COUNT = 1
)

var _ = Describe("TLS Upgrade", func() {
	var (
		manifest etcd.Manifest
		spammers []*helpers.Spammer
		watcher  *helpers.Watcher
	)

	AfterEach(func() {
		if !CurrentGinkgoTestDescription().Failed {
			err := client.DeleteDeployment(manifest.Name)
			Expect(err).NotTo(HaveOccurred())
		}
	})

	It("keeps writing to an etcd cluster without interruption", func() {
		By("deploy non tls etcd", func() {
			var err error
			manifest, err = helpers.NewEtcdWithInstanceCount("tls_upgrade", 3, client, config, false)
			Expect(err).NotTo(HaveOccurred())

			manifest, err = helpers.SetTestConsumerInstanceCount(5, manifest)
			Expect(err).NotTo(HaveOccurred())

			err = helpers.ResolveVersionsAndDeploy(manifest, client)
			Expect(err).NotTo(HaveOccurred())

			Eventually(func() ([]bosh.VM, error) {
				return helpers.DeploymentVMs(client, manifest.Name)
			}, "1m", "10s").Should(ConsistOf(helpers.GetVMsFromManifest(manifest)))
		})

		By("saving keys from the non tls cluster", func() {
			etcdJobIndex, err := helpers.FindJobIndexByName(manifest, "etcd_z1")
			Expect(err).NotTo(HaveOccurred())

			watcher = helpers.NewEtcdWatcher(manifest.Jobs[etcdJobIndex].Networks[0].StaticIPs)
		})

		By("spamming the cluster", func() {
			testConsumerJobIndex, err := helpers.FindJobIndexByName(manifest, "testconsumer_z1")
			Expect(err).NotTo(HaveOccurred())

			for i, ip := range manifest.Jobs[testConsumerJobIndex].Networks[0].StaticIPs {
				etcdClient := etcdclient.NewClient(fmt.Sprintf("http://%s:6769", ip))
				spammer := helpers.NewSpammer(etcdClient, 1*time.Second, fmt.Sprintf("tls-upgrade-%d", i))
				spammers = append(spammers, spammer)

				spammer.Spam()
			}
		})

		By("deploy tls etcd, scale down non-tls etcd, deploy proxy, and switch clients to tls etcd", func() {
			var err error
			manifest, err = helpers.NewEtcdManifestWithTLSUpgrade(manifest.Name, client, config)
			Expect(err).NotTo(HaveOccurred())

			err = helpers.ResolveVersionsAndDeploy(manifest, client)
			Expect(err).NotTo(HaveOccurred())

			Eventually(func() ([]bosh.VM, error) {
				return helpers.DeploymentVMs(client, manifest.Name)
			}, "1m", "10s").Should(ConsistOf(helpers.GetVMsFromManifest(manifest)))

		})

		By("migrating the non tls data to the tls cluster", func() {
			watcher.Stop <- true

			etcdJobIndex, err := helpers.FindJobIndexByName(manifest, "etcd_z1")
			Expect(err).NotTo(HaveOccurred())

			ip := manifest.Jobs[etcdJobIndex].Networks[0].StaticIPs[0]
			etcdClient := helpers.NewEtcdClient([]string{fmt.Sprintf("http://%s:4001", ip)})

			for key, value := range watcher.Data() {
				err := etcdClient.Set(key, value)
				Expect(err).NotTo(HaveOccurred())
			}
		})

		By("removing the proxy", func() {
			manifest = manifest.RemoveJob("etcd_z1")
			err := helpers.ResolveVersionsAndDeploy(manifest, client)
			Expect(err).NotTo(HaveOccurred())

			Eventually(func() ([]bosh.VM, error) {
				return helpers.DeploymentVMs(client, manifest.Name)
			}, "1m", "10s").Should(ConsistOf(helpers.GetVMsFromManifest(manifest)))
		})

		By("stopping the spammers", func() {
			for _, spammer := range spammers {
				spammer.Stop()
			}
		})

		By("reading from the cluster", func() {
			for _, spammer := range spammers {
				spammerErrors := spammer.Check()

				unexpectedHttpStatusErrorCountThreshold := 3
				unexpectedErrCount := 0
				errorSet := spammerErrors.(helpers.ErrorSet)
				etcdErrorCount := 0
				testConsumerConnectionResetErrorCount := 0
				otherErrors := helpers.ErrorSet{}

				for err, occurrences := range errorSet {
					switch {
					// This happens when the etcd leader is killed and a request is issued while an election is happening
					case strings.Contains(err, "Unexpected HTTP status code"):
						unexpectedErrCount += occurrences
					// This happens when the etcd server is down during etcd->etcd_proxy roll
					case strings.Contains(err, "last error: Put"):
						etcdErrorCount += occurrences
					// This happens when a connection is severed by the etcd server
					case strings.Contains(err, "EOF"):
						testConsumerConnectionResetErrorCount += occurrences
					default:
						otherErrors.Add(errors.New(err))
					}
				}

				Expect(etcdErrorCount).To(BeNumerically("<=", PUT_ERROR_COUNT_THRESHOLD))
				Expect(testConsumerConnectionResetErrorCount).To(BeNumerically("<=", TEST_CONSUMER_CONNECTION_RESET_ERROR_COUNT))
				Expect(unexpectedErrCount).To(BeNumerically("<=", unexpectedHttpStatusErrorCountThreshold))
				Expect(otherErrors).To(HaveLen(0))
			}
		})
	})
})
