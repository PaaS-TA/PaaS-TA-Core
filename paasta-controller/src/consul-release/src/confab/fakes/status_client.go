package fakes

type StatusClient struct {
	LeaderCall struct {
		Stub      func() (string, error)
		CallCount int
		Returns   struct {
			Leader string
			Error  error
		}
	}
}

func (c *StatusClient) Leader() (string, error) {
	c.LeaderCall.CallCount++

	if c.LeaderCall.Stub != nil {
		return c.LeaderCall.Stub()
	}

	return c.LeaderCall.Returns.Leader, c.LeaderCall.Returns.Error
}
