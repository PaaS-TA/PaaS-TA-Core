package agent_test

import (
	"errors"
	"fmt"
	"io/ioutil"
	"math/rand"
	"os"
	"path/filepath"
	"runtime"

	"code.cloudfoundry.org/lager"

	"github.com/cloudfoundry-incubator/consul-release/src/confab/agent"
	"github.com/cloudfoundry-incubator/consul-release/src/confab/fakes"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	. "github.com/pivotal-cf-experimental/gomegamatchers"
)

var _ = Describe("Runner", func() {
	var (
		runner *agent.Runner
		logger *fakes.Logger
	)

	BeforeEach(func() {
		var err error
		configDir, err := ioutil.TempDir("", "fake-agent-config-dir")
		Expect(err).NotTo(HaveOccurred())

		pidFile, err := ioutil.TempFile("", "fake-agent-pid")
		Expect(err).NotTo(HaveOccurred())
		pidFile.Close()
		os.Remove(pidFile.Name()) // so that the pid file doesn't exist at all
		pidFileName := pidFile.Name()

		logger = &fakes.Logger{}

		runner = &agent.Runner{
			Path:      pathToFakeProcess,
			ConfigDir: configDir,
			Recursors: []string{"8.8.8.8", "10.0.2.3"},
			PIDFile:   pidFileName,
			Logger:    logger,
			// Stdout:    os.Stdout,  // uncomment this to see output from test agent
			// Stderr:    os.Stderr,
		}
	})

	AfterEach(func() {
		os.Remove(runner.PIDFile)
		os.RemoveAll(runner.ConfigDir)
	})

	Describe("Cleanup", func() {
		It("deletes the PID file for the consul agent", func() {
			_, err := os.Stat(runner.PIDFile)
			Expect(err).To(BeAnOsIsNotExistError())

			file, err := os.Create(runner.PIDFile)
			Expect(err).NotTo(HaveOccurred())
			file.Close()

			_, err = os.Stat(runner.PIDFile)
			Expect(err).NotTo(HaveOccurred())

			err = runner.Cleanup()
			Expect(err).NotTo(HaveOccurred())

			_, err = os.Stat(runner.PIDFile)
			Expect(err).To(BeAnOsIsNotExistError())

			Expect(logger.Messages()).To(ContainSequence([]fakes.LoggerMessage{
				{
					Action: "agent-runner.cleanup.remove",
					Data: []lager.Data{{
						"pidfile": runner.PIDFile,
					}},
				},
				{
					Action: "agent-runner.cleanup.success",
				},
			}))
		})

		Context("when the PIDFile does not exist", func() {
			It("returns the error", func() {
				err := runner.Cleanup()
				Expect(err).To(BeAnOsIsNotExistError())
				Expect(logger.Messages()).To(ContainSequence([]fakes.LoggerMessage{
					{
						Action: "agent-runner.cleanup.remove",
						Data: []lager.Data{{
							"pidfile": runner.PIDFile,
						}},
					},
					{
						Action: "agent-runner.cleanup.remove.failed",
						Error:  errors.New(err.Error()),
						Data: []lager.Data{{
							"pidfile": runner.PIDFile,
						}},
					},
				}))
			})
		})
	})

	Describe("Stop", func() {
		It("kills the process", func() {
			By("launching the process, configured to spin", func() {
				Expect(ioutil.WriteFile(filepath.Join(runner.ConfigDir, "options.json"), []byte(`{ "WaitForHUP": true }`), 0600)).To(Succeed())
				Expect(runner.Run()).To(Succeed())
				Expect(runner.WritePID()).To(Succeed())
			})

			By("waiting for the process to start enough that it has ignored signals", func() {
				Eventually(func() error {
					_, err := os.Stat(filepath.Join(runner.ConfigDir, "fake-output.json"))
					return err
				}).Should(Succeed())
			})

			By("calling stop", func() {
				pid, err := getPID(runner)
				Expect(err).NotTo(HaveOccurred())

				Expect(runner.Stop()).To(Succeed())
				Expect(logger.Messages()).To(ContainSequence([]fakes.LoggerMessage{
					{
						Action: "agent-runner.stop.get-process",
					},
					{
						Action: "agent-runner.stop.get-process.result",
						Data: []lager.Data{{
							"pid": pid,
						}},
					},
					{
						Action: "agent-runner.stop.signal",
						Data: []lager.Data{{
							"pid": pid,
						}},
					},
					{
						Action: "agent-runner.stop.success",
					},
				}))
			})

			By("checking that the process no longer exists", func() {
				Eventually(runner.Exited).Should(BeTrue())
			})
		})

		Context("when the PID file cannot be read", func() {
			It("returns an error", func() {
				runner.PIDFile = "/tmp/nope-i-do-not-exist"
				err := runner.Stop()
				Expect(err).To(BeAnOsIsNotExistError())
				Expect(logger.Messages()).To(ContainSequence([]fakes.LoggerMessage{
					{
						Action: "agent-runner.stop.get-process",
					},
					{
						Action: "agent-runner.stop.get-process.failed",
						Error:  errors.New(err.Error()),
					},
				}))
			})
		})

		Context("when the PID file contains nonsense", func() {
			It("returns an error", func() {
				Expect(ioutil.WriteFile(runner.PIDFile, []byte("nonsense"), 0644)).To(Succeed())
				Expect(runner.Stop()).To(MatchError(ContainSubstring("ParseInt")))
			})
		})

		Context("when the PID file has the wrong PID", func() {
			It("returns an error", func() {
				Expect(ioutil.WriteFile(runner.PIDFile, []byte("-1"), 0644)).To(Succeed())
				Expect(runner.Stop()).To(HaveOccurred())
			})
		})
	})

	Describe("stop & wait", func() {
		It("stops the process / waits until it exits", func() {
			By("launching the process, configured to spin", func() {
				Expect(ioutil.WriteFile(filepath.Join(runner.ConfigDir, "options.json"), []byte(`{ "WaitForHUP": true }`), 0600)).To(Succeed())
				Expect(runner.Run()).To(Succeed())
				Expect(runner.WritePID()).To(Succeed())
			})

			By("waiting for the process to get started", func() {
				Eventually(func() error {
					_, err := os.Stat(filepath.Join(runner.ConfigDir, "fake-output.json"))
					return err
				}).Should(Succeed())
			})

			By("checking that the process has not exited", func() {
				Expect(runner.Exited()).To(BeFalse())
			})

			done := make(chan struct{})
			By("checking that Wait() blocks", func() {
				go func() {
					if err := runner.Wait(); err != nil {
						panic(fmt.Sprintf("%#v\n", err))
					}
					done <- struct{}{}
				}()
				Consistently(done, "100ms").ShouldNot(Receive())
			})

			By("stopping the process", func() {
				Expect(runner.Stop()).To(Succeed())
			})

			By("checking that Wait returns", func() {
				pid, err := getPID(runner)
				Expect(err).NotTo(HaveOccurred())

				Expect(runner.Wait()).To(Succeed())
				Expect(logger.Messages()).To(ContainSequence([]fakes.LoggerMessage{
					{
						Action: "agent-runner.wait.get-process",
					},
					{
						Action: "agent-runner.wait.get-process.result",
						Data: []lager.Data{{
							"pid": pid,
						}},
					},
					{
						Action: "agent-runner.wait.signal",
						Data: []lager.Data{{
							"pid": pid,
						}},
					},
					{
						Action: "agent-runner.wait.success",
					},
				}))
			})

			By("checking that the process no longer exists", func() {
				Eventually(runner.Exited).Should(BeTrue())
			})

			Eventually(done).Should(Receive())
		})

		Context("when the PID file cannot be read", func() {
			It("returns an error", func() {
				runner.PIDFile = "/tmp/nope-i-do-not-exist"
				err := runner.Wait()
				Expect(err).To(BeAnOsIsNotExistError())
				Expect(logger.Messages()).To(ContainSequence([]fakes.LoggerMessage{
					{
						Action: "agent-runner.wait.get-process",
					},
					{
						Action: "agent-runner.wait.get-process.failed",
						Error:  errors.New(err.Error()),
					},
				}))
			})
		})

		Context("when the PID file contains nonsense", func() {
			It("returns an error", func() {
				Expect(ioutil.WriteFile(runner.PIDFile, []byte("nonsense"), 0644)).To(Succeed())
				Expect(runner.Wait()).To(MatchError(ContainSubstring("ParseInt")))
			})
		})
	})

	Describe("WritePID", func() {
		BeforeEach(func() {
			Expect(runner.Run()).To(Succeed())
		})

		It("writes the pid of the agent process to the pid file", func() {
			Expect(runner.WritePID()).To(Succeed())

			pid, err := getPID(runner)
			Expect(err).NotTo(HaveOccurred())

			Expect(logger.Messages()).To(ContainSequence([]fakes.LoggerMessage{
				{
					Action: "agent-runner.run.write-pidfile",
					Data: []lager.Data{{
						"pid":  pid,
						"path": runner.PIDFile,
					}},
				},
			}))

			Expect(runner.Wait()).To(Succeed())
		})

		It("makes the pid file world readable", func() {
			Expect(runner.WritePID()).To(Succeed())

			fileInfo, err := os.Stat(runner.PIDFile)
			Expect(err).NotTo(HaveOccurred())
			if runtime.GOOS == "windows" {
				//Windows doesn't honor unix file permissions
				Expect(fileInfo.Mode().Perm()).To(BeEquivalentTo(0666))
			} else {
				Expect(fileInfo.Mode().Perm()).To(BeEquivalentTo(0644))
			}

			Expect(runner.Wait()).To(Succeed())
		})

		Context("when writing the PID file errors", func() {
			It("returns the error", func() {
				Expect(os.Mkdir(runner.PIDFile, os.ModeDir)).To(Succeed())
				err := runner.WritePID()
				Expect(err).To(MatchError(ContainSubstring("error writing PID file")))
				Expect(runner.WritePID()).To(MatchError(ContainSubstring("is a directory")))
			})
		})
	})

	Describe("Run", func() {
		It("starts the process", func() {
			Expect(runner.Run()).To(Succeed())
			Expect(runner.WritePID()).To(Succeed())

			pid, err := getPID(runner)
			Expect(err).NotTo(HaveOccurred())

			Expect(logger.Messages()).To(ContainSequence([]fakes.LoggerMessage{
				{
					Action: "agent-runner.run.start",
					Data: []lager.Data{{
						"cmd": runner.Path,
						"args": []string{
							"agent",
							fmt.Sprintf("-config-dir=%s", runner.ConfigDir),
							"-recursor=8.8.8.8",
							"-recursor=10.0.2.3",
						},
					}},
				},
				{
					Action: "agent-runner.run.success",
				},
			}))

			Expect(runner.Wait()).To(Succeed())
			pid, err = getPID(runner)
			Expect(err).NotTo(HaveOccurred())

			outputs := getFakeAgentOutput(runner)
			Expect(pid).To(Equal(outputs.PID))
		})

		It("sets the arguments correctly", func() {
			Expect(runner.Run()).To(Succeed())
			Expect(runner.WritePID()).To(Succeed())
			Expect(runner.Wait()).To(Succeed())

			Expect(getFakeAgentOutput(runner).Args).To(Equal([]string{
				"agent",
				fmt.Sprintf("-config-dir=%s", runner.ConfigDir),
				fmt.Sprintf("-recursor=%s", "8.8.8.8"),
				fmt.Sprintf("-recursor=%s", "10.0.2.3"),
			}))
		})

		It("returns without waiting for the process to exit", func() {
			Expect(ioutil.WriteFile(filepath.Join(runner.ConfigDir, "options.json"), []byte(`{ "WaitForHUP": true }`), 0600)).To(Succeed())
			done := make(chan struct{})
			go func() {
				runner.Run()
				Expect(runner.WritePID()).To(Succeed())
				done <- struct{}{}
			}()
			Eventually(done, "1s").Should(Receive())

			Expect(runner.Exited()).To(BeFalse())

			err := runner.Stop()
			Expect(err).NotTo(HaveOccurred())
		})

		It("wires up the stdout and stderr pipes", func() {
			stdoutBytes := newConcurrentSafeBuffer()
			stderrBytes := newConcurrentSafeBuffer()
			runner.Stdout = stdoutBytes
			runner.Stderr = stderrBytes

			Expect(runner.Run()).To(Succeed())
			Expect(runner.WritePID()).To(Succeed())
			Expect(runner.Wait()).To(Succeed())

			Expect(stdoutBytes.String()).To(Equal("some standard out"))
			Expect(stderrBytes.String()).To(Equal("some standard error"))
		})

		Context("when starting the process fails", func() {
			It("returns the error", func() {
				runner.Path = "/tmp/not-a-thing-we-can-launch"
				// If the command cannot start the
				// returned error type is undefined.
				err := runner.Run()
				Expect(err).ToNot(BeNil())

				Expect(logger.Messages()).To(ContainSequence([]fakes.LoggerMessage{
					{
						Action: "agent-runner.run.start",
						Data: []lager.Data{{
							"cmd": runner.Path,
							"args": []string{
								"agent",
								fmt.Sprintf("-config-dir=%s", runner.ConfigDir),
								"-recursor=8.8.8.8",
								"-recursor=10.0.2.3",
							},
						}},
					},
					{
						Action: "agent-runner.run.start.failed",
						Error:  errors.New(err.Error()),
						Data: []lager.Data{{
							"cmd": runner.Path,
							"args": []string{
								"agent",
								fmt.Sprintf("-config-dir=%s", runner.ConfigDir),
								"-recursor=8.8.8.8",
								"-recursor=10.0.2.3",
							},
						}},
					},
				}))
			})
		})

		Context("when the ConfigDir is missing", func() {
			It("returns an error immediately, without starting a process", func() {
				runner.ConfigDir = fmt.Sprintf("/tmp/this-directory-does-not-existi-%x", rand.Int31())
				Expect(runner.Run()).To(MatchError(ContainSubstring("config dir does not exist")))

				Expect(logger.Messages()).To(ContainSequence([]fakes.LoggerMessage{
					{
						Action: "agent-runner.run.config-dir-missing",
						Error:  fmt.Errorf("config dir does not exist: %s", runner.ConfigDir),
					},
				}))
			})
		})
	})
})
