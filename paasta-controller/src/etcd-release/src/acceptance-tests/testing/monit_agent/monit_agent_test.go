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

var _ = Describe("MonitAgent", func() {
	var (
		agentPort string
	)

	BeforeEach(func() {
		var err error
		agentPort, err = openPort()
		Expect(err).NotTo(HaveOccurred())
	})

	Context("when /start?job={job_name} is called", func() {
		It("calls monit start", func() {
			command := exec.Command(pathToMonitAgent, "--port", agentPort, "--monitCommand", "echo")

			session, err := gexec.Start(command, GinkgoWriter, GinkgoWriter)
			Expect(err).NotTo(HaveOccurred())

			waitForServerToStart(agentPort)

			req, err := http.NewRequest("GET", fmt.Sprintf("http://localhost:%s/start?job=fake_job", agentPort), strings.NewReader(""))
			Expect(err).NotTo(HaveOccurred())

			resp, err := http.DefaultClient.Do(req)
			Expect(err).NotTo(HaveOccurred())

			respContents, err := ioutil.ReadAll(resp.Body)
			Expect(err).NotTo(HaveOccurred())
			Expect(string(respContents)).To(BeEmpty())

			Expect(resp.StatusCode).To(Equal(http.StatusOK))

			Expect(string(session.Out.Contents())).To(ContainSubstring("start fake_job"))
		})
	})

	Context("when /stop?job={job_name} is called", func() {
		It("calls monit stop", func() {
			command := exec.Command(pathToMonitAgent, "--port", agentPort, "--monitCommand", "echo")

			session, err := gexec.Start(command, GinkgoWriter, GinkgoWriter)
			Expect(err).NotTo(HaveOccurred())

			waitForServerToStart(agentPort)

			req, err := http.NewRequest("GET", fmt.Sprintf("http://localhost:%s/stop?job=fake_job", agentPort), strings.NewReader(""))
			Expect(err).NotTo(HaveOccurred())

			resp, err := http.DefaultClient.Do(req)
			Expect(err).NotTo(HaveOccurred())

			respContents, err := ioutil.ReadAll(resp.Body)
			Expect(err).NotTo(HaveOccurred())
			Expect(string(respContents)).To(BeEmpty())

			Expect(resp.StatusCode).To(Equal(http.StatusOK))

			Expect(string(session.Out.Contents())).To(ContainSubstring("stop fake_job"))
		})
	})

	Context("when /restart?job={job_name} is called", func() {
		It("calls monit restart", func() {
			command := exec.Command(pathToMonitAgent, "--port", agentPort, "--monitCommand", "echo")

			session, err := gexec.Start(command, GinkgoWriter, GinkgoWriter)
			Expect(err).NotTo(HaveOccurred())

			waitForServerToStart(agentPort)

			req, err := http.NewRequest("GET", fmt.Sprintf("http://localhost:%s/restart?job=fake_job", agentPort), strings.NewReader(""))
			Expect(err).NotTo(HaveOccurred())

			resp, err := http.DefaultClient.Do(req)
			Expect(err).NotTo(HaveOccurred())

			respContents, err := ioutil.ReadAll(resp.Body)
			Expect(err).NotTo(HaveOccurred())
			Expect(string(respContents)).To(BeEmpty())

			Expect(resp.StatusCode).To(Equal(http.StatusOK))

			Expect(string(session.Out.Contents())).To(ContainSubstring("restart fake_job"))
		})
	})
})
