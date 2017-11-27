package dropsonde_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"log"
	"testing"
)

func TestDropsonde(t *testing.T) {
	log.SetOutput(GinkgoWriter)
	RegisterFailHandler(Fail)
	RunSpecs(t, "Dropsonde Suite")
}
