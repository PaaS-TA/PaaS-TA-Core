package deploy_test

import (
	"errors"
	"fmt"
	"strings"
	"time"

	etcdclient "github.com/cloudfoundry-incubator/etcd-release/src/acceptance-tests/testing/etcd"
	"github.com/cloudfoundry-incubator/etcd-release/src/acceptance-tests/testing/helpers"
	"github.com/pivotal-cf-experimental/bosh-test/bosh"
	"github.com/pivotal-cf-experimental/destiny/ops"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Migrate instance name change", func() {
	var (
		manifest     string
		manifestName string

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

		manifest, err = helpers.DeployEtcdWithInstanceCount("migrate-instance-name-change", 3, false, boshClient)
		Expect(err).NotTo(HaveOccurred())

		manifestName, err = ops.ManifestName(manifest)
		Expect(err).NotTo(HaveOccurred())

		Eventually(func() ([]bosh.VM, error) {
			return helpers.DeploymentVMs(boshClient, manifestName)
		}, "1m", "10s").Should(ConsistOf(helpers.GetVMsFromManifest(manifest)))

		testConsumerIPs, err := helpers.GetVMIPs(boshClient, manifestName, "testconsumer")
		Expect(err).NotTo(HaveOccurred())

		etcdClient = etcdclient.NewClient(fmt.Sprintf("http://%s:6769", testConsumerIPs[0]))

		spammer = helpers.NewSpammer(etcdClient, 1*time.Second, "migrate-instance-name-change")
	})

	AfterEach(func() {
		if !CurrentGinkgoTestDescription().Failed {
			err := boshClient.DeleteDeployment(manifestName)
			Expect(err).NotTo(HaveOccurred())
		}
	})

	It("migrates an etcd cluster sucessfully when instance name is changed", func() {
		By("setting a persistent value", func() {
			err := etcdClient.Set(testKey, testValue)
			Expect(err).ToNot(HaveOccurred())
		})

		By("deploying with a new name", func() {
			var err error
			manifest, err = ops.ApplyOps(manifest, []ops.Op{
				{
					Type:  "replace",
					Path:  "/instance_groups/name=etcd/name",
					Value: "new_etcd",
				},
				{
					Type: "replace",
					Path: "/instance_groups/name=new_etcd/migrated_from?/-",
					Value: map[string]string{
						"name": "etcd",
					},
				},
				{
					Type:  "replace",
					Path:  "/instance_groups/name=new_etcd/cluster?/name=etcd/name",
					Value: "new_etcd",
				},
			})
			Expect(err).ToNot(HaveOccurred())

			_, err = boshClient.Deploy([]byte(manifest))
			Expect(err).NotTo(HaveOccurred())

			spammer.Spam()

			Eventually(func() ([]bosh.VM, error) {
				return helpers.DeploymentVMs(boshClient, manifestName)
			}, "1m", "10s").Should(ConsistOf(helpers.GetVMsFromManifest(manifest)))

			spammer.Stop()
		})

		By("getting a persistent value", func() {
			value, err := etcdClient.Get(testKey)
			Expect(err).ToNot(HaveOccurred())
			Expect(value).To(Equal(testValue))
		})

		By("checking the spammer", func() {
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
})
