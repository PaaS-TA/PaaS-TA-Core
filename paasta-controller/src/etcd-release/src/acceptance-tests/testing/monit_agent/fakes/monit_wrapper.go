package fakes

type MonitWrapper struct {
	RunCall struct {
		CallCount int
		Receives  struct {
			Args []string
		}
		Returns struct {
			Error error
		}
	}
	OutputCall struct {
		CallCount int
		Receives  struct {
			Args []string
		}
		Returns struct {
			Output string
			Error  error
		}
	}
}

func (m *MonitWrapper) Output(args []string) (string, error) {
	m.OutputCall.CallCount++
	m.OutputCall.Receives.Args = args
	return m.OutputCall.Returns.Output, m.OutputCall.Returns.Error
}

func (m *MonitWrapper) Run(args []string) error {
	m.RunCall.CallCount++
	m.RunCall.Receives.Args = args
	return m.RunCall.Returns.Error
}
