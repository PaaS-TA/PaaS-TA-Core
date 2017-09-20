package haproxy_test

import (
	"code.cloudfoundry.org/cf-tcp-router/configurer/haproxy"
	"code.cloudfoundry.org/cf-tcp-router/models"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("HaproxyConfiguration", func() {
	Describe("BackendServerInfoToHaProxyConfig", func() {
		Context("when configuration is valid", func() {
			It("returns a valid haproxy configuration representation", func() {
				bs := models.BackendServerInfo{Address: "some-ip", Port: 1234}
				str, err := haproxy.BackendServerInfoToHaProxyConfig(bs)
				Expect(err).ShouldNot(HaveOccurred())
				Expect(str).Should(Equal("server server_some-ip_1234 some-ip:1234\n"))
			})
		})

		Context("when configuration is invalid", func() {
			Context("when address is empty", func() {
				It("returns an error", func() {
					bs := models.BackendServerInfo{Address: "", Port: 1234}
					_, err := haproxy.BackendServerInfoToHaProxyConfig(bs)
					Expect(err).Should(HaveOccurred())
					Expect(err.Error()).To(ContainSubstring("backend_server.address"))
				})
			})

			Context("when port is invalid", func() {
				It("returns an error", func() {
					bs := models.BackendServerInfo{Address: "some-ip", Port: 0}
					_, err := haproxy.BackendServerInfoToHaProxyConfig(bs)
					Expect(err).Should(HaveOccurred())
					Expect(err.Error()).To(ContainSubstring("backend_server.port"))
				})
			})
		})
	})

	Describe("RoutingTableEntryToHaProxyConfig", func() {
		Context("when configuration is valid", func() {
			Context("when single backend server info is provided", func() {
				It("returns a valid haproxy configuration representation", func() {
					routingKey := models.RoutingKey{Port: 8880}
					routingTableEntry := models.RoutingTableEntry{
						Backends: map[models.BackendServerKey]models.BackendServerDetails{
							models.BackendServerKey{Address: "some-ip", Port: 1234}: models.BackendServerDetails{},
						},
					}
					str, err := haproxy.RoutingTableEntryToHaProxyConfig(routingKey, routingTableEntry)
					Expect(err).ShouldNot(HaveOccurred())
					Expect(str).Should(Equal("listen listen_cfg_8880\n  mode tcp\n  bind :8880\n  server server_some-ip_1234 some-ip:1234\n"))
				})
			})

			Context("when multiple backend server infos are provided", func() {
				It("returns a valid haproxy configuration representation", func() {
					routingKey := models.RoutingKey{Port: 8880}
					routingTableEntry := models.RoutingTableEntry{
						Backends: map[models.BackendServerKey]models.BackendServerDetails{
							models.BackendServerKey{Address: "some-ip-1", Port: 1234}: models.BackendServerDetails{},
							models.BackendServerKey{Address: "some-ip-2", Port: 1235}: models.BackendServerDetails{},
						},
					}
					str, err := haproxy.RoutingTableEntryToHaProxyConfig(routingKey, routingTableEntry)
					Expect(err).ShouldNot(HaveOccurred())
					Expect(str).Should(ContainSubstring("listen listen_cfg_8880\n  mode tcp\n  bind :8880\n"))
					Expect(str).Should(ContainSubstring("server server_some-ip-1_1234 some-ip-1:1234\n"))
					Expect(str).Should(ContainSubstring("server server_some-ip-2_1235 some-ip-2:1235\n"))
				})
			})
		})

		Context("when configuration is invalid", func() {
			Context("when front end port is invalid", func() {
				It("returns an error", func() {
					routingKey := models.RoutingKey{Port: 0}
					routingTableEntry := models.RoutingTableEntry{
						Backends: map[models.BackendServerKey]models.BackendServerDetails{
							models.BackendServerKey{Address: "some-ip", Port: 1234}: models.BackendServerDetails{},
						},
					}
					_, err := haproxy.RoutingTableEntryToHaProxyConfig(routingKey, routingTableEntry)
					Expect(err).Should(HaveOccurred())
					Expect(err.Error()).To(ContainSubstring("listen_configuration.port"))
				})
			})

			Context("when backend server is invalid", func() {
				It("returns an error", func() {
					routingKey := models.RoutingKey{Port: 8080}
					routingTableEntry := models.RoutingTableEntry{
						Backends: map[models.BackendServerKey]models.BackendServerDetails{
							models.BackendServerKey{Address: "", Port: 1234}: models.BackendServerDetails{},
						},
					}
					_, err := haproxy.RoutingTableEntryToHaProxyConfig(routingKey, routingTableEntry)
					Expect(err).Should(HaveOccurred())
					Expect(err.Error()).To(ContainSubstring("backend_server.address"))
				})
			})

			Context("when no backend servers are provided", func() {
				It("returns an error", func() {
					routingKey := models.RoutingKey{Port: 8080}
					routingTableEntry := models.RoutingTableEntry{
						Backends: map[models.BackendServerKey]models.BackendServerDetails{},
					}
					_, err := haproxy.RoutingTableEntryToHaProxyConfig(routingKey, routingTableEntry)
					Expect(err).Should(HaveOccurred())
					Expect(err.Error()).To(ContainSubstring("listen_configuration.backends"))
				})
			})
		})
	})
})
