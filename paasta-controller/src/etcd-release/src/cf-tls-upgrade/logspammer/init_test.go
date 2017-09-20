package logspammer_test

import (
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func TestLogspammer(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "logspammer")
}
