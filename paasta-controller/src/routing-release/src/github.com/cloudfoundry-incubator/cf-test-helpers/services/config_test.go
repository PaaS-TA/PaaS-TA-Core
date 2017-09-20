package services_test

import (
	"github.com/cloudfoundry-incubator/cf-test-helpers/services"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("ValidateConfig", func() {
	Context("with a valid config", func() {
		It("returns no error", func() {
			validConfig := services.Config{
				AppsDomain:    "bosh-lite.com",
				ApiEndpoint:   "api.bosh-lite.com",
				AdminUser:     "admin",
				AdminPassword: "admin",
			}

			err := services.ValidateConfig(&validConfig)
			Expect(err).NotTo(HaveOccurred())
		})
	})

	Context("with an invalid config", func() {
		It("returns an error if ApiEndpoint not set", func() {
			invalidConfig := services.Config{
				AppsDomain:    "bosh-lite.com",
				AdminUser:     "admin",
				AdminPassword: "admin",
			}

			err := services.ValidateConfig(&invalidConfig)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(MatchRegexp(`Field 'api' must not be empty`))
		})

		It("returns an error if AdminUser not set", func() {
			invalidConfig := services.Config{
				AppsDomain:    "bosh-lite.com",
				ApiEndpoint:   "api.bosh-lite.com",
				AdminPassword: "admin",
			}

			err := services.ValidateConfig(&invalidConfig)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(MatchRegexp(`Field 'admin_user' must not be empty`))
		})

		It("returns an error if AdminPassword not set", func() {
			invalidConfig := services.Config{
				AppsDomain:  "bosh-lite.com",
				ApiEndpoint: "api.bosh-lite.com",
				AdminUser:   "admin",
			}

			err := services.ValidateConfig(&invalidConfig)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(MatchRegexp(`Field 'admin_password' must not be empty`))
		})

		It("returns an error if custom SpaceName given without a custom OrgName", func() {
			invalidConfig := services.Config{
				AppsDomain:    "bosh-lite.com",
				ApiEndpoint:   "api.bosh-lite.com",
				AdminUser:     "admin",
				AdminPassword: "admin",
				SpaceName:     "my-cool-space",
			}

			err := services.ValidateConfig(&invalidConfig)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(MatchRegexp(`Field 'space_name' cannot be set unless 'org_name' is also set`))
		})
	})
})
