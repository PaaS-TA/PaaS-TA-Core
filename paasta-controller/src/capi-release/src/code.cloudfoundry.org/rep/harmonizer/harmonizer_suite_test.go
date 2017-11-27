package harmonizer_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"testing"
)

func TestHarmonizer(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Harmonizer Suite")
}
