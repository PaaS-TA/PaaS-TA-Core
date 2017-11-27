package loggregator_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"testing"
)

func TestCompatability(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Compatability Suite")
}
