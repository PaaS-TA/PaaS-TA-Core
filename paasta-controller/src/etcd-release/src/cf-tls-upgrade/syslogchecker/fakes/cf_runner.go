package fakes

type CFRunner struct {
	RunCommand struct {
		Commands [][]string
		Receives struct {
			Args []string
			Stub func(args ...string) ([]byte, error)
		}
		Returns struct {
			Error error
		}
	}
}

func (r *CFRunner) Run(args ...string) ([]byte, error) {
	r.RunCommand.Commands = append(r.RunCommand.Commands, args)
	r.RunCommand.Receives.Args = args

	if r.RunCommand.Receives.Stub != nil {
		return r.RunCommand.Receives.Stub(args...)
	}

	return []byte(""), r.RunCommand.Returns.Error
}
