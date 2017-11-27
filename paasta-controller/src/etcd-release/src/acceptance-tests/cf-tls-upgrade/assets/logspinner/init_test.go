package main_test

import (
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"
)

func TestLogspinner(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "logspinner")
}

var (
	pathToLogspinner string
)

var _ = BeforeSuite(func() {
	var err error
	pathToLogspinner, err = gexec.Build("github.com/cloudfoundry-incubator/etcd-release/src/acceptance-tests/cf-tls-upgrade/assets/logspinner")
	Expect(err).NotTo(HaveOccurred())
})
