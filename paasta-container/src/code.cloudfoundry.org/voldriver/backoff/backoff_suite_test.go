package backoff

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"testing"
)

func TestBackoff(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Backoff Suite")
}

var _ = SynchronizedBeforeSuite(func() []byte {
	return nil
}, func(pathsByte []byte) {

})

var _ = SynchronizedAfterSuite(func() {

}, func() {
})
