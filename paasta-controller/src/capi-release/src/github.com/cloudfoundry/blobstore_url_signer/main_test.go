package main_test

import (
	"net"
	"os"
	"os/exec"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gexec"
)

var _ = Describe("main", func() {
	var result *Session
	var unixAddress string = "/tmp/test.sock"

	BeforeEach(func() {
		result = runSigner("-network", "unix", "-laddr", unixAddress)
		time.Sleep(10 * time.Millisecond)
	})

	AfterEach(func() {
		os.Remove(unixAddress)
		result.Kill()
	})

	It("Listens on the network and address supplied by argument", func() {
		_, err := net.Dial("unix", unixAddress)
		Expect(err).ToNot(HaveOccurred())
	})
})

func runSigner(args ...string) *Session {
	path, err := Build("github.com/cloudfoundry/blobstore_url_signer/")
	Expect(err).NotTo(HaveOccurred())

	session, err := Start(exec.Command(path, args...), GinkgoWriter, GinkgoWriter)
	Expect(err).NotTo(HaveOccurred())

	return session
}
