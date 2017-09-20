package helpers_test

import (
	"acceptance-tests/testing/helpers"
	"io/ioutil"

	"gopkg.in/yaml.v2"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/pivotal-cf-experimental/gomegamatchers"
)

var _ = Describe("Manifest", func() {

	Context("when marshalling and un-marshalling the cf manifest", func() {
		It("does not lose information", func() {
			manifest := helpers.Manifest{}
			manifestContent, err := ioutil.ReadFile("fixtures/non-tls-cf-manifest.yml")
			Expect(err).NotTo(HaveOccurred())
			err = yaml.Unmarshal(manifestContent, &manifest)
			Expect(err).NotTo(HaveOccurred())

			marshaledContent, err := yaml.Marshal(manifest)
			Expect(err).NotTo(HaveOccurred())

			Expect(marshaledContent).To(gomegamatchers.MatchYAML(manifestContent))
		})
	})

	Context("when marshalling and un-marshalling the diego manifest", func() {
		It("does not lose information", func() {
			manifest := helpers.Manifest{}
			manifestContent, err := ioutil.ReadFile("fixtures/non-tls-diego-manifest.yml")
			Expect(err).NotTo(HaveOccurred())
			err = yaml.Unmarshal(manifestContent, &manifest)
			Expect(err).NotTo(HaveOccurred())

			marshaledContent, err := yaml.Marshal(manifest)
			Expect(err).NotTo(HaveOccurred())

			Expect(marshaledContent).To(gomegamatchers.MatchYAML(manifestContent))
		})

	})
})
