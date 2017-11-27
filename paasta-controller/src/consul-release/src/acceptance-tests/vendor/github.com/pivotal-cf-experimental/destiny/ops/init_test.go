package ops_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"testing"
)

func TestOps(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "ops")
}
