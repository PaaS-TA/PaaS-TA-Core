package consul_test

import (
	"github.com/pivotal-cf-experimental/destiny/consul"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("ConfigV2", func() {
	Context("PopulateDefaultConfigNodes", func() {
		It("overrides the nodes count to 1 if there are no nodes set", func() {
			config := consul.ConfigV2{
				AZs: []consul.ConfigAZ{
					{Name: "az1"},
				},
			}
			config.PopulateDefaultConfigNodes()
			Expect(config.AZs[0].Nodes).To(Equal(1))
		})
	})
})
