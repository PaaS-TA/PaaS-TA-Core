package main_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"
	"github.com/onsi/gomega/ghttp"

	"testing"
)

var (
	binaryPath string
	fakeDriver *ghttp.Server
)

func TestVolman(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Volman Suite")
}

var _ = SynchronizedBeforeSuite(func() []byte {
	var err error
	binaryPath, err = gexec.Build("code.cloudfoundry.org/volman/cmd/volman", "-race")
	Expect(err).NotTo(HaveOccurred())

	return []byte(binaryPath)
}, func(bytes []byte) {
	binaryPath = string(bytes)
})

var _ = BeforeEach(func() {
	fakeDriver = ghttp.NewServer()
	fakeDriver.AllowUnhandledRequests = true
})
