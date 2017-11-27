package config_test

import (
	"encoding/json"
	"io/ioutil"
	"os"

	"github.com/cloudfoundry-incubator/etcd-release/src/etcdfab/config"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Config", func() {
	Describe("ConfigFromJSON", func() {
		var (
			configFilePath string
		)

		BeforeEach(func() {
			tmpDir, err := ioutil.TempDir("", "")
			Expect(err).NotTo(HaveOccurred())

			configFile, err := ioutil.TempFile(tmpDir, "config-file")
			Expect(err).NotTo(HaveOccurred())

			err = configFile.Close()
			Expect(err).NotTo(HaveOccurred())

			configFilePath = configFile.Name()

			configuration := map[string]interface{}{
				"node": map[string]interface{}{
					"name":        "some_name",
					"index":       3,
					"external_ip": "some-external-ip",
				},
				"etcd": map[string]interface{}{
					"etcd_path":                          "path-to-etcd",
					"heartbeat_interval_in_milliseconds": 10,
					"election_timeout_in_milliseconds":   20,
					"peer_require_ssl":                   false,
					"peer_ip":                            "some-peer-ip",
					"require_ssl":                        false,
					"client_ip":                          "some-client-ip",
					"advertise_urls_dns_suffix":          "some-dns-suffix",
				},
			}
			configData, err := json.Marshal(configuration)
			Expect(err).NotTo(HaveOccurred())

			err = ioutil.WriteFile(configFilePath, configData, os.ModePerm)
			Expect(err).NotTo(HaveOccurred())
		})

		It("returns a configuration populated with values from the specified file", func() {
			cfg, err := config.ConfigFromJSON(configFilePath)
			Expect(err).NotTo(HaveOccurred())

			Expect(cfg).To(Equal(config.Config{
				Node: config.Node{
					Name:       "some_name",
					Index:      3,
					ExternalIP: "some-external-ip",
				},
				Etcd: config.Etcd{
					EtcdPath:               "path-to-etcd",
					HeartbeatInterval:      10,
					ElectionTimeout:        20,
					PeerRequireSSL:         false,
					PeerIP:                 "some-peer-ip",
					RequireSSL:             false,
					ClientIP:               "some-client-ip",
					AdvertiseURLsDNSSuffix: "some-dns-suffix",
				},
			}))
		})

		It("defaults values that are not specified in the JSON file", func() {
			err := ioutil.WriteFile(configFilePath, []byte("{}"), os.ModePerm)
			Expect(err).NotTo(HaveOccurred())

			cfg, err := config.ConfigFromJSON(configFilePath)
			Expect(err).NotTo(HaveOccurred())

			Expect(cfg.Etcd.EtcdPath).To(Equal("/var/vcap/packages/etcd/etcd"))
		})

		Context("failure cases", func() {
			Context("when it cannot read the config file", func() {
				It("returns the error to the caller and logs a helpful message", func() {
					_, err := config.ConfigFromJSON("/path/to/missing/config")
					Expect(err).To(MatchError("open /path/to/missing/config: no such file or directory"))
				})
			})

			Context("when it cannot unmarshal the config file", func() {
				BeforeEach(func() {
					err := ioutil.WriteFile(configFilePath, []byte("%%%"), os.ModePerm)
					Expect(err).NotTo(HaveOccurred())
				})

				It("returns the error to the caller and logs a helpful message", func() {
					_, err := config.ConfigFromJSON(configFilePath)
					Expect(err).To(MatchError("invalid character '%' looking for beginning of value"))
				})
			})
		})
	})
})
