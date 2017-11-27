package deploy_test

import (
	"fmt"
	"io/ioutil"
	"math/rand"
	"net/http"
	"strings"

	etcdclient "github.com/cloudfoundry-incubator/etcd-release/src/acceptance-tests/testing/etcd"

	"github.com/cloudfoundry-incubator/etcd-release/src/acceptance-tests/testing/helpers"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	ginkgoconfig "github.com/onsi/ginkgo/config"

	"github.com/pivotal-cf-experimental/bosh-test/bosh"
	"github.com/pivotal-cf-experimental/destiny/ops"
)

var _ = Describe("consistency checker", func() {
	ConsistencyCheckerTest := func(enableSSL bool) {
		It("checks if etcd consistency checker reports failures to bosh during split brain", func() {
			var (
				manifest     string
				manifestName string

				partitionedJobIndex int
				partitionedJobIP    string
				otherJobIPs         []string

				etcdClient etcdclient.Client
			)

			By("deploying etcd cluster", func() {
				deploymentName := "consistency-checker-non-tls"
				if enableSSL {
					deploymentName = "consistency-checker-tls"
				}

				var err error
				manifest, err = helpers.NewEtcdManifestWithInstanceCount(deploymentName, 3, enableSSL, boshClient)
				Expect(err).NotTo(HaveOccurred())

				manifestName, err = ops.ManifestName(manifest)
				Expect(err).NotTo(HaveOccurred())

				manifest, err = ops.ApplyOp(manifest, ops.Op{
					Type: "replace",
					Path: "/instance_groups/name=etcd/jobs/-",
					Value: map[string]string{
						"name":    "iptables_agent",
						"release": "etcd",
					},
				})
				Expect(err).NotTo(HaveOccurred())

				_, err = boshClient.Deploy([]byte(manifest))
				Expect(err).NotTo(HaveOccurred())

				testConsumerIPs, err := helpers.GetVMIPs(boshClient, manifestName, "testconsumer")
				Expect(err).NotTo(HaveOccurred())

				etcdClient = etcdclient.NewClient(fmt.Sprintf("http://%s:6769", testConsumerIPs[0]))
			})

			By("checking if etcd consistency check reports no split brain", func() {
				Eventually(func() ([]bosh.VM, error) {
					return helpers.DeploymentVMs(boshClient, manifestName)
				}, "1m", "10s").Should(ConsistOf(helpers.GetVMsFromManifest(manifest)))
			})

			By("blocking all network traffic on a random etcd node", func() {
				rand.Seed(ginkgoconfig.GinkgoConfig.RandomSeed)
				partitionedJobIndex = rand.Intn(2) + 1

				vms, err := boshClient.DeploymentVMs(manifestName)
				Expect(err).NotTo(HaveOccurred())

				for _, vm := range vms {
					if vm.JobName != "etcd" {
						continue
					}

					if vm.Index == partitionedJobIndex {
						partitionedJobIP = vm.IPs[0]
					} else {
						otherJobIPs = append(otherJobIPs, vm.IPs[0])
					}
				}

				err = blockEtcdTraffic(partitionedJobIP, otherJobIPs)
				Expect(err).NotTo(HaveOccurred())
			})

			By("restarting the partitioned node", func() {
				err := boshClient.Restart(manifestName, "etcd", partitionedJobIndex)
				Expect(err).NotTo(HaveOccurred())
			})

			By("removing the blockage of traffic on the partitioned node", func() {
				err := unblockEtcdTraffic(partitionedJobIP, otherJobIPs)
				Expect(err).NotTo(HaveOccurred())
			})

			By("checking if etcd consistency check reports a split brain", func() {
				vms := []bosh.VM{}
				for _, vm := range helpers.GetVMsFromManifest(manifest) {
					if vm.JobName == "etcd" {
						vm.State = "failing"
					}
					vms = append(vms, vm)
				}

				Eventually(func() ([]bosh.VM, error) {
					return helpers.DeploymentVMs(boshClient, manifestName)
				}, "5m", "10s").Should(ConsistOf(vms))
			})

			By("deleting the deployment", func() {
				err := boshClient.DeleteDeployment(manifestName)
				Expect(err).NotTo(HaveOccurred())
			})
		})
	}

	Context("without TLS", func() {
		ConsistencyCheckerTest(false)
	})

	Context("with TLS", func() {
		ConsistencyCheckerTest(true)
	})
})

func blockEtcdTraffic(machineIP string, etcdJobIPs []string) error {
	for _, etcdJobIP := range etcdJobIPs {
		req, err := http.NewRequest("PUT", fmt.Sprintf("http://%s:5678/drop?addr=%s", machineIP, etcdJobIP), strings.NewReader(""))
		if err != nil {
			return err
		}

		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			return err
		}

		if resp.StatusCode != http.StatusOK {
			respBody, err := ioutil.ReadAll(resp.Body)
			if err != nil {
				respBody = []byte("could not read body")
			}

			return fmt.Errorf("unexpected status: %d, error: %s", resp.StatusCode, string(respBody))
		}
	}
	return nil
}

func unblockEtcdTraffic(machineIP string, etcdJobIPs []string) error {
	for _, etcdJobIP := range etcdJobIPs {
		req, err := http.NewRequest("DELETE", fmt.Sprintf("http://%s:5678/drop?addr=%s", machineIP, etcdJobIP), strings.NewReader(""))
		if err != nil {
			return err
		}

		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			return err
		}

		if resp.StatusCode != http.StatusOK {
			respBody, err := ioutil.ReadAll(resp.Body)
			if err != nil {
				respBody = []byte("could not read body")
			}

			return fmt.Errorf("unexpected status: %d, error: %s", resp.StatusCode, string(respBody))
		}
	}
	return nil
}
