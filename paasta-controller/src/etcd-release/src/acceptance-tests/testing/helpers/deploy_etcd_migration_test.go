package helpers_test

import (
	"acceptance-tests/testing/helpers"
	"io/ioutil"

	"github.com/pivotal-cf-experimental/gomegamatchers"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("CreateCFTLSMigrationManifest", func() {
	var (
		nonTLSCFManifest      []byte
		expectedTLSCFManifest []byte
	)

	BeforeEach(func() {
		var err error
		nonTLSCFManifest, err = ioutil.ReadFile("fixtures/non-tls-cf-manifest.yml")
		Expect(err).NotTo(HaveOccurred())

		expectedTLSCFManifest, err = ioutil.ReadFile("fixtures/tls-cf-manifest.yml")
		Expect(err).NotTo(HaveOccurred())
	})

	Context("given a non-tls cf deployment", func() {
		It("generates a deployment entry with tls etcd", func() {
			tlsCFManifestOutput, err := helpers.CreateCFTLSMigrationManifest(nonTLSCFManifest)
			Expect(err).NotTo(HaveOccurred())
			Expect([]byte(tlsCFManifestOutput)).To(gomegamatchers.MatchYAML(expectedTLSCFManifest))
		})
	})

	Context("failure cases", func() {
		It("returns an error when bad yaml is passed in", func() {
			_, err := helpers.CreateCFTLSMigrationManifest([]byte("%%%%%%%"))
			Expect(err).To(MatchError("yaml: could not find expected directive name"))
		})
	})
})
