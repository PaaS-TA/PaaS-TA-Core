package fakes

type BootstrapChecker struct {
	StartInBootstrapModeCall struct {
		CallCount int
		Returns   struct {
			Bootstrap bool
			Error     error
		}
	}
}

func (b *BootstrapChecker) StartInBootstrapMode() (bool, error) {
	b.StartInBootstrapModeCall.CallCount++
	return b.StartInBootstrapModeCall.Returns.Bootstrap, b.StartInBootstrapModeCall.Returns.Error
}
