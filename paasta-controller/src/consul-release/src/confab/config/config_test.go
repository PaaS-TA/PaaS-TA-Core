package config_test

import (
	"path/filepath"

	"github.com/cloudfoundry-incubator/consul-release/src/confab/config"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Config", func() {
	Describe("ConfigFromJSON", func() {
		Context("when given a fully populated config", func() {
			It("returns a non-default config", func() {
				json := []byte(`{
					"node": {
						"name": "nodename",
						"index": 1234,
						"external_ip": "10.0.0.1",
						"zone": "z1"
					},
					"path": {
						"agent_path": "/path/to/agent",
						"consul_config_dir": "/consul/config/dir",
						"pid_file": "/path/to/pidfile",
						"keyring_file": "/path/to/keyring",
						"data_dir": "/path/to/data/dir"
					},
					"consul": {
						"agent": {
							"services": {
								"myservice": {
									"name" : "myservicename"
								}
							},
							"mode": "server",
							"datacenter": "dc1",
							"log_level": "debug",
							"protocol_version": 1,
							"servers": {
								"lan": ["server1", "server2", "server3"],
								"wan": ["wan-server1", "wan-server2", "wan-server3"]
							},
							"telemetry": {
								"statsd_address": "myhost:8125"
							},
							"dns_config": {
								"allow_stale": true,
								"max_stale": "15s",
								"recursor_timeout": "15s"
							},
							"require_ssl": true
						},
						"encrypt_keys": ["key-1", "key-2"]
					},
					"confab": {
						"timeout_in_seconds": 30
					}
				}`)

				cfg, err := config.ConfigFromJSON(json, []byte("{}"))
				Expect(err).NotTo(HaveOccurred())
				Expect(cfg).To(Equal(config.Config{
					Path: config.ConfigPath{
						AgentPath:       "/path/to/agent",
						ConsulConfigDir: "/consul/config/dir",
						PIDFile:         "/path/to/pidfile",
						KeyringFile:     "/path/to/keyring",
						DataDir:         "/path/to/data/dir",
					},
					Node: config.ConfigNode{
						Name:       "nodename",
						Index:      1234,
						ExternalIP: "10.0.0.1",
						Zone:       "z1",
					},
					Consul: config.ConfigConsul{
						Agent: config.ConfigConsulAgent{
							Services: map[string]config.ServiceDefinition{
								"myservice": {
									Name: "myservicename",
								},
							},
							Mode:            "server",
							Datacenter:      "dc1",
							LogLevel:        "debug",
							ProtocolVersion: 1,
							Servers: config.ConfigConsulAgentServers{
								LAN: []string{"server1", "server2", "server3"},
								WAN: []string{"wan-server1", "wan-server2", "wan-server3"},
							},
							Telemetry: config.ConfigConsulTelemetry{
								StatsdAddress: "myhost:8125",
							},
							DnsConfig: config.ConfigConsulAgentDnsConfig{
								AllowStale:      true,
								MaxStale:        "15s",
								RecursorTimeout: "15s",
							},
							RequireSSL: true,
						},
						EncryptKeys: []string{"key-1", "key-2"},
					},
					Confab: config.ConfigConfab{
						TimeoutInSeconds: 30,
					},
				}))
			})
		})

		Context("when passing an empty config", func() {
			It("returns a config with default values", func() {
				json := []byte(`{}`)
				cfg, err := config.ConfigFromJSON(json, json)

				cfg.Path.AgentPath = filepath.ToSlash(cfg.Path.AgentPath)
				cfg.Path.ConsulConfigDir = filepath.ToSlash(cfg.Path.ConsulConfigDir)
				cfg.Path.PIDFile = filepath.ToSlash(cfg.Path.PIDFile)
				cfg.Path.KeyringFile = filepath.ToSlash(cfg.Path.KeyringFile)
				cfg.Path.DataDir = filepath.ToSlash(cfg.Path.DataDir)

				Expect(err).NotTo(HaveOccurred())
				Expect(cfg).To(Equal(config.Config{
					Path: config.ConfigPath{
						AgentPath:       "/var/vcap/packages/consul/bin/consul",
						ConsulConfigDir: "/var/vcap/jobs/consul_agent/config",
						PIDFile:         "/var/vcap/sys/run/consul_agent/consul_agent.pid",
						KeyringFile:     "/var/vcap/data/consul_agent/serf/local.keyring",
						DataDir:         "/var/vcap/data/consul_agent",
					},
					Consul: config.ConfigConsul{
						Agent: config.ConfigConsulAgent{
							Servers: config.ConfigConsulAgentServers{
								LAN: []string{},
								WAN: []string{},
							},
							DnsConfig: config.ConfigConsulAgentDnsConfig{
								AllowStale:      true,
								MaxStale:        "30s",
								RecursorTimeout: "5s",
							},
						},
					},
					Confab: config.ConfigConfab{
						TimeoutInSeconds: 55,
					},
				}))
			})
		})

		Context("when passing consul link properties", func() {
			It("returns a config with the link properties set, preferring the link properties", func() {
				json := []byte(`{
					"consul": {
						"agent": {
							"mode": "server",
							"datacenter": "dc1",
							"servers": {
								"lan": ["server1", "server2", "server3"],
								"wan": ["wan-server1", "wan-server2", "wan-server3"]
							},
							"dns_config": { "allow_stale": true, "max_stale": "15s", "recursor_timeout": "15s" }
						},
						"encrypt_keys": ["key-1", "key-2"]
					}
				}`)
				consulLinkJson := []byte(`{
					"agent": {
						"services": {
							"myservice": { "name" : "mylinkservicename" }
						},
						"log_level": "debug",
						"datacenter": "dc1000"
					},
					"encrypt_keys": ["link-key-1", "link-key-2"]
				}`)

				cfg, err := config.ConfigFromJSON(json, consulLinkJson)
				Expect(err).NotTo(HaveOccurred())
				Expect(cfg.Consul).To(Equal(config.ConfigConsul{
					Agent: config.ConfigConsulAgent{
						Mode:       "server",
						Datacenter: "dc1000",
						LogLevel:   "debug",
						Servers: config.ConfigConsulAgentServers{
							LAN: []string{"server1", "server2", "server3"},
							WAN: []string{"wan-server1", "wan-server2", "wan-server3"},
						},
						DnsConfig: config.ConfigConsulAgentDnsConfig{
							AllowStale:      true,
							MaxStale:        "15s",
							RecursorTimeout: "15s",
						},
						Services: map[string]config.ServiceDefinition{
							"myservice": {
								Name: "mylinkservicename",
							},
						},
					},
					EncryptKeys: []string{"link-key-1", "link-key-2"},
				}))
			})
		})

		Context("when passing an config that is in server mode and has no specified keyring file and data dir", func() {
			It("returns a config with keyring file and data dir paths containing /var/vcap/store/", func() {
				json := []byte(`{"consul": {"agent": {"mode": "server"}}}`)
				cfg, err := config.ConfigFromJSON(json, []byte("{}"))

				Expect(err).NotTo(HaveOccurred())
				Expect(filepath.ToSlash(cfg.Path.KeyringFile)).To(Equal("/var/vcap/store/consul_agent/serf/local.keyring"))
				Expect(filepath.ToSlash(cfg.Path.DataDir)).To(Equal("/var/vcap/store/consul_agent"))
			})
		})

		It("returns an error on invalid json", func() {
			json := []byte(`{%%%{{}{}{{}{}{{}}}}}}}`)
			_, err := config.ConfigFromJSON(json, json)
			Expect(err).To(MatchError(ContainSubstring("invalid character")))
		})
	})
})
