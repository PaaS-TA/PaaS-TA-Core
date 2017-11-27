package chaperon_test

import (
	"errors"

	"github.com/cloudfoundry-incubator/consul-release/src/confab/chaperon"
	"github.com/cloudfoundry-incubator/consul-release/src/confab/config"
	"github.com/cloudfoundry-incubator/consul-release/src/confab/fakes"
	"github.com/cloudfoundry-incubator/consul-release/src/confab/utils"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Server", func() {
	var (
		server           chaperon.Server
		timeout          *fakes.Timeout
		controller       *fakes.Controller
		bootstrapChecker *fakes.BootstrapChecker

		cfg          config.Config
		configWriter *fakes.ConfigWriter
	)

	BeforeEach(func() {
		cfg = config.Config{
			Node: config.ConfigNode{
				Name: "some-name",
			},
		}

		controller = &fakes.Controller{}
		configWriter = &fakes.ConfigWriter{}
		bootstrapChecker = &fakes.BootstrapChecker{}

		server = chaperon.NewServer(controller, configWriter, bootstrapChecker)

		timeout = &fakes.Timeout{}
	})

	Describe("Start", func() {
		It("writes the consul configuration file", func() {
			err := server.Start(cfg, timeout)
			Expect(err).NotTo(HaveOccurred())
			Expect(configWriter.WriteCall.Receives.Config).To(Equal(cfg))
		})

		It("writes the service definitions", func() {
			err := server.Start(cfg, timeout)
			Expect(err).NotTo(HaveOccurred())
			Expect(controller.WriteServiceDefinitionsCall.CallCount).To(Equal(1))
		})

		It("boots the agent process", func() {
			err := server.Start(cfg, timeout)
			Expect(err).NotTo(HaveOccurred())
			Expect(controller.BootAgentCall.CallCount).To(Equal(1))
			Expect(controller.BootAgentCall.Receives.Timeout).To(Equal(timeout))
		})

		It("configures the server", func() {
			err := server.Start(cfg, timeout)
			Expect(err).NotTo(HaveOccurred())
			Expect(controller.ConfigureServerCall.CallCount).To(Equal(1))
			Expect(controller.ConfigureServerCall.Receives.Timeout).To(Equal(timeout))
		})

		It("checks for a leader or bootstrapped node", func() {
			err := server.Start(cfg, timeout)
			Expect(err).NotTo(HaveOccurred())
			Expect(bootstrapChecker.StartInBootstrapModeCall.CallCount).To(Equal(1))
		})

		Context("bootstrap mode", func() {
			BeforeEach(func() {
				cfg.Consul.Agent.Bootstrap = false
				bootstrapChecker.StartInBootstrapModeCall.Returns.Bootstrap = true
			})

			It("restarts the server when there is no leader or server in bootstrap mode", func() {
				err := server.Start(cfg, timeout)
				Expect(err).NotTo(HaveOccurred())

				Expect(controller.StopAgentCall.CallCount).To(Equal(1))

				Expect(controller.ConfigureServerCall.CallCount).To(Equal(1))
				Expect(controller.WriteServiceDefinitionsCall.CallCount).To(Equal(1))

				Expect(controller.BootAgentCall.CallCount).To(Equal(2))

				Expect(configWriter.WriteCall.CallCount).To(Equal(2))
				Expect(configWriter.WriteCall.Configs[0]).To(Equal(cfg))
				Expect(configWriter.WriteCall.Configs[1].Consul.Agent.Bootstrap).To(BeTrue())
			})

			Context("failure cases", func() {
				It("returns an error when the bootstrap checker fails", func() {
					bootstrapChecker.StartInBootstrapModeCall.Returns.Error = errors.New("failed to check")
					err := server.Start(cfg, timeout)
					Expect(err).To(MatchError("failed to check"))

					Expect(configWriter.WriteCall.CallCount).To(Equal(1))
					Expect(controller.WriteServiceDefinitionsCall.CallCount).To(Equal(1))
					Expect(controller.BootAgentCall.CallCount).To(Equal(1))
					Expect(controller.ConfigureServerCall.CallCount).To(Equal(0))
				})

				It("returns an error when the new consul config fails to write", func() {
					configWriter.WriteCall.Stub = func(cfg config.Config) error {
						if configWriter.WriteCall.CallCount > 1 {
							return errors.New("failed to write config")
						}
						return nil
					}
					err := server.Start(cfg, timeout)
					Expect(err).To(MatchError("failed to write config"))
					Expect(configWriter.WriteCall.CallCount).To(Equal(2))
					Expect(controller.WriteServiceDefinitionsCall.CallCount).To(Equal(1))
					Expect(controller.BootAgentCall.CallCount).To(Equal(1))
					Expect(controller.StopAgentCall.CallCount).To(Equal(1))
					Expect(controller.ConfigureServerCall.CallCount).To(Equal(0))
				})

				It("returns an error when the new agent does not bootup", func() {
					controller.BootAgentCall.Stub = func(timeout utils.Timeout) error {
						if controller.BootAgentCall.CallCount > 1 {
							return errors.New("failed to start the agent")
						}
						return nil
					}
					err := server.Start(cfg, timeout)
					Expect(err).To(MatchError("failed to start the agent"))
					Expect(controller.BootAgentCall.CallCount).To(Equal(2))
					Expect(configWriter.WriteCall.CallCount).To(Equal(2))
					Expect(controller.WriteServiceDefinitionsCall.CallCount).To(Equal(1))
					Expect(controller.StopAgentCall.CallCount).To(Equal(1))
					Expect(controller.ConfigureServerCall.CallCount).To(Equal(0))
				})
			})
		})

		Context("failure cases", func() {
			Context("when writing the consul config file fails", func() {
				It("returns an error", func() {
					configWriter.WriteCall.Returns.Error = errors.New("failed to write config")

					err := server.Start(cfg, timeout)
					Expect(err).To(MatchError(errors.New("failed to write config")))
				})
			})

			Context("when writing the service definitions fails", func() {
				It("returns an error", func() {
					controller.WriteServiceDefinitionsCall.Returns.Error = errors.New("failed to write service definitions")

					err := server.Start(cfg, timeout)
					Expect(err).To(MatchError(errors.New("failed to write service definitions")))
				})
			})

			Context("when booting the agent fails", func() {
				It("returns an error", func() {
					controller.BootAgentCall.Returns.Error = errors.New("failed to boot agent")

					err := server.Start(cfg, timeout)
					Expect(err).To(MatchError(errors.New("failed to boot agent")))

					Expect(configWriter.WriteCall.CallCount).To(Equal(1))
					Expect(controller.WriteServiceDefinitionsCall.CallCount).To(Equal(1))
				})
			})

			Context("when configuring the server fails", func() {
				It("returns an error", func() {
					controller.ConfigureServerCall.Returns.Error = errors.New("failed to configure server")

					err := server.Start(cfg, timeout)
					Expect(err).To(MatchError(errors.New("failed to configure server")))
				})
			})
		})
	})

	Describe("Stop", func() {
		It("calls stop agent", func() {
			server.Stop()
			Expect(controller.StopAgentCall.CallCount).To(Equal(1))
		})
	})
})
