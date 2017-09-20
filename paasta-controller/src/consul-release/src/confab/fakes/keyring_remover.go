package fakes

type KeyringRemover struct {
	ExecuteCall struct {
		CallCount int
		Returns   struct {
			Error error
		}
	}
}

func (kr *KeyringRemover) Execute() error {
	kr.ExecuteCall.CallCount++

	return kr.ExecuteCall.Returns.Error
}
