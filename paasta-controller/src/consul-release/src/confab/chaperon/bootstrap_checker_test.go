package chaperon_test

import (
	"errors"
	"time"

	"code.cloudfoundry.org/lager"
	"github.com/cloudfoundry-incubator/consul-release/src/confab/chaperon"
	"github.com/cloudfoundry-incubator/consul-release/src/confab/fakes"
	"github.com/hashicorp/consul/api"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	. "github.com/pivotal-cf-experimental/gomegamatchers"
)

var _ = Describe("BootstrapChecker", func() {
	Describe("StartInBootstrapMode", func() {
		var (
			logger           *fakes.Logger
			agentClient      *fakes.AgentClient
			statusClient     *fakes.StatusClient
			bootstrapChecker chaperon.BootstrapChecker
			sleeper          func(d time.Duration)
		)

		BeforeEach(func() {
			logger = &fakes.Logger{}
			agentClient = &fakes.AgentClient{}
			statusClient = &fakes.StatusClient{}
			sleeper = func(d time.Duration) {}

			bootstrapChecker = chaperon.NewBootstrapChecker(logger, agentClient, statusClient, sleeper)
		})

		Context("when there is no leader or bootstrap node in the cluster", func() {
			It("returns true", func() {
				startInBootstrap, err := bootstrapChecker.StartInBootstrapMode()
				Expect(err).NotTo(HaveOccurred())
				Expect(startInBootstrap).To(BeTrue())

				Expect(agentClient.MembersCall.CallCount).To(Equal(1))
				Expect(agentClient.MembersCall.Receives.WAN).To(BeFalse())

				Expect(statusClient.LeaderCall.CallCount).To(Equal(20))

				Expect(logger.Messages()).To(ContainSequence([]fakes.LoggerMessage{
					{
						Action: "chaperon-bootstrap-checker.start-in-bootstrap-mode.agent-client.members",
					},
					{
						Action: "chaperon-bootstrap-checker.start-in-bootstrap-mode.status-client.leader",
					},
					{
						Action: "chaperon-bootstrap-checker.start-in-bootstrap-mode",
						Data: []lager.Data{
							{
								"bootstrap": true,
							},
						},
					},
				}))
			})
		})

		Context("when there is a bootstrap node in the cluster", func() {
			It("returns false", func() {
				agentClient.MembersCall.Returns.Members = []*api.AgentMember{
					{
						Name: "some-member",
						Tags: map[string]string{
							"bootstrap": "1",
						},
					},
				}

				startInBootstrap, err := bootstrapChecker.StartInBootstrapMode()
				Expect(err).NotTo(HaveOccurred())
				Expect(startInBootstrap).To(BeFalse())

				Expect(agentClient.MembersCall.CallCount).To(Equal(1))
				Expect(agentClient.MembersCall.Receives.WAN).To(BeFalse())

				Expect(statusClient.LeaderCall.CallCount).To(Equal(0))

				Expect(logger.Messages()).To(ContainSequence([]fakes.LoggerMessage{
					{
						Action: "chaperon-bootstrap-checker.start-in-bootstrap-mode.bootstrap-node-exists",
						Data: []lager.Data{
							{
								"bootstrap-node": "some-member",
							},
						},
					},
					{
						Action: "chaperon-bootstrap-checker.start-in-bootstrap-mode",
						Data: []lager.Data{
							{
								"bootstrap": false,
							},
						},
					},
				}))
			})
		})

		Context("leader", func() {
			It("returns false when there is a leader in the cluster", func() {
				statusClient.LeaderCall.Returns.Leader = "some-leader"

				startInBootstrap, err := bootstrapChecker.StartInBootstrapMode()
				Expect(err).NotTo(HaveOccurred())
				Expect(startInBootstrap).To(BeFalse())

				Expect(agentClient.MembersCall.CallCount).To(Equal(1))
				Expect(agentClient.MembersCall.Receives.WAN).To(BeFalse())

				Expect(statusClient.LeaderCall.CallCount).To(Equal(1))

				Expect(logger.Messages()).To(ContainSequence([]fakes.LoggerMessage{
					{
						Action: "chaperon-bootstrap-checker.start-in-bootstrap-mode.leader-exists",
						Data: []lager.Data{
							{
								"leader": "some-leader",
							},
						},
					},
					{
						Action: "chaperon-bootstrap-checker.start-in-bootstrap-mode",
						Data: []lager.Data{
							{
								"bootstrap": false,
							},
						},
					},
				}))
			})

			It("returns true when there are no other consul nodes", func() {
				statusClient.LeaderCall.Returns.Error = errors.New("No known Consul servers")
				bootstrapMode, err := bootstrapChecker.StartInBootstrapMode()
				Expect(err).NotTo(HaveOccurred())
				Expect(bootstrapMode).To(BeTrue())

				Expect(logger.Messages()).To(ContainSequence([]fakes.LoggerMessage{
					{
						Action: "chaperon-bootstrap-checker.start-in-bootstrap-mode.status-client.leader",
					},
					{
						Action: "chaperon-bootstrap-checker.start-in-bootstrap-mode.status-client.leader.no-known-consul-servers",
					},
					{
						Action: "chaperon-bootstrap-checker.start-in-bootstrap-mode",
						Data: []lager.Data{
							{
								"bootstrap": true,
							},
						},
					},
				}))
			})

			It("attempts to retry leader status check till it timesout", func() {
				sleeperDuration := 0 * time.Second
				sleeper = func(d time.Duration) {
					sleeperDuration = d
				}
				bootstrapChecker = chaperon.NewBootstrapChecker(logger, agentClient, statusClient, sleeper)

				leaderCallCount := 0
				statusClient.LeaderCall.Stub = func() (string, error) {
					leaderCallCount++
					if leaderCallCount > 3 {
						return "some-leader", nil
					}
					return "", nil
				}

				startInBootstrap, err := bootstrapChecker.StartInBootstrapMode()
				Expect(err).NotTo(HaveOccurred())
				Expect(startInBootstrap).To(BeFalse())

				Expect(statusClient.LeaderCall.CallCount).To(Equal(4))
				Expect(sleeperDuration).To(Equal(100 * time.Millisecond))

				Expect(logger.Messages()).To(ContainSequence([]fakes.LoggerMessage{
					{
						Action: "chaperon-bootstrap-checker.start-in-bootstrap-mode.leader-exists",
						Data: []lager.Data{
							{
								"leader": "some-leader",
							},
						},
					},
					{
						Action: "chaperon-bootstrap-checker.start-in-bootstrap-mode",
						Data: []lager.Data{
							{
								"bootstrap": false,
							},
						},
					},
				}))
			})
		})

		Context("failure cases", func() {
			It("returns an error when the members check fails", func() {
				agentClient.MembersCall.Returns.Error = errors.New("error checking members")
				_, err := bootstrapChecker.StartInBootstrapMode()
				Expect(err).To(MatchError("error checking members"))
				Expect(logger.Messages()).To(ContainSequence([]fakes.LoggerMessage{
					{
						Action: "chaperon-bootstrap-checker.start-in-bootstrap-mode.agent-client.members",
					},
					{
						Action: "chaperon-bootstrap-checker.start-in-bootstrap-mode.agent-client.members.failed",
						Error:  errors.New("error checking members"),
					},
					{
						Action: "chaperon-bootstrap-checker.start-in-bootstrap-mode",
						Data: []lager.Data{
							{
								"bootstrap": false,
							},
						},
					},
				}))
			})

			It("returns an error when the leader check fails", func() {
				statusClient.LeaderCall.Returns.Error = errors.New("error checking leader")
				_, err := bootstrapChecker.StartInBootstrapMode()
				Expect(err).To(MatchError("error checking leader"))
				Expect(logger.Messages()).To(ContainSequence([]fakes.LoggerMessage{
					{
						Action: "chaperon-bootstrap-checker.start-in-bootstrap-mode.status-client.leader",
					},
					{
						Action: "chaperon-bootstrap-checker.start-in-bootstrap-mode.status-client.leader.failed",
						Error:  errors.New("error checking leader"),
					},
					{
						Action: "chaperon-bootstrap-checker.start-in-bootstrap-mode",
						Data: []lager.Data{
							{
								"bootstrap": false,
							},
						},
					},
				}))
			})
		})
	})
})
