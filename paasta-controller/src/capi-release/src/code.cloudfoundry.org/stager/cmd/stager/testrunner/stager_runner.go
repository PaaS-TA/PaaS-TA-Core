package testrunner

import (
	"encoding/json"
	"io/ioutil"
	"os/exec"
	"time"

	"code.cloudfoundry.org/stager/config"

	"github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"
)

type StagerRunner struct {
	Config      config.StagerConfig
	CompilerUrl string
	session     *gexec.Session
}

func New(config config.StagerConfig) *StagerRunner {
	return &StagerRunner{
		Config: config,
	}
}

func (r *StagerRunner) Start(stagerBin string) {
	if r.session != nil {
		panic("starting more than one stager runner!!!")
	}

	stagerFile, err := ioutil.TempFile("", "stager_config")
	Expect(err).NotTo(HaveOccurred())

	stagerJSON, err := json.Marshal(r.Config)
	Expect(err).NotTo(HaveOccurred())
	err = ioutil.WriteFile(stagerFile.Name(), stagerJSON, 0644)
	Expect(err).NotTo(HaveOccurred())

	stagerSession, err := gexec.Start(
		exec.Command(
			stagerBin,
			append([]string{
				"-configPath", stagerFile.Name(),
			})...,
		),
		gexec.NewPrefixedWriter("\x1b[32m[o]\x1b[95m[stager]\x1b[0m ", ginkgo.GinkgoWriter),
		gexec.NewPrefixedWriter("\x1b[91m[e]\x1b[95m[stager]\x1b[0m ", ginkgo.GinkgoWriter),
	)

	Expect(err).NotTo(HaveOccurred())

	r.session = stagerSession
}

func (r *StagerRunner) Stop() {
	if r.session != nil {
		r.session.Interrupt().Wait(5 * time.Second)
		r.session = nil
	}
}

func (r *StagerRunner) KillWithFire() {
	if r.session != nil {
		r.session.Kill().Wait(5 * time.Second)
		r.session = nil
	}
}

func (r *StagerRunner) Session() *gexec.Session {
	return r.session
}
