package lrpstatus_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"testing"
)

func TestLrpstatus(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Lrpstatus Suite")
}
