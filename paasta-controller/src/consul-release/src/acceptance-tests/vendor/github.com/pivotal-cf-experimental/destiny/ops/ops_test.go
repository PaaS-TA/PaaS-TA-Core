package ops_test

import (
	"errors"

	"github.com/pivotal-cf-experimental/destiny/ops"
	"github.com/pivotal-cf-experimental/gomegamatchers"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Ops", func() {
	Describe("ApplyOp", func() {
		It("returns a manifest with a replace op applied", func() {
			manifest := "name: some-name"
			modifiedManifest, err := ops.ApplyOp(manifest, ops.Op{
				Type:  "replace",
				Path:  "/name",
				Value: "some-changed-name",
			})
			Expect(err).NotTo(HaveOccurred())

			Expect(modifiedManifest).To(Equal("name: some-changed-name"))
		})

		It("returns a manifest with a remove op applied", func() {
			manifest := `
---
name: some-name
favorite_color: blue`
			modifiedManifest, err := ops.ApplyOp(manifest, ops.Op{
				Type: "remove",
				Path: "/name",
			})
			Expect(err).NotTo(HaveOccurred())

			Expect(modifiedManifest).To(gomegamatchers.MatchYAML(`
---
favorite_color: blue`))
		})
	})

	Describe("ApplyOps", func() {
		It("returns a manifest with a set of ops changes", func() {
			manifest := `
---
name: some-name
favorite_color: blue`
			modifiedManifest, err := ops.ApplyOps(manifest, []ops.Op{
				{
					Type:  "replace",
					Path:  "/name",
					Value: "some-changed-name",
				},
				{
					Type:  "replace",
					Path:  "/favorite_color",
					Value: "red",
				},
			})
			Expect(err).NotTo(HaveOccurred())

			Expect(modifiedManifest).To(gomegamatchers.MatchYAML(`
---
name: some-changed-name
favorite_color: red`))
		})

		Context("failure cases", func() {
			Context("when apply ops fails to marshal", func() {
				BeforeEach(func() {
					ops.SetMarshal(func(interface{}) ([]byte, error) {
						return []byte{}, errors.New("failed to marshal")
					})
				})

				AfterEach(func() {
					ops.ResetMarshal()
				})

				It("returns an error", func() {
					_, err := ops.ApplyOps("some-manifest", []ops.Op{})
					Expect(err).To(MatchError("failed to marshal"))
				})
			})

			Context("when apply ops fails to unmarshal", func() {
				It("returns an error", func() {
					_, err := ops.ApplyOps("%%%", []ops.Op{})
					Expect(err).To(MatchError("yaml: could not find expected directive name"))
				})
			})

			Context("when the op type is not supported", func() {
				It("returns an error", func() {
					_, err := ops.ApplyOps("some-manifest", []ops.Op{
						{
							Type: "other",
						},
					})
					Expect(err).To(MatchError("op type other not supported by destiny"))
				})
			})

			Context("when the replace op path is bad", func() {
				It("returns an error", func() {
					_, err := ops.ApplyOps("some-manifest", []ops.Op{
						{
							Type: "replace",
							Path: "%%%",
						},
					})
					Expect(err).To(MatchError("Expected to start with '/'"))
				})
			})

			Context("when the remove op path is bad", func() {
				It("returns an error", func() {
					_, err := ops.ApplyOps("some-manifest", []ops.Op{
						{
							Type: "remove",
							Path: "%%%",
						},
					})
					Expect(err).To(MatchError("Expected to start with '/'"))
				})
			})
		})
	})

	Describe("FindOp", func() {
		It("returns a value provided an op path", func() {
			manifest := `name: |-
  some-name
  some-name2`
			name, err := ops.FindOp(manifest, "/name")
			Expect(err).NotTo(HaveOccurred())

			Expect(name.(string)).To(Equal("some-name\nsome-name2"))
		})

		Context("failure cases", func() {
			Context("when find op fails to unmarshal", func() {
				It("returns an error", func() {
					_, err := ops.FindOp("%%%", "")
					Expect(err).To(MatchError("yaml: could not find expected directive name"))
				})
			})

			Context("when the op path is bad", func() {
				It("returns an error", func() {
					_, err := ops.FindOp("some-manifest", "%%%")
					Expect(err).To(MatchError("Expected to start with '/'"))
				})
			})
		})
	})
})
