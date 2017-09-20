package helpersinternal_test

import (
	"fmt"
	"os/exec"
	"time"

	"github.com/cloudfoundry-incubator/cf-test-helpers/commandstarter"
	"github.com/cloudfoundry-incubator/cf-test-helpers/helpers/internal"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/gexec"
)

type fakeStarter struct {
	calledWith struct {
		executable string
		args       []string
	}
	toReturn struct {
		output    string
		err       error
		exitCode  int
		sleepTime int
	}
}

func (s *fakeStarter) Start(reporter commandstarter.Reporter, executable string, args ...string) (*gexec.Session, error) {
	s.calledWith.executable = executable
	s.calledWith.args = args

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

var _ = Describe("Curl", func() {
	var cmdTimeout time.Duration
	BeforeEach(func() {
		cmdTimeout = 30 * time.Second
	})

	It("outputs the body of the given URL", func() {
		starter := new(fakeStarter)
		starter.toReturn.output = "HTTP/1.1 200 OK"

		session := helpersinternal.Curl(starter, false, "-I", "http://example.com")

		session.Wait(cmdTimeout)
		Expect(session).To(gexec.Exit(0))
		Expect(session.Out).To(Say("HTTP/1.1 200 OK"))
		Expect(starter.calledWith.executable).To(Equal("curl"))
		Expect(starter.calledWith.args).To(ConsistOf("-I", "-s", "http://example.com"))
	})

	It("panics when the starter returns an error", func() {
		starter := new(fakeStarter)
		starter.toReturn.err = fmt.Errorf("error")

		Expect(func() {
			helpersinternal.Curl(starter, false, "-I", "http://example.com")
		}).To(Panic())

	})
})
