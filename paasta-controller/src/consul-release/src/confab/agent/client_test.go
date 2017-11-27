package agent_test

import (
	"errors"

	"code.cloudfoundry.org/lager"

	"github.com/cloudfoundry-incubator/consul-release/src/confab/agent"
	"github.com/cloudfoundry-incubator/consul-release/src/confab/fakes"
	"github.com/hashicorp/consul/api"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	. "github.com/pivotal-cf-experimental/gomegamatchers"
)

var _ = Describe("Client", func() {
	var (
		consulAPIAgent    *fakes.FakeconsulAPIAgent
		consulAPIOperator *fakes.FakeconsulAPIOperator
		logger            *fakes.Logger
		client            agent.Client
	)

	BeforeEach(func() {
		consulAPIAgent = &fakes.FakeconsulAPIAgent{}
		consulAPIOperator = &fakes.FakeconsulAPIOperator{}
		logger = &fakes.Logger{}
		client = agent.Client{
			ConsulAPIAgent:    consulAPIAgent,
			ConsulAPIOperator: consulAPIOperator,
			Logger:            logger,
		}
	})

	Describe("VerifyJoined", func() {
		Context("when the set of members includes at least one that we expect", func() {
			It("succeeds", func() {
				client.ExpectedMembers = []string{"member1", "member2", "member3"}
				consulAPIAgent.MembersReturns([]*api.AgentMember{
					&api.AgentMember{
						Addr: "member1",
						Tags: map[string]string{
							"role": "consul",
						},
					},
					&api.AgentMember{
						Addr: "member2",
						Tags: map[string]string{
							"role": "consul",
						},
					},
					&api.AgentMember{
						Addr: "member3",
						Tags: map[string]string{
							"role": "consul",
						},
					},
				}, nil)

				Expect(client.VerifyJoined()).To(Succeed())
				Expect(consulAPIAgent.MembersArgsForCall(0)).To(BeFalse())

				Expect(logger.Messages()).To(ContainSequence([]fakes.LoggerMessage{
					{
						Action: "agent-client.verify-joined.members.request",
						Data: []lager.Data{{
							"wan": false,
						}},
					},
					{
						Action: "agent-client.verify-joined.members.response",
						Data: []lager.Data{{
							"wan":     false,
							"members": []string{"member1", "member2", "member3"},
						}},
					},
					{
						Action: "agent-client.verify-joined.members.joined",
					},
				}))
			})
		})

		Context("when the members are all strangers", func() {
			It("returns an error", func() {
				client.ExpectedMembers = []string{"member1", "member2", "member3"}
				consulAPIAgent.MembersReturns([]*api.AgentMember{
					&api.AgentMember{Addr: "member4"},
					&api.AgentMember{Addr: "member5"},
				}, nil)

				Expect(client.VerifyJoined()).To(MatchError("no expected members"))
				Expect(consulAPIAgent.MembersArgsForCall(0)).To(BeFalse())

				Expect(logger.Messages()).To(ContainSequence([]fakes.LoggerMessage{
					{
						Action: "agent-client.verify-joined.members.request",
						Data: []lager.Data{{
							"wan": false,
						}},
					},
					{
						Action: "agent-client.verify-joined.members.response",
						Data: []lager.Data{{
							"wan":     false,
							"members": []string{"member4", "member5"},
						}},
					},
					{
						Action: "agent-client.verify-joined.members.not-joined",
						Error:  errors.New("no expected members"),
						Data: []lager.Data{{
							"wan":     false,
							"members": []string{"member4", "member5"},
						}},
					},
				}))
			})
		})

		Context("when the members call fails", func() {
			It("returns an error", func() {
				consulAPIAgent.MembersReturns([]*api.AgentMember{}, errors.New("members call error"))
				client.ExpectedMembers = []string{}

				Expect(client.VerifyJoined()).To(MatchError("members call error"))
				Expect(consulAPIAgent.MembersArgsForCall(0)).To(BeFalse())

				Expect(logger.Messages()).To(ContainSequence([]fakes.LoggerMessage{
					{
						Action: "agent-client.verify-joined.members.request",
						Data: []lager.Data{{
							"wan": false,
						}},
					},
					{
						Action: "agent-client.verify-joined.members.request.failed",
						Error:  errors.New("members call error"),
						Data: []lager.Data{{
							"wan": false,
						}},
					},
				}))
			})
		})
	})

	Describe("VerifySynced", func() {
		BeforeEach(func() {
			consulAPIAgent.SelfCall.Returns.SelfInfo = map[string]map[string]interface{}{
				"Stats": map[string]interface{}{
					"raft": map[string]interface{}{
						"commit_index":   "2",
						"last_log_index": "2",
					},
				},
			}
		})

		It("verifies the sync state of the raft log", func() {
			Expect(client.VerifySynced()).To(Succeed())
			Expect(consulAPIAgent.SelfCall.CallCount).To(Equal(1))
			Expect(logger.Messages()).To(ContainSequence([]fakes.LoggerMessage{
				{
					Action: "agent-client.verify-synced.stats.request",
				},
				{
					Action: "agent-client.verify-synced.stats.response",
					Data: []lager.Data{{
						"commit_index":   "2",
						"last_log_index": "2",
					}},
				},
				{
					Action: "agent-client.verify-synced.synced",
				},
			}))
		})

		Context("when the last_log_index never catches up", func() {
			BeforeEach(func() {
				consulAPIAgent.SelfCall.Returns.SelfInfo = map[string]map[string]interface{}{
					"Stats": map[string]interface{}{
						"raft": map[string]interface{}{
							"commit_index":   "2",
							"last_log_index": "1",
						},
					},
				}
			})

			It("returns an error", func() {
				Expect(client.VerifySynced()).To(MatchError("log not in sync"))
				Expect(consulAPIAgent.SelfCall.CallCount).To(Equal(1))
				Expect(logger.Messages()).To(ContainSequence([]fakes.LoggerMessage{
					{
						Action: "agent-client.verify-synced.stats.request",
					},
					{
						Action: "agent-client.verify-synced.stats.response",
						Data: []lager.Data{{
							"commit_index":   "2",
							"last_log_index": "1",
						}},
					},
					{
						Action: "agent-client.verify-synced.not-synced",
						Error:  errors.New("log not in sync"),
					},
				}))
			})
		})

		Context("when the ConsulAPIAgent returns an error", func() {
			BeforeEach(func() {
				consulAPIAgent.SelfCall.Returns.Error = errors.New("failed to query self")
			})

			It("immediately returns an error", func() {
				Expect(client.VerifySynced()).To(MatchError("failed to query self"))
				Expect(consulAPIAgent.SelfCall.CallCount).To(Equal(1))
				Expect(logger.Messages()).To(ContainSequence([]fakes.LoggerMessage{
					{
						Action: "agent-client.verify-synced.stats.request",
					},
					{
						Action: "agent-client.verify-synced.stats.request.failed",
						Error:  errors.New("failed to query self"),
					},
				}))
			})
		})

		Context("when the commit index is 0", func() {
			BeforeEach(func() {
				consulAPIAgent.SelfCall.Returns.SelfInfo = map[string]map[string]interface{}{
					"Stats": map[string]interface{}{
						"raft": map[string]interface{}{
							"commit_index":   "0",
							"last_log_index": "0",
						},
					},
				}
			})

			It("immediately returns an error", func() {
				Expect(client.VerifySynced()).To(MatchError("commit index must not be zero"))
				Expect(consulAPIAgent.SelfCall.CallCount).To(Equal(1))
				Expect(logger.Messages()).To(ContainSequence([]fakes.LoggerMessage{
					{
						Action: "agent-client.verify-synced.stats.request",
					},
					{
						Action: "agent-client.verify-synced.stats.response",
						Data: []lager.Data{{
							"commit_index":   "0",
							"last_log_index": "0",
						}},
					},
					{
						Action: "agent-client.verify-synced.zero-index",
						Error:  errors.New("commit index must not be zero"),
					},
				}))
			})
		})
	})

	Describe("JoinMembers", func() {
		BeforeEach(func() {
			client.ExpectedMembers = []string{"member1", "member2", "member3"}
		})

		Context("when we are able to successfully join each expected member", func() {
			It("returns without errors", func() {
				err := client.JoinMembers()
				Expect(err).NotTo(HaveOccurred())
				Expect(consulAPIAgent.JoinCall.CallCount).To(Equal(3))
				Expect(consulAPIAgent.JoinCall.Receives.Members).To(Equal(client.ExpectedMembers))
				Expect(consulAPIAgent.JoinCall.Receives.WAN).To(BeFalse())
				Expect(logger.Messages()).To(ContainSequence([]fakes.LoggerMessage{
					{
						Action: "agent-client.join-members.consul-api-agent.join",
						Data: []lager.Data{
							{
								"member": "member1",
							},
						},
					},
					{
						Action: "agent-client.join-members.consul-api-agent.join",
						Data: []lager.Data{
							{
								"member": "member2",
							},
						},
					},
					{
						Action: "agent-client.join-members.consul-api-agent.join",
						Data: []lager.Data{
							{
								"member": "member3",
							},
						},
					},
					{
						Action: "agent-client.join-members.success",
					},
				}))
			})
		})

		Context("when we are unable to join some expected members", func() {
			It("returns without errors when there is a 'i/o timeout' message", func() {
				consulAPIAgent.JoinCall.Stub = func(member string, wan bool) error {
					if member == "member2" {
						return errors.New("dial tcp 127.0.0.1:8500: i/o timeout")
					}
					return nil
				}
				err := client.JoinMembers()
				Expect(err).NotTo(HaveOccurred())
				Expect(consulAPIAgent.JoinCall.CallCount).To(Equal(3))
				Expect(consulAPIAgent.JoinCall.Receives.Members).To(Equal(client.ExpectedMembers))
				Expect(consulAPIAgent.JoinCall.Receives.WAN).To(BeFalse())
				Expect(logger.Messages()).To(ContainSequence([]fakes.LoggerMessage{
					{
						Action: "agent-client.join-members.consul-api-agent.join.unable-to-join",
						Data: []lager.Data{{
							"reason": "dial tcp 127.0.0.1:8500: i/o timeout",
						}},
					},
				}))
			})
			It("returns without errors when there is a 'no route to host' message", func() {
				consulAPIAgent.JoinCall.Stub = func(member string, wan bool) error {
					if member == "member2" {
						return errors.New("dial tcp 127.0.0.1:8500: getsockopt: no route to host")
					}
					return nil
				}
				err := client.JoinMembers()
				Expect(err).NotTo(HaveOccurred())
				Expect(consulAPIAgent.JoinCall.CallCount).To(Equal(3))
				Expect(consulAPIAgent.JoinCall.Receives.Members).To(Equal(client.ExpectedMembers))
				Expect(consulAPIAgent.JoinCall.Receives.WAN).To(BeFalse())
				Expect(logger.Messages()).To(ContainSequence([]fakes.LoggerMessage{
					{
						Action: "agent-client.join-members.consul-api-agent.join.unable-to-join",
						Data: []lager.Data{{
							"reason": "dial tcp 127.0.0.1:8500: getsockopt: no route to host",
						}},
					},
				}))
			})

			It("returns without errors", func() {
				consulAPIAgent.JoinCall.Stub = func(member string, wan bool) error {
					if member == "member2" {
						return errors.New("dial tcp 127.0.0.1:8500: getsockopt: connection refused")
					}
					return nil
				}
				err := client.JoinMembers()
				Expect(err).NotTo(HaveOccurred())
				Expect(consulAPIAgent.JoinCall.CallCount).To(Equal(3))
				Expect(consulAPIAgent.JoinCall.Receives.Members).To(Equal(client.ExpectedMembers))
				Expect(consulAPIAgent.JoinCall.Receives.WAN).To(BeFalse())
				Expect(logger.Messages()).To(ContainSequence([]fakes.LoggerMessage{
					{
						Action: "agent-client.join-members.consul-api-agent.join.unable-to-join",
						Data: []lager.Data{{
							"reason": "dial tcp 127.0.0.1:8500: getsockopt: connection refused",
						}},
					},
				}))
			})
		})

		Context("when we are unable to join any expected members", func() {
			It("returns a no members to join error", func() {
				consulAPIAgent.JoinCall.Returns.Error = errors.New("dial tcp 127.0.0.1:8500: getsockopt: connection refused")
				err := client.JoinMembers()
				Expect(err).To(MatchError(agent.NoMembersToJoinError))
				Expect(logger.Messages()).To(ContainSequence([]fakes.LoggerMessage{
					{
						Action: "agent-client.join-members.no-members-to-join",
					},
				}))
			})
		})

		Context("failure cases", func() {
			Context("when client api agent join fails", func() {
				It("returns an error", func() {
					consulAPIAgent.JoinCall.Returns.Error = errors.New("failed to join")
					err := client.JoinMembers()
					Expect(err).To(MatchError("failed to join"))
					Expect(logger.Messages()).To(ContainSequence([]fakes.LoggerMessage{
						{
							Action: "agent-client.join-members.consul-api-agent.join.failed",
							Error:  errors.New("failed to join"),
						},
					}))
				})
			})
		})
	})

	Describe("Self", func() {
		It("does not return an error when the agent is ready", func() {
			err := client.Self()
			Expect(err).NotTo(HaveOccurred())
			Expect(consulAPIAgent.SelfCall.CallCount).To(Equal(1))
		})

		Context("failure case", func() {
			It("returns an error when self call fails", func() {
				consulAPIAgent.SelfCall.Returns.Error = errors.New("some error occurred")

				err := client.Self()
				Expect(err).To(MatchError("some error occurred"))
				Expect(consulAPIAgent.SelfCall.CallCount).To(Equal(1))
			})
		})

	})

	Describe("Members", func() {
		It("returns the consul agent api members call", func() {
			consulAPIAgent.MembersReturns([]*api.AgentMember{
				&api.AgentMember{Addr: "member1", Tags: map[string]string{"role": "consul"}},
				&api.AgentMember{Addr: "member2", Tags: map[string]string{"role": "consul"}},
				&api.AgentMember{Addr: "member3", Tags: map[string]string{"role": "consul"}},
			}, nil)

			members, err := client.Members(false)
			Expect(err).NotTo(HaveOccurred())
			Expect(members).To(Equal([]*api.AgentMember{
				&api.AgentMember{Addr: "member1", Tags: map[string]string{"role": "consul"}},
				&api.AgentMember{Addr: "member2", Tags: map[string]string{"role": "consul"}},
				&api.AgentMember{Addr: "member3", Tags: map[string]string{"role": "consul"}},
			}))

			Expect(consulAPIAgent.MembersCallCount()).To(Equal(1))
			Expect(consulAPIAgent.MembersArgsForCall(0)).To(BeFalse())
		})

		Context("failure cases", func() {
			Context("when the consul api agent members call fails", func() {
				It("returns an error", func() {
					consulAPIAgent.MembersReturns(nil, errors.New("failed to list members"))
					_, err := client.Members(false)
					Expect(err).To(MatchError("failed to list members"))
				})
			})
		})
	})

	Describe("SetKeys", func() {
		encryptedKey1 := "5v4WCjw2FyuezPYYUvo0zA=="
		encryptedKey2 := "gcC8kpXH4sUwLaxtiz2mBw=="
		encryptedKeyPercent := "OLJdB+hlOnGSUEIR7S6ekA=="

		It("installs the given keys", func() {
			Expect(client.SetKeys([]string{encryptedKey1, "key2", "key%%"})).To(Succeed())

			Expect(logger.Messages()).To(ContainSequence([]fakes.LoggerMessage{
				{
					Action: "agent-client.set-keys.list-keys.request",
				},
				{
					Action: "agent-client.set-keys.list-keys.response",
					Data: []lager.Data{{
						"keys": []string{},
					}},
				},
				{
					Action: "agent-client.set-keys.install-key.request",
					Data: []lager.Data{{
						"key": encryptedKey1,
					}},
				},
				{
					Action: "agent-client.set-keys.install-key.response",
					Data: []lager.Data{{
						"key": encryptedKey1,
					}},
				},
				{
					Action: "agent-client.set-keys.install-key.request",
					Data: []lager.Data{{
						"key": encryptedKey2,
					}},
				},
				{
					Action: "agent-client.set-keys.install-key.response",
					Data: []lager.Data{{
						"key": encryptedKey2,
					}},
				},
				{
					Action: "agent-client.set-keys.install-key.request",
					Data: []lager.Data{{
						"key": encryptedKeyPercent,
					}},
				},
				{
					Action: "agent-client.set-keys.install-key.response",
					Data: []lager.Data{{
						"key": encryptedKeyPercent,
					}},
				},
				{
					Action: "agent-client.set-keys.use-key.request",
					Data: []lager.Data{{
						"key": encryptedKey1,
					}},
				},
				{
					Action: "agent-client.set-keys.use-key.response",
					Data: []lager.Data{{
						"key": encryptedKey1,
					}},
				},
				{
					Action: "agent-client.set-keys.success",
				},
			}))
		})

		Context("when there are extra keys", func() {
			It("removes extra keys", func() {
				consulAPIOperator.KeyringListCall.Returns.KeyringResponse = []*api.KeyringResponse{
					&api.KeyringResponse{
						WAN: false,
						Keys: map[string]int{
							"key3": 1,
							"key4": 1,
						},
					},
				}

				Expect(client.SetKeys([]string{"key1", "key2"})).To(Succeed())

				msgs := logger.Messages()
				Expect(len(msgs)).Should(BeNumerically(">=", 2))
				Expect(msgs[0].Action).To(Equal("agent-client.set-keys.list-keys.request"))
				Expect(msgs[1].Action).To(Equal("agent-client.set-keys.list-keys.response"))
				Expect(len(msgs[1].Data)).To(Equal(1))
				Expect(msgs[1].Data[0]).To(HaveKey("keys"))
				Expect(msgs[1].Data[0]["keys"]).To(ConsistOf([]string{"key3", "key4"}))

				Expect(logger.Messages()).To(ContainSequence([]fakes.LoggerMessage{
					{
						Action: "agent-client.set-keys.remove-key.request",
						Data: []lager.Data{{
							"key": "key3",
						}},
					},
					{
						Action: "agent-client.set-keys.remove-key.response",
						Data: []lager.Data{{
							"key": "key3",
						}},
					},
				}))

				Expect(logger.Messages()).To(ContainSequence([]fakes.LoggerMessage{
					{
						Action: "agent-client.set-keys.remove-key.request",
						Data: []lager.Data{{
							"key": "key4",
						}},
					},
					{
						Action: "agent-client.set-keys.remove-key.response",
						Data: []lager.Data{{
							"key": "key4",
						}},
					},
				}))

				Expect(logger.Messages()).To(ContainSequence([]fakes.LoggerMessage{
					{
						Action: "agent-client.set-keys.install-key.request",
						Data: []lager.Data{{
							"key": encryptedKey1,
						}},
					},
					{
						Action: "agent-client.set-keys.install-key.response",
						Data: []lager.Data{{
							"key": encryptedKey1,
						}},
					},
					{
						Action: "agent-client.set-keys.install-key.request",
						Data: []lager.Data{{
							"key": encryptedKey2,
						}},
					},
					{
						Action: "agent-client.set-keys.install-key.response",
						Data: []lager.Data{{
							"key": encryptedKey2,
						}},
					},
					{
						Action: "agent-client.set-keys.use-key.request",
						Data: []lager.Data{{
							"key": encryptedKey1,
						}},
					},
					{
						Action: "agent-client.set-keys.use-key.response",
						Data: []lager.Data{{
							"key": encryptedKey1,
						}},
					},
					{
						Action: "agent-client.set-keys.success",
					},
				}))
			})
		})

		Context("failure cases", func() {
			Context("when provided with a nil slice", func() {
				It("returns a reasonably named error", func() {
					Expect(client.SetKeys(nil)).To(MatchError("must provide a non-nil slice of keys"))
					Expect(logger.Messages()).To(ContainSequence([]fakes.LoggerMessage{
						{
							Action: "agent-client.set-keys.nil-slice",
							Error:  errors.New("must provide a non-nil slice of keys"),
						},
					}))
				})
			})

			Context("when provided with an empty slice", func() {
				It("returns a reasonably named error", func() {
					Expect(client.SetKeys([]string{})).To(MatchError("must provide a non-empty slice of keys"))
					Expect(logger.Messages()).To(ContainSequence([]fakes.LoggerMessage{
						{
							Action: "agent-client.set-keys.empty-slice",
							Error:  errors.New("must provide a non-empty slice of keys"),
						},
					}))
				})
			})

			Context("when ListKeys returns an error", func() {
				It("returns the error", func() {
					consulAPIOperator.KeyringListCall.Returns.Error = errors.New("list keys error")

					Expect(client.SetKeys([]string{"key1"})).To(MatchError("list keys error"))
					Expect(logger.Messages()).To(ContainSequence([]fakes.LoggerMessage{
						{
							Action: "agent-client.set-keys.list-keys.request",
						},
						{
							Action: "agent-client.set-keys.list-keys.request.failed",
							Error:  errors.New("list keys error"),
						},
					}))
				})
			})

			Context("when RemoveKeys returns an error", func() {
				It("returns the error", func() {
					consulAPIOperator.KeyringRemoveCall.Returns.Error = errors.New("remove key error")
					consulAPIOperator.KeyringListCall.Returns.KeyringResponse = []*api.KeyringResponse{
						&api.KeyringResponse{
							WAN: false,
							Keys: map[string]int{
								"key2": 1,
							},
						},
					}

					Expect(client.SetKeys([]string{"key1"})).To(MatchError("remove key error"))
					Expect(logger.Messages()).To(ContainSequence([]fakes.LoggerMessage{
						{
							Action: "agent-client.set-keys.list-keys.request",
						},
						{
							Action: "agent-client.set-keys.list-keys.response",
							Data: []lager.Data{{
								"keys": []string{"key2"},
							}},
						},
						{
							Action: "agent-client.set-keys.remove-key.request",
							Data: []lager.Data{{
								"key": "key2",
							}},
						},
						{
							Action: "agent-client.set-keys.remove-key.request.failed",
							Error:  errors.New("remove key error"),
							Data: []lager.Data{{
								"key": "key2",
							}},
						},
					}))
				})
			})

			Context("when InstallKey returns an error", func() {
				It("returns the error", func() {
					consulAPIOperator.KeyringInstallCall.Returns.Error = errors.New("install key error")

					Expect(client.SetKeys([]string{"key1"})).To(MatchError("install key error"))
					Expect(logger.Messages()).To(ContainSequence([]fakes.LoggerMessage{
						{
							Action: "agent-client.set-keys.list-keys.request",
						},
						{
							Action: "agent-client.set-keys.list-keys.response",
							Data: []lager.Data{{
								"keys": []string{},
							}},
						},
						{
							Action: "agent-client.set-keys.install-key.request",
							Data: []lager.Data{{
								"key": encryptedKey1,
							}},
						},
						{
							Action: "agent-client.set-keys.install-key.request.failed",
							Error:  errors.New("install key error"),
							Data: []lager.Data{{
								"key": encryptedKey1,
							}},
						},
					}))
				})
			})

			Context("when UseKey returns an error", func() {
				It("returns the error", func() {
					consulAPIOperator.KeyringUseCall.Returns.Error = errors.New("use key error")

					Expect(client.SetKeys([]string{"key1"})).To(MatchError("use key error"))
					Expect(logger.Messages()).To(ContainSequence([]fakes.LoggerMessage{
						{
							Action: "agent-client.set-keys.list-keys.request",
						},
						{
							Action: "agent-client.set-keys.list-keys.response",
							Data: []lager.Data{{
								"keys": []string{},
							}},
						},
						{
							Action: "agent-client.set-keys.install-key.request",
							Data: []lager.Data{{
								"key": encryptedKey1,
							}},
						},
						{
							Action: "agent-client.set-keys.install-key.response",
							Data: []lager.Data{{
								"key": encryptedKey1,
							}},
						},
						{
							Action: "agent-client.set-keys.use-key.request",
							Data: []lager.Data{{
								"key": encryptedKey1,
							}},
						},
						{
							Action: "agent-client.set-keys.use-key.request.failed",
							Error:  errors.New("use key error"),
							Data: []lager.Data{{
								"key": encryptedKey1,
							}},
						},
					}))
				})
			})
		})
	})

	Describe("ListKeys", func() {
		It("returns the list of keys", func() {
			keysMap := make(map[string]int)
			keysMap["key-1"] = 1
			keysMap["key-2"] = 1
			consulAPIOperator.KeyringListCall.Returns.KeyringResponse = []*api.KeyringResponse{
				&api.KeyringResponse{
					WAN:  false,
					Keys: keysMap,
				},
			}

			keys, err := client.ListKeys()
			Expect(err).NotTo(HaveOccurred())
			Expect(consulAPIOperator.KeyringListCall.CallCount).To(Equal(1))
			Expect(keys).To(ContainElement("key-1"))
			Expect(keys).To(ContainElement("key-2"))
		})

		It("returns an error when keyringList fails", func() {
			consulAPIOperator.KeyringListCall.Returns.Error = errors.New("keyring list failed")

			_, err := client.ListKeys()
			Expect(err).To(MatchError("keyring list failed"))
		})
	})

	Describe("InstallKey", func() {
		It("makes the call to InstallKey", func() {
			err := client.InstallKey("key-1")
			Expect(err).NotTo(HaveOccurred())
			Expect(consulAPIOperator.KeyringInstallCall.CallCount).To(Equal(1))
			Expect(consulAPIOperator.KeyringInstallCall.Receives.Key).To(Equal("key-1"))
		})

		It("returns an error when keyringInstall fails", func() {
			consulAPIOperator.KeyringInstallCall.Returns.Error = errors.New("keyring install failed")

			err := client.InstallKey("some-string")
			Expect(err).To(MatchError("keyring install failed"))
		})
	})

	Describe("UseKey", func() {
		It("makes the call to UseKey", func() {
			err := client.UseKey("key-1")
			Expect(err).NotTo(HaveOccurred())
			Expect(consulAPIOperator.KeyringUseCall.CallCount).To(Equal(1))
			Expect(consulAPIOperator.KeyringUseCall.Receives.Key).To(Equal("key-1"))
		})

		It("returns an error when keyringUse fails", func() {
			consulAPIOperator.KeyringUseCall.Returns.Error = errors.New("keyring use failed")

			err := client.UseKey("some-string")
			Expect(err).To(MatchError("keyring use failed"))
		})
	})

	Describe("RemoveKey", func() {
		It("makes the call to RemoveKey", func() {
			err := client.RemoveKey("key-1")
			Expect(err).NotTo(HaveOccurred())
			Expect(consulAPIOperator.KeyringRemoveCall.CallCount).To(Equal(1))
			Expect(consulAPIOperator.KeyringRemoveCall.Receives.Key).To(Equal("key-1"))
		})

		It("returns an error when keyringRemove fails", func() {
			consulAPIOperator.KeyringRemoveCall.Returns.Error = errors.New("keyring remove failed")

			err := client.RemoveKey("some-string")
			Expect(err).To(MatchError("keyring remove failed"))
		})
	})

	Describe("Leave", func() {
		It("leaves the cluster", func() {
			Expect(client.Leave()).To(Succeed())
			Expect(consulAPIAgent.LeaveCall.CallCount).To(Equal(1))
			Expect(logger.Messages()).To(ContainSequence([]fakes.LoggerMessage{
				{
					Action: "agent-client.leave.leave.request",
				},
				{
					Action: "agent-client.leave.leave.response",
				},
			}))
		})

		Context("when consul's api agent leave fails", func() {
			It("returns an error", func() {
				consulAPIAgent.LeaveCall.Returns.Error = errors.New("failed to leave")
				err := client.Leave()
				Expect(err).To(MatchError("failed to leave"))
				Expect(logger.Messages()).To(ContainSequence([]fakes.LoggerMessage{
					{
						Action: "agent-client.leave.leave.request",
					},
					{
						Action: "agent-client.leave.leave.request.failed",
						Error:  errors.New("failed to leave"),
					},
				}))
			})
		})
	})

	Describe("RaftStats", func() {
		BeforeEach(func() {
			consulAPIAgent.SelfCall.Returns.SelfInfo = map[string]map[string]interface{}{
				"Stats": map[string]interface{}{
					"raft": map[string]interface{}{
						"commit_index":   "2",
						"last_log_index": "2",
					},
				},
			}
		})

		It("returns the stats.raft from /v1/agent/self", func() {
			raftStats, err := client.RaftStats()
			Expect(err).NotTo(HaveOccurred())
			Expect(raftStats).To(Equal(map[string]interface{}{
				"commit_index":   "2",
				"last_log_index": "2",
			}))
		})

		Context("when it fails to query self", func() {
			BeforeEach(func() {
				consulAPIAgent.SelfCall.Returns.Error = errors.New("failed to query self")
			})

			It("returns an error", func() {
				_, err := client.RaftStats()
				Expect(err).To(MatchError("failed to query self"))
			})
		})
	})
})
