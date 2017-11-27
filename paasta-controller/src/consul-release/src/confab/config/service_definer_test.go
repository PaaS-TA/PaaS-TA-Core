package config_test

import (
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"

	"code.cloudfoundry.org/lager"

	"github.com/cloudfoundry-incubator/consul-release/src/confab/config"
	"github.com/cloudfoundry-incubator/consul-release/src/confab/fakes"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	. "github.com/pivotal-cf-experimental/gomegamatchers"
)

var _ = Describe("ServiceDefiner", func() {
	var (
		definer config.ServiceDefiner
		logger  *fakes.Logger
	)

	BeforeEach(func() {
		logger = &fakes.Logger{}
		definer = config.ServiceDefiner{
			Logger: logger,
		}
	})

	AfterEach(func() {
		config.ResetCreateFile()
		config.ResetSyncFile()
	})

	Describe("GenerateDefinitions", func() {
		It("logs the definitions that it generates", func() {
			definer.GenerateDefinitions(config.Config{
				Node: config.ConfigNode{
					Name:  "some_node",
					Index: 0,
				},
				Consul: config.ConfigConsul{
					Agent: config.ConfigConsulAgent{
						Services: map[string]config.ServiceDefinition{
							"router":           {},
							"cloud_controller": {},
							"doppler":          {},
						},
					},
				},
			})
			Expect(logger.Messages()).To(ContainElement(fakes.LoggerMessage{
				Action: "service-definer.generate-definitions.define",
				Data: []lager.Data{{
					"service": "router",
				}},
			}))
			Expect(logger.Messages()).To(ContainElement(fakes.LoggerMessage{
				Action: "service-definer.generate-definitions.define",
				Data: []lager.Data{{
					"service": "cloud_controller",
				}},
			}))
			Expect(logger.Messages()).To(ContainElement(fakes.LoggerMessage{
				Action: "service-definer.generate-definitions.define",
				Data: []lager.Data{{
					"service": "doppler",
				}},
			}))
		})

		Context("when running on linux", func() {
			BeforeEach(func() {
				config.SetGOOS("linux")
			})

			AfterEach(func() {
				config.ResetGOOS()
			})

			It("generates a definition with the default values", func() {
				definitions := definer.GenerateDefinitions(config.Config{
					Node: config.ConfigNode{
						Name:  "some_node",
						Index: 0,
						Zone:  "z1",
					},
					Consul: config.ConfigConsul{
						Agent: config.ConfigConsulAgent{
							Services: map[string]config.ServiceDefinition{
								"router": {},
							},
						},
					},
				})
				Expect(definitions).To(ConsistOf([]config.ServiceDefinition{
					{
						ServiceName: "router",
						Name:        "router",
						Check: &config.ServiceDefinitionCheck{
							Name:     "dns_health_check",
							Script:   "/var/vcap/jobs/router/bin/dns_health_check",
							Interval: "3s",
						},
						Tags: []string{"some-node-0", "z1"},
					},
				}))
			})
		})

		Context("when running on windows", func() {
			BeforeEach(func() {
				config.SetGOOS("windows")
			})

			AfterEach(func() {
				config.ResetGOOS()
			})

			It("generates a definition with the default values", func() {
				definitions := definer.GenerateDefinitions(config.Config{
					Node: config.ConfigNode{
						Name:  "some_node",
						Index: 0,
					},
					Consul: config.ConfigConsul{
						Agent: config.ConfigConsulAgent{
							Services: map[string]config.ServiceDefinition{
								"router": {},
							},
						},
					},
				})
				Expect(definitions).To(ConsistOf([]config.ServiceDefinition{
					{
						ServiceName: "router",
						Name:        "router",
						Check: &config.ServiceDefinitionCheck{
							Name:     "dns_health_check",
							Script:   "powershell -Command /var/vcap/jobs/router/bin/dns_health_check.ps1; Exit $LASTEXITCODE",
							Interval: "3s",
						},
						Tags: []string{"some-node-0"},
					},
				}))
			})
		})

		It("generates a definition with the service name dasherized", func() {
			definitions := definer.GenerateDefinitions(config.Config{
				Node: config.ConfigNode{
					Name:  "some_node",
					Index: 0,
				},
				Consul: config.ConfigConsul{
					Agent: config.ConfigConsulAgent{
						Services: map[string]config.ServiceDefinition{
							"cloud_controller": {},
						},
					},
				},
			})

			script := "/var/vcap/jobs/cloud_controller/bin/dns_health_check"
			if Windows {
				script = "powershell -Command /var/vcap/jobs/cloud_controller/bin/dns_health_check.ps1; Exit $LASTEXITCODE"
			}

			Expect(definitions).To(ConsistOf([]config.ServiceDefinition{
				{
					ServiceName: "cloud_controller",
					Name:        "cloud-controller",
					Check: &config.ServiceDefinitionCheck{
						Name:     "dns_health_check",
						Script:   script,
						Interval: "3s",
					},
					Tags: []string{"some-node-0"},
				},
			}))
		})

		It("generates a definition with the check field overridden", func() {
			definitions := definer.GenerateDefinitions(config.Config{
				Node: config.ConfigNode{
					Name:  "some_node",
					Index: 0,
				},
				Consul: config.ConfigConsul{
					Agent: config.ConfigConsulAgent{
						Services: map[string]config.ServiceDefinition{
							"doppler": {
								Check: &config.ServiceDefinitionCheck{
									Name:     "my-script",
									Script:   "/var/vcap/jobs/doppler/bin/my-script",
									Interval: "5m",
								},
							},
						},
					},
				},
			})
			Expect(definitions).To(ConsistOf([]config.ServiceDefinition{
				{
					ServiceName: "doppler",
					Name:        "doppler",
					Check: &config.ServiceDefinitionCheck{
						Name:     "my-script",
						Script:   "/var/vcap/jobs/doppler/bin/my-script",
						Interval: "5m",
					},
					Tags: []string{"some-node-0"},
				},
			}))
		})

		It("generates a definition with the checks field specified", func() {
			definitions := definer.GenerateDefinitions(config.Config{
				Node: config.ConfigNode{
					Name:  "some_node",
					Index: 0,
				},
				Consul: config.ConfigConsul{
					Agent: config.ConfigConsulAgent{
						Services: map[string]config.ServiceDefinition{
							"uaa": {
								Checks: []config.ServiceDefinitionCheck{{
									Name:     "check-login",
									Script:   "/var/vcap/jobs/uaa/bin/check-login",
									Interval: "1m",
								}},
							},
						},
					},
				},
			})

			script := "/var/vcap/jobs/uaa/bin/dns_health_check"
			if Windows {
				script = "powershell -Command /var/vcap/jobs/uaa/bin/dns_health_check.ps1; Exit $LASTEXITCODE"
			}

			Expect(definitions).To(ConsistOf([]config.ServiceDefinition{
				{
					ServiceName: "uaa",
					Name:        "uaa",
					Check: &config.ServiceDefinitionCheck{
						Name:     "dns_health_check",
						Script:   script,
						Interval: "3s",
					},
					Checks: []config.ServiceDefinitionCheck{{
						Name:     "check-login",
						Script:   "/var/vcap/jobs/uaa/bin/check-login",
						Interval: "1m",
					}},
					Tags: []string{"some-node-0"},
				},
			}))
		})

		It("generates a definition with the name field overridden", func() {
			definitions := definer.GenerateDefinitions(config.Config{
				Node: config.ConfigNode{
					Name:  "some_node",
					Index: 0,
				},
				Consul: config.ConfigConsul{
					Agent: config.ConfigConsulAgent{
						Services: map[string]config.ServiceDefinition{
							"cell": {
								Name: "cell_z1",
							},
						},
					},
				},
			})

			script := "/var/vcap/jobs/cell/bin/dns_health_check"
			if Windows {
				script = "powershell -Command /var/vcap/jobs/cell/bin/dns_health_check.ps1; Exit $LASTEXITCODE"
			}

			Expect(definitions).To(ConsistOf([]config.ServiceDefinition{
				{
					ServiceName: "cell",
					Name:        "cell_z1",
					Check: &config.ServiceDefinitionCheck{
						Name:     "dns_health_check",
						Script:   script,
						Interval: "3s",
					},
					Tags: []string{"some-node-0"},
				},
			}))
		})

		It("generates a definition with the tag field overridden", func() {
			definitions := definer.GenerateDefinitions(config.Config{
				Node: config.ConfigNode{
					Name:  "some_node",
					Index: 0,
				},
				Consul: config.ConfigConsul{
					Agent: config.ConfigConsulAgent{
						Services: map[string]config.ServiceDefinition{
							"dea": {
								Tags: []string{"runner"},
							},
						},
					},
				},
			})

			script := "/var/vcap/jobs/dea/bin/dns_health_check"
			if Windows {
				script = "powershell -Command /var/vcap/jobs/dea/bin/dns_health_check.ps1; Exit $LASTEXITCODE"
			}

			Expect(definitions).To(ConsistOf([]config.ServiceDefinition{
				{
					ServiceName: "dea",
					Name:        "dea",
					Check: &config.ServiceDefinitionCheck{
						Name:     "dns_health_check",
						Script:   script,
						Interval: "3s",
					},
					Tags: []string{"runner"},
				},
			}))
		})

		It("generates definitions with the address field specified", func() {
			definitions := definer.GenerateDefinitions(config.Config{
				Node: config.ConfigNode{
					Name:  "some_node",
					Index: 0,
				},
				Consul: config.ConfigConsul{
					Agent: config.ConfigConsulAgent{
						Services: map[string]config.ServiceDefinition{
							"dea": {
								Address: "192.168.1.1",
							},
						},
					},
				},
			})

			script := "/var/vcap/jobs/dea/bin/dns_health_check"
			if Windows {
				script = "powershell -Command /var/vcap/jobs/dea/bin/dns_health_check.ps1; Exit $LASTEXITCODE"
			}

			Expect(definitions).To(ConsistOf([]config.ServiceDefinition{
				{
					ServiceName: "dea",
					Name:        "dea",
					Address:     "192.168.1.1",
					Check: &config.ServiceDefinitionCheck{
						Name:     "dns_health_check",
						Script:   script,
						Interval: "3s",
					},
					Tags: []string{"some-node-0"},
				},
			}))
		})

		It("generates definitions with the port field specified", func() {
			definitions := definer.GenerateDefinitions(config.Config{
				Node: config.ConfigNode{
					Name:  "some_node",
					Index: 0,
				},
				Consul: config.ConfigConsul{
					Agent: config.ConfigConsulAgent{
						Services: map[string]config.ServiceDefinition{
							"router": {
								Port: 12345,
							},
						},
					},
				},
			})

			script := "/var/vcap/jobs/router/bin/dns_health_check"
			if Windows {
				script = "powershell -Command /var/vcap/jobs/router/bin/dns_health_check.ps1; Exit $LASTEXITCODE"
			}

			Expect(definitions).To(ConsistOf([]config.ServiceDefinition{
				{
					ServiceName: "router",
					Name:        "router",
					Port:        12345,
					Check: &config.ServiceDefinitionCheck{
						Name:     "dns_health_check",
						Script:   script,
						Interval: "3s",
					},
					Tags: []string{"some-node-0"},
				},
			}))
		})

		It("generates definitions with the EnableTagOverride field specified", func() {
			definitions := definer.GenerateDefinitions(config.Config{
				Node: config.ConfigNode{
					Name:  "some_node",
					Index: 0,
				},
				Consul: config.ConfigConsul{
					Agent: config.ConfigConsulAgent{
						Services: map[string]config.ServiceDefinition{
							"router": {
								EnableTagOverride: true,
							},
						},
					},
				},
			})

			script := "/var/vcap/jobs/router/bin/dns_health_check"
			if Windows {
				script = "powershell -Command /var/vcap/jobs/router/bin/dns_health_check.ps1; Exit $LASTEXITCODE"
			}

			Expect(definitions).To(ConsistOf([]config.ServiceDefinition{
				{
					ServiceName:       "router",
					Name:              "router",
					EnableTagOverride: true,
					Check: &config.ServiceDefinitionCheck{
						Name:     "dns_health_check",
						Script:   script,
						Interval: "3s",
					},
					Tags: []string{"some-node-0"},
				},
			}))
		})

		It("generates definitions with the Id field specified", func() {
			definitions := definer.GenerateDefinitions(config.Config{
				Node: config.ConfigNode{
					Name:  "some_node",
					Index: 0,
				},
				Consul: config.ConfigConsul{
					Agent: config.ConfigConsulAgent{
						Services: map[string]config.ServiceDefinition{
							"router": {
								ID: "some-id",
							},
						},
					},
				},
			})

			script := "/var/vcap/jobs/router/bin/dns_health_check"
			if Windows {
				script = "powershell -Command /var/vcap/jobs/router/bin/dns_health_check.ps1; Exit $LASTEXITCODE"
			}

			Expect(definitions).To(ConsistOf([]config.ServiceDefinition{
				{
					ServiceName: "router",
					Name:        "router",
					ID:          "some-id",
					Check: &config.ServiceDefinitionCheck{
						Name:     "dns_health_check",
						Script:   script,
						Interval: "3s",
					},
					Tags: []string{"some-node-0"},
				},
			}))
		})

		It("generates definitions with the Token field specified", func() {
			definitions := definer.GenerateDefinitions(config.Config{
				Node: config.ConfigNode{
					Name:  "some_node",
					Index: 0,
				},
				Consul: config.ConfigConsul{
					Agent: config.ConfigConsulAgent{
						Services: map[string]config.ServiceDefinition{
							"router": {
								Token: "some-token",
							},
						},
					},
				},
			})

			script := "/var/vcap/jobs/router/bin/dns_health_check"
			if Windows {
				script = "powershell -Command /var/vcap/jobs/router/bin/dns_health_check.ps1; Exit $LASTEXITCODE"
			}

			Expect(definitions).To(ConsistOf([]config.ServiceDefinition{
				{
					ServiceName: "router",
					Name:        "router",
					Token:       "some-token",
					Check: &config.ServiceDefinitionCheck{
						Name:     "dns_health_check",
						Script:   script,
						Interval: "3s",
					},
					Tags: []string{"some-node-0"},
				},
			}))
		})

		It("generates definitions with a check type given the overrides", func() {
			definitions := definer.GenerateDefinitions(config.Config{
				Node: config.ConfigNode{
					Name:  "some_node",
					Index: 0,
				},
				Consul: config.ConfigConsul{
					Agent: config.ConfigConsulAgent{
						Services: map[string]config.ServiceDefinition{
							"router": {
								Check: &config.ServiceDefinitionCheck{
									Name:              "some-check-name",
									ID:                "some-check-id",
									Script:            "/var/vcap/jobs/router/bin/my-script",
									HTTP:              "http://some-endpoint.com",
									TCP:               "localhost:2120",
									TTL:               "30s",
									Interval:          "10s",
									Timeout:           "20s",
									Notes:             "some-notes",
									DockerContainerID: "some-docker-container-id",
									Shell:             "/bin/bash",
									Status:            "some-status",
									ServiceID:         "some-service-id",
								},
							},
						},
					},
				},
			})
			Expect(definitions).To(ConsistOf([]config.ServiceDefinition{
				{
					ServiceName: "router",
					Name:        "router",
					Check: &config.ServiceDefinitionCheck{
						Name:              "some-check-name",
						ID:                "some-check-id",
						Script:            "/var/vcap/jobs/router/bin/my-script",
						HTTP:              "http://some-endpoint.com",
						TCP:               "localhost:2120",
						TTL:               "30s",
						Interval:          "10s",
						Timeout:           "20s",
						Notes:             "some-notes",
						DockerContainerID: "some-docker-container-id",
						Shell:             "/bin/bash",
						Status:            "some-status",
						ServiceID:         "some-service-id",
					},
					Tags: []string{"some-node-0"},
				},
			}))
		})
	})

	Describe("WriteDefinitions", func() {
		var tempDir string
		BeforeEach(func() {
			var err error
			tempDir, err = ioutil.TempDir("", "conf-dir")
			Expect(err).NotTo(HaveOccurred())
		})

		It("logs the files that it writes out", func() {
			err := definer.WriteDefinitions(tempDir, []config.ServiceDefinition{
				{
					ServiceName: "cloud_controller",
					Name:        "cloud-controller",
				},
				{
					ServiceName: "api",
					Name:        "api",
				},
			})
			Expect(err).NotTo(HaveOccurred())
			Expect(logger.Messages()).To(ContainSequence([]fakes.LoggerMessage{
				{
					Action: "service-definer.write-definitions.write",
					Data: []lager.Data{{
						"path": filepath.Join(tempDir, "service-cloud_controller.json"),
					}},
				},
				{
					Action: "service-definer.write-definitions.write.success",
					Data: []lager.Data{{
						"path": filepath.Join(tempDir, "service-cloud_controller.json"),
					}},
				},
				{
					Action: "service-definer.write-definitions.write",
					Data: []lager.Data{{
						"path": filepath.Join(tempDir, "service-api.json"),
					}},
				},
				{
					Action: "service-definer.write-definitions.write.success",
					Data: []lager.Data{{
						"path": filepath.Join(tempDir, "service-api.json"),
					}},
				},
			}))
		})

		It("writes out a definition file per service", func() {
			err := definer.WriteDefinitions(tempDir, []config.ServiceDefinition{
				{
					ServiceName: "cloud_controller",
					Name:        "cloud-controller",
				},
				{
					ServiceName: "api",
					Name:        "api",
					Check: &config.ServiceDefinitionCheck{
						Name:              "some-check-name",
						ID:                "some-check-id",
						Script:            "/var/vcap/jobs/router/bin/my-script",
						HTTP:              "http://some-endpoint.com",
						TCP:               "localhost:2120",
						TTL:               "30s",
						Interval:          "10s",
						Timeout:           "20s",
						Notes:             "some-notes",
						DockerContainerID: "some-docker-container-id",
						Shell:             "/bin/bash",
						Status:            "some-status",
						ServiceID:         "some-service-id",
					},
					Checks: []config.ServiceDefinitionCheck{
						{
							Name:              "some-check-name-1",
							ID:                "some-check-id-1",
							Script:            "/var/vcap/jobs/router/bin/my-script-1",
							HTTP:              "http://some-endpoint.com-1",
							TCP:               "localhost:2120-1",
							TTL:               "30s-1",
							Interval:          "10s-1",
							Timeout:           "20s-1",
							Notes:             "some-notes-1",
							DockerContainerID: "some-docker-container-id-1",
							Shell:             "/bin/bash-1",
							Status:            "some-status-1",
							ServiceID:         "some-service-id-1",
						},
						{
							Name:              "some-check-name-2",
							ID:                "some-check-id-2",
							Script:            "/var/vcap/jobs/router/bin/my-script-2",
							HTTP:              "http://some-endpoint.com-2",
							TCP:               "localhost:2120-2",
							TTL:               "30s-2",
							Interval:          "10s-2",
							Timeout:           "20s-2",
							Notes:             "some-notes-2",
							DockerContainerID: "some-docker-container-id-2",
							Shell:             "/bin/bash-2",
							Status:            "some-status-2",
							ServiceID:         "some-service-id-2",
						},
					},
					Tags:              []string{"node-0"},
					Address:           "192.168.1.1",
					Port:              8080,
					EnableTagOverride: true,
					ID:                "some-id",
					Token:             "1234567890",
				},
			})
			Expect(err).NotTo(HaveOccurred())

			configFile, err := ioutil.ReadFile(fmt.Sprintf("%s/service-cloud_controller.json", tempDir))
			Expect(err).NotTo(HaveOccurred())

			Expect(string(configFile)).To(MatchJSON(`{
				"service": {
					"name": "cloud-controller"
				}
			}`))

			configFile, err = ioutil.ReadFile(fmt.Sprintf("%s/service-api.json", tempDir))
			Expect(err).NotTo(HaveOccurred())

			Expect(string(configFile)).To(MatchJSON(`{
				"service": {
					"name": "api",
					"check": {
						"name": "some-check-name",
						"id": "some-check-id",
						"script": "/var/vcap/jobs/router/bin/my-script",
						"http": "http://some-endpoint.com",
						"tcp": "localhost:2120",
						"ttl": "30s",
						"interval": "10s",
						"timeout": "20s",
						"notes": "some-notes",
						"docker_container_id": "some-docker-container-id",
						"shell": "/bin/bash",
						"status": "some-status",
						"service_id": "some-service-id"
					},
					"checks":[
						{
							"name": "some-check-name-1",
							"id": "some-check-id-1",
							"script": "/var/vcap/jobs/router/bin/my-script-1",
							"http": "http://some-endpoint.com-1",
							"tcp": "localhost:2120-1",
							"ttl": "30s-1",
							"interval": "10s-1",
							"timeout": "20s-1",
							"notes": "some-notes-1",
							"docker_container_id": "some-docker-container-id-1",
							"shell": "/bin/bash-1",
							"status": "some-status-1",
							"service_id": "some-service-id-1"
						},
						{
							"name": "some-check-name-2",
							"id": "some-check-id-2",
							"script": "/var/vcap/jobs/router/bin/my-script-2",
							"http": "http://some-endpoint.com-2",
							"tcp": "localhost:2120-2",
							"ttl": "30s-2",
							"interval": "10s-2",
							"timeout": "20s-2",
							"notes": "some-notes-2",
							"docker_container_id": "some-docker-container-id-2",
							"shell": "/bin/bash-2",
							"status": "some-status-2",
							"service_id": "some-service-id-2"
						}
					],
					"tags": ["node-0"],
					"address": "192.168.1.1",
					"port": 8080,
					"enableTagOverride": true,
					"id": "some-id",
					"token": "1234567890"
				}
			}`))
		})

		It("syncs the file", func() {
			syncFileCallCount := 0
			config.SetSyncFile(func(*os.File) error {
				syncFileCallCount++
				return nil
			})
			err := definer.WriteDefinitions(tempDir, []config.ServiceDefinition{
				{
					ServiceName: "some-service",
				},
			})
			Expect(err).NotTo(HaveOccurred())

			Expect(syncFileCallCount).To(Equal(1))
		})

		Context("failure cases", func() {
			It("errors when the file cannot be created", func() {
				const RandomPath = "/some/random/path"
				var errFmt string
				if Windows {
					errFmt = "open %s: The system cannot find the path specified."
				} else {
					errFmt = "open %s: no such file or directory"
				}
				errMsg := fmt.Errorf(errFmt, filepath.Join(RandomPath, "service-cloud_controller.json"))

				err := definer.WriteDefinitions(RandomPath, []config.ServiceDefinition{
					{
						ServiceName: "cloud_controller",
					},
				})

				Expect(err).To(MatchError(Or(
					ContainSubstring("no such file or directory"),
					ContainSubstring("The system cannot find the path specified."),
				)))

				Expect(logger.Messages()).To(ContainSequence([]fakes.LoggerMessage{
					{
						Action: "service-definer.write-definitions.write",
						Data: []lager.Data{{
							"path": filepath.FromSlash("/some/random/path/service-cloud_controller.json"),
						}},
					},
					{
						Action: "service-definer.write-definitions.write.failed",
						Error:  errMsg,
						Data: []lager.Data{{
							"path": filepath.FromSlash("/some/random/path/service-cloud_controller.json"),
						}},
					},
				}))

			})

			It("errors when the file cannot be written to", func() {
				config.SetCreateFile(func(path string) (*os.File, error) {
					file, err := os.Create(path)
					if err != nil {
						return nil, err
					}

					err = file.Close()
					if err != nil {
						return nil, err
					}

					return file, nil

				})

				err := definer.WriteDefinitions(tempDir, []config.ServiceDefinition{
					{
						ServiceName: "cloud_controller",
					},
				})

				Expect(err).To(MatchError(ContainSubstring("file already closed")))

				errPath := filepath.Join(tempDir, "service-cloud_controller.json")
				Expect(logger.Messages()).To(ContainSequence([]fakes.LoggerMessage{
					{
						Action: "service-definer.write-definitions.write",
						Data: []lager.Data{{
							"path": errPath,
						}},
					},
					{
						Action: "service-definer.write-definitions.write.failed",
						Error:  fmt.Errorf("write %s: %s", errPath, "file already closed"),
						Data: []lager.Data{{
							"path": errPath,
						}},
					},
				}))
			})

			It("returns an error when it fails to sync the file", func() {
				config.SetSyncFile(func(*os.File) error {
					return errors.New("something bad happened")
				})
				err := definer.WriteDefinitions(tempDir, []config.ServiceDefinition{
					{
						ServiceName: "some-service",
					},
				})
				Expect(err).To(MatchError("something bad happened"))
			})
		})
	})
})
