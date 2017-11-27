package turbulence_test

import (
	"fmt"
	"math/rand"
	"strconv"
	"strings"
	"time"

	etcdclient "github.com/cloudfoundry-incubator/etcd-release/src/acceptance-tests/testing/etcd"
	turbulenceclient "github.com/pivotal-cf-experimental/bosh-test/turbulence"

	"github.com/cloudfoundry-incubator/etcd-release/src/acceptance-tests/testing/helpers"

	"github.com/pivotal-cf-experimental/bosh-test/bosh"
	"github.com/pivotal-cf-experimental/destiny/ops"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("quorum loss", func() {
	QuorumLossTest := func(enableSSL bool) {
		var (
			turbulenceManifest     string
			turbulenceManifestName string

			etcdManifest     string
			etcdManifestName string

			etcdClient etcdclient.Client

			initialKey   string
			initialValue string
		)

		BeforeEach(func() {
			deploymentSuffix := "quorum-loss-non-tls"
			if enableSSL {
				deploymentSuffix = "quorum-loss-tls"
			}

			By("deploying turbulence", func() {
				var err error
				turbulenceManifest, err = helpers.DeployTurbulence(deploymentSuffix, boshClient)
				Expect(err).NotTo(HaveOccurred())

				turbulenceManifestName, err = ops.ManifestName(turbulenceManifest)

				Eventually(func() ([]bosh.VM, error) {
					return helpers.DeploymentVMs(boshClient, turbulenceManifestName)
				}, "1m", "10s").Should(ConsistOf(helpers.GetVMsFromManifest(turbulenceManifest)))

				turbulencePassword, err := ops.FindOp(turbulenceManifest, "/instance_groups/name=api/properties/password")
				Expect(err).NotTo(HaveOccurred())

				turbulenceIPs, err := helpers.GetVMIPs(boshClient, turbulenceManifestName, "api")
				Expect(err).NotTo(HaveOccurred())

				turbulenceClient = turbulenceclient.NewClient(fmt.Sprintf("https://turbulence:%s@%s:8080", turbulencePassword, turbulenceIPs[0]), 5*time.Minute, 2*time.Second)
			})

			By("deploying a 5 node etcd", func() {
				var err error

				etcdManifest, err = helpers.DeployEtcdWithInstanceCount(deploymentSuffix, 5, enableSSL, boshClient)
				Expect(err).NotTo(HaveOccurred())

				etcdManifestName, err = ops.ManifestName(etcdManifest)
				Expect(err).NotTo(HaveOccurred())

				Eventually(func() ([]bosh.VM, error) {
					return helpers.DeploymentVMs(boshClient, etcdManifestName)
				}, "1m", "10s").Should(ConsistOf(helpers.GetVMsFromManifest(etcdManifest)))

				testConsumerIPs, err := helpers.GetVMIPs(boshClient, etcdManifestName, "testconsumer")
				Expect(err).NotTo(HaveOccurred())

				etcdClient = etcdclient.NewClient(fmt.Sprintf("http://%s:6769", testConsumerIPs[0]))
			})
		})

		AfterEach(func() {
			By("deleting the deployment", func() {
				if !CurrentGinkgoTestDescription().Failed {
					for i := 0; i < 5; i++ {
						err := boshClient.SetVMResurrection(etcdManifestName, "etcd", i, true)
						Expect(err).NotTo(HaveOccurred())
					}

					Eventually(func() error {
						return boshClient.ScanAndFixAll([]byte(etcdManifest))
					}, "12m", "1m").ShouldNot(HaveOccurred())

					Eventually(func() ([]bosh.VM, error) {
						return helpers.DeploymentVMs(boshClient, etcdManifestName)
					}, "10m", "1m").Should(ConsistOf(helpers.GetVMsFromManifest(etcdManifest)))

					Eventually(func() ([]string, error) {
						return lockedDeployments(boshClient)
					}, "12m", "1m").ShouldNot(ContainElement(etcdManifestName))

					err := boshClient.DeleteDeployment(etcdManifestName)
					Expect(err).NotTo(HaveOccurred())
				}
			})

			By("deleting turbulence", func() {
				err := boshClient.DeleteDeployment(turbulenceManifestName)
				Expect(err).NotTo(HaveOccurred())
			})
		})

		Context("when a etcd node is killed", func() {
			It("is still able to function on healthy vms", func() {
				By("setting and getting a value", func() {
					guid, err := helpers.NewGUID()
					Expect(err).NotTo(HaveOccurred())
					initialKey = "etcd-key-" + guid
					initialValue = "etcd-value-" + guid
					err = etcdClient.Set(initialKey, initialValue)
					Expect(err).NotTo(HaveOccurred())

					value, err := etcdClient.Get(initialKey)
					Expect(err).NotTo(HaveOccurred())
					Expect(value).To(Equal(initialValue))
				})

				By("killing indices", func() {
					for i := 0; i < 5; i++ {
						err := boshClient.SetVMResurrection(etcdManifestName, "etcd", i, false)
						Expect(err).NotTo(HaveOccurred())
					}

					leader, err := jobIndexOfLeader(etcdClient)
					Expect(err).NotTo(HaveOccurred())

					rand.Seed(time.Now().Unix())
					startingIndex := rand.Intn(3)
					instances := []int{startingIndex, startingIndex + 1, startingIndex + 2}

					if leader < startingIndex || leader > startingIndex+2 {
						instances[0] = leader
					}

					jobIndexToResurrect := startingIndex + 1

					vmIDs, err := helpers.GetVMIDByIndices(boshClient, etcdManifestName, "etcd", instances)
					Expect(err).NotTo(HaveOccurred())

					err = turbulenceClient.KillIDs(vmIDs)
					Expect(err).NotTo(HaveOccurred())

					err = boshClient.SetVMResurrection(etcdManifestName, "etcd", jobIndexToResurrect, true)
					Expect(err).NotTo(HaveOccurred())

					Eventually(func() error {
						return boshClient.ScanAndFix(etcdManifestName, "etcd", []int{jobIndexToResurrect})
					}, "12m", "1m").ShouldNot(HaveOccurred())

					Eventually(func() ([]bosh.VM, error) {
						return helpers.DeploymentVMs(boshClient, etcdManifestName)
					}, "10m", "1m").Should(ContainElement(bosh.VM{JobName: "etcd", Index: jobIndexToResurrect, State: "running"}))
				})

				By("getting the previous key and value", func() {
					value, err := etcdClient.Get(initialKey)
					Expect(err).NotTo(HaveOccurred())
					Expect(value).To(Equal(initialValue))
				})

				By("setting and getting a new value", func() {
					guid, err := helpers.NewGUID()
					Expect(err).NotTo(HaveOccurred())
					testKey := "etcd-key-" + guid
					testValue := "etcd-value-" + guid

					err = etcdClient.Set(testKey, testValue)
					Expect(err).NotTo(HaveOccurred())

					value, err := etcdClient.Get(testKey)
					Expect(err).NotTo(HaveOccurred())
					Expect(value).To(Equal(testValue))
				})
			})
		})
	}

	Context("without TLS", func() {
		QuorumLossTest(false)
	})

	Context("with TLS", func() {
		QuorumLossTest(true)
	})
})

func jobIndexOfLeader(etcdClient etcdclient.Client) (int, error) {
	leader, err := etcdClient.Leader()
	if err != nil {
		return -1, err
	}

	leaderNameParts := strings.Split(leader, "-")

	leaderIndex, err := strconv.Atoi(leaderNameParts[len(leaderNameParts)-1])
	if err != nil {
		return -1, err
	}

	return leaderIndex, nil
}

func lockedDeployments(boshClient bosh.Client) ([]string, error) {
	var lockNames []string
	locks, err := boshClient.Locks()
	if err != nil {
		return []string{}, err
	}
	for _, lock := range locks {
		lockNames = append(lockNames, lock.Resource[0])
	}
	return lockNames, nil
}
