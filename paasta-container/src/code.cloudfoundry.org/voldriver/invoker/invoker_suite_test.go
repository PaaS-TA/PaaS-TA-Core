package invoker_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"testing"
	"io"
	"fmt"
)

func TestInvoker(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Invoker Suite")
}

var _ = SynchronizedBeforeSuite(func() []byte {
	return nil
}, func(pathsByte []byte) {

})

var _ = SynchronizedAfterSuite(func() {

}, func() {
})


// testing support types:

type errCloser struct{ io.Reader }

func (errCloser) Close() error                     { return nil }
func (errCloser) Read(p []byte) (n int, err error) { return 0, fmt.Errorf("any") }

type stringCloser struct{ io.Reader }

func (stringCloser) Close() error { return nil }