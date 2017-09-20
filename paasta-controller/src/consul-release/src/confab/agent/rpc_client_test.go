package agent_test

import (
	"github.com/cloudfoundry-incubator/consul-release/src/confab/agent"
	consulagent "github.com/hashicorp/consul/command/agent"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("HandleErrors", func() {
	Context("when there are no errors", func() {
		It("returns nil", func() {
			err := agent.HandleRPCErrors([]consulagent.KeyringInfo{
				{},
				{},
			})
			Expect(err).NotTo(HaveOccurred())
		})
	})

	Context("when there are errors", func() {
		It("returns nil", func() {
			err := agent.HandleRPCErrors([]consulagent.KeyringInfo{
				{},
				{Error: "there was a bad"},
			})
			Expect(err).To(MatchError("there was a bad"))
		})
	})
})
