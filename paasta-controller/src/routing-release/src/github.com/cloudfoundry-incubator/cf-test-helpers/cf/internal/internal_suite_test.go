package cfinternal_test

import (
	"bytes"
	"fmt"
	"os/exec"
	"time"

	"github.com/cloudfoundry-incubator/cf-test-helpers/commandstarter"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"

	"testing"
)

type fakeCmdStarter struct {
	calledWith struct {
		executable string
		args       []string
		reporter   commandstarter.Reporter
	}
	toReturn struct {
		output    string
		err       error
		exitCode  int
		sleepTime int
	}
}

type fakeReporter struct {
	calledWith struct {
		startTime time.Time
		cmd       *exec.Cmd
	}
	outputBuffer *bytes.Buffer
}

func (s *fakeCmdStarter) Start(reporter commandstarter.Reporter, executable string, args ...string) (*gexec.Session, error) {
	s.calledWith.executable = executable
	s.calledWith.args = args
	s.calledWith.reporter = reporter

	// Default return values
	if s.toReturn.output == "" {
		s.toReturn.output = `\{\}`
	}

	reporter.Report(time.Now(), exec.Command(executable, args...))
	cmd := exec.Command(
		"bash",
		"-c",
		fmt.Sprintf(
			"echo %s; sleep %d; exit %d",
			s.toReturn.output,
			s.toReturn.sleepTime,
			s.toReturn.exitCode,
		),
	)
	session, _ := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
	return session, s.toReturn.err
}

func (r *fakeReporter) Report(startTime time.Time, cmd *exec.Cmd) {
	r.calledWith.startTime = startTime
	r.calledWith.cmd = cmd

	fmt.Fprintf(r.outputBuffer, "Reporter reporting for duty")
}

func TestInternal(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "CF Internal Suite")
}
