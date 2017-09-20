package acceptance_test

import (
	"os/exec"
	"testing"

	"github.com/cloudfoundry-incubator/check-a-record/acceptance-test/dnsserver"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"
)

func TestAcceptance(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "acceptance")
}

var (
	pathToCheckARecord string
	dnsServer          dnsserver.Server
)

var _ = BeforeSuite(func() {
	var err error
	pathToCheckARecord, err = gexec.Build("github.com/cloudfoundry-incubator/check-a-record")
	Expect(err).NotTo(HaveOccurred())

	dnsServer = dnsserver.NewServer()
	dnsServer.Start()
})

var _ = AfterSuite(func() {
	err := dnsServer.Stop()
	Expect(err).NotTo(HaveOccurred())

	gexec.CleanupBuildArtifacts()
})

func checkARecord(args []string) *gexec.Session {
	cmd := exec.Command(pathToCheckARecord, args...)
	session, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
	Expect(err).NotTo(HaveOccurred())

	return session
}
