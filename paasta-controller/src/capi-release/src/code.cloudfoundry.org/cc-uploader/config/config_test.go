package config_test

import (
	"io/ioutil"
	"os"
	"time"

	. "code.cloudfoundry.org/cc-uploader/config"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Config", func() {
	var (
		configFile        *os.File
		configFileContent string
	)

	JustBeforeEach(func() {
		var err error
		configFile, err = ioutil.TempFile("", "config.json")
		Expect(err).NotTo(HaveOccurred())
		_, err = configFile.Write([]byte(configFileContent))
		Expect(err).NotTo(HaveOccurred())
		err = configFile.Close()
		Expect(err).NotTo(HaveOccurred())
	})

	AfterEach(func() {
		os.Remove(configFile.Name())
	})

	Describe("Uploader config", func() {
		Context("when all values are provided in the config file", func() {
			BeforeEach(func() {
				configFileContent = `{
					"consul_cluster": "consul_cluster",
					"debug_server_config": {
						"debug_address": "debug_address"
					},
					"dropsonde_port": 12,
					"lager_config": {
						"log_level": "fatal"
					},
					"listen_addr": "listen_addr",
					"job_polling_interval": "5s",

					"cc_client_cert": "/path/to/server.cert",
					"cc_client_key": "/path/to/server.key",
					"cc_ca_cert": "/path/to/server-ca.cert",

					"mutual_tls": {
						"listen_addr": "mtls_listen_addr",
						"ca_cert": "ca-cert",
						"server_cert": "server-cert",
						"server_key": "server-key"
					}
				}`
			})

			It("reads from the config file and populates the config", func() {
				uploaderConfig, err := NewUploaderConfig(configFile.Name())
				Expect(err).ToNot(HaveOccurred())

				Expect(uploaderConfig.DropsondePort).To(Equal(12))
				Expect(uploaderConfig.LagerConfig.LogLevel).To(Equal("fatal"))
				Expect(uploaderConfig.ListenAddress).To(Equal("listen_addr"))
				Expect(uploaderConfig.CCJobPollingInterval).To(Equal(Duration(5 * time.Second)))
				Expect(uploaderConfig.ConsulCluster).To(Equal("consul_cluster"))
				Expect(uploaderConfig.DebugServerConfig.DebugAddress).To(Equal("debug_address"))
				Expect(uploaderConfig.CCClientCert).To(Equal("/path/to/server.cert"))
				Expect(uploaderConfig.CCClientKey).To(Equal("/path/to/server.key"))
				Expect(uploaderConfig.CCCACert).To(Equal("/path/to/server-ca.cert"))
				Expect(uploaderConfig.MutualTLS.ListenAddress).To(Equal("mtls_listen_addr"))
				Expect(uploaderConfig.MutualTLS.CACert).To(Equal("ca-cert"))
				Expect(uploaderConfig.MutualTLS.ServerCert).To(Equal("server-cert"))
				Expect(uploaderConfig.MutualTLS.ServerKey).To(Equal("server-key"))
			})
		})

		Context("when only required values are in the config file", func() {
			BeforeEach(func() {
				configFileContent = `{
					"mutual_tls": {
						"listen_addr": "mtls_listen_addr",
						"ca_cert": "ca-cert",
						"server_cert": "server-cert",
						"server_key": "server-key"
					}
				}`
			})

			It("generates a config with the default values", func() {
				uploaderConfig, err := NewUploaderConfig(configFile.Name())
				Expect(err).ToNot(HaveOccurred())

				Expect(uploaderConfig.DropsondePort).To(Equal(3457))
				Expect(uploaderConfig.LagerConfig.LogLevel).To(Equal("info"))
				Expect(uploaderConfig.ListenAddress).To(Equal("0.0.0.0:9090"))
				Expect(uploaderConfig.CCJobPollingInterval).To(Equal(Duration(1 * time.Second)))
			})
		})

		Context("when all required values are missing", func() {
			BeforeEach(func() {
				configFileContent = "{}"
			})

			It("returns an error", func() {
				_, err := NewUploaderConfig(configFile.Name())
				Expect(err).To(MatchError("The following required config values were not provided: 'mutual_tls.listen_addr','mutual_tls.ca_cert','mutual_tls.server_cert','mutual_tls.server_key'"))
			})
		})

		Context("when mutual_tls.listen_addr is missing", func() {
			BeforeEach(func() {
				configFileContent = `{
					"mutual_tls": {
						"ca_cert": "ca-cert",
						"server_cert": "server-cert",
						"server_key": "server-key"
					}
				}`
			})

			It("returns an error", func() {
				_, err := NewUploaderConfig(configFile.Name())
				Expect(err).To(MatchError("The following required config values were not provided: 'mutual_tls.listen_addr'"))
			})
		})

		Context("when mutual_tls.ca_cert is missing", func() {
			BeforeEach(func() {
				configFileContent = `{
					"mutual_tls": {
						"listen_addr": "mtls_listen_addr",
						"server_cert": "server-cert",
						"server_key": "server-key"
					}
				}`
			})

			It("returns an error", func() {
				_, err := NewUploaderConfig(configFile.Name())
				Expect(err).To(MatchError("The following required config values were not provided: 'mutual_tls.ca_cert'"))
			})
		})

		Context("when mutual_tls.server_cert is missing", func() {
			BeforeEach(func() {
				configFileContent = `{
					"mutual_tls": {
						"listen_addr": "mtls_listen_addr",
						"ca_cert": "ca-cert",
						"server_key": "server-key"
					}
				}`
			})

			It("returns an error", func() {
				_, err := NewUploaderConfig(configFile.Name())
				Expect(err).To(MatchError("The following required config values were not provided: 'mutual_tls.server_cert'"))
			})
		})

		Context("when mutual_tls.server_key is missing", func() {
			BeforeEach(func() {
				configFileContent = `{
					"mutual_tls": {
						"listen_addr": "mtls_listen_addr",
						"ca_cert": "ca-cert",
						"server_cert": "server-cert"
					}
				}`
			})

			It("returns an error", func() {
				_, err := NewUploaderConfig(configFile.Name())
				Expect(err).To(MatchError("The following required config values were not provided: 'mutual_tls.server_key'"))
			})
		})
	})
})
