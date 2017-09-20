package nats_emitter_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"testing"
)

func TestNatsEmitter(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "NATS Emitter Suite")
}
