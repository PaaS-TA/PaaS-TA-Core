package gomegamatchers_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/pivotal-cf-experimental/gomegamatchers"
)

type animalStringer struct {
	Data string
}

func (a animalStringer) String() string {
	return a.Data
}

var _ = Describe("MatchYAMLMatcher", func() {
	Describe("Match", func() {
		var animals, plants string

		BeforeEach(func() {
			animals = "- cats:\n  - lion\n- fish:\n  - salmon"
			plants = "- tropical:\n  - palm\n- desert:\n  - cactus"
		})

		Context("when arguments are strings", func() {
			It("returns true when the YAML matches", func() {
				isMatch, err := gomegamatchers.MatchYAML(animals).Match(animals)
				Expect(err).NotTo(HaveOccurred())
				Expect(isMatch).To(BeTrue())
			})

			It("returns false when the YAML does not match", func() {
				isMatch, err := gomegamatchers.MatchYAML(animals).Match(plants)
				Expect(err).NotTo(HaveOccurred())
				Expect(isMatch).To(BeFalse())
			})
		})

		Context("when an input is a byte slice", func() {
			var animalBytes []byte

			BeforeEach(func() {
				animalBytes = []byte(animals)
			})

			It("returns true when the YAML matches", func() {
				isMatch, err := gomegamatchers.MatchYAML(animalBytes).Match(animals)
				Expect(err).NotTo(HaveOccurred())
				Expect(isMatch).To(BeTrue())
			})

			It("returns false when the YAML does not match", func() {
				isMatch, err := gomegamatchers.MatchYAML(animalBytes).Match(plants)
				Expect(err).NotTo(HaveOccurred())
				Expect(isMatch).To(BeFalse())
			})
		})

		Context("when an input is a Stringer", func() {
			var stringer animalStringer

			BeforeEach(func() {
				stringer = animalStringer{
					Data: animals,
				}
			})

			It("returns true when the YAML matches", func() {
				isMatch, err := gomegamatchers.MatchYAML(stringer).Match(animals)
				Expect(err).NotTo(HaveOccurred())
				Expect(isMatch).To(BeTrue())
			})

			It("returns false when the YAML does not match", func() {
				isMatch, err := gomegamatchers.MatchYAML(stringer).Match(plants)
				Expect(err).NotTo(HaveOccurred())
				Expect(isMatch).To(BeFalse())
			})
		})

		Describe("errors", func() {
			It("returns an error when one of the inputs is not a string, byte slice, or Stringer", func() {
				_, err := gomegamatchers.MatchYAML(animals).Match(123213)
				Expect(err.Error()).To(ContainSubstring("MatchYAMLMatcher matcher requires a string or stringer."))
				Expect(err.Error()).To(ContainSubstring("Got:\n    <int>: 123213"))
			})

			It("returns an error when the YAML is invalid", func() {
				_, err := gomegamatchers.MatchYAML(animals).Match("some: invalid: yaml")
				Expect(err.Error()).To(ContainSubstring("mapping values are not allowed in this context"))
			})
		})
	})

	Describe("FailureMessage", func() {
		It("returns a failure message", func() {
			actualMessage := gomegamatchers.MatchYAML("a: 1").FailureMessage("b: 2")
			Expect(actualMessage).To(ContainSubstring("Expected"))
			Expect(actualMessage).To(ContainSubstring("<string>: b: 2"))
			Expect(actualMessage).To(ContainSubstring("to match YAML of"))
			Expect(actualMessage).To(ContainSubstring("<string>: a: 1"))
		})
	})

	Describe("NegatedFailureMessage", func() {
		It("returns a negated failure message", func() {
			actualMessage := gomegamatchers.MatchYAML("a: 1").NegatedFailureMessage("b: 2")
			Expect(actualMessage).To(ContainSubstring("Expected"))
			Expect(actualMessage).To(ContainSubstring("<string>: b: 2"))
			Expect(actualMessage).To(ContainSubstring("not to match YAML of"))
			Expect(actualMessage).To(ContainSubstring("<string>: a: 1"))
		})
	})
})
