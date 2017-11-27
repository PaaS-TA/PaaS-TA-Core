package deploy_test

import (
	etcdclient "github.com/cloudfoundry-incubator/etcd-release/src/acceptance-tests/testing/etcd"

	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/cloudfoundry-incubator/etcd-release/src/acceptance-tests/testing/helpers"

	"github.com/pivotal-cf-experimental/bosh-test/bosh"
	"github.com/pivotal-cf-experimental/destiny/ops"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

const (
	PUT_ERROR_COUNT_THRESHOLD                  = 8
	TEST_CONSUMER_CONNECTION_RESET_ERROR_COUNT = 1
)

var _ = Describe("TLS Upgrade", func() {
	var (
		manifest     string
		manifestName string

		spammers []*helpers.Spammer
		watcher  *helpers.Watcher
	)

	AfterEach(func() {
		if !CurrentGinkgoTestDescription().Failed {
			err := boshClient.DeleteDeployment(manifestName)
			Expect(err).NotTo(HaveOccurred())
		}
	})

	It("keeps writing to an etcd cluster without interruption", func() {
		By("deploy non tls etcd", func() {
			var err error
			manifest, err = helpers.NewEtcdManifestWithInstanceCount("tls-upgrade", 3, false, boshClient)
			Expect(err).NotTo(HaveOccurred())

			manifestName, err = ops.ManifestName(manifest)
			Expect(err).NotTo(HaveOccurred())

			manifest, err = ops.ApplyOp(manifest, ops.Op{
				Type:  "replace",
				Path:  "/instance_groups/name=testconsumer/instances",
				Value: 5,
			})
			Expect(err).NotTo(HaveOccurred())

			_, err = boshClient.Deploy([]byte(manifest))
			Expect(err).NotTo(HaveOccurred())

			Eventually(func() ([]bosh.VM, error) {
				return helpers.DeploymentVMs(boshClient, manifestName)
			}, "1m", "10s").Should(ConsistOf(helpers.GetVMsFromManifest(manifest)))
		})

		By("setup watcher to save keys from the non tls cluster", func() {
			etcdIPs, err := helpers.GetVMIPs(boshClient, manifestName, "etcd")
			Expect(err).NotTo(HaveOccurred())

			watcher = helpers.NewEtcdWatcher(etcdIPs)
		})

		By("spamming the cluster", func() {
			testConsumerIPs, err := helpers.GetVMIPs(boshClient, manifestName, "testconsumer")
			Expect(err).NotTo(HaveOccurred())

			for i, ip := range testConsumerIPs {
				etcdClient := etcdclient.NewClient(fmt.Sprintf("http://%s:6769", ip))
				spammer := helpers.NewSpammer(etcdClient, 1*time.Second, fmt.Sprintf("tls-upgrade-%d", i))
				spammers = append(spammers, spammer)

				spammer.Spam()
			}
		})

		By("deploy tls etcd, scale down non-tls etcd, deploy proxy, and switch clients to tls etcd", func() {
			var err error
			manifest, err = helpers.NewEtcdManifestWithInstanceCount("tls-upgrade", 3, true, boshClient)
			Expect(err).NotTo(HaveOccurred())

			caCert, err := ops.FindOp(manifest, "/instance_groups/name=etcd/properties/etcd/ca_cert")
			Expect(err).NotTo(HaveOccurred())

			clientCert, err := ops.FindOp(manifest, "/instance_groups/name=etcd/properties/etcd/client_cert")
			Expect(err).NotTo(HaveOccurred())

			clientKey, err := ops.FindOp(manifest, "/instance_groups/name=etcd/properties/etcd/client_key")
			Expect(err).NotTo(HaveOccurred())

			testConsumer, err := ops.FindOp(manifest, "/instance_groups/name=testconsumer")
			Expect(err).NotTo(HaveOccurred())

			manifest, err = ops.ApplyOps(manifest, []ops.Op{
				{
					Type:  "replace",
					Path:  "/instance_groups/name=etcd/name",
					Value: "etcd_tls",
				},
				{
					Type: "replace",
					Path: "/instance_groups/-",
					Value: map[string]interface{}{
						"name":      "etcd",
						"instances": 1,
						"azs":       []string{"z1"},
						"jobs": []map[string]interface{}{
							{
								"name":    "consul_agent",
								"release": "consul",
								"consumes": map[string]interface{}{
									"consul": map[string]string{"from": "consul_server"},
								},
							},
							{
								"name":    "etcd_proxy",
								"release": "etcd",
							},
						},
						"vm_type":              "default",
						"stemcell":             "default",
						"persistent_disk_type": "1GB",
						"networks": []map[string]string{
							{
								"name": "private",
							},
						},
						"properties": map[string]interface{}{
							"etcd_proxy": map[string]interface{}{
								"etcd": map[string]string{
									"ca_cert":     caCert.(string),
									"client_cert": clientCert.(string),
									"client_key":  clientKey.(string),
								},
							},
						},
					},
				},
				{
					Type: "remove",
					Path: "/instance_groups/name=testconsumer",
				},
				{
					Type:  "replace",
					Path:  "/instance_groups/-",
					Value: testConsumer,
				},
				{
					Type:  "replace",
					Path:  "/instance_groups/name=testconsumer/instances",
					Value: 5,
				},
			})
			Expect(err).NotTo(HaveOccurred())

			_, err = boshClient.Deploy([]byte(manifest))
			Expect(err).NotTo(HaveOccurred())

			Eventually(func() ([]bosh.VM, error) {
				return helpers.DeploymentVMs(boshClient, manifestName)
			}, "1m", "10s").Should(ConsistOf(helpers.GetVMsFromManifest(manifest)))
		})

		By("migrating the non tls data to the tls cluster", func() {
			watcher.Stop <- true

			etcdIPs, err := helpers.GetVMIPs(boshClient, manifestName, "etcd")
			Expect(err).NotTo(HaveOccurred())

			etcdClient := helpers.NewEtcdClient([]string{fmt.Sprintf("http://%s:4001", etcdIPs[0])})

			for key, value := range watcher.Data() {
				err := etcdClient.Set(key, value)
				Expect(err).NotTo(HaveOccurred())
			}
		})

		By("removing the proxy", func() {
			var err error
			manifest, err = ops.ApplyOp(manifest, ops.Op{
				Type: "remove",
				Path: "/instance_groups/name=etcd",
			})
			Expect(err).NotTo(HaveOccurred())

			_, err = boshClient.Deploy([]byte(manifest))
			Expect(err).NotTo(HaveOccurred())

			Eventually(func() ([]bosh.VM, error) {
				return helpers.DeploymentVMs(boshClient, manifestName)
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
