package cc_conv_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"testing"
)

func TestCcConv(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "CcConv Suite")
}
