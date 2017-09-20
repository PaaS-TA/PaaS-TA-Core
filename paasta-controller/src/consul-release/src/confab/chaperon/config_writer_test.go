package chaperon_test

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"runtime"

	"code.cloudfoundry.org/lager"

	"github.com/cloudfoundry-incubator/consul-release/src/confab/chaperon"
	"github.com/cloudfoundry-incubator/consul-release/src/confab/config"
	"github.com/cloudfoundry-incubator/consul-release/src/confab/fakes"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	. "github.com/pivotal-cf-experimental/gomegamatchers"
)

var _ = Describe("ConfigWriter", func() {
	var (
		configDir string
		dataDir   string
		cfg       config.Config
		writer    chaperon.ConfigWriter
		logger    *fakes.Logger
	)

	Describe("Write", func() {
		BeforeEach(func() {
			logger = &fakes.Logger{}

			var err error
			configDir, err = ioutil.TempDir("", "")
			Expect(err).NotTo(HaveOccurred())

			dataDir, err = ioutil.TempDir("", "")
			Expect(err).NotTo(HaveOccurred())

			cfg = config.Config{}
			cfg.Consul.Agent.DnsConfig.MaxStale = "5s"
			cfg.Consul.Agent.DnsConfig.RecursorTimeout = "5s"
			cfg.Node = config.ConfigNode{Name: "node", Index: 0}
			cfg.Path.ConsulConfigDir = configDir
			cfg.Path.DataDir = dataDir

			writer = chaperon.NewConfigWriter(configDir, logger)
		})

		It("writes a config file to the consul_config dir", func() {
			err := writer.Write(cfg)
			Expect(err).NotTo(HaveOccurred())

			buf, err := ioutil.ReadFile(filepath.Join(configDir, "config.json"))
			Expect(err).NotTo(HaveOccurred())

			conf := map[string]interface{}{
				"server":                 false,
				"domain":                 "",
				"datacenter":             "",
				"data_dir":               dataDir,
				"log_level":              "",
				"node_name":              "node-0",
				"rejoin_after_leave":     true,
				"bind_addr":              "",
				"disable_remote_exec":    true,
				"disable_update_check":   true,
				"protocol":               0,
				"verify_outgoing":        true,
				"verify_incoming":        true,
				"verify_server_hostname": true,
				"ca_file":                filepath.Join(configDir, "certs", "ca.crt"),
				"key_file":               filepath.Join(configDir, "certs", "agent.key"),
				"cert_file":              filepath.Join(configDir, "certs", "agent.crt"),
				"dns_config": map[string]interface{}{
					"allow_stale":      false,
					"max_stale":        "5s",
					"recursor_timeout": "5s",
				},
				"ports": map[string]int{
					"dns": 53,
				},
				"performance": map[string]int{
					"raft_multiplier": 1,
				},
			}
			body, err := json.Marshal(conf)
			Expect(err).To(BeNil())
			Expect(buf).To(MatchJSON(body))

			Expect(logger.Messages()).To(ContainSequence([]fakes.LoggerMessage{
				{
					Action: "config-writer.write.generate-configuration",
				},
				{
					Action: "config-writer.write.write-file",
					Data: []lager.Data{{
						"config": config.GenerateConfiguration(cfg, configDir, "node-0"),
					}},
				},
				{
					Action: "config-writer.write.success",
				},
			}))
		})

		Context("node name", func() {
			Context("when node-name.json does not exist", func() {
				It("uses the job name-index and writes node-name.json", func() {
					err := writer.Write(cfg)
					Expect(err).NotTo(HaveOccurred())

					buf, err := ioutil.ReadFile(filepath.Join(dataDir, "node-name.json"))
					Expect(err).NotTo(HaveOccurred())

					Expect(buf).To(MatchJSON(`{"node_name":"node-0"}`))

					buf, err = ioutil.ReadFile(filepath.Join(configDir, "config.json"))
					Expect(err).NotTo(HaveOccurred())

					var config map[string]interface{}

					err = json.Unmarshal(buf, &config)
					Expect(err).NotTo(HaveOccurred())
					Expect(config["node_name"]).To(Equal("node-0"))

					Expect(logger.Messages()).To(ContainSequence([]fakes.LoggerMessage{
						{
							Action: "config-writer.write.determine-node-name",
							Data: []lager.Data{{
								"node-name": "node-0",
							}},
						},
					}))
				})
			})

			Context("when node-name.json exists", func() {
				It("uses the the name from the file", func() {
					err := ioutil.WriteFile(filepath.Join(dataDir, "node-name.json"),
						[]byte(`{"node_name": "some-node-name"}`), os.ModePerm)
					Expect(err).NotTo(HaveOccurred())

					err = writer.Write(cfg)
					Expect(err).NotTo(HaveOccurred())

					buf, err := ioutil.ReadFile(filepath.Join(dataDir, "node-name.json"))
					Expect(err).NotTo(HaveOccurred())

					Expect(buf).To(MatchJSON(`{"node_name":"some-node-name"}`))

					buf, err = ioutil.ReadFile(filepath.Join(configDir, "config.json"))
					Expect(err).NotTo(HaveOccurred())

					var config map[string]interface{}

					err = json.Unmarshal(buf, &config)
					Expect(err).NotTo(HaveOccurred())
					Expect(config["node_name"]).To(Equal("some-node-name"))
				})
			})

			Context("when config has a node name specified", func() {
				It("honors the node name specified in the config", func() {
					cfg.Consul.Agent.NodeName = "some-random-node-name"
					err := writer.Write(cfg)
					Expect(err).NotTo(HaveOccurred())

					buf, err := ioutil.ReadFile(filepath.Join(configDir, "config.json"))
					Expect(err).NotTo(HaveOccurred())

					var config map[string]interface{}

					err = json.Unmarshal(buf, &config)
					Expect(err).NotTo(HaveOccurred())
					Expect(config["node_name"]).To(Equal("some-random-node-name"))
				})
			})

			Context("failure cases", func() {
				It("logs errors", func() {
					cfg.Path.DataDir = "/some/fake/path"
					writer.Write(cfg)

					var expected fakes.LoggerMessage
					for _, msg := range logger.Messages() {
						if msg.Action == "config-writer.write.determine-node-name.failed" {
							expected = msg
						}
					}
					Expect(expected.Error).To(BeAnOsIsNotExistError())
				})

				It("returns an error when the data dir does not exist", func() {
					cfg.Path.DataDir = "/some/fake/path"

					err := writer.Write(cfg)
					Expect(err).To(BeAnOsIsNotExistError())
				})

				It("returns an error when node-name.json has malformed json", func() {
					err := ioutil.WriteFile(filepath.Join(dataDir, "node-name.json"),
						[]byte(`%%%%%`), os.ModePerm)
					Expect(err).NotTo(HaveOccurred())

					err = writer.Write(cfg)
					Expect(err).To(MatchError("invalid character '%' looking for beginning of value"))
				})

				It("returns an error when node-name.json cannot be written to", func() {
					if runtime.GOOS == "windows" {
						Skip("Test doesn't work on Windows")
					}
					err := os.Chmod(dataDir, 0555)
					Expect(err).NotTo(HaveOccurred())

					err = writer.Write(cfg)
					Expect(err).To(MatchError(ContainSubstring("node-name.json: permission denied")))
				})

				It("returns an error when node-name.json cannot be read", func() {
					if runtime.GOOS == "windows" {
						Skip("Test doesn't work on Windows")
					}
					err := ioutil.WriteFile(filepath.Join(dataDir, "node-name.json"),
						[]byte(`%%%%%`), 0)
					Expect(err).NotTo(HaveOccurred())

					err = writer.Write(cfg)
					Expect(err).To(MatchError(ContainSubstring("node-name.json: permission denied")))
				})
			})
		})

		Context("failure cases", func() {
			It("returns an error when the config file can't be written to", func() {
				configFile := filepath.Join(configDir, "config.json")
				Expect(os.Mkdir(configFile, os.ModeDir)).To(Succeed())

				err := writer.Write(cfg)
				Expect(err).To(MatchError(ContainSubstring("is a directory")))

				Expect(logger.Messages()).To(ContainSequence([]fakes.LoggerMessage{
					{
						Action: "config-writer.write.generate-configuration",
					},
					{
						Action: "config-writer.write.write-file",
						Data: []lager.Data{{
							"config": config.GenerateConfiguration(cfg, configDir, "node-0"),
						}},
					},
					{
						Action: "config-writer.write.write-file.failed",
						Error:  fmt.Errorf("open %s: is a directory", filepath.Join(configDir, "config.json")),
					},
				}))
			})
		})
	})
})
