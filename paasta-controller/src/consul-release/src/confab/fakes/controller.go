package fakes

import (
	"github.com/cloudfoundry-incubator/consul-release/src/confab/utils"
	"github.com/hashicorp/consul/command/agent"
)

type Controller struct {
	WriteServiceDefinitionsCall struct {
		CallCount int
		Returns   struct {
			Error error
		}
	}

	BootAgentCall struct {
		CallCount int
		Stub      func(timeout utils.Timeout) error
		Receives  struct {
			Timeout utils.Timeout
		}
		Returns struct {
			Error error
		}
	}

	ConfigureServerCall struct {
		CallCount int
		Receives  struct {
			Timeout   utils.Timeout
			RPCClient *agent.RPCClient
		}
		Returns struct {
			Error error
		}
	}

	ConfigureClientCall struct {
		CallCount int
		Returns   struct {
			Error error
		}
	}

	StopAgentCall struct {
		CallCount int
		Receives  struct {
			RPCClient *agent.RPCClient
		}
	}
}

func (c *Controller) WriteServiceDefinitions() error {
	c.WriteServiceDefinitionsCall.CallCount++

	return c.WriteServiceDefinitionsCall.Returns.Error
}

func (c *Controller) BootAgent(timeout utils.Timeout) error {
	c.BootAgentCall.CallCount++
	c.BootAgentCall.Receives.Timeout = timeout

	if c.BootAgentCall.Stub != nil {
		return c.BootAgentCall.Stub(timeout)
	}

	return c.BootAgentCall.Returns.Error
}

func (c *Controller) ConfigureServer(timeout utils.Timeout, rpcClient *agent.RPCClient) error {
	c.ConfigureServerCall.CallCount++
	c.ConfigureServerCall.Receives.Timeout = timeout
	c.ConfigureServerCall.Receives.RPCClient = rpcClient

	return c.ConfigureServerCall.Returns.Error
}

func (c *Controller) ConfigureClient() error {
	c.ConfigureClientCall.CallCount++

	return c.ConfigureClientCall.Returns.Error
}

func (c *Controller) StopAgent(rpcClient *agent.RPCClient) {
	c.StopAgentCall.CallCount++
	c.StopAgentCall.Receives.RPCClient = rpcClient
}
