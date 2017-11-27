package operationq_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"testing"
)

func TestOperationq(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Operationq Suite")
}
