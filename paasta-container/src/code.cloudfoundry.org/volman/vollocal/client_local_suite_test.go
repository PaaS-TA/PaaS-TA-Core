package vollocal_test

import (
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"strings"
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"code.cloudfoundry.org/volman"
	"github.com/onsi/gomega/gexec"
	"github.com/tedsuo/ifrit"
	"github.com/tedsuo/ifrit/ginkgomon"
)

var client volman.Manager

var defaultPluginsDirectory string
var secondPluginsDirectory string

var localDriverPath string
var localDriverServerPort int
var debugServerAddress string
var localDriverProcess ifrit.Process
var localDriverRunner *ginkgomon.Runner

var tmpDriversPath string

func TestDriver(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Volman Local Client Suite")
}

var _ = SynchronizedBeforeSuite(func() []byte {
	var err error

	localDriverPath, err = gexec.Build("code.cloudfoundry.org/localdriver/cmd/localdriver", "-race")
	Expect(err).NotTo(HaveOccurred())
	return []byte(localDriverPath)
}, func(pathsByte []byte) {
	path := string(pathsByte)
	localDriverPath = strings.Split(path, ",")[0]
})

var _ = BeforeEach(func() {
	var err error
	tmpDriversPath, err = ioutil.TempDir("", "driversPath")
	Expect(err).NotTo(HaveOccurred())

	defaultPluginsDirectory, err = ioutil.TempDir(os.TempDir(), "clienttest")
	Expect(err).ShouldNot(HaveOccurred())

	secondPluginsDirectory, err = ioutil.TempDir(os.TempDir(), "clienttest2")
	Expect(err).ShouldNot(HaveOccurred())

	localDriverServerPort = 9750 + GinkgoParallelNode()

	debugServerAddress = fmt.Sprintf("0.0.0.0:%d", 9850+GinkgoParallelNode())
	localDriverRunner = ginkgomon.New(ginkgomon.Config{
		Name: "local-driver",
		Command: exec.Command(
			localDriverPath,
			"-listenAddr", fmt.Sprintf("0.0.0.0:%d", localDriverServerPort),
			"-debugAddr", debugServerAddress,
			"-driversPath", defaultPluginsDirectory,
		),
		StartCheck: "local-driver-server.started",
	})
})

var _ = AfterEach(func() {
	ginkgomon.Kill(localDriverProcess)
})

var _ = SynchronizedAfterSuite(func() {

}, func() {
	gexec.CleanupBuildArtifacts()
})

// testing support types:

type errCloser struct{ io.Reader }

func (errCloser) Close() error                     { return nil }
func (errCloser) Read(p []byte) (n int, err error) { return 0, fmt.Errorf("any") }

type stringCloser struct{ io.Reader }

func (stringCloser) Close() error { return nil }
