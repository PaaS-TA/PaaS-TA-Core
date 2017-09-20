package containermetrics_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"testing"
)

func TestContainerMetrics(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "ContainerMetrics Suite")
}
