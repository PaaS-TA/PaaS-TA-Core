package main_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"testing"
)

func TestStatsdGoClient(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Go Client Suite")
}
