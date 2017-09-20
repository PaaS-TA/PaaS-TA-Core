package deploy_test

import (
	"acceptance-tests/testing/helpers"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"io/ioutil"
	"math/rand"
	"net/http"
	"strings"

	etcdtc "acceptance-tests/testing/etcd"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	ginkgoconfig "github.com/onsi/ginkgo/config"

	"github.com/pivotal-cf-experimental/bosh-test/bosh"
	"github.com/pivotal-cf-experimental/destiny/etcd"
)

var _ = Describe("consistency checker", func() {
	ConsistencyCheckerTest := func(enableSSL bool) {
		It("checks if etcd consistency checker reports failures to bosh during split brain", func() {
			var (
				etcdManifest        etcd.Manifest
				partitionedJobIndex int
				partitionedJobIP    string
				otherJobIPs         []string
				tcClient            etcdtc.Client
			)

			By("deploying etcd cluster", func() {
				configCopy := config
				configCopy.IPTablesAgent = true
				Expect(config.IPTablesAgent).To(BeFalse())

				var err error
				etcdManifest, err = helpers.DeployEtcdWithInstanceCount("consistency_checker", 3, client, configCopy, enableSSL)
				Expect(err).NotTo(HaveOccurred())

				testConsumerIndex, err := helpers.FindJobIndexByName(etcdManifest, "testconsumer_z1")
				Expect(err).NotTo(HaveOccurred())
				tcClient = etcdtc.NewClient(fmt.Sprintf("http://%s:6769", etcdManifest.Jobs[testConsumerIndex].Networks[0].StaticIPs[0]))
			})

			By("checking if etcd consistency check reports no split brain", func() {
				Eventually(func() ([]bosh.VM, error) {
					return helpers.DeploymentVMs(client, etcdManifest.Name)
				}, "1m", "10s").Should(ConsistOf(helpers.GetVMsFromManifest(etcdManifest)))
			})

			By("blocking all network traffic on a random etcd node", func() {
				rand.Seed(ginkgoconfig.GinkgoConfig.RandomSeed)
				partitionedJobIndex = rand.Intn(2) + 1

				for _, job := range etcdManifest.Jobs {
					if job.Name == "etcd_z1" {
						Expect(job.Networks).To(HaveLen(1))
						Expect(job.Networks[0].StaticIPs).To(HaveLen(3))
						for i, ip := range job.Networks[0].StaticIPs {
							if i == partitionedJobIndex {
								partitionedJobIP = ip
							} else {
								otherJobIPs = append(otherJobIPs, ip)
							}
						}
					}
				}

				err := blockEtcdTraffic(partitionedJobIP, otherJobIPs)
				Expect(err).NotTo(HaveOccurred())
			})

			By("restarting the partitioned node", func() {
				err := client.Restart(etcdManifest.Name, "etcd_z1", partitionedJobIndex)
				Expect(err).NotTo(HaveOccurred())
			})

			By("removing the blockage of traffic on the partitioned node", func() {
				err := unblockEtcdTraffic(partitionedJobIP, otherJobIPs)
				Expect(err).NotTo(HaveOccurred())
			})

			By("checking if etcd consistency check reports a split brain", func() {
				vms := []bosh.VM{}
				for _, vm := range helpers.GetVMsFromManifest(etcdManifest) {
					if vm.JobName == "etcd_z1" {
						vm.State = "failing"
					}
					vms = append(vms, vm)
				}

				Eventually(func() ([]bosh.VM, error) {
					return helpers.DeploymentVMs(client, etcdManifest.Name)
				}, "1m", "10s").Should(ConsistOf(vms))
			})

			By("deleting the deployment", func() {
				err := client.DeleteDeployment(etcdManifest.Name)
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

func etcdTLSClient(manifest etcd.Manifest) (*http.Client, error) {
	cert, err := tls.X509KeyPair([]byte(manifest.Properties.Etcd.ServerCert), []byte(manifest.Properties.Etcd.ServerKey))
	if err != nil {
		return &http.Client{}, err
	}

	caCert := manifest.Properties.Etcd.CACert

	caCertPool := x509.NewCertPool()
	caCertPool.AppendCertsFromPEM([]byte(caCert))

	tlsConfig := &tls.Config{
		Certificates: []tls.Certificate{cert},
		RootCAs:      caCertPool,
	}
	tlsConfig.BuildNameToCertificate()
	transport := &http.Transport{TLSClientConfig: tlsConfig}
	return &http.Client{Transport: transport}, nil
}
