package transformer_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"testing"
)

func TestTransformer(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Transformer Suite")
}
