package main_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"

	"testing"
)

func TestRoutingApiCli(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "RoutingApiCli Suite")
}

var path string

var _ = BeforeSuite(func() {
	var err error
	path, err = gexec.Build("code.cloudfoundry.org/routing-api-cli")
	Expect(err).NotTo(HaveOccurred())
})
