package fakes

type FakeconsulAPIStatus struct {
	LeaderCall struct {
		CallCount int
		Returns   struct {
			Leader string
			Error  error
		}
	}
}

func (s *FakeconsulAPIStatus) Leader() (string, error) {
	s.LeaderCall.CallCount++
	return s.LeaderCall.Returns.Leader, s.LeaderCall.Returns.Error
}
