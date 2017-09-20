package core_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/pivotal-cf-experimental/destiny/core"
)

var _ = Describe("IP", func() {
	Describe("ParseIP", func() {
		It("returns an IP object that represents IP from string", func() {
			ip, err := core.ParseIP("10.0.16.255")
			Expect(err).NotTo(HaveOccurred())
			Expect(ip.String()).To(Equal("10.0.16.255"))
		})

		Context("failure cases", func() {
			It("returns an error if it cannot parse ip", func() {
				_, err := core.ParseIP("not valid")
				Expect(err).To(MatchError(ContainSubstring("not a valid ip address")))
			})

			It("returns an error if ip parts are not digits", func() {
				_, err := core.ParseIP("x.x.x.x")
				Expect(err).To(MatchError(ContainSubstring("invalid syntax")))
			})

			It("returns an error if ip parts are out of the allowed range", func() {
				_, err := core.ParseIP("999.999.999.999")
				Expect(err).To(MatchError(ContainSubstring("values out of range")))
			})

			It("returns an error if ip has too many parts", func() {
				_, err := core.ParseIP("1.1.1.1.1.1.1")
				Expect(err).To(MatchError(ContainSubstring("not a valid ip address")))
			})
		})
	})

	Describe("Add", func() {
		It("returns an IP object that represents IP offsetted by 1", func() {
			ip, err := core.ParseIP("10.0.16.1")
			ip = ip.Add(1)
			Expect(err).NotTo(HaveOccurred())
			Expect(ip.String()).To(Equal("10.0.16.2"))
		})
	})

	Describe("Subtract", func() {
		It("returns an IP object that represents IP offsetted by -1", func() {
			ip, err := core.ParseIP("10.0.16.2")
			ip = ip.Subtract(1)
			Expect(err).NotTo(HaveOccurred())
			Expect(ip.String()).To(Equal("10.0.16.1"))
		})
	})

	Describe("To", func() {
		It("returns a list of IPs from current IP to provided IP", func() {
			ip, err := core.ParseIP("10.0.16.1")
			ip2, err := core.ParseIP("10.0.16.5")
			ips := ip.To(ip2)
			Expect(err).NotTo(HaveOccurred())
			Expect(ips).To(Equal([]string{
				"10.0.16.1",
				"10.0.16.2",
				"10.0.16.3",
				"10.0.16.4",
				"10.0.16.5",
			}))
		})
	})

	Describe("String", func() {
		It("returns a string representation of IP object", func() {
			ip, err := core.ParseIP("10.0.16.1")
			Expect(err).NotTo(HaveOccurred())
			Expect(ip.String()).To(Equal("10.0.16.1"))
		})
	})
})
