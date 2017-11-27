package haproxy_test

import (
	"fmt"
	"io/ioutil"
	"os"

	"code.cloudfoundry.org/cf-tcp-router/configurer/haproxy"
	"code.cloudfoundry.org/cf-tcp-router/configurer/haproxy/fakes"
	"code.cloudfoundry.org/cf-tcp-router/models"
	monitorFakes "code.cloudfoundry.org/cf-tcp-router/monitor/fakes"
	"code.cloudfoundry.org/cf-tcp-router/testutil"
	"code.cloudfoundry.org/cf-tcp-router/utils"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("HaproxyConfigurer", func() {
	Describe("Configure", func() {
		const (
			haproxyConfigTemplate = "fixtures/haproxy.cfg.template"
			haproxyConfigFile     = "fixtures/haproxy.cfg"
		)

		var (
			haproxyConfigurer *haproxy.Configurer
			fakeMonitor       *monitorFakes.FakeMonitor
		)

		BeforeEach(func() {
			fakeMonitor = &monitorFakes.FakeMonitor{}
		})

		verifyHaProxyConfigContent := func(haproxyFileName, expectedContent string, present bool) {
			data, err := ioutil.ReadFile(haproxyFileName)
			Expect(err).ShouldNot(HaveOccurred())
			if present {
				Expect(string(data)).Should(ContainSubstring(expectedContent))
			} else {
				Expect(string(data)).ShouldNot(ContainSubstring(expectedContent))
			}
		}

		Context("when empty base configuration file is passed", func() {
			It("returns a ErrRouterConfigFileNotFound error", func() {
				_, err := haproxy.NewHaProxyConfigurer(logger, "", haproxyConfigFile, fakeMonitor, nil)
				Expect(err).Should(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring(haproxy.ErrRouterConfigFileNotFound))
			})
		})

		Context("when empty configuration file is passed", func() {
			It("returns a ErrRouterConfigFileNotFound error", func() {
				_, err := haproxy.NewHaProxyConfigurer(logger, haproxyConfigTemplate, "", fakeMonitor, nil)
				Expect(err).Should(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring(haproxy.ErrRouterConfigFileNotFound))
			})
		})

		Context("when base configuration file does not exist", func() {
			It("returns a ErrRouterConfigFileNotFound error", func() {
				_, err := haproxy.NewHaProxyConfigurer(logger, "file/path/does/not/exist", haproxyConfigFile, fakeMonitor, nil)
				Expect(err).Should(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring(haproxy.ErrRouterConfigFileNotFound))
			})
		})

		Context("when configuration file does not exist", func() {
			It("returns a ErrRouterConfigFileNotFound error", func() {
				_, err := haproxy.NewHaProxyConfigurer(logger, haproxyConfigTemplate, "file/path/does/not/exist", fakeMonitor, nil)
				Expect(err).Should(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring(haproxy.ErrRouterConfigFileNotFound))
			})
		})

		Context("when invalid routing table is passed", func() {
			var (
				haproxyConfigTemplateContent []byte
				generatedHaproxyCfgFile      string
				haproxyCfgBackupFile         string
				err                          error
			)
			BeforeEach(func() {
				generatedHaproxyCfgFile = testutil.RandomFileName("fixtures/haproxy_", ".cfg")
				haproxyCfgBackupFile = fmt.Sprintf("%s.bak", generatedHaproxyCfgFile)
				utils.CopyFile(haproxyConfigTemplate, generatedHaproxyCfgFile)

				haproxyConfigTemplateContent, err = ioutil.ReadFile(generatedHaproxyCfgFile)
				Expect(err).ShouldNot(HaveOccurred())

				haproxyConfigurer, err = haproxy.NewHaProxyConfigurer(logger, haproxyConfigTemplate, generatedHaproxyCfgFile, fakeMonitor, nil)
				Expect(err).ShouldNot(HaveOccurred())
			})

			AfterEach(func() {
				err := os.Remove(generatedHaproxyCfgFile)
				Expect(err).ShouldNot(HaveOccurred())

				Expect(utils.FileExists(haproxyCfgBackupFile)).To(BeTrue())
				err = os.Remove(haproxyCfgBackupFile)
				Expect(err).ShouldNot(HaveOccurred())
			})

			It("doesn't update config file with invalid routing table entry", func() {
				invalidRoutingKey := models.RoutingKey{Port: 0}
				invalidRoutingTableEntry := models.RoutingTableEntry{
					Backends: map[models.BackendServerKey]models.BackendServerDetails{
						models.BackendServerKey{Address: "some-ip-1", Port: 1234}: models.BackendServerDetails{},
					},
				}
				routingTable := models.NewRoutingTable(logger)
				ok := routingTable.Set(invalidRoutingKey, invalidRoutingTableEntry)
				Expect(ok).To(BeTrue())

				routingKey := models.RoutingKey{Port: 80}
				routingTableEntry := models.RoutingTableEntry{
					Backends: map[models.BackendServerKey]models.BackendServerDetails{
						models.BackendServerKey{Address: "some-ip-2", Port: 1234}: models.BackendServerDetails{},
					},
				}
				ok = routingTable.Set(routingKey, routingTableEntry)
				Expect(ok).To(BeTrue())

				err := haproxyConfigurer.Configure(routingTable)
				Expect(err).ShouldNot(HaveOccurred())

				validListenCfg := "\nlisten listen_cfg_80\n  mode tcp\n  bind :80\n"
				validServerConfig := "server server_some-ip-2_1234 some-ip-2:1234"
				verifyHaProxyConfigContent(generatedHaproxyCfgFile, validListenCfg, true)
				verifyHaProxyConfigContent(generatedHaproxyCfgFile, validServerConfig, true)
				verifyHaProxyConfigContent(generatedHaproxyCfgFile, string(haproxyConfigTemplateContent), true)

				invalidListenCfg := "\nlisten listen_cfg_0\n  mode tcp\n  bind :0\n"
				invalidServerConfig := "server server_some-ip-1_1234 some-ip-1:1234"
				verifyHaProxyConfigContent(generatedHaproxyCfgFile, invalidListenCfg, false)
				verifyHaProxyConfigContent(generatedHaproxyCfgFile, invalidServerConfig, false)
			})
		})

		Context("when valid routing table and config file is passed", func() {
			var (
				scriptRunner                 *fakes.FakeScriptRunner
				generatedHaproxyCfgFile      string
				haproxyCfgBackupFile         string
				err                          error
				haproxyConfigTemplateContent []byte
			)

			BeforeEach(func() {
				generatedHaproxyCfgFile = testutil.RandomFileName("fixtures/haproxy_", ".cfg")
				haproxyCfgBackupFile = fmt.Sprintf("%s.bak", generatedHaproxyCfgFile)
				utils.CopyFile(haproxyConfigTemplate, generatedHaproxyCfgFile)

				haproxyConfigTemplateContent, err = ioutil.ReadFile(generatedHaproxyCfgFile)
				Expect(err).ShouldNot(HaveOccurred())

				scriptRunner = &fakes.FakeScriptRunner{}

				haproxyConfigurer, err = haproxy.NewHaProxyConfigurer(logger, haproxyConfigTemplate, generatedHaproxyCfgFile, fakeMonitor, scriptRunner)
				Expect(err).ShouldNot(HaveOccurred())
			})

			AfterEach(func() {
				err := os.Remove(generatedHaproxyCfgFile)
				Expect(err).ShouldNot(HaveOccurred())

				Expect(utils.FileExists(haproxyCfgBackupFile)).To(BeTrue())
				err = os.Remove(haproxyCfgBackupFile)
				Expect(err).ShouldNot(HaveOccurred())
			})

			Context("when Configure is called once", func() {
				Context("when only one mapping is provided as part of request", func() {

					BeforeEach(func() {
						routingTable := models.NewRoutingTable(logger)
						routingTableEntry := models.NewRoutingTableEntry(
							[]models.BackendServerInfo{
								models.BackendServerInfo{Address: "some-ip-1", Port: 1234},
								models.BackendServerInfo{Address: "some-ip-2", Port: 1235},
							},
						)
						routinTableKey := models.RoutingKey{Port: 2222}
						ok := routingTable.Set(routinTableKey, routingTableEntry)
						Expect(ok).To(BeTrue())
						err = haproxyConfigurer.Configure(routingTable)
						Expect(err).ShouldNot(HaveOccurred())
					})

					It("appends the haproxy config with new listen configuration", func() {
						listenCfg :=
							"\nlisten listen_cfg_2222\n  mode tcp\n  bind :2222\n"
						serverConfig1 := "server server_some-ip-1_1234 some-ip-1:1234"
						serverConfig2 := "server server_some-ip-2_1235 some-ip-2:1235"
						verifyHaProxyConfigContent(generatedHaproxyCfgFile, listenCfg, true)
						verifyHaProxyConfigContent(generatedHaproxyCfgFile, serverConfig1, true)
						verifyHaProxyConfigContent(generatedHaproxyCfgFile, serverConfig2, true)
						Expect(fakeMonitor.StopWatchingCallCount()).To(Equal(1))
						Expect(scriptRunner.RunCallCount()).To(Equal(1))
						Expect(fakeMonitor.StartWatchingCallCount()).To(Equal(1))
					})
				})

				Context("when multiple mappings are provided as part of one request", func() {

					BeforeEach(func() {
						routingTable := models.NewRoutingTable(logger)
						routingTableEntry := models.NewRoutingTableEntry(
							[]models.BackendServerInfo{
								models.BackendServerInfo{Address: "some-ip-1", Port: 1234},
								models.BackendServerInfo{Address: "some-ip-2", Port: 1235},
							},
						)
						routinTableKey := models.RoutingKey{Port: 2222}
						ok := routingTable.Set(routinTableKey, routingTableEntry)
						Expect(ok).To(BeTrue())
						routingTableEntry = models.NewRoutingTableEntry(
							[]models.BackendServerInfo{
								models.BackendServerInfo{Address: "some-ip-3", Port: 1234},
								models.BackendServerInfo{Address: "some-ip-4", Port: 1235},
							},
						)
						routinTableKey = models.RoutingKey{Port: 3333}
						ok = routingTable.Set(routinTableKey, routingTableEntry)
						Expect(ok).To(BeTrue())

						err = haproxyConfigurer.Configure(routingTable)
						Expect(err).ShouldNot(HaveOccurred())
					})

					It("appends the haproxy config with new listen configuration", func() {
						listenCfg1 := `
listen listen_cfg_2222
  mode tcp
  bind :2222
`
						listenCfg2 := `
listen listen_cfg_3333
  mode tcp
  bind :3333
`
						verifyHaProxyConfigContent(generatedHaproxyCfgFile, listenCfg1, true)
						verifyHaProxyConfigContent(generatedHaproxyCfgFile, "server server_some-ip-1_1234 some-ip-1:1234", true)
						verifyHaProxyConfigContent(generatedHaproxyCfgFile, "server server_some-ip-2_1235 some-ip-2:1235", true)
						verifyHaProxyConfigContent(generatedHaproxyCfgFile, listenCfg2, true)
						verifyHaProxyConfigContent(generatedHaproxyCfgFile, "server server_some-ip-3_1234 some-ip-3:1234", true)
						verifyHaProxyConfigContent(generatedHaproxyCfgFile, "server server_some-ip-4_1235 some-ip-4:1235", true)
						verifyHaProxyConfigContent(generatedHaproxyCfgFile, string(haproxyConfigTemplateContent), true)
						Expect(fakeMonitor.StopWatchingCallCount()).To(Equal(1))
						Expect(scriptRunner.RunCallCount()).To(Equal(1))
						Expect(fakeMonitor.StartWatchingCallCount()).To(Equal(1))
					})
				})
			})

			Context("when Configure is called multiple times with different routes", func() {
				BeforeEach(func() {
					routingTable := models.NewRoutingTable(logger)
					routingTableEntry := models.NewRoutingTableEntry(
						[]models.BackendServerInfo{
							models.BackendServerInfo{Address: "some-ip-1", Port: 1234},
							models.BackendServerInfo{Address: "some-ip-2", Port: 1235},
						},
					)
					routinTableKey := models.RoutingKey{Port: 2222}
					ok := routingTable.Set(routinTableKey, routingTableEntry)
					Expect(ok).To(BeTrue())
					err = haproxyConfigurer.Configure(routingTable)
					Expect(err).ShouldNot(HaveOccurred())

					routingTable = models.NewRoutingTable(logger)
					routingTableEntry = models.NewRoutingTableEntry(
						[]models.BackendServerInfo{
							models.BackendServerInfo{Address: "some-ip-3", Port: 2345},
							models.BackendServerInfo{Address: "some-ip-4", Port: 3456},
						},
					)
					routinTableKey = models.RoutingKey{Port: 3333}
					ok = routingTable.Set(routinTableKey, routingTableEntry)
					Expect(ok).To(BeTrue())
					err = haproxyConfigurer.Configure(routingTable)
					Expect(err).ShouldNot(HaveOccurred())
				})

				It("persists the last routing table in haproxy config", func() {
					listenCfg := `
listen listen_cfg_3333
  mode tcp
  bind :3333
`
					notPresentCfg := `
listen listen_cfg_2222
  mode tcp
  bind :2222
`
					verifyHaProxyConfigContent(generatedHaproxyCfgFile, listenCfg, true)
					verifyHaProxyConfigContent(generatedHaproxyCfgFile, "server server_some-ip-3_2345 some-ip-3:2345", true)
					verifyHaProxyConfigContent(generatedHaproxyCfgFile, "server server_some-ip-4_3456 some-ip-4:3456", true)

					verifyHaProxyConfigContent(generatedHaproxyCfgFile, notPresentCfg, false)
					verifyHaProxyConfigContent(generatedHaproxyCfgFile, "server server_some-ip-1_1234 some-ip-1:1234", false)
					verifyHaProxyConfigContent(generatedHaproxyCfgFile, "server server_some-ip-2_1235 some-ip-2:1235", false)
					verifyHaProxyConfigContent(generatedHaproxyCfgFile, string(haproxyConfigTemplateContent), true)
					Expect(fakeMonitor.StopWatchingCallCount()).To(Equal(2))
					Expect(scriptRunner.RunCallCount()).To(Equal(2))
					Expect(fakeMonitor.StartWatchingCallCount()).To(Equal(2))
				})
			})
		})
	})
})
