package volhttp_test

import (
	"io/ioutil"
	"strings"
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"
)

var tmpDriversPath string
var localDriverPath string

func TestHandlers(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Volman Handlers Suite")
}

var _ = SynchronizedBeforeSuite(func() []byte {
	var err error

	localDriverPath, err = gexec.Build("code.cloudfoundry.org/localdriver", "-race")
	Expect(err).NotTo(HaveOccurred())
	return []byte(strings.Join([]string{localDriverPath}, ","))
}, func(pathsByte []byte) {
	path := string(pathsByte)
	localDriverPath = strings.Split(path, ",")[0]
})

var _ = BeforeEach(func() {
	var err error
	tmpDriversPath, err = ioutil.TempDir("", "driversPath")
	Expect(err).NotTo(HaveOccurred())
})

var _ = SynchronizedAfterSuite(func() {

}, func() {
	gexec.CleanupBuildArtifacts()
})
