package chaperon_test

import (
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"
)

func TestChaperon(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "chaperon")
}

var (
	pathToFakeProcess string
)

var _ = BeforeSuite(func() {
	var err error
	pathToFakeProcess, err = gexec.Build("github.com/cloudfoundry-incubator/consul-release/src/confab/fakes/process")
	Expect(err).NotTo(HaveOccurred())
})

var _ = AfterSuite(func() {
	gexec.CleanupBuildArtifacts()
})
