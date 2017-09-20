package eachers_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"testing"
)

func TestEachers(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Eachers Suite")
}
