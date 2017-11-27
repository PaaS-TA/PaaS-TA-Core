package deploy_test

import (
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	etcdclient "github.com/cloudfoundry-incubator/etcd-release/src/acceptance-tests/testing/etcd"

	"github.com/cloudfoundry-incubator/etcd-release/src/acceptance-tests/testing/helpers"

	"github.com/pivotal-cf-experimental/bosh-test/bosh"
	"github.com/pivotal-cf-experimental/destiny/ops"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Multiple instance rolling upgrade", func() {
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

		releaseNumber := os.Getenv("LATEST_ETCD_RELEASE_VERSION")

		enableSSL := true
		manifest, err = helpers.DeployEtcdWithInstanceCountAndReleaseVersion("multiple-instance-rolling-upgrade", 3, enableSSL, boshClient, releaseNumber)
		Expect(err).NotTo(HaveOccurred())

		manifestName, err = ops.ManifestName(manifest)
		Expect(err).NotTo(HaveOccurred())

		Eventually(func() ([]bosh.VM, error) {
			return helpers.DeploymentVMs(boshClient, manifestName)
		}, "1m", "10s").Should(ConsistOf(helpers.GetVMsFromManifest(manifest)))

		testConsumerIPs, err := helpers.GetVMIPs(boshClient, manifestName, "testconsumer")
		Expect(err).NotTo(HaveOccurred())

		etcdClient = etcdclient.NewClient(fmt.Sprintf("http://%s:6769", testConsumerIPs[0]))

		spammer = helpers.NewSpammer(etcdClient, 1*time.Second, "multiple-instance-rolling-upgrade")
	})

	AfterEach(func() {
		if !CurrentGinkgoTestDescription().Failed {
			err := boshClient.DeleteDeployment(manifestName)
			Expect(err).NotTo(HaveOccurred())
		}
	})

	It("persists data throughout the rolling upgrade", func() {
		By("setting a persistent value", func() {
			err := etcdClient.Set(testKey, testValue)
			Expect(err).NotTo(HaveOccurred())
		})

		By("deploying the latest dev build of etcd-release", func() {
			var err error
			manifest, err = ops.ApplyOp(manifest, ops.Op{
				Type:  "replace",
				Path:  "/releases/name=etcd/version",
				Value: helpers.EtcdDevReleaseVersion(),
			})
			Expect(err).NotTo(HaveOccurred())

			spammer.Spam()

			_, err = boshClient.Deploy([]byte(manifest))
			Expect(err).NotTo(HaveOccurred())

			Eventually(func() ([]bosh.VM, error) {
				return helpers.DeploymentVMs(boshClient, manifestName)
			}, "1m", "10s").Should(ConsistOf(helpers.GetVMsFromManifest(manifest)))

			err = helpers.VerifyDeploymentRelease(boshClient, manifestName, helpers.EtcdDevReleaseVersion())
			Expect(err).NotTo(HaveOccurred())

			spammer.Stop()
		})

		By("reading the values from etcd", func() {
			value, err := etcdClient.Get(testKey)
			Expect(err).NotTo(HaveOccurred())
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
})
