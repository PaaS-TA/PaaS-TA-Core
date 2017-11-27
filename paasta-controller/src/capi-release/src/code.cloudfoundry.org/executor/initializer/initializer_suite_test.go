package initializer_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"testing"
)

func TestInitializer(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Initializer Suite")
}
