package fakes

import "github.com/hashicorp/consul/api"

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
	ListKeysCall struct {
		CallCount int
		Returns   struct {
			Keys  []string
			Error error
		}
	}
	InstallKeyCall struct {
		CallCount int
		Receives  struct {
			Key string
		}
		Returns struct {
			Error error
		}
	}
	UseKeyCall struct {
		CallCount int
		Receives  struct {
			Key string
		}
		Returns struct {
			Error error
		}
	}
	RemoveKeyCall struct {
		CallCount int
		Receives  struct {
			Key string
		}
		Returns struct {
			Error error
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

func (c *AgentClient) Members(wan bool) ([]*api.AgentMember, error) {
	c.MembersCall.CallCount++
	c.MembersCall.Receives.WAN = wan
	return c.MembersCall.Returns.Members, c.MembersCall.Returns.Error
}

func (c *AgentClient) JoinMembers() error {
	c.JoinMembersCall.CallCount++
	return c.JoinMembersCall.Returns.Error
}

func (c *AgentClient) ListKeys() ([]string, error) {
	c.ListKeysCall.CallCount++
	return c.ListKeysCall.Returns.Keys, c.ListKeysCall.Returns.Error
}

func (c *AgentClient) InstallKey(key string) error {
	c.InstallKeyCall.CallCount++
	c.InstallKeyCall.Receives.Key = key
	return c.InstallKeyCall.Returns.Error
}

func (c *AgentClient) UseKey(key string) error {
	c.UseKeyCall.CallCount++
	c.UseKeyCall.Receives.Key = key
	return c.UseKeyCall.Returns.Error
}

func (c *AgentClient) RemoveKey(key string) error {
	c.RemoveKeyCall.CallCount++
	c.RemoveKeyCall.Receives.Key = key
	return c.RemoveKeyCall.Returns.Error
}
