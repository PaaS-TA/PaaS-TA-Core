package etcd

import goetcd "github.com/coreos/go-etcd/etcd"

type GoEtcd interface {
	Get(key string, sort, recursive bool) (*goetcd.Response, error)
	Set(key string, value string, ttl uint64) (*goetcd.Response, error)
	Close()
}

type Client struct {
	goEtcdClient GoEtcd
}

func NewClient(client GoEtcd) Client {
	return Client{
		goEtcdClient: client,
	}
}

func (c Client) Get(key string) (string, error) {
	response, err := c.goEtcdClient.Get(key, false, false)
	if err != nil {
		return "", err
	}

	return response.Node.Value, nil
}

func (c Client) Set(key string, value string) error {
	_, err := c.goEtcdClient.Set(key, value, 6000)
	if err != nil {
		return err
	}

	return nil
}

func (c Client) Close() {
	if c.goEtcdClient != nil {
		c.goEtcdClient.Close()
	}
}
