package turbulence_test

import (
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func TestTurbulence(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "turbulence")
}
