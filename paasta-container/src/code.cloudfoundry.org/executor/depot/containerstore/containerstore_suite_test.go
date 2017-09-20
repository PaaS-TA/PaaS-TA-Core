package containerstore_test

import (
	"code.cloudfoundry.org/lager/lagertest"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"testing"
)

var logger *lagertest.TestLogger

func TestContainerstore(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Containerstore Suite")
}

var _ = BeforeEach(func() {
	logger = lagertest.NewTestLogger("test")
})
