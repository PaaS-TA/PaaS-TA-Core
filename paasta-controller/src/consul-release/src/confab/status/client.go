package status

type consulAPIStatus interface {
	Leader() (string, error)
}

type Client struct {
	ConsulAPIStatus consulAPIStatus
}

func (c Client) Leader() (string, error) {
	return c.ConsulAPIStatus.Leader()
}
