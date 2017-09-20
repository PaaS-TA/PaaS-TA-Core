package core_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/pivotal-cf-experimental/destiny/core"
)

var _ = Describe("Network", func() {
	var network core.Network

	Describe("StaticIPs", func() {
		BeforeEach(func() {
			network = core.Network{
				Subnets: []core.NetworkSubnet{
					{Static: []string{"10.0.0.1", "10.0.0.2"}},
					{Static: []string{"10.0.0.3", "10.0.0.4"}},
				},
			}
		})

		It("returns the requested number of ips", func() {
			ips := network.StaticIPs(3)

			Expect(ips).To(HaveLen(3))
			Expect(ips).To(ConsistOf([]string{"10.0.0.1", "10.0.0.2", "10.0.0.3"}))
		})

		It("returns an empty slice when there are fewer ips available than requested", func() {
			ips := network.StaticIPs(5)
			Expect(ips).To(HaveLen(0))
		})
	})

	Describe("StaticIPsFromRange", func() {
		BeforeEach(func() {
			network = core.Network{
				Subnets: []core.NetworkSubnet{
					{Static: []string{"10.0.0.1-10.0.0.2"}},
					{Static: []string{"10.0.0.3 - 10.0.0.4"}},
				},
			}
		})

		It("returns the requested number of ips", func() {
			ips, err := network.StaticIPsFromRange(3)
			Expect(err).NotTo(HaveOccurred())

			Expect(ips).To(HaveLen(3))
			Expect(ips).To(ConsistOf([]string{"10.0.0.1", "10.0.0.2", "10.0.0.3"}))
		})

		Context("failure cases", func() {
			It("returns an error when the count is greater than the number of ips in range", func() {
				_, err := network.StaticIPsFromRange(5)
				Expect(err).To(MatchError("can't allocate 5 ips from 4 available ips"))
			})

			It("returns an error when the count is less than zero", func() {
				_, err := network.StaticIPsFromRange(-1)
				Expect(err).To(MatchError("count must be greater than or equal to zero"))
			})

			It("returns an error when subnet's static field is not a range", func() {
				network = core.Network{
					Subnets: []core.NetworkSubnet{
						{Static: []string{"10.0.0.1"}},
					},
				}

				_, err := network.StaticIPsFromRange(5)
				Expect(err).To(MatchError("static ip's must be a range in the form of x.x.x.x-x.x.x.x"))
			})

			It("returns an error when it cannot parse the first ip", func() {
				network = core.Network{
					Subnets: []core.NetworkSubnet{
						{Static: []string{"fake.ip-10.0.0.1"}},
					},
				}

				_, err := network.StaticIPsFromRange(5)
				Expect(err).To(MatchError("'fake.ip' is not a valid ip address"))
			})

			It("returns an error when it cannot parse the second ip", func() {
				network = core.Network{
					Subnets: []core.NetworkSubnet{
						{Static: []string{"10.0.0.1-fake.ip"}},
					},
				}

				_, err := network.StaticIPsFromRange(5)
				Expect(err).To(MatchError("'fake.ip' is not a valid ip address"))
			})
		})
	})
})
