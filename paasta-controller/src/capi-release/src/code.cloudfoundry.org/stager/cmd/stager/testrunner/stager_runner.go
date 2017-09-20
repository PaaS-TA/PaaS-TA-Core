package testrunner

import (
	"os/exec"
	"time"

	"github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"
)

type StagerRunner struct {
	Config      Config
	CompilerUrl string
	session     *gexec.Session
}

type Config struct {
	StagerBin          string
	ListenAddress      string
	TaskCallbackURL    string
	BBSURL             string
	CCBaseURL          string
	DockerStagingStack string
	ConsulCluster      string
}

func New(config Config) *StagerRunner {
	return &StagerRunner{
		Config: config,
	}
}

func (r *StagerRunner) Start(args ...string) {
	if r.session != nil {
		panic("starting more than one stager runner!!!")
	}

	stagerSession, err := gexec.Start(
		exec.Command(
			r.Config.StagerBin,
			append([]string{
				"-bbsAddress", r.Config.BBSURL,
				"-listenAddress", r.Config.ListenAddress,
				"-stagingTaskCallbackURL", r.Config.TaskCallbackURL,
				"-ccBaseURL", r.Config.CCBaseURL,
				"-dockerStagingStack", r.Config.DockerStagingStack,
				"-consulCluster", r.Config.ConsulCluster,
			}, args...)...,
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
