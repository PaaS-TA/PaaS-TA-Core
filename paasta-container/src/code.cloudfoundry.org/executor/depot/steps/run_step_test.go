package steps_test

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"code.cloudfoundry.org/lager/lagertest"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"

	"code.cloudfoundry.org/bbs/models"
	"code.cloudfoundry.org/clock/fakeclock"
	"code.cloudfoundry.org/garden"
	"code.cloudfoundry.org/garden/gardenfakes"

	"code.cloudfoundry.org/executor"
	"code.cloudfoundry.org/executor/depot/log_streamer/fake_log_streamer"
	"code.cloudfoundry.org/executor/depot/steps"
	"code.cloudfoundry.org/executor/fakes"
)

var _ = Describe("RunAction", func() {
	var (
		step steps.Step

		runAction                           models.RunAction
		fakeStreamer                        *fake_log_streamer.FakeLogStreamer
		gardenClient                        *fakes.FakeGardenClient
		logger                              *lagertest.TestLogger
		fileDescriptorLimit, processesLimit uint64
		externalIP, internalIP              string
		portMappings                        []executor.PortMapping
		exportNetworkEnvVars                bool
		fakeClock                           *fakeclock.FakeClock
		suppressExitStatusCode              bool

		spawnedProcess *gardenfakes.FakeProcess
		runError       error
		testLogSource  string
	)

	BeforeEach(func() {
		fileDescriptorLimit = 17
		processesLimit = 1024
		suppressExitStatusCode = false
		testLogSource = "testlogsource"

		runAction = models.RunAction{
			Path: "sudo",
			Args: []string{"reboot"},
			Dir:  "/some-dir",
			Env: []*models.EnvironmentVariable{
				{Name: "A", Value: "1"},
				{Name: "B", Value: "2"},
			},
			ResourceLimits: &models.ResourceLimits{
				Nofile: &fileDescriptorLimit,
				Nproc:  &processesLimit,
			},
			User: "notroot",
		}

		fakeStreamer = new(fake_log_streamer.FakeLogStreamer)
		fakeStreamer.StdoutReturns(noOpWriter{})
		fakeStreamer.SourceNameReturns(testLogSource)
		gardenClient = fakes.NewGardenClient()

		logger = lagertest.NewTestLogger("test")

		spawnedProcess = new(gardenfakes.FakeProcess)
		runError = nil

		gardenClient.Connection.RunStub = func(string, garden.ProcessSpec, garden.ProcessIO) (garden.Process, error) {
			return spawnedProcess, runError
		}

		externalIP = "external-ip"
		internalIP = "internal-ip"
		portMappings = nil
		exportNetworkEnvVars = false
		fakeClock = fakeclock.NewFakeClock(time.Unix(123, 456))
	})

	handle := "some-container-handle"

	JustBeforeEach(func() {
		gardenClient.Connection.CreateReturns(handle, nil)

		container, err := gardenClient.Create(garden.ContainerSpec{})
		Expect(err).NotTo(HaveOccurred())

		step = steps.NewRun(
			container,
			runAction,
			fakeStreamer,
			logger,
			externalIP,
			internalIP,
			portMappings,
			exportNetworkEnvVars,
			fakeClock,
			suppressExitStatusCode,
		)
	})

	Describe("Perform", func() {
		var stepErr error

		JustBeforeEach(func() {
			stepErr = step.Perform()
		})

		Context("when the script succeeds", func() {
			BeforeEach(func() {
				gardenClient.Connection.RunStub = func(string, garden.ProcessSpec, garden.ProcessIO) (garden.Process, error) {
					fakeClock.Increment(time.Minute)
					return spawnedProcess, runError
				}
				spawnedProcess.WaitReturns(0, nil)
			})

			It("does not return an error", func() {
				Expect(stepErr).NotTo(HaveOccurred())
			})

			It("executes the command in the passed-in container", func() {
				ranHandle, spec, _ := gardenClient.Connection.RunArgsForCall(0)
				Expect(ranHandle).To(Equal(handle))
				Expect(spec.Path).To(Equal("sudo"))
				Expect(spec.Args).To(Equal([]string{"reboot"}))
				Expect(spec.Dir).To(Equal("/some-dir"))
				Expect(*spec.Limits.Nofile).To(BeNumerically("==", fileDescriptorLimit))
				Expect(*spec.Limits.Nproc).To(BeNumerically("==", processesLimit))
				Expect(spec.Env).To(ContainElement("A=1"))
				Expect(spec.Env).To(ContainElement("B=2"))
				Expect(spec.User).To(Equal("notroot"))
			})

			It("logs the step", func() {
				Expect(logger.TestSink.LogMessages()).To(ConsistOf([]string{
					"test.run-step.running",
					"test.run-step.creating-process",
					"test.run-step.successful-process-create",
					"test.run-step.process-exit",
				}))

			})

			It("logs the duration for process creation", func() {
				Eventually(logger).Should(gbytes.Say("test.run-step.successful-process-create.+\"duration\":%d", time.Minute))
			})
		})

		Context("when the script fails", func() {
			var waitErr error

			BeforeEach(func() {
				waitErr = errors.New("wait-error")
				spawnedProcess.WaitReturns(0, waitErr)
			})

			Context("when logs are suppressed", func() {
				BeforeEach(func() {
					runAction.SuppressLogOutput = true
				})

				It("returns an error", func() {
					Expect(stepErr).To(MatchError(waitErr))
				})

				It("logs the step", func() {
					Expect(logger.TestSink.LogMessages()).To(ConsistOf([]string{
						"test.run-step.running",
						"test.run-step.creating-process",
						"test.run-step.successful-process-create",
						"test.run-step.running-error",
					}))

				})
			})

			Context("when logs are not suppressed", func() {
				BeforeEach(func() {
					runAction.SuppressLogOutput = false
				})

				It("returns an error", func() {
					Expect(stepErr).To(MatchError(waitErr))
				})

				It("logs the step", func() {
					Expect(logger.TestSink.LogMessages()).To(ConsistOf([]string{
						"test.run-step.running",
						"test.run-step.creating-process",
						"test.run-step.successful-process-create",
						"test.run-step.running-error",
					}))

				})
			})
		})

		Context("CF_INSTANCE_* networking env vars", func() {
			Context("when exportNetworkEnvVars is set to true", func() {
				BeforeEach(func() {
					exportNetworkEnvVars = true
				})

				It("sets CF_INSTANCE_IP on the container", func() {
					_, spec, _ := gardenClient.Connection.RunArgsForCall(0)
					Expect(spec.Env).To(ContainElement("CF_INSTANCE_IP=external-ip"))
				})

				It("sets CF_INSTANCE_INTERNAL_IP on the container", func() {
					_, spec, _ := gardenClient.Connection.RunArgsForCall(0)
					Expect(spec.Env).To(ContainElement("CF_INSTANCE_INTERNAL_IP=internal-ip"))
				})

				Context("when the container has port mappings configured", func() {
					BeforeEach(func() {
						portMappings = []executor.PortMapping{
							{HostPort: 1, ContainerPort: 2},
							{HostPort: 3, ContainerPort: 4},
						}
					})

					It("sets CF_INSTANCE_* networking env vars", func() {
						_, spec, _ := gardenClient.Connection.RunArgsForCall(0)
						Expect(spec.Env).To(ContainElement("CF_INSTANCE_PORT=1"))
						Expect(spec.Env).To(ContainElement("CF_INSTANCE_ADDR=external-ip:1"))

						var cfPortsValue string
						for _, env := range spec.Env {
							if strings.HasPrefix(env, "CF_INSTANCE_PORTS=") {
								cfPortsValue = strings.Split(env, "=")[1]
								break
							}
						}
						Expect(cfPortsValue).To(MatchJSON("[{\"internal\":2,\"external\":1},{\"internal\":4,\"external\":3}]"))
					})
				})

				Context("when the container does not have any port mappings configured", func() {
					BeforeEach(func() {
						portMappings = []executor.PortMapping{}
					})

					It("sets all port-related env vars to the empty string", func() {
						_, spec, _ := gardenClient.Connection.RunArgsForCall(0)
						Expect(spec.Env).To(ContainElement("CF_INSTANCE_PORT="))
						Expect(spec.Env).To(ContainElement("CF_INSTANCE_ADDR="))
						Expect(spec.Env).To(ContainElement("CF_INSTANCE_PORTS=[]"))
					})
				})
			})

			Context("when exportNetworkEnvVars is set to false", func() {
				BeforeEach(func() {
					exportNetworkEnvVars = false
				})

				It("does not set CF_INSTANCE_IP on the container", func() {
					_, spec, _ := gardenClient.Connection.RunArgsForCall(0)
					Expect(spec.Env).NotTo(ContainElement("CF_INSTANCE_IP=external-ip"))
				})
			})
		})

		Context("when resource limits are not configured", func() {
			BeforeEach(func() {
				runAction.ResourceLimits = nil
				spawnedProcess.WaitReturns(0, nil)
			})

			It("does not enforce a file descriptor limit on the process", func() {
				_, spec, _ := gardenClient.Connection.RunArgsForCall(0)
				Expect(spec.Limits.Nofile).To(BeNil())
			})

			It("does not enforce a process limit on the process", func() {
				_, spec, _ := gardenClient.Connection.RunArgsForCall(0)
				Expect(spec.Limits.Nproc).To(BeNil())
			})
		})

		Context("when the script has a non-zero exit code", func() {
			BeforeEach(func() {
				spawnedProcess.WaitReturns(19, nil)
			})

			Context("when logs are not suppressed", func() {
				BeforeEach(func() {
					runAction.SuppressLogOutput = false
				})

				It("should return an emittable error with the exit code", func() {
					errMsg := fmt.Sprintf("%s: Exited with status 19", testLogSource)
					Expect(stepErr).To(MatchError(steps.NewEmittableError(nil, errMsg)))
				})
			})

			Context("when logs are suppressed", func() {
				BeforeEach(func() {
					runAction.SuppressLogOutput = true
				})

				It("should return an emittable error with the exit code", func() {
					errMsg := fmt.Sprintf("%s: Exited with status 19", testLogSource)
					Expect(stepErr).To(MatchError(steps.NewEmittableError(nil, errMsg)))
				})
			})
		})

		Context("when Garden errors", func() {
			disaster := errors.New("I, like, tried but failed")

			BeforeEach(func() {
				runError = disaster
			})

			It("returns the error", func() {
				Expect(stepErr).To(Equal(disaster))
			})

			It("logs the step", func() {
				Expect(logger.TestSink.LogMessages()).To(ConsistOf([]string{
					"test.run-step.running",
					"test.run-step.creating-process",
					"test.run-step.failed-creating-process",
				}))
			})
			Context("", func() {
				BeforeEach(func() {
					gardenClient.Connection.RunStub = func(string, garden.ProcessSpec, garden.ProcessIO) (garden.Process, error) {
						fakeClock.Increment(time.Minute)
						return spawnedProcess, runError
					}
				})
				It("logs the duration for process creation", func() {
					Eventually(logger).Should(gbytes.Say("test.run-step.failed-creating-process.+\"duration\":%d", time.Minute))
				})
			})
		})

		// Garden-RunC capitalizes out the O in out of memory whereas Garden-linux does not
		Context("regardless of status code, when an Out of memory event has occured", func() {
			BeforeEach(func() {
				gardenClient.Connection.InfoReturns(
					garden.ContainerInfo{
						Events: []string{"happy land", "Out of memory", "another event"},
					},
					nil,
				)

				spawnedProcess.WaitReturns(19, nil)
			})

			It("returns an emittable error", func() {
				errMsg := fmt.Sprintf("%s: Exited with status 19 (out of memory)", testLogSource)
				Expect(stepErr).To(MatchError(steps.NewEmittableError(nil, errMsg)))
			})
		})

		Context("regardless of status code, when an out of memory event has occured", func() {
			BeforeEach(func() {
				gardenClient.Connection.InfoReturns(
					garden.ContainerInfo{
						Events: []string{"happy land", "out of memory", "another event"},
					},
					nil,
				)

				spawnedProcess.WaitReturns(19, nil)
			})

			It("returns an emittable error", func() {
				errMsg := fmt.Sprintf("%s: Exited with status 19 (out of memory)", testLogSource)
				Expect(stepErr).To(MatchError(steps.NewEmittableError(nil, errMsg)))
			})
		})

		Context("when container info cannot be retrieved", func() {
			BeforeEach(func() {
				gardenClient.Connection.InfoReturns(garden.ContainerInfo{}, errors.New("info-error"))
				spawnedProcess.WaitReturns(19, nil)
			})

			It("logs the step", func() {
				Expect(logger.TestSink.LogMessages()).To(ConsistOf([]string{
					"test.run-step.running",
					"test.run-step.creating-process",
					"test.run-step.successful-process-create",
					"test.run-step.process-exit",
					"test.run-step.failed-to-get-info",
					"test.run-step.run-step-failed-with-nonzero-status-code",
				}))

			})
		})

		Describe("emitting logs", func() {
			var stdoutBuffer, stderrBuffer *gbytes.Buffer

			BeforeEach(func() {
				stdoutBuffer = gbytes.NewBuffer()
				stderrBuffer = gbytes.NewBuffer()

				fakeStreamer.StdoutReturns(stdoutBuffer)
				fakeStreamer.StderrReturns(stderrBuffer)

				spawnedProcess.WaitStub = func() (int, error) {
					_, _, io := gardenClient.Connection.RunArgsForCall(0)

					_, err := io.Stdout.Write([]byte("hi out"))
					Expect(err).NotTo(HaveOccurred())

					_, err = io.Stderr.Write([]byte("hi err"))
					Expect(err).NotTo(HaveOccurred())

					return 34, nil
				}
			})

			Context("when logs are not suppressed", func() {

				It("emits the output chunks as they come in", func() {
					Expect(stdoutBuffer).To(gbytes.Say("hi out"))
					Expect(stderrBuffer).To(gbytes.Say("hi err"))
				})

				It("should flush the output when the code exits", func() {
					Expect(fakeStreamer.FlushCallCount()).To(Equal(1))
				})

				It("emits the exit status code", func() {
					Expect(stdoutBuffer).To(gbytes.Say("Exit status 34"))
				})

				Context("when exit code suppressed for healthcheck", func() {
					BeforeEach(func() {
						suppressExitStatusCode = true
					})

					It("does not emits the exit status code", func() {
						Expect(stdoutBuffer).NotTo(gbytes.Say("Exit status 34"))
					})
				})

				Context("when out of memory", func() {
					BeforeEach(func() {
						gardenClient.Connection.InfoReturns(
							garden.ContainerInfo{
								Events: []string{"happy land", "Out of memory", "another event"},
							},
							nil,
						)

						spawnedProcess.WaitReturns(34, nil)
					})

					It("emits the exit status code", func() {
						Expect(stdoutBuffer).To(gbytes.Say(`Exit status 34 \(out of memory\)`))
					})

					Context("when there are multiple out of memory events", func() {
						BeforeEach(func() {
							gardenClient.Connection.InfoReturns(
								garden.ContainerInfo{
									Events: []string{"Out of memory", "Out of memory", "another event"},
								},
								nil,
							)

							spawnedProcess.WaitReturns(34, nil)
						})

						It("emits only one out of memory error", func() {
							Expect(stdoutBuffer).To(gbytes.Say("Exit status 34"))
							Expect(stdoutBuffer).To(gbytes.Say(`(out of memory)`))
							Expect(stdoutBuffer).ToNot(gbytes.Say(`(out of memory)`))
						})
					})

					Context("when exit code suppressed for healthcheck", func() {
						BeforeEach(func() {
							suppressExitStatusCode = true
						})

						It("does not emits the exit status code", func() {
							Expect(stdoutBuffer).ToNot(gbytes.Say("Exit status 34"))
							Expect(stdoutBuffer).To(gbytes.Say(`(out of memory)`))
						})
					})
				})
			})

			Context("when logs are suppressed", func() {

				BeforeEach(func() {
					runAction.SuppressLogOutput = true
				})

				It("does not emit the output chunks as they come in", func() {
					Expect(stdoutBuffer).ToNot(gbytes.Say("hi out"))
					Expect(stderrBuffer).ToNot(gbytes.Say("hi err"))
				})

				It("does not emit the exit status code", func() {
					Expect(stdoutBuffer).ToNot(gbytes.Say("Exit status 34"))
				})
			})

		})
	})

	Describe("Cancel", func() {
		var (
			performErr chan error

			waiting    chan struct{}
			waitExited chan int
		)

		BeforeEach(func() {
			performErr = make(chan error)

			waitingCh := make(chan struct{})
			waiting = waitingCh

			waitExitedCh := make(chan int, 1)
			waitExited = waitExitedCh

			spawnedProcess.WaitStub = func() (int, error) {
				close(waitingCh)
				return <-waitExitedCh, nil
			}
		})

		Context("when cancelling after perform", func() {
			JustBeforeEach(func() {
				go func() {
					performErr <- step.Perform()
					close(performErr)
				}()

				Eventually(waiting).Should(BeClosed())
				step.Cancel()
			})

			AfterEach(func() {
				close(waitExited)
				Eventually(performErr).Should(BeClosed())
			})

			It("sends an interrupt to the process", func() {
				Eventually(spawnedProcess.SignalCallCount).Should(Equal(1))
				Expect(spawnedProcess.SignalArgsForCall(0)).To(Equal(garden.SignalTerminate))
			})

			Context("when the process exits", func() {
				It("completes the perform without having sent kill", func() {
					Eventually(spawnedProcess.SignalCallCount).Should(Equal(1))

					waitExited <- (128 + 15)

					Eventually(performErr).Should(Receive(Equal(steps.ErrCancelled)))

					Expect(spawnedProcess.SignalCallCount()).To(Equal(1))
					Expect(spawnedProcess.SignalArgsForCall(0)).To(Equal(garden.SignalTerminate))
				})
			})

			Context("when the process does not exit after 10s", func() {
				It("sends a kill signal to the process", func() {
					Eventually(spawnedProcess.SignalCallCount).Should(Equal(1))

					fakeClock.WaitForWatcherAndIncrement(steps.TerminateTimeout + 1*time.Second)

					Eventually(spawnedProcess.SignalCallCount).Should(Equal(2))
					Expect(spawnedProcess.SignalArgsForCall(1)).To(Equal(garden.SignalKill))

					waitExited <- (128 + 9)

					Eventually(performErr).Should(Receive(Equal(steps.ErrCancelled)))
				})

				Context("when the process *still* does not exit after 1m", func() {
					It("finishes performing with failure", func() {
						Eventually(spawnedProcess.SignalCallCount).Should(Equal(1))

						fakeClock.WaitForWatcherAndIncrement(steps.TerminateTimeout)

						Eventually(spawnedProcess.SignalCallCount).Should(Equal(2))
						Expect(spawnedProcess.SignalArgsForCall(1)).To(Equal(garden.SignalKill))

						fakeClock.WaitForWatcherAndIncrement(steps.ExitTimeout / 2)

						Consistently(performErr).ShouldNot(Receive())

						fakeClock.WaitForWatcherAndIncrement(steps.ExitTimeout / 2)

						Eventually(performErr).Should(Receive(Equal(steps.ErrExitTimeout)))

						Expect(logger.TestSink.LogMessages()).To(ContainElement(
							ContainSubstring("process-did-not-exit"),
						))
					})
				})
			})

		})

		Context("when Garden hangs on spawning a process", func() {
			var hangChan chan struct{}
			BeforeEach(func() {
				hangChan = make(chan struct{})
				gardenClient.Connection.RunStub = func(string, garden.ProcessSpec, garden.ProcessIO) (garden.Process, error) {
					//hang until test is finished
					<-hangChan
					return nil, nil
				}
			})

			JustBeforeEach(func() {
				go func() {
					performErr <- step.Perform()
					close(performErr)
				}()

				Eventually(gardenClient.Connection.RunCallCount).Should(Equal(1))
				step.Cancel()
			})

			AfterEach(func() {
				close(hangChan)
				Eventually(performErr).Should(BeClosed())
			})

			It("finishes performing with failure", func() {
				Eventually(performErr).Should(Receive(Equal(steps.ErrCancelled)))
			})
		})

		Context("when cancelling before perform", func() {
			JustBeforeEach(func() {
				step.Cancel()

				go func() {
					performErr <- step.Perform()
					close(performErr)
				}()
			})

			AfterEach(func() {
				close(waitExited)
				Eventually(performErr).Should(BeClosed())
			})

			It("sends an interrupt to the process", func() {
				Consistently(waiting).ShouldNot(BeClosed())
			})
		})
	})
})

type noOpWriter struct{}

func (w noOpWriter) Write(b []byte) (int, error) { return len(b), nil }
