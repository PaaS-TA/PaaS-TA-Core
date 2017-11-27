package main_test

import (
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"
)

var healthCheck string

func TestHealthCheck(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "HealthCheck Suite")
}

var _ = SynchronizedBeforeSuite(func() []byte {
	healthCheckPath := buildHealthCheck()
	return []byte(healthCheckPath)
}, func(healthCheckPath []byte) {
	healthCheck = string(healthCheckPath)
})

var _ = SynchronizedAfterSuite(func() {
	//noop
}, func() {
	gexec.CleanupBuildArtifacts()
})
