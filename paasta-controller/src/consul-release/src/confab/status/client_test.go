package status_test

import (
	"errors"

	"github.com/cloudfoundry-incubator/consul-release/src/confab/fakes"
	"github.com/cloudfoundry-incubator/consul-release/src/confab/status"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Client", func() {
	var (
		consulAPIStatus *fakes.FakeconsulAPIStatus
		client          status.Client
	)

	BeforeEach(func() {
		consulAPIStatus = &fakes.FakeconsulAPIStatus{}
		client = status.Client{
			ConsulAPIStatus: consulAPIStatus,
		}
	})

	Describe("Leader", func() {
		It("returns the current cluster leader", func() {
			consulAPIStatus.LeaderCall.Returns.Leader = "some-leader"

			leader, err := client.Leader()
			Expect(err).NotTo(HaveOccurred())
			Expect(leader).To(Equal("some-leader"))
		})

		Context("failure case", func() {
			It("returns an error when leader call fails", func() {
				consulAPIStatus.LeaderCall.Returns.Error = errors.New("some error occurred")

				_, err := client.Leader()
				Expect(err).To(MatchError("some error occurred"))
			})
		})
	})
})
