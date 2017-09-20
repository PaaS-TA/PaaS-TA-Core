package leaderfinder_test

import (
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func TestLeaderfinder(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "etcd-proxy/leaderfinder")
}
