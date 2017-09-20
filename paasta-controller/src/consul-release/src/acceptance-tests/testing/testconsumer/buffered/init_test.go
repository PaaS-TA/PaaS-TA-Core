package buffered_test

import (
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func TestBuffered(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "testing/testconsumer/buffered")
}
