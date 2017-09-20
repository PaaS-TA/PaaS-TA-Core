package signer_test

import (
	"github.com/cloudfoundry/blobstore_url_signer/signer"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Signer", func() {

	Context("Sign", func() {
		It("returns a signed URL", func() {
			expires := "2147483647"
			secret := "secret"
			path := "/s/link"

			signer := signer.NewSigner(secret)
			signedUrl := signer.Sign(expires, path)
			Expect(signedUrl).To(Equal("http://blobstore.service.cf.internal/read/s/link?md5=6lMW7aVzFsz2QK8bdgDVaA&expires=2147483647"))
		})
	})

	Context("SanitizeString", func() {
		It("replaces '/' with '_'", func() {
			sanitizedString := signer.SanitizeString("i am /a /string")
			Expect(sanitizedString).To(Equal("i am _a _string"))
		})

		It("replaces '+' with '-'", func() {
			sanitizedString := signer.SanitizeString("i am +a +string")
			Expect(sanitizedString).To(Equal("i am -a -string"))
		})

		It("removes '='", func() {
			sanitizedString := signer.SanitizeString("i am =a =string")
			Expect(sanitizedString).To(Equal("i am a string"))
		})
	})
})
