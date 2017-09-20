package cloudconfig_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"testing"
)

func TestCloudConfig(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "cloudconfig")
}
