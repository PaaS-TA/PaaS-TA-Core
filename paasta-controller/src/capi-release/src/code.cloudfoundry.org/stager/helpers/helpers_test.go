package helpers_test

import (
	"code.cloudfoundry.org/stager/helpers"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Stager helpers", func() {

	Describe("BuildDockerStagingData", func() {

		It("builds the correct json", func() {
			lifecycleData, err := helpers.BuildDockerStagingData("cloudfoundry/diego-docker-app")
			Expect(err).NotTo(HaveOccurred())

			json := []byte(*lifecycleData)
			Expect(json).To(MatchJSON(`{"docker_image":"cloudfoundry/diego-docker-app"}`))
		})
	})

})
