package chaperon_test

import (
	"errors"
	"time"

	"code.cloudfoundry.org/lager"

	"github.com/cloudfoundry-incubator/consul-release/src/confab/agent"
	"github.com/cloudfoundry-incubator/consul-release/src/confab/chaperon"
	"github.com/cloudfoundry-incubator/consul-release/src/confab/config"
	"github.com/cloudfoundry-incubator/consul-release/src/confab/fakes"
	"github.com/cloudfoundry-incubator/consul-release/src/confab/utils"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	. "github.com/pivotal-cf-experimental/gomegamatchers"
)

var _ = Describe("Controller", func() {
	var (
		clock          *fakes.Clock
		agentRunner    *fakes.AgentRunner
		agentClient    *fakes.AgentClient
		logger         *fakes.Logger
		serviceDefiner *fakes.ServiceDefiner
		controller     chaperon.Controller
	)

	BeforeEach(func() {
		clock = &fakes.Clock{}
		logger = &fakes.Logger{}

		agentClient = &fakes.AgentClient{}
		agentClient.VerifySyncedCalls.Returns.Errors = []error{nil}

		agentRunner = &fakes.AgentRunner{}
		agentRunner.RunCalls.Returns.Errors = []error{nil}

		serviceDefiner = &fakes.ServiceDefiner{}

		confabConfig := config.Config{}
		confabConfig.Node = config.ConfigNode{Name: "node", Index: 0}

		controller = chaperon.Controller{
			AgentClient:    agentClient,
			AgentRunner:    agentRunner,
			Retrier:        utils.NewRetrier(clock, 10*time.Millisecond),
			EncryptKeys:    []string{"key 1", "key 2", "key 3"},
			Logger:         logger,
			ConfigDir:      "/tmp/config",
			ServiceDefiner: serviceDefiner,
			Config:         confabConfig,
		}
	})

	Describe("ConfigureClient", func() {
		It("writes the pid file", func() {
			err := controller.ConfigureClient()
			Expect(err).NotTo(HaveOccurred())

			Expect(agentRunner.WritePIDCall.CallCount).To(Equal(1))
		})

		Context("failure cases", func() {
			It("returns an error when the pid file can not be written", func() {
				agentRunner.WritePIDCall.Returns.Error = errors.New("something bad happened")

				err := controller.ConfigureClient()
				Expect(err).To(MatchError("something bad happened"))
			})
		})
	})

	Describe("WriteServiceDefinitions", func() {
		It("delegates to the service definer", func() {
			definitions := []config.ServiceDefinition{{
				Name: "banana",
			}}
			serviceDefiner.GenerateDefinitionsCall.Returns.Definitions = definitions

			Expect(controller.WriteServiceDefinitions()).To(Succeed())
			Expect(serviceDefiner.GenerateDefinitionsCall.Receives.Config).To(Equal(controller.Config))
			Expect(serviceDefiner.WriteDefinitionsCall.Receives.ConfigDir).To(Equal("/tmp/config"))
			Expect(serviceDefiner.WriteDefinitionsCall.Receives.Definitions).To(Equal(definitions))

			Expect(logger.Messages()).To(ContainSequence([]fakes.LoggerMessage{
				{
					Action: "controller.write-service-definitions.generate-definitions",
				},
				{
					Action: "controller.write-service-definitions.write",
				},
				{
					Action: "controller.write-service-definitions.success",
				},
			}))
		})

		Context("when there is an error", func() {
			It("returns the error", func() {
				serviceDefiner.WriteDefinitionsCall.Returns.Error = errors.New("write definitions error")

				err := controller.WriteServiceDefinitions()
				Expect(err).To(MatchError(errors.New("write definitions error")))

				Expect(logger.Messages()).To(ContainSequence([]fakes.LoggerMessage{
					{
						Action: "controller.write-service-definitions.generate-definitions",
					},
					{
						Action: "controller.write-service-definitions.write",
					},
					{
						Action: "controller.write-service-definitions.write.failed",
						Error:  errors.New("write definitions error"),
					},
				}))
			})
		})
	})

	Describe("BootAgent", func() {
		It("launches the consul agent and confirms that it joined the cluster", func() {
			Expect(controller.BootAgent(utils.NewTimeout(make(chan time.Time)))).To(Succeed())

			Expect(agentClient.JoinMembersCall.CallCount).To(Equal(1))

			Expect(agentRunner.RunCalls.CallCount).To(Equal(1))
			Expect(agentClient.VerifyJoinedCalls.CallCount).To(Equal(1))
			Expect(agentClient.SelfCall.CallCount).To(Equal(1))
			Expect(logger.Messages()).To(ContainSequence([]fakes.LoggerMessage{
				{
					Action: "controller.boot-agent.run",
				},
				{
					Action: "controller.boot-agent.agent-client.waiting-for-agent",
				},
				{
					Action: "controller.boot-agent.agent-client.join-members",
				},
				{
					Action: "controller.boot-agent.verify-joined",
				},
				{
					Action: "controller.boot-agent.success",
				},
			}))
		})

		Context("when starting the agent fails", func() {
			It("immediately returns an error", func() {
				agentRunner.RunCalls.Returns.Errors = []error{errors.New("some error")}

				Expect(controller.BootAgent(utils.NewTimeout(make(chan time.Time)))).To(MatchError("some error"))
				Expect(agentRunner.RunCalls.CallCount).To(Equal(1))
				Expect(agentClient.JoinMembersCall.CallCount).To(Equal(0))
				Expect(agentClient.VerifyJoinedCalls.CallCount).To(Equal(0))
				Expect(logger.Messages()).To(ContainSequence([]fakes.LoggerMessage{
					{
						Action: "controller.boot-agent.run",
					},
					{
						Action: "controller.boot-agent.run.failed",
						Error:  errors.New("some error"),
					},
				}))
			})
		})

		Context("when the client does not respond within given timeout", func() {
			It("retries self call until it succeeds", func() {
				agentClient.SelfCall.Returns.Errors = make([]error, 10)
				for i := 0; i < 9; i++ {
					agentClient.SelfCall.Returns.Errors[i] = errors.New("some error occurred")
				}
				err := controller.BootAgent(utils.NewTimeout(make(chan time.Time)))
				Expect(err).NotTo(HaveOccurred())
				Expect(clock.SleepCall.CallCount).To(Equal(9))
				Expect(clock.SleepCall.Receives.Duration).To(Equal(10 * time.Millisecond))
				Expect(agentClient.SelfCall.CallCount).To(Equal(10))
				Expect(logger.Messages()).To(ContainSequence([]fakes.LoggerMessage{
					{
						Action: "controller.boot-agent.agent-client.waiting-for-agent",
					},
				}))
			})

			It("returns an error after timeout", func() {
				agentClient.SelfCall.Returns.Error = errors.New("some error occurred")

				timeout := utils.NewTimeout(time.After(20 * time.Millisecond))

				err := controller.BootAgent(timeout)
				Expect(err).To(MatchError(`timeout exceeded: "some error occurred"`))
				Expect(clock.SleepCall.CallCount).NotTo(Equal(0))

				Expect(agentClient.SelfCall.CallCount).NotTo(Equal(0))
				Expect(logger.Messages()).To(ContainSequence([]fakes.LoggerMessage{
					{
						Action: "controller.boot-agent.agent-client.waiting-for-agent",
					},
				}))

			})
		})

		Context("when join members fails", func() {
			Context("when fails to join any members", func() {
				It("ignores and continue to bootstrap", func() {
					agentClient.JoinMembersCall.Returns.Error = agent.NoMembersToJoinError
					err := controller.BootAgent(utils.NewTimeout(make(chan time.Time)))
					Expect(err).NotTo(HaveOccurred())
					Expect(agentRunner.RunCalls.CallCount).To(Equal(1))
					Expect(agentClient.JoinMembersCall.CallCount).To(Equal(1))
					Expect(agentClient.VerifyJoinedCalls.CallCount).To(Equal(1))

					Expect(logger.Messages()).To(ContainSequence([]fakes.LoggerMessage{
						{
							Action: "controller.boot-agent.agent-client.join-members",
						},
						{
							Action: "controller.boot-agent.agent-client.join-members.no-members-to-join",
							Error:  agent.NoMembersToJoinError,
						},
					}))

				})
			})

			Context("when fails with any other error", func() {
				It("returns an error", func() {
					agentClient.JoinMembersCall.Returns.Error = errors.New("some error")
					err := controller.BootAgent(utils.NewTimeout(make(chan time.Time)))
					Expect(err).To(MatchError("some error"))

					Expect(logger.Messages()).To(ContainSequence([]fakes.LoggerMessage{
						{
							Action: "controller.boot-agent.agent-client.join-members",
						},
						{
							Action: "controller.boot-agent.agent-client.join-members.failed",
							Error:  errors.New("some error"),
						},
					}))
				})

			})
		})

		Context("joining fails", func() {
			It("returns an errors", func() {
				agentClient.VerifyJoinedCalls.Returns.Error = errors.New("some error")
				err := controller.BootAgent(utils.NewTimeout(make(chan time.Time)))
				Expect(err).To(MatchError("some error"))
				Expect(agentClient.VerifyJoinedCalls.CallCount).To(Equal(1))
				Expect(logger.Messages()).To(ContainSequence([]fakes.LoggerMessage{
					{
						Action: "controller.boot-agent.verify-joined",
					},
					{
						Action: "controller.boot-agent.verify-joined.failed",
						Error:  err,
					},
				}))
			})
		})
	})

	Describe("StopAgent", func() {
		It("tells client to leave the cluster and waits for the agent to stop", func() {
			controller.StopAgent()
			Expect(agentClient.LeaveCall.CallCount).To(Equal(1))
			Expect(agentRunner.WaitCall.CallCount).To(Equal(1))
			Expect(agentRunner.CleanupCall.CallCount).To(Equal(1))
			Expect(logger.Messages()).To(ContainSequence([]fakes.LoggerMessage{
				{
					Action: "controller.stop-agent.leave",
				},
				{
					Action: "controller.stop-agent.wait",
				},
				{
					Action: "controller.stop-agent.cleanup",
				},
				{
					Action: "controller.stop-agent.success",
				},
			}))
		})

		Context("when the agent client Leave() returns an error", func() {
			BeforeEach(func() {
				agentClient.LeaveCall.Returns.Error = errors.New("leave error")
			})

			It("tells the runner to stop the agent", func() {
				controller.StopAgent()
				Expect(agentRunner.StopCall.CallCount).To(Equal(1))
				Expect(agentRunner.WaitCall.CallCount).To(Equal(1))
				Expect(agentRunner.CleanupCall.CallCount).To(Equal(1))
				Expect(logger.Messages()).To(ContainSequence([]fakes.LoggerMessage{
					{
						Action: "controller.stop-agent.leave",
					},
					{
						Action: "controller.stop-agent.leave.failed",
						Error:  errors.New("leave error"),
					},
					{
						Action: "controller.stop-agent.stop",
					},
					{
						Action: "controller.stop-agent.wait",
					},
					{
						Action: "controller.stop-agent.cleanup",
					},
					{
						Action: "controller.stop-agent.success",
					},
				}))
			})

			Context("when agent runner Stop() returns an error", func() {
				BeforeEach(func() {
					agentRunner.StopCall.Returns.Error = errors.New("stop error")
				})

				It("logs the error", func() {
					controller.StopAgent()
					Expect(logger.Messages()).To(ContainSequence([]fakes.LoggerMessage{
						{
							Action: "controller.stop-agent.leave",
						},
						{
							Action: "controller.stop-agent.leave.failed",
							Error:  errors.New("leave error"),
						},
						{
							Action: "controller.stop-agent.stop",
						},
						{
							Action: "controller.stop-agent.stop.failed",
							Error:  errors.New("stop error"),
						},
						{
							Action: "controller.stop-agent.wait",
						},
						{
							Action: "controller.stop-agent.cleanup",
						},
						{
							Action: "controller.stop-agent.success",
						},
					}))
				})
			})
		})

		Context("when agent runner Wait() returns an error", func() {
			BeforeEach(func() {
				agentRunner.WaitCall.Returns.Error = errors.New("wait error")
			})

			It("logs the error", func() {
				controller.StopAgent()
				Expect(logger.Messages()).To(ContainSequence([]fakes.LoggerMessage{
					{
						Action: "controller.stop-agent.leave",
					},
					{
						Action: "controller.stop-agent.wait",
					},
					{
						Action: "controller.stop-agent.wait.failed",
						Error:  errors.New("wait error"),
					},
					{
						Action: "controller.stop-agent.cleanup",
					},
					{
						Action: "controller.stop-agent.success",
					},
				}))
			})
		})

		Context("when agent runner Cleanup() returns an error", func() {
			BeforeEach(func() {
				agentRunner.CleanupCall.Returns.Error = errors.New("cleanup error")
			})

			It("logs the error", func() {
				controller.StopAgent()
				Expect(logger.Messages()).To(ContainSequence([]fakes.LoggerMessage{
					{
						Action: "controller.stop-agent.leave",
					},
					{
						Action: "controller.stop-agent.wait",
					},
					{
						Action: "controller.stop-agent.cleanup",
					},
					{
						Action: "controller.stop-agent.cleanup.failed",
						Error:  errors.New("cleanup error"),
					},
					{
						Action: "controller.stop-agent.success",
					},
				}))
			})
		})
	})

	Describe("ConfigureServer", func() {
		var (
			timeout utils.Timeout
		)

		BeforeEach(func() {
			timeout = utils.NewTimeout(make(chan time.Time))
		})

		Context("setting keys", func() {
			It("sets the encryption keys used by the agent", func() {
				Expect(controller.ConfigureServer(timeout)).To(Succeed())
				Expect(agentClient.SetKeysCall.Receives.Keys).To(Equal([]string{
					"key 1",
					"key 2",
					"key 3",
				}))
				Expect(logger.Messages()).To(ContainSequence([]fakes.LoggerMessage{
					{
						Action: "controller.configure-server.set-keys",
						Data: []lager.Data{{
							"keys": []string{"key 1", "key 2", "key 3"},
						}},
					},
					{
						Action: "controller.configure-server.success",
					},
				}))
			})

			Context("when setting keys errors", func() {
				It("returns the error", func() {
					timeout = utils.NewTimeout(time.After(10 * time.Millisecond))
					agentClient.SetKeysCall.Returns.Error = errors.New("oh noes")

					Expect(controller.ConfigureServer(timeout)).To(MatchError(`timeout exceeded: "oh noes"`))
					Expect(agentClient.SetKeysCall.Receives.Keys).To(Equal([]string{
						"key 1",
						"key 2",
						"key 3",
					}))
					Expect(agentRunner.WritePIDCall.CallCount).To(Equal(0))
					Expect(logger.Messages()).To(ContainSequence([]fakes.LoggerMessage{
						{
							Action: "controller.configure-server.set-keys",
							Data: []lager.Data{{
								"keys": []string{"key 1", "key 2", "key 3"},
							}},
						},
						{
							Action: "controller.configure-server.set-keys.failed",
							Error:  errors.New(`timeout exceeded: "oh noes"`),
							Data: []lager.Data{{
								"keys": []string{"key 1", "key 2", "key 3"},
							}},
						},
					}))
				})
			})

			Context("when ssl is enabled but no keys are provided", func() {
				BeforeEach(func() {
					controller.EncryptKeys = []string{}
				})

				It("returns an error", func() {
					Expect(controller.ConfigureServer(timeout)).To(MatchError("encrypt keys cannot be empty if ssl is enabled"))
					Expect(agentClient.SetKeysCall.Receives.Keys).To(BeNil())
					Expect(agentRunner.WritePIDCall.CallCount).To(Equal(0))

					Expect(logger.Messages()).To(ContainSequence([]fakes.LoggerMessage{
						{
							Action: "controller.configure-server.no-encrypt-keys",
							Error:  errors.New("encrypt keys cannot be empty if ssl is enabled"),
						},
					}))
				})
			})
		})

		Context("when starting the server", func() {
			It("checks that it is synced", func() {
				Expect(controller.ConfigureServer(timeout)).To(Succeed())
				Expect(agentClient.VerifySyncedCalls.CallCount).To(Equal(1))
				Expect(agentRunner.WritePIDCall.CallCount).To(Equal(1))

				Expect(logger.Messages()).To(ContainSequence([]fakes.LoggerMessage{
					{
						Action: "controller.configure-server.verify-synced",
					},
					{
						Action: "controller.configure-server.set-keys",
						Data: []lager.Data{{
							"keys": []string{"key 1", "key 2", "key 3"},
						}},
					},
					{
						Action: "controller.configure-server.success",
					},
				}))
			})

			Context("verifying sync fails at first but later succeeds", func() {
				It("retries until it verifies sync successfully", func() {
					agentClient.VerifySyncedCalls.Returns.Errors = make([]error, 10)
					for i := 0; i < 9; i++ {
						agentClient.VerifySyncedCalls.Returns.Errors[i] = errors.New("some error")
					}

					Expect(controller.ConfigureServer(timeout)).To(Succeed())
					Expect(agentClient.VerifySyncedCalls.CallCount).To(Equal(10))
					Expect(clock.SleepCall.CallCount).To(Equal(9))
					Expect(clock.SleepCall.Receives.Duration).To(Equal(10 * time.Millisecond))
					Expect(agentRunner.WritePIDCall.CallCount).To(Equal(1))

					Expect(logger.Messages()).To(ContainSequence([]fakes.LoggerMessage{
						{
							Action: "controller.configure-server.verify-synced",
						},
						{
							Action: "controller.configure-server.set-keys",
							Data: []lager.Data{{
								"keys": []string{"key 1", "key 2", "key 3"},
							}},
						},
						{
							Action: "controller.configure-server.success",
						},
					}))
				})
			})

			Context("verifying synced never succeeds within the timeout period", func() {
				It("immediately returns an error", func() {
					agentClient.VerifySyncedCalls.Returns.Error = errors.New("some error")

					timeout = utils.NewTimeout(time.After(10 * time.Millisecond))

					err := controller.ConfigureServer(timeout)
					Expect(err).To(MatchError(`timeout exceeded: "some error"`))
					Expect(agentClient.VerifySyncedCalls.CallCount).NotTo(Equal(0))
					Expect(agentClient.SetKeysCall.Receives.Keys).To(BeNil())
					Expect(agentRunner.WritePIDCall.CallCount).To(Equal(0))

					Expect(logger.Messages()).To(ContainSequence([]fakes.LoggerMessage{
						{
							Action: "controller.configure-server.verify-synced",
						},
						{
							Action: "controller.configure-server.verify-synced.failed",
							Error:  errors.New(`timeout exceeded: "some error"`),
						},
					}))
				})
			})
		})

		Context("when writing the PID file fails", func() {
			It("returns the error", func() {
				agentRunner.WritePIDCall.Returns.Error = errors.New("failed to write PIDFILE")

				err := controller.ConfigureServer(timeout)
				Expect(err).To(MatchError("failed to write PIDFILE"))

				Expect(agentRunner.WritePIDCall.CallCount).To(Equal(1))
				Expect(logger.Messages()).To(ContainSequence([]fakes.LoggerMessage{
					{
						Action: "controller.configure-server.set-keys",
						Error:  nil,
						Data: []lager.Data{
							{
								"keys": []string{"key 1", "key 2", "key 3"},
							},
						},
					},
					{
						Action: "controller.configure-server.write-pid.failed",
						Error:  errors.New("failed to write PIDFILE"),
					},
				}))
			})
		})
	})
})
