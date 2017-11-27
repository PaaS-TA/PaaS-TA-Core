package durationjson_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"testing"
)

func TestDurationjson(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Durationjson Suite")
}
