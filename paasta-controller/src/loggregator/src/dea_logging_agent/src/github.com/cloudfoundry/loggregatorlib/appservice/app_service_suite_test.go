package appservice_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"testing"
)

func TestAppService(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "AppService Suite")
}
