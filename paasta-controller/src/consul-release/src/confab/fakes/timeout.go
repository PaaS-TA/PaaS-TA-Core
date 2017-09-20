package fakes

type Timeout struct{}

func (t *Timeout) Done() <-chan struct{} {
	return nil
}
