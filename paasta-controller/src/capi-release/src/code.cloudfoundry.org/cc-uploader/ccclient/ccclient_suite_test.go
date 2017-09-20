package ccclient_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"testing"
)

func TestCcclient(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Ccclient Suite")
}
