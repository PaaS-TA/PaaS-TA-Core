package syslogchecker_test

import (
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func TestSyslogchecker(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "syslogchecker")
}
