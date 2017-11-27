package deploy_test

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"strconv"
	"strings"

	"github.com/cloudfoundry-incubator/etcd-release/src/acceptance-tests/testing/helpers"
	"github.com/pivotal-cf-experimental/bosh-test/bosh"
	"github.com/pivotal-cf-experimental/destiny/ops"

	etcdclient "github.com/cloudfoundry-incubator/etcd-release/src/acceptance-tests/testing/etcd"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

type release struct {
	Name    string
	Version string
}

type instanceGroupJob struct {
	Name     string
	Release  string
	Consumes instanceGroupJobLink `yaml:",omitempty"`
}

type instanceGroupJobLink struct {
	API map[string]string `yaml:"api,omitempty"`
}

var _ = Describe("split brain one leader", func() {
	var (
		etcdManifest     string
		etcdManifestName string

		etcdClient etcdclient.Client

		etcd0IP string
		etcd1IP string
		etcd2IP string

		leaderIndex   int
		followerIndex int
	)

	BeforeEach(func() {
		deploymentSuffix := "split-brain-one-leader"

		By("deploying a 3 node etcd", func() {
			var err error

			etcdManifest, err = helpers.NewEtcdManifestWithInstanceCount(deploymentSuffix, 3, true, boshClient)
			Expect(err).NotTo(HaveOccurred())

			etcdManifestName, err = ops.ManifestName(etcdManifest)
			Expect(err).NotTo(HaveOccurred())

			etcdManifest, err = ops.ApplyOps(etcdManifest, []ops.Op{
				{
					Type: "replace",
					Path: "/instance_groups/name=etcd?/jobs/-",
					Value: map[string]string{
						"name":    "iptables_agent",
						"release": "etcd",
					},
				},
				{
					Type: "replace",
					Path: "/instance_groups/name=etcd?/jobs/-",
					Value: map[string]string{
						"name":    "monit_agent",
						"release": "etcd",
					},
				},
			})
			Expect(err).NotTo(HaveOccurred())

			_, err = boshClient.Deploy([]byte(etcdManifest))
			Expect(err).NotTo(HaveOccurred())

			Eventually(func() ([]bosh.VM, error) {
				return helpers.DeploymentVMs(boshClient, etcdManifestName)
			}, "1m", "10s").Should(ConsistOf(helpers.GetVMsFromManifest(etcdManifest)))

			testConsumerIPs, err := helpers.GetVMIPs(boshClient, etcdManifestName, "testconsumer")
			Expect(err).NotTo(HaveOccurred())

			etcdClient = etcdclient.NewClient(fmt.Sprintf("http://%s:6769", testConsumerIPs[0]))
		})

		By("isolating etcd/1", func() {
			var err error
			etcd0IP, err = helpers.GetVMIPByIndex(boshClient, etcdManifestName, "etcd", 0)
			Expect(err).NotTo(HaveOccurred())
			etcd1IP, err = helpers.GetVMIPByIndex(boshClient, etcdManifestName, "etcd", 1)
			Expect(err).NotTo(HaveOccurred())
			etcd2IP, err = helpers.GetVMIPByIndex(boshClient, etcdManifestName, "etcd", 2)
			Expect(err).NotTo(HaveOccurred())

			err = blockEtcdTraffic(etcd1IP, []string{etcd0IP, etcd2IP})
			Expect(err).NotTo(HaveOccurred())
		})

		By("restarting etcd/1 to cause a split brain", func() {
			err := boshClient.Restart(etcdManifestName, "etcd", 1)
			Expect(err).NotTo(HaveOccurred())
		})

		By("unblock traffic to and from etcd/1", func() {
			err := unblockEtcdTraffic(etcd1IP, []string{etcd0IP, etcd2IP})
			Expect(err).NotTo(HaveOccurred())
		})

		By("blocking traffic between leader and follower in the 2-member cluster", func() {
			err := blockEtcdTraffic(etcd0IP, []string{etcd2IP})
			Expect(err).NotTo(HaveOccurred())
		})

		By("stopping the consul agent on the follower in the 2-member cluster", func() {
			leaderName, err := etcdClient.LeaderByNodeURL(fmt.Sprintf("https://etcd-%s.etcd.service.cf.internal:4001", 0))
			Expect(err).NotTo(HaveOccurred())

			leaderIndex, err = strconv.Atoi(strings.Split(leaderName, "-")[1])

			if leaderIndex == 0 {
				followerIndex = 2
			} else {
				followerIndex = 0
			}

			followerIP, err := helpers.GetVMIPByIndex(boshClient, etcdManifestName, "etcd", followerIndex)
			Expect(err).NotTo(HaveOccurred())

			monit("stop", followerIP, "consul_agent", false)

			Eventually(func() string {
				return monitJobStatus(followerIP, "consul_agent")
			}, "2m", "10s").Should(Equal("not monitored"))
		})

		By("restarting the leader in the 2-member cluster", func() {
			leaderIP, err := helpers.GetVMIPByIndex(boshClient, etcdManifestName, "etcd", leaderIndex)
			Expect(err).NotTo(HaveOccurred())

			monit("stop", leaderIP, "etcd", true)

			Eventually(func() string {
				return monitJobStatus(leaderIP, "etcd")
			}, "2m", "10s").Should(Equal("not monitored"))

			monit("start", leaderIP, "etcd", true)

			Eventually(func() string {
				return monitJobStatus(leaderIP, "etcd")
			}, "2m", "10s").Should(Equal("running"))
		})

		By("unblocking traffic between leader and follower in the 2-member cluster", func() {
			err := unblockEtcdTraffic(etcd0IP, []string{etcd2IP})
			Expect(err).NotTo(HaveOccurred())
		})

		By("starting the consul agent on the follower in the 2-member cluster", func() {
			followerIP, err := helpers.GetVMIPByIndex(boshClient, etcdManifestName, "etcd", followerIndex)
			Expect(err).NotTo(HaveOccurred())

			monit("start", followerIP, "consul_agent", false)

			Eventually(func() string {
				return monitJobStatus(followerIP, "consul_agent")
			}, "2m", "10s").Should(Equal("running"))
		})
	})

	AfterEach(func() {
		By("deleting the deployment", func() {
			if !CurrentGinkgoTestDescription().Failed {
				err := boshClient.DeleteDeployment(etcdManifestName)
				Expect(err).NotTo(HaveOccurred())
			}
		})
	})

	It("bubbles up a failure from consistency checker to bosh", func() {
		vms := helpers.GetVMsFromManifest(etcdManifest)
		vmsWithFailingEtcdVMs := []bosh.VM{}
		for _, vm := range vms {
			if vm.JobName == "etcd" {
				vm.State = "failing"
			}
			vmsWithFailingEtcdVMs = append(vmsWithFailingEtcdVMs, vm)
		}

		Eventually(func() ([]bosh.VM, error) {
			vms, err := helpers.DeploymentVMs(boshClient, etcdManifestName)
			if err != nil {
				return []bosh.VM{}, err
			}
			return vms, nil
		}, "10m", "1m").Should(ConsistOf(vmsWithFailingEtcdVMs))
	})
})

func monit(command, machineIP, job string, deleteStoreDir bool) {
	resp, err := http.Get(fmt.Sprintf("http://%s:6789/%s?job=%s&delete_store=%t", machineIP, command, job, deleteStoreDir))
	Expect(err).NotTo(HaveOccurred())
	defer resp.Body.Close()

	Expect(resp.StatusCode).To(Equal(http.StatusOK))
}

func monitJobStatus(machineIP, job string) string {
	resp, err := http.Get(fmt.Sprintf("http://%s:6789/job_status?job=%s", machineIP, job))
	Expect(err).NotTo(HaveOccurred())
	defer resp.Body.Close()

	Expect(resp.StatusCode).To(Equal(http.StatusOK))

	body, err := ioutil.ReadAll(resp.Body)
	Expect(err).NotTo(HaveOccurred())

	return string(body)
}
