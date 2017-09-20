package testhelpers_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"testing"
)

func TestTesthelpers(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Testhelpers Suite")
}
