package evacuation_context_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"testing"
)

func TestEvacuationContext(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "EvacuationContext Suite")
}
