package testrunner

import (
	"encoding/json"
	"io/ioutil"
	"os"
	"os/exec"
	"time"

	"code.cloudfoundry.org/rep/cmd/rep/config"

	"github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/gexec"
)

type Runner struct {
	binPath           string
	Session           *gexec.Session
	StartCheck        string
	repConfig         config.RepConfig
	repConfigFilePath string
}

func New(binPath string, repConfig config.RepConfig) *Runner {
	return &Runner{
		binPath:    binPath,
		StartCheck: "rep.started",
		repConfig:  repConfig,
	}
}

func (r *Runner) Start() {
	if r.Session != nil && r.Session.ExitCode() == -1 {
		panic("starting more than one rep!!!")
	}

	f, err := ioutil.TempFile("", "rep")
	Expect(err).NotTo(HaveOccurred())

	encoder := json.NewEncoder(f)

	err = encoder.Encode(r.repConfig)
	Expect(err).NotTo(HaveOccurred())

	r.repConfigFilePath = f.Name()

	args := []string{
		"--config", r.repConfigFilePath,
	}

	repSession, err := gexec.Start(
		exec.Command(
			r.binPath,
			args...,
		),
		gexec.NewPrefixedWriter("\x1b[32m[o]\x1b[32m[rep]\x1b[0m ", ginkgo.GinkgoWriter),
		gexec.NewPrefixedWriter("\x1b[91m[e]\x1b[32m[rep]\x1b[0m ", ginkgo.GinkgoWriter),
	)

	Expect(err).NotTo(HaveOccurred())
	r.Session = repSession

	Eventually(r.Session.Buffer(), 2).Should(gbytes.Say(r.StartCheck))
}

func (r *Runner) Stop() {
	err := os.RemoveAll(r.repConfigFilePath)
	Expect(err).NotTo(HaveOccurred())

	if r.Session != nil {
		r.Session.Interrupt().Wait(5 * time.Second)
	}
}

func (r *Runner) KillWithFire() {
	err := os.RemoveAll(r.repConfigFilePath)
	Expect(err).NotTo(HaveOccurred())

	if r.Session != nil {
		r.Session.Kill().Wait(5 * time.Second)
	}
}
