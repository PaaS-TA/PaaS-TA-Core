package driverhttp_test

import (
	"fmt"
	"io"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"
	"github.com/tedsuo/ifrit"
	"github.com/tedsuo/ifrit/ginkgomon"

	"testing"
)

var debugServerAddress string
var localDriverPath string

var fakedriverServerPort int
var fakedriverProcess ifrit.Process
var tcpRunner *ginkgomon.Runner

func TestDriver(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Driver Remote Client and Handlers Suite")
}

var _ = SynchronizedBeforeSuite(func() []byte {
	var err error

	localDriverPath, err = gexec.Build("code.cloudfoundry.org/localdriver/cmd/localdriver", "-race")
	Expect(err).NotTo(HaveOccurred())
	return []byte(localDriverPath)
}, func(pathsByte []byte) {
	localDriverPath = string(pathsByte)
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
