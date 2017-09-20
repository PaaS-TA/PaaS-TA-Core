package main_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"time"

	"github.com/cloudfoundry-incubator/consul-release/src/confab/utils"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

const COMMAND_TIMEOUT = time.Second * 15
const POLL_INTERVAL = time.Millisecond * 250

var _ = Describe("confab", func() {
	var (
		tempDir         string
		dataDir         string
		consulConfigDir string
		pidFile         *os.File
		configFile      *os.File
	)

	BeforeEach(func() {
		var err error
		tempDir, err = ioutil.TempDir("", "testing")
		Expect(err).NotTo(HaveOccurred())

		dataDir, err = ioutil.TempDir("", "dataDir")
		Expect(err).NotTo(HaveOccurred())

		consulConfigDir, err = ioutil.TempDir(tempDir, "fake-agent-config-dir")
		Expect(err).NotTo(HaveOccurred())

		pidFile, err = ioutil.TempFile(tempDir, "fake-pid-file")
		Expect(err).NotTo(HaveOccurred())

		Expect(pidFile.Close()).To(Succeed())

		err = os.Remove(pidFile.Name())
		Expect(err).NotTo(HaveOccurred())

		configFile, err = ioutil.TempFile(tempDir, "config-file")
		Expect(err).NotTo(HaveOccurred())

		err = configFile.Close()
		Expect(err).NotTo(HaveOccurred())

		writeConfigurationFile(configFile.Name(), map[string]interface{}{})

		options := []byte(`{"Members": ["member-1", "member-2", "member-3"]}`)
		err = ioutil.WriteFile(filepath.Join(consulConfigDir, "options.json"), options, 0600)
		Expect(err).NotTo(HaveOccurred())
	})

	AfterEach(func() {
		killProcessAttachedToPort(8400)
		killProcessAttachedToPort(8500)

		err := os.Chmod(consulConfigDir, os.ModePerm)
		Expect(err).NotTo(HaveOccurred())

		err = os.RemoveAll(tempDir)
		Expect(err).NotTo(HaveOccurred())
	})

	Context("when managing the entire process lifecycle", func() {
		BeforeEach(func() {
			writeConfigurationFile(configFile.Name(), map[string]interface{}{
				"node": map[string]interface{}{
					"name":        "my-node",
					"index":       3,
					"external_ip": "10.0.0.1",
				},
				"path": map[string]interface{}{
					"agent_path":        pathToFakeAgent,
					"consul_config_dir": consulConfigDir,
					"pid_file":          pidFile.Name(),
					"data_dir":          dataDir,
				},
				"consul": map[string]interface{}{
					"encrypt_keys": []string{"banana"},
					"agent": map[string]interface{}{
						"domain":     "some-domain",
						"datacenter": "dc1",
						"log_level":  "debug",
						"servers": map[string]interface{}{
							"lan": []string{"member-1", "member-2", "member-3"},
							"wan": []string{"wan-member-1", "wan-member-2", "wan-member-3"},
						},
						"services": map[string]interface{}{
							"cloud_controller": map[string]interface{}{
								"checks": []map[string]string{{
									"name":     "do_something",
									"script":   "/var/vcap/jobs/cloudcontroller/bin/do_something",
									"interval": "5m",
								}},
							},
							"router": map[string]interface{}{
								"name": "gorouter",
							},
						},
					},
					"performance": map[string]int{
						"raft_multiplier": 1,
					},
				},
			})
		})

		AfterEach(func() {
			Expect(os.Remove(configFile.Name())).NotTo(HaveOccurred())
		})

		testStoppingConsulProcess := func(pid int) {

			stop := exec.Command(pathToConfab,
				"stop",
				"--config-file", configFile.Name(),
			)
			Eventually(stop.Run, COMMAND_TIMEOUT, COMMAND_TIMEOUT).Should(Succeed())

			Eventually(func() bool {
				return utils.IsPIDRunning(pid)
			}, COMMAND_TIMEOUT, time.Millisecond*250).Should(BeFalse())

			Expect(fakeAgentOutputFromFile(consulConfigDir, "fake-output.json")).To(Equal(FakeAgentOutputData{
				PID: pid,
				Args: []string{
					"agent",
					fmt.Sprintf("-config-dir=%s", consulConfigDir),
					"-recursor=8.8.8.8",
					"-recursor=10.0.2.3",
				},
				LeaveCallCount: 1,
			}))

			serviceConfig, err := ioutil.ReadFile(filepath.Join(consulConfigDir, "service-cloud_controller.json"))
			Expect(err).NotTo(HaveOccurred())
			Expect(string(serviceConfig)).To(MatchJSON(`{
				"service": {
					"name": "cloud-controller",
					"check": {
						"name": "dns_health_check",
						"script": "/var/vcap/jobs/cloud_controller/bin/dns_health_check",
						"interval": "3s"
					},
					"checks": [
						{
							"name": "do_something",
							"script": "/var/vcap/jobs/cloudcontroller/bin/do_something",
							"interval": "5m"
						}
					],
					"tags": ["my-node-3"]
				}
			}`))

			serviceConfig, err = ioutil.ReadFile(filepath.Join(consulConfigDir, "service-router.json"))
			Expect(err).NotTo(HaveOccurred())
			Expect(string(serviceConfig)).To(MatchJSON(`{
				"service": {
					"name": "gorouter",
					"check": {
						"name": "dns_health_check",
						"script": "/var/vcap/jobs/router/bin/dns_health_check",
						"interval": "3s"
					},
					"tags": ["my-node-3"]
				}
			}`))

			consulConfig, err := ioutil.ReadFile(filepath.Join(consulConfigDir, "config.json"))
			Expect(err).NotTo(HaveOccurred())

			conf := map[string]interface{}{
				"server":                 false,
				"domain":                 "some-domain",
				"datacenter":             "dc1",
				"data_dir":               dataDir,
				"log_level":              "debug",
				"node_name":              "my-node-3",
				"rejoin_after_leave":     true,
				"bind_addr":              "10.0.0.1",
				"disable_remote_exec":    true,
				"disable_update_check":   true,
				"protocol":               0,
				"verify_outgoing":        true,
				"verify_incoming":        true,
				"verify_server_hostname": true,
				"ca_file":                filepath.Join(consulConfigDir, "certs", "ca.crt"),
				"key_file":               filepath.Join(consulConfigDir, "certs", "agent.key"),
				"cert_file":              filepath.Join(consulConfigDir, "certs", "agent.crt"),
				"encrypt":                "enqzXBmgKOy13WIGsmUk+g==",
				"dns_config": map[string]interface{}{
					"allow_stale":      true,
					"max_stale":        "30s",
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
			Expect(string(consulConfig)).To(MatchJSON(body))
		}

		It("starts and stops the consul process as a daemon", func() {
			start := exec.Command(pathToConfab,
				"start",
				"--recursor", "8.8.8.8",
				"--recursor", "10.0.2.3",
				"--config-file", configFile.Name(),
			)
			Eventually(start.Run, COMMAND_TIMEOUT, COMMAND_TIMEOUT).Should(Succeed())

			pid, err := getPID(pidFile.Name())
			Expect(err).NotTo(HaveOccurred())
			Expect(utils.IsPIDRunning(pid)).To(BeTrue())

			testStoppingConsulProcess(pid)
		})

		It("starts and stops the consul process as a foreground process", func() {
			start := exec.Command(pathToConfab,
				"start",
				"--foreground",
				"--recursor", "8.8.8.8",
				"--recursor", "10.0.2.3",
				"--config-file", configFile.Name(),
			)
			Eventually(start.Start, COMMAND_TIMEOUT, COMMAND_TIMEOUT).Should(Succeed())

			wg := new(sync.WaitGroup)
			wg.Add(1)
			go func() {
				defer GinkgoRecover()
				defer wg.Done()
				Expect(start.Wait()).To(Succeed())
			}()

			// Wait until pidfile exists
			Eventually(func() error {
				_, err := os.Stat(pidFile.Name())
				return err
			}, COMMAND_TIMEOUT, time.Second*1).Should(Succeed())

			pid, err := getPID(pidFile.Name())
			Expect(err).NotTo(HaveOccurred())
			Expect(utils.IsPIDRunning(pid)).To(BeTrue())

			testStoppingConsulProcess(pid)
			Eventually(func() error { wg.Wait(); return nil }).Should(Succeed())
		})

		It("the foreground process exits when the runner process exits", func() {
			start := exec.Command(pathToConfab,
				"start",
				"--foreground",
				"--recursor", "8.8.8.8",
				"--recursor", "10.0.2.3",
				"--config-file", configFile.Name(),
			)
			Eventually(start.Start, COMMAND_TIMEOUT, COMMAND_TIMEOUT).Should(Succeed())

			wg := new(sync.WaitGroup)
			wg.Add(1)
			go func() {
				defer GinkgoRecover()
				defer wg.Done()
				Expect(start.Wait()).To(Succeed())
			}()

			// Wait until pidfile exists
			Eventually(func() error {
				_, err := os.Stat(pidFile.Name())
				return err
			}, COMMAND_TIMEOUT, time.Second*1).Should(Succeed())

			pid, err := getPID(pidFile.Name())
			Expect(err).NotTo(HaveOccurred())
			Expect(utils.IsPIDRunning(pid)).To(BeTrue())

			killProcessAttachedToPort(8400)
			killProcessAttachedToPort(8500)

			Eventually(func() error { wg.Wait(); return nil }).Should(Succeed())
		})

	})

	Context("when starting", func() {
		BeforeEach(func() {
			writeConfigurationFile(configFile.Name(), map[string]interface{}{
				"path": map[string]interface{}{
					"agent_path":        pathToFakeAgent,
					"consul_config_dir": consulConfigDir,
					"pid_file":          pidFile.Name(),
					"data_dir":          dataDir,
				},
				"consul": map[string]interface{}{
					"agent": map[string]interface{}{
						"servers": map[string]interface{}{
							"lan": []string{"member-1", "member-2", "member-3"},
						},
					},
				},
			})
		})

		AfterEach(func() {
			err := os.Remove(configFile.Name())
			Expect(err).NotTo(HaveOccurred())

			killProcessWithPIDFile(pidFile.Name())
		})

		Context("for a client", func() {
			It("starts a consul agent as a client", func() {
				cmd := exec.Command(pathToConfab,
					"start",
					"--config-file", configFile.Name(),
				)
				Eventually(cmd.Run, COMMAND_TIMEOUT, COMMAND_TIMEOUT).Should(Succeed())

				pid, err := getPID(pidFile.Name())
				Expect(err).NotTo(HaveOccurred())
				Expect(utils.IsPIDRunning(pid)).To(BeTrue())

				Expect(fakeAgentOutputFromFile(consulConfigDir, "fake-output.json")).To(Equal(FakeAgentOutputData{
					PID: pid,
					Args: []string{
						"agent",
						fmt.Sprintf("-config-dir=%s", consulConfigDir),
					},
				}))
			})
		})

		Context("for a server", func() {
			BeforeEach(func() {
				options := []byte(`{"Members": ["member-1", "member-2", "member-3"]}`)
				Expect(ioutil.WriteFile(filepath.Join(consulConfigDir, "options.json"), options, 0600)).To(Succeed())
				writeConfigurationFile(configFile.Name(), map[string]interface{}{
					"path": map[string]interface{}{
						"agent_path":        pathToFakeAgent,
						"consul_config_dir": consulConfigDir,
						"pid_file":          pidFile.Name(),
						"data_dir":          dataDir,
					},
					"consul": map[string]interface{}{
						"agent": map[string]interface{}{
							"mode": "server",
							"servers": map[string]interface{}{
								"lan": []string{"member-1", "member-2", "member-3"},
							},
						},
						"encrypt_keys": []string{"key-1", "key-2"},
					},
				})
			})

			AfterEach(func() {
				killProcessWithPIDFile(pidFile.Name())
			})

			It("starts a consul agent as a bootstrap server", func() {
				cmd := exec.Command(pathToConfab,
					"start",
					"--config-file", configFile.Name(),
				)
				Eventually(cmd.Run, COMMAND_TIMEOUT, COMMAND_TIMEOUT).Should(Succeed())

				pid, err := getPID(pidFile.Name())
				Expect(err).NotTo(HaveOccurred())
				Expect(utils.IsPIDRunning(pid)).To(BeTrue())

				Eventually(func() ([]string, error) {
					fakeAgentOutputData, err := fakeAgentOutputFromFile(consulConfigDir, "fake-output.json")
					return fakeAgentOutputData.Args, err
				}, "5s").Should(Equal([]string{
					"agent",
					fmt.Sprintf("-config-dir=%s", consulConfigDir),
				}))

				fakeAgentOutputData, err := fakeAgentOutputFromFile(consulConfigDir, "fake-output.json")
				Expect(err).NotTo(HaveOccurred())
				Expect(fakeAgentOutputData.ConsulConfig).To(Equal(ConsulConfig{
					Server: true,
				}))

				Eventually(func() (FakeAgentOutputData, error) {
					return fakeAgentOutputFromFile(consulConfigDir, "fake-output-2.json")
				}, "2s").Should(Equal(FakeAgentOutputData{
					Args: []string{
						"agent",
						fmt.Sprintf("-config-dir=%s", consulConfigDir),
					},
					PID:                 pid,
					UseKeyCallCount:     1,
					InstallKeyCallCount: 2,
					StatsCallCount:      1,
					ConsulConfig: ConsulConfig{
						Server:    true,
						Bootstrap: true,
					},
				}))
			})

			It("checks sync state up to the timeout", func() {
				if Windows {
					// On Windows the timeout works, but due to the
					// overhead of shelling out the elapsed time
					// regularly exceeds the 1 second tolerance.
					Skip("Flaky on Windows due to the overhead of shelling out")
				}
				writeConfigurationFile(configFile.Name(), map[string]interface{}{
					"path": map[string]interface{}{
						"agent_path":        pathToFakeAgent,
						"consul_config_dir": consulConfigDir,
						"pid_file":          pidFile.Name(),
						"data_dir":          dataDir,
					},
					"consul": map[string]interface{}{
						"agent": map[string]interface{}{
							"mode": "server",
							"servers": map[string]interface{}{
								"lan": []string{"member-1", "member-2", "member-3"},
							},
						},
						"encrypt_keys": []string{"key-1", "key-2"},
					},
					"confab": map[string]interface{}{
						"timeout_in_seconds": 5,
					},
				})

				options := []byte(`{"Members": ["member-1", "member-2", "member-3"], "FailStatsEndpoint": true}`)
				Expect(ioutil.WriteFile(filepath.Join(consulConfigDir, "options.json"), options, 0600)).To(Succeed())

				cmd := exec.Command(pathToConfab,
					"start",
					"--config-file", configFile.Name(),
				)

				start := time.Now()
				Eventually(cmd.Run, COMMAND_TIMEOUT, COMMAND_TIMEOUT).ShouldNot(Succeed())
				Expect(time.Now()).To(BeTemporally("~", start.Add(6*time.Second), 1*time.Second))

				output, err := fakeAgentOutputFromFile(consulConfigDir, "fake-output-2.json")
				Expect(err).NotTo(HaveOccurred())
				Expect(output.StatsCallCount).To(BeNumerically(">", 0))
				Expect(output.StatsCallCount).To(BeNumerically("<", 4))
			})
		})
	})

	Context("when stopping", func() {
		BeforeEach(func() {
			options := []byte(`{"Members": ["member-1", "member-2", "member-3"]}`)
			Expect(ioutil.WriteFile(filepath.Join(consulConfigDir, "options.json"), options, 0600)).To(Succeed())

			writeConfigurationFile(configFile.Name(), map[string]interface{}{
				"path": map[string]interface{}{
					"agent_path":        pathToFakeAgent,
					"consul_config_dir": consulConfigDir,
					"pid_file":          pidFile.Name(),
					"data_dir":          dataDir,
				},
				"consul": map[string]interface{}{
					"agent": map[string]interface{}{
						"mode": "server",
						"servers": map[string]interface{}{
							"lan": []string{"member-1", "member-2", "member-3"},
						},
					},
					"encrypt_keys": []string{"key-1", "key-2"},
				},
			})
		})

		It("stops the consul agent", func() {
			cmd := exec.Command(pathToConfab,
				"start",
				"--config-file", configFile.Name(),
			)
			Eventually(cmd.Run, COMMAND_TIMEOUT, COMMAND_TIMEOUT).Should(Succeed())

			Eventually(func() error {
				conn, err := net.Dial("tcp", "localhost:8400")
				if err == nil {
					conn.Close()
				}
				return err
			}, "5s").Should(Succeed())

			pid, err := getPID(pidFile.Name())
			Expect(err).NotTo(HaveOccurred())

			cmd = exec.Command(pathToConfab,
				"stop",
				"--config-file", configFile.Name(),
			)
			Eventually(cmd.Run, COMMAND_TIMEOUT, COMMAND_TIMEOUT).Should(Succeed())

			Eventually(func() bool {
				return utils.IsRunningProcess(pidFile.Name())
			}, "5s").Should(BeFalse())

			Eventually(func() (FakeAgentOutputData, error) {
				return fakeAgentOutputFromFile(consulConfigDir, "fake-output-2.json")
			}, COMMAND_TIMEOUT, POLL_INTERVAL).Should(Equal(FakeAgentOutputData{
				PID: pid,
				Args: []string{
					"agent",
					fmt.Sprintf("-config-dir=%s", consulConfigDir),
				},
				ConsulConfig: ConsulConfig{
					Server:    true,
					Bootstrap: true,
				},
				LeaveCallCount:      1,
				InstallKeyCallCount: 2,
				UseKeyCallCount:     1,
				StatsCallCount:      1,
			}))
		})
	})

	Context("failure cases", func() {
		BeforeEach(func() {
			writeConfigurationFile(configFile.Name(), map[string]interface{}{
				"path": map[string]interface{}{
					"agent_path":        pathToFakeAgent,
					"consul_config_dir": consulConfigDir,
					"pid_file":          pidFile.Name(),
					"data_dir":          dataDir,
				},
			})
		})

		Context("when no arguments are provided", func() {
			It("returns a non-zero status code and prints usage", func() {
				cmd := exec.Command(pathToConfab)
				buffer := bytes.NewBuffer([]byte{})
				cmd.Stderr = buffer
				Eventually(cmd.Run, COMMAND_TIMEOUT, COMMAND_TIMEOUT).ShouldNot(Succeed())
				Expect(buffer).To(ContainSubstring("invalid number of arguments"))

				usageLines := []string{
					"usage: confab COMMAND OPTIONS",
					"COMMAND: \"start\" or \"stop\"",
					"-config-file",
					"specifies the config file",
				}
				for _, line := range usageLines {
					Expect(buffer).To(ContainSubstring(line))
				}
			})
		})

		Context("when no command is provided", func() {
			It("returns a non-zero status code and prints usage", func() {
				cmd := exec.Command(pathToConfab,
					"--recursor=8.8.8.8",
					"--config-file", configFile.Name(),
				)
				buffer := bytes.NewBuffer([]byte{})
				cmd.Stderr = buffer
				Eventually(cmd.Run, COMMAND_TIMEOUT, COMMAND_TIMEOUT).ShouldNot(Succeed())
				Expect(buffer).To(ContainSubstring("invalid COMMAND \"--recursor=8.8.8.8\""))
				Expect(buffer).To(ContainSubstring("usage: confab COMMAND OPTIONS"))
			})
		})

		Context("when an invalid command is provided", func() {
			It("returns a non-zero status code and prints usage", func() {
				cmd := exec.Command(pathToConfab, "banana",
					"--config-file", configFile.Name())
				buffer := bytes.NewBuffer([]byte{})
				cmd.Stderr = buffer
				Eventually(cmd.Run, COMMAND_TIMEOUT, COMMAND_TIMEOUT).ShouldNot(Succeed())
				Expect(buffer).To(ContainSubstring("invalid COMMAND \"banana\""))
				Expect(buffer).To(ContainSubstring("usage: confab COMMAND OPTIONS"))
			})
		})

		Context("expected-member is missing", func() {
			It("prints an error and usage", func() {
				cmd := exec.Command(pathToConfab, "start",
					"--config-file", configFile.Name())
				buffer := bytes.NewBuffer([]byte{})
				cmd.Stderr = buffer
				Eventually(cmd.Run, COMMAND_TIMEOUT, COMMAND_TIMEOUT).ShouldNot(Succeed())
				Expect(buffer).To(ContainSubstring("at least one \"expected-member\" must be provided"))
				Expect(buffer).To(ContainSubstring("usage: confab COMMAND OPTIONS"))
			})
		})

		Context("when the agent executable does not exist", func() {
			BeforeEach(func() {
				writeConfigurationFile(configFile.Name(), map[string]interface{}{
					"path": map[string]interface{}{
						"agent_path":        "/tmp/path/that/does/not/exist",
						"consul_config_dir": consulConfigDir,
						"pid_file":          pidFile.Name(),
						"data_dir":          dataDir,
					},
					"consul": map[string]interface{}{
						"agent": map[string]interface{}{
							"servers": map[string]interface{}{
								"lan": []string{"member-1"},
							},
						},
					},
				})
			})

			It("prints an error and usage", func() {
				cmd := exec.Command(pathToConfab, "start",
					"--config-file", configFile.Name())
				buffer := bytes.NewBuffer([]byte{})
				cmd.Stderr = buffer
				Eventually(cmd.Run, COMMAND_TIMEOUT, COMMAND_TIMEOUT).ShouldNot(Succeed())
				Expect(buffer).To(ContainSubstring("\"agent_path\" \"/tmp/path/that/does/not/exist\" cannot be found"))
				Expect(buffer).To(ContainSubstring("usage: confab COMMAND OPTIONS"))
			})
		})

		Context("when the PID file option is not provided", func() {
			BeforeEach(func() {
				writeConfigurationFile(configFile.Name(), map[string]interface{}{
					"path": map[string]interface{}{
						"agent_path":        pathToFakeAgent,
						"consul_config_dir": consulConfigDir,
						"pid_file":          "",
						"data_dir":          dataDir,
					},
					"consul": map[string]interface{}{
						"agent": map[string]interface{}{
							"servers": map[string]interface{}{
								"lan": []string{"member-1"},
							},
						},
					},
				})
			})

			It("prints an error and usage", func() {
				cmd := exec.Command(pathToConfab, "start",
					"--config-file", configFile.Name())
				buffer := bytes.NewBuffer([]byte{})
				cmd.Stderr = buffer
				Eventually(cmd.Run, COMMAND_TIMEOUT, COMMAND_TIMEOUT).ShouldNot(Succeed())
				Expect(buffer).To(ContainSubstring("\"pid_file\" cannot be empty"))
				Expect(buffer).To(ContainSubstring("usage: confab COMMAND OPTIONS"))
			})
		})

		Context("when the consul config dir is not provided", func() {
			BeforeEach(func() {
				writeConfigurationFile(configFile.Name(), map[string]interface{}{
					"path": map[string]interface{}{
						"agent_path":        pathToFakeAgent,
						"consul_config_dir": "/tmp/path/that/does/not/exist",
						"pid_file":          pidFile.Name(),
						"data_dir":          dataDir,
					},
					"consul": map[string]interface{}{
						"agent": map[string]interface{}{
							"servers": map[string]interface{}{
								"lan": []string{"member-1"},
							},
						},
					},
				})
			})

			It("prints an error and usage", func() {
				cmd := exec.Command(pathToConfab, "start",
					"--config-file", configFile.Name())
				buffer := bytes.NewBuffer([]byte{})
				cmd.Stderr = buffer
				Eventually(cmd.Run, COMMAND_TIMEOUT, COMMAND_TIMEOUT).ShouldNot(Succeed())
				Expect(buffer).To(ContainSubstring("\"consul_config_dir\" \"/tmp/path/that/does/not/exist\" could not be found"))
				Expect(buffer).To(ContainSubstring("usage: confab COMMAND OPTIONS"))
			})
		})

		Context("when the pid file contains the pid of a running process", func() {
			BeforeEach(func() {
				writeConfigurationFile(configFile.Name(), map[string]interface{}{
					"path": map[string]interface{}{
						"agent_path":        pathToFakeAgent,
						"consul_config_dir": consulConfigDir,
						"pid_file":          pidFile.Name(),
						"data_dir":          dataDir,
					},
					"consul": map[string]interface{}{
						"agent": map[string]interface{}{
							"servers": map[string]interface{}{
								"lan": []string{"member-1", "member-2", "member-3"},
							},
						},
					},
				})
			})

			It("prints an error and exits status 1 without killing the process", func() {
				cmd := exec.Command(pathToConfab,
					"start",
					"--config-file", configFile.Name(),
				)
				Eventually(cmd.Run, COMMAND_TIMEOUT, COMMAND_TIMEOUT).Should(Succeed())

				cmd = exec.Command(pathToConfab,
					"start",
					"--config-file", configFile.Name(),
				)

				stdout := bytes.NewBuffer([]byte{})
				stderr := bytes.NewBuffer([]byte{})
				cmd.Stdout = stdout
				cmd.Stderr = stderr

				Eventually(cmd.Run, COMMAND_TIMEOUT, COMMAND_TIMEOUT).ShouldNot(Succeed())
				Expect(stderr).To(ContainSubstring("consul_agent is already running, please stop it first"))

				pid, err := getPID(pidFile.Name())
				Expect(err).NotTo(HaveOccurred())
				Expect(utils.IsPIDRunning(pid)).To(BeTrue())
			})
		})

		Context("when the rpc connection cannot be created", func() {
			BeforeEach(func() {
				writeConfigurationFile(configFile.Name(), map[string]interface{}{
					"path": map[string]interface{}{
						"agent_path":        pathToFakeAgent,
						"consul_config_dir": consulConfigDir,
						"pid_file":          pidFile.Name(),
						"data_dir":          dataDir,
					},
					"consul": map[string]interface{}{
						"agent": map[string]interface{}{
							"mode": "server",
							"servers": map[string]interface{}{
								"lan": []string{"member-1", "member-2", "member-3"},
							},
						},
					},
				})
			})

			AfterEach(func() {
				err := os.Remove(configFile.Name())
				Expect(err).NotTo(HaveOccurred())
			})

			It("returns an error and exits with status 1", func() {
				options := []byte(`{ "Members": ["member-1", "member-2", "member-3"], "FailRPCServer": true }`)
				Expect(ioutil.WriteFile(filepath.Join(consulConfigDir, "options.json"), options, 0600)).To(Succeed())

				cmd := exec.Command(pathToConfab,
					"start",
					"--config-file", configFile.Name(),
				)
				buffer := bytes.NewBuffer([]byte{})
				cmd.Stderr = buffer
				Eventually(cmd.Run, COMMAND_TIMEOUT, COMMAND_TIMEOUT).ShouldNot(Succeed())
				Expect(buffer).To(ContainSubstring("error during start"))
				Expect(buffer).To(Or(
					ContainSubstring("connection refused"),
					ContainSubstring("No connection could be made"), // Windows
				))
			})
		})

		Context("when an invalid flag is provided", func() {
			It("exits and prints usage", func() {
				cmd := exec.Command(pathToConfab, "start", "--banana")
				buffer := bytes.NewBuffer([]byte{})
				cmd.Stderr = buffer

				Eventually(cmd.Run, COMMAND_TIMEOUT, COMMAND_TIMEOUT).ShouldNot(Succeed())
				Expect(buffer).To(ContainSubstring("flag provided but not defined: -banana"))
				Expect(buffer).NotTo(ContainSubstring("usage: confab COMMAND OPTIONS"))
			})
		})

		Context("when the config file does not exist", func() {
			It("returns an error and exits with status 1", func() {
				cmd := exec.Command(pathToConfab,
					"start",
					"--config-file", "/some-missing-file.json",
				)
				buffer := bytes.NewBuffer([]byte{})
				cmd.Stderr = buffer
				Eventually(cmd.Run, COMMAND_TIMEOUT, COMMAND_TIMEOUT).ShouldNot(Succeed())
				Expect(buffer).To(Or(
					ContainSubstring("no such file or directory"),
					ContainSubstring("The system cannot find the file specified"), // Windows
				))
			})
		})

		Context("when the config file is malformed json", func() {
			It("returns an error and exits with status 1", func() {
				tmpFile, err := ioutil.TempFile(tempDir, "config")
				Expect(err).NotTo(HaveOccurred())
				defer tmpFile.Close()

				_, err = tmpFile.Write([]byte(`%%%%%%%%%`))
				Expect(err).NotTo(HaveOccurred())

				cmd := exec.Command(pathToConfab,
					"start",
					"--config-file", tmpFile.Name(),
				)
				buffer := bytes.NewBuffer([]byte{})
				cmd.Stderr = buffer
				Eventually(cmd.Run, COMMAND_TIMEOUT, COMMAND_TIMEOUT).ShouldNot(Succeed())
				Expect(buffer).To(ContainSubstring("invalid character"))
			})
		})

		Context("when the consul config dir is not writeable", func() {
			BeforeEach(func() {
				writeConfigurationFile(configFile.Name(), map[string]interface{}{
					"path": map[string]interface{}{
						"agent_path":        pathToFakeAgent,
						"consul_config_dir": consulConfigDir,
						"pid_file":          pidFile.Name(),
						"data_dir":          dataDir,
					},
					"consul": map[string]interface{}{
						"agent": map[string]interface{}{
							"services": map[string]interface{}{
								"router": map[string]interface{}{
									"name": "gorouter",
								},
							},
							"servers": map[string]interface{}{
								"lan": []string{"member-1"},
							},
						},
					},
				})
			})

			AfterEach(func() {
				err := os.Remove(configFile.Name())
				Expect(err).NotTo(HaveOccurred())
			})

			It("returns an error and exits with status 1", func() {
				if Windows {
					Skip("Not implemented on Windows")
				}
				err := os.Chmod(consulConfigDir, 0000)
				Expect(err).NotTo(HaveOccurred())

				cmd := exec.Command(pathToConfab,
					"start",
					"--config-file", configFile.Name(),
				)
				buffer := bytes.NewBuffer([]byte{})
				cmd.Stderr = buffer
				Eventually(cmd.Run, COMMAND_TIMEOUT, COMMAND_TIMEOUT).ShouldNot(Succeed())
				Expect(buffer).To(ContainSubstring(fmt.Sprintf("error during start: open %s: permission denied", filepath.Join(consulConfigDir, "config.json"))))
			})
		})
	})
})
