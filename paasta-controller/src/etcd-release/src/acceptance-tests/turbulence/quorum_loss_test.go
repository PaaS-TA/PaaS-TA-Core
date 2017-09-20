package turbulence_test

import (
	"fmt"
	"math/rand"
	"strconv"
	"strings"
	"time"

	"acceptance-tests/testing/helpers"

	etcdclient "acceptance-tests/testing/etcd"

	"github.com/pivotal-cf-experimental/bosh-test/bosh"
	"github.com/pivotal-cf-experimental/destiny/etcd"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("quorum loss", func() {
	QuorumLossTest := func(enableSSL bool) {
		var (
			etcdManifest etcd.Manifest
			etcdClient   etcdclient.Client

			initialKey   string
			initialValue string
		)

		BeforeEach(func() {
			By("deploying turbulence", func() {
				var err error
				turbulenceManifest, err = helpers.DeployTurbulence(boshClient, config)
				Expect(err).NotTo(HaveOccurred())

				Eventually(func() ([]bosh.VM, error) {
					return helpers.DeploymentVMs(boshClient, turbulenceManifest.Name)
				}, "1m", "10s").Should(ConsistOf(helpers.GetTurbulenceVMsFromManifest(turbulenceManifest)))

				turbulenceClient = helpers.NewTurbulenceClient(turbulenceManifest)
			})

			By("deploying a 5 node etcd", func() {
				var err error

				etcdManifest, err = helpers.DeployEtcdWithInstanceCount("quorum_loss", 5, boshClient, config, enableSSL)
				Expect(err).NotTo(HaveOccurred())

				Eventually(func() ([]bosh.VM, error) {
					return helpers.DeploymentVMs(boshClient, etcdManifest.Name)
				}, "1m", "10s").Should(ConsistOf(helpers.GetVMsFromManifest(etcdManifest)))

				testConsumerIndex, err := helpers.FindJobIndexByName(etcdManifest, "testconsumer_z1")
				Expect(err).NotTo(HaveOccurred())
				etcdClient = etcdclient.NewClient(fmt.Sprintf("http://%s:6769", etcdManifest.Jobs[testConsumerIndex].Networks[0].StaticIPs[0]))
			})
		})

		AfterEach(func() {
			By("deleting the deployment", func() {
				if !CurrentGinkgoTestDescription().Failed {
					for i := 0; i < 5; i++ {
						err := boshClient.SetVMResurrection(etcdManifest.Name, "etcd_z1", i, true)
						Expect(err).NotTo(HaveOccurred())
					}

					yaml, err := etcdManifest.ToYAML()
					Expect(err).NotTo(HaveOccurred())

					Eventually(func() error {
						return boshClient.ScanAndFixAll(yaml)
					}, "12m", "1m").ShouldNot(HaveOccurred())

					Eventually(func() ([]bosh.VM, error) {
						return helpers.DeploymentVMs(boshClient, etcdManifest.Name)
					}, "1m", "10s").Should(ConsistOf(helpers.GetVMsFromManifest(etcdManifest)))

					err = boshClient.DeleteDeployment(etcdManifest.Name)
					Expect(err).NotTo(HaveOccurred())
				}
			})

			By("deleting turbulence", func() {
				err := boshClient.DeleteDeployment(turbulenceManifest.Name)
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
						err := boshClient.SetVMResurrection(etcdManifest.Name, "etcd_z1", i, false)
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

					err = turbulenceClient.KillIndices(etcdManifest.Name, "etcd_z1", instances)
					Expect(err).NotTo(HaveOccurred())

					err = boshClient.SetVMResurrection(etcdManifest.Name, "etcd_z1", jobIndexToResurrect, true)
					Expect(err).NotTo(HaveOccurred())

					Eventually(func() error {
						return boshClient.ScanAndFix(etcdManifest.Name, "etcd_z1", []int{jobIndexToResurrect})
					}, "12m", "1m").ShouldNot(HaveOccurred())

					Eventually(func() ([]bosh.VM, error) {
						return helpers.DeploymentVMs(boshClient, etcdManifest.Name)
					}, "5m", "1m").Should(ContainElement(bosh.VM{JobName: "etcd_z1", Index: jobIndexToResurrect, State: "running"}))
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
