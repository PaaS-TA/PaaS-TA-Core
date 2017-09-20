package fakes

import "github.com/coreos/go-etcd/etcd"

type Client struct {
	GetCall struct {
		Returns struct {
			Value func(string) string
			Error error
		}
	}

	SetCall struct {
		Receives struct {
			Key   string
			Value string
			TTL   uint64
		}
		Returns struct {
			Error error
		}
	}

	CloseCall struct {
		CallCount int
	}
}

func NewClient() *Client {
	client := Client{}
	client.GetCall.Returns.Value = func(key string) string {
		return ""
	}

	return &client
}

func (c *Client) Get(key string, sort, recursive bool) (*etcd.Response, error) {
	return &etcd.Response{
		Node: &etcd.Node{
			Value: c.GetCall.Returns.Value(key),
		},
	}, c.GetCall.Returns.Error
}

func (c *Client) Set(key string, value string, ttl uint64) (*etcd.Response, error) {
	c.SetCall.Receives.Key = key
	c.SetCall.Receives.Value = value
	c.SetCall.Receives.TTL = ttl

	return nil, c.SetCall.Returns.Error
}

func (c *Client) Close() {
	c.CloseCall.CallCount++
}
