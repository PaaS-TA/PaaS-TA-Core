package main_test

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"
)

var _ = Describe("Agent", func() {
	Describe("start", func() {
		var (
			agentConfigDir string
			session        *gexec.Session
		)

		BeforeEach(func() {
			var err error
			agentConfigDir, err = ioutil.TempDir("", "")
			Expect(err).NotTo(HaveOccurred())
			contents := []byte(`{"ConfigFileName": "fake}`)

			err = ioutil.WriteFile(filepath.Join(agentConfigDir, "options.json"), contents, os.ModePerm)
			Expect(err).NotTo(HaveOccurred())

			err = ioutil.WriteFile(filepath.Join(agentConfigDir, "config.json"), []byte("{}"), os.ModePerm)
			Expect(err).NotTo(HaveOccurred())
		})

		AfterEach(func() {
			session.Kill().Wait()
		})

		It("starts a fake consul agent", func() {
			var err error
			startCmd := exec.Command(pathToFakeAgent,
				"agent",
				"-recursor=1.2.3.4",
				fmt.Sprintf("-config-dir=%s", agentConfigDir),
			)
			session, err = gexec.Start(startCmd, GinkgoWriter, GinkgoWriter)
			Expect(err).NotTo(HaveOccurred())
			Eventually(func() (int, error) {
				resp, err := http.Get("http://localhost:8500/v1/agent/members")
				if err != nil {
					return -1, err
				}
				return resp.StatusCode, nil
			}, "2s", "1s").Should(Equal(200))

			Expect(filepath.Join(agentConfigDir, "fake-output.json")).To(BeAnExistingFile())
		})

		It("can write up to two fake-output.json", func() {
			var err error
			startCmd := exec.Command(pathToFakeAgent,
				"agent",
				"-recursor=1.2.3.4",
				fmt.Sprintf("-config-dir=%s", agentConfigDir),
			)
			session, err = gexec.Start(startCmd, GinkgoWriter, GinkgoWriter)
			Expect(err).NotTo(HaveOccurred())

			Eventually(filepath.Join(agentConfigDir, "fake-output.json")).Should(BeAnExistingFile())

			session.Kill().Wait()

			startCmd = exec.Command(pathToFakeAgent,
				"agent",
				"-recursor=1.2.3.4",
				fmt.Sprintf("-config-dir=%s", agentConfigDir),
			)
			session, err = gexec.Start(startCmd, GinkgoWriter, GinkgoWriter)
			Expect(err).NotTo(HaveOccurred())

			Eventually(filepath.Join(agentConfigDir, "fake-output-2.json")).Should(BeAnExistingFile())

		})
	})
})
