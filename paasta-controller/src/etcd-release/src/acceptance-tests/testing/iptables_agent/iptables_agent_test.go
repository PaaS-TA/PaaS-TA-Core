package main_test

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"os/exec"
	"strings"

	"github.com/onsi/gomega/gexec"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("IPTablesAgent", func() {
	var (
		agentPort string
	)

	BeforeEach(func() {
		var err error
		agentPort, err = openPort()
		Expect(err).NotTo(HaveOccurred())
	})

	Context("when /drop?addr={ip} is called", func() {
		It("applies iptables drop output request", func() {
			command := exec.Command(pathToIPTablesAgent, "--port", agentPort, "--iptablesCommand", "echo")

			_, err := gexec.Start(command, GinkgoWriter, GinkgoWriter)
			Expect(err).NotTo(HaveOccurred())

			waitForServerToStart(agentPort)

			req, err := http.NewRequest("PUT", fmt.Sprintf("http://localhost:%s/drop?addr=some-ip-addr", agentPort), strings.NewReader(""))
			Expect(err).NotTo(HaveOccurred())

			resp, err := http.DefaultClient.Do(req)
			Expect(err).NotTo(HaveOccurred())

			respContents, err := ioutil.ReadAll(resp.Body)
			Expect(err).NotTo(HaveOccurred())
			Expect(respContents).To(BeEmpty())

			Expect(resp.StatusCode).To(Equal(http.StatusOK))
		})
	})
})
