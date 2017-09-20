package fakes

type AgentRunner struct {
	RunCalls struct {
		CallCount int
		Returns   struct {
			Errors []error
		}
	}

	StopCall struct {
		CallCount int
		Returns   struct {
			Error error
		}
	}

	WaitCall struct {
		CallCount int
		Returns   struct {
			Error error
		}
	}

	CleanupCall struct {
		CallCount int
		Returns   struct {
			Error error
		}
	}

	WritePIDCall struct {
		CallCount int
		Returns   struct {
			Error error
		}
	}
}

func (r *AgentRunner) Run() error {
	var err error
	if len(r.RunCalls.Returns.Errors) > r.RunCalls.CallCount {
		err = r.RunCalls.Returns.Errors[r.RunCalls.CallCount]
	}
	r.RunCalls.CallCount++
	return err
}

func (r *AgentRunner) Stop() error {
	r.StopCall.CallCount++
	return r.StopCall.Returns.Error
}

func (r *AgentRunner) Wait() error {
	r.WaitCall.CallCount++
	return r.WaitCall.Returns.Error
}

func (r *AgentRunner) Cleanup() error {
	r.CleanupCall.CallCount++
	return r.CleanupCall.Returns.Error
}

func (r *AgentRunner) WritePID() error {
	r.WritePIDCall.CallCount++
	return r.WritePIDCall.Returns.Error
}
