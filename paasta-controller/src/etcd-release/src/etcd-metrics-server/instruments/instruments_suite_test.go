package instruments_test

import (
	"testing"
	"time"

	"code.cloudfoundry.org/cfhttp"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var timeout time.Duration

func TestInstruments(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Instruments Suite")
}

var _ = BeforeSuite(func() {
	timeout = 1 * time.Second
	cfhttp.Initialize(timeout)
})
