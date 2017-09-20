package fakes

import (
	"github.com/cloudfoundry-incubator/consul-release/src/confab/agent"
	"github.com/hashicorp/consul/api"
)

type AgentClient struct {
	VerifyJoinedCalls struct {
		CallCount int
		Returns   struct {
			Error error
		}
	}
	VerifySyncedCalls struct {
		CallCount int
		Returns   struct {
			Errors []error
			Error  error
		}
	}

	SetKeysCall struct {
		Receives struct {
			Keys []string
		}
		Returns struct {
			Error error
		}
	}

	LeaveCall struct {
		CallCount int
		Returns   struct {
			Error error
		}
	}

	SetConsulRPCClientCall struct {
		CallCount int
		Receives  struct {
			ConsulRPCClient agent.ConsulRPCClient
		}
	}

	MembersCall struct {
		CallCount int
		Receives  struct {
			WAN bool
		}
		Returns struct {
			Members []*api.AgentMember
			Error   error
		}
	}

	JoinMembersCall struct {
		CallCount int
		Returns   struct {
			Error error
		}
	}
	SelfCall struct {
		CallCount int
		Returns   struct {
			Error  error
			Errors []error
		}
	}
}

func (c *AgentClient) Self() error {
	err := c.SelfCall.Returns.Error
	if len(c.SelfCall.Returns.Errors) > c.SelfCall.CallCount {
		err = c.SelfCall.Returns.Errors[c.SelfCall.CallCount]
	}
	c.SelfCall.CallCount++
	return err
}

func (c *AgentClient) VerifyJoined() error {
	c.VerifyJoinedCalls.CallCount++
	return c.VerifyJoinedCalls.Returns.Error
}

func (c *AgentClient) VerifySynced() error {
	var err error

	if c.VerifySyncedCalls.Returns.Error != nil {
		err = c.VerifySyncedCalls.Returns.Error
	} else {
		err = c.VerifySyncedCalls.Returns.Errors[c.VerifySyncedCalls.CallCount]
	}

	c.VerifySyncedCalls.CallCount++
	return err
}

func (c *AgentClient) SetKeys(keys []string) error {
	c.SetKeysCall.Receives.Keys = keys
	return c.SetKeysCall.Returns.Error
}

func (c *AgentClient) Leave() error {
	c.LeaveCall.CallCount++
	return c.LeaveCall.Returns.Error
}

func (c *AgentClient) SetConsulRPCClient(rpcClient agent.ConsulRPCClient) {
	c.SetConsulRPCClientCall.CallCount++
	c.SetConsulRPCClientCall.Receives.ConsulRPCClient = rpcClient
}

func (c *AgentClient) Members(wan bool) ([]*api.AgentMember, error) {
	c.MembersCall.CallCount++
	c.MembersCall.Receives.WAN = wan
	return c.MembersCall.Returns.Members, c.MembersCall.Returns.Error
}

func (c *AgentClient) JoinMembers() error {
	c.JoinMembersCall.CallCount++
	return c.JoinMembersCall.Returns.Error
}
