package depot_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"testing"
)

func TestAllocationstore(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Depot Suite")
}
