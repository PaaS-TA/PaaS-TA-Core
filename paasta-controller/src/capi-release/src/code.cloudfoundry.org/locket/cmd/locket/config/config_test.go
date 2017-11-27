package config_test

import (
	"encoding/json"
	"io/ioutil"
	"os"

	"code.cloudfoundry.org/debugserver"
	"code.cloudfoundry.org/lager/lagerflags"
	"code.cloudfoundry.org/locket/cmd/locket/config"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("LocketConfig", func() {
	var configFilePath, configData string

	BeforeEach(func() {
		configData = `{
			"log_level": "debug",
			"listen_address": "1.2.3.4:9090",
			"database_driver": "mysql",
			"max_open_database_connections": 1000,
			"database_connection_string": "stuff",
			"debug_address": "some-more-stuff",
			"consul_cluster": "http://127.0.0.1:1234,http://127.0.0.1:12345",
			"ca_file": "i am a ca file",
			"cert_file": "i am a cert file",
			"key_file": "i am a key file"
		}`
	})

	JustBeforeEach(func() {
		configFile, err := ioutil.TempFile("", "config-file")
		Expect(err).NotTo(HaveOccurred())

		n, err := configFile.WriteString(configData)
		Expect(err).NotTo(HaveOccurred())
		Expect(n).To(Equal(len(configData)))

		configFilePath = configFile.Name()
	})

	AfterEach(func() {
		err := os.RemoveAll(configFilePath)
		Expect(err).NotTo(HaveOccurred())
	})

	It("correctly parses the config file", func() {
		locketConfig, err := config.NewLocketConfig(configFilePath)
		Expect(err).NotTo(HaveOccurred())

		config := config.LocketConfig{
			DatabaseDriver:             "mysql",
			ListenAddress:              "1.2.3.4:9090",
			DatabaseConnectionString:   "stuff",
			MaxOpenDatabaseConnections: 1000,
			ConsulCluster:              "http://127.0.0.1:1234,http://127.0.0.1:12345",
			LagerConfig: lagerflags.LagerConfig{
				LogLevel: "debug",
			},
			DebugServerConfig: debugserver.DebugServerConfig{
				DebugAddress: "some-more-stuff",
			},
			CaFile:   "i am a ca file",
			CertFile: "i am a cert file",
			KeyFile:  "i am a key file",
		}

		Expect(locketConfig).To(Equal(config))
	})

	Context("when the file does not exist", func() {
		It("returns an error", func() {
			_, err := config.NewLocketConfig("foobar")
			Expect(err).To(HaveOccurred())
		})
	})

	Context("when the file does not contain valid json", func() {
		BeforeEach(func() {
			configData = "{{"
		})

		It("returns an error", func() {
			_, err := config.NewLocketConfig(configFilePath)
			Expect(err).To(HaveOccurred())
		})

	})

	Context("default values", func() {
		BeforeEach(func() {
			configData = `{}`
		})

		It("uses default values when they are not specified", func() {
			locketConfig, err := config.NewLocketConfig(configFilePath)
			Expect(err).NotTo(HaveOccurred())

			Expect(locketConfig).To(Equal(config.DefaultLocketConfig()))
		})

		Context("when serialized from LocketConfig", func() {
			BeforeEach(func() {
				locketConfig := config.LocketConfig{}
				bytes, err := json.Marshal(locketConfig)
				Expect(err).NotTo(HaveOccurred())
				configData = string(bytes)
			})

			It("uses default values when they are not specified", func() {
				locketConfig, err := config.NewLocketConfig(configFilePath)
				Expect(err).NotTo(HaveOccurred())

				Expect(locketConfig).To(Equal(config.DefaultLocketConfig()))
			})
		})
	})
})
