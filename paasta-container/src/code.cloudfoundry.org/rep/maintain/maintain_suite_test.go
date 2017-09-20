package maintain_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"testing"
)

func TestMaintain(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Maintain Suite")
}
