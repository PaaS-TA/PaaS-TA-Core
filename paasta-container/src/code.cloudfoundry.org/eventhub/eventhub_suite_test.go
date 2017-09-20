package eventhub_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"testing"
)

func TestEventhub(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Eventhub Suite")
}
