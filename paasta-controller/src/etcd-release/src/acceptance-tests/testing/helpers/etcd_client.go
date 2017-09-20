package helpers

import (
	"acceptance-tests/testing/testconsumer/etcd"
	"fmt"

	goetcd "github.com/coreos/go-etcd/etcd"
)

func NewEtcdWatcher(machines []string) *Watcher {
	addresses := []string{}
	for _, machine := range machines {
		addresses = append(addresses, fmt.Sprintf("http://%s:4001", machine))
	}
	return Watch(goetcd.NewClient(addresses), "/")
}

func NewEtcdClient(machines []string) etcd.Client {
	client := goetcd.NewClient(machines)
	client.SetConsistency(goetcd.STRONG_CONSISTENCY)

	return etcd.NewClient(client)
}

func NewEtcdTLSClient(machines []string, certFile, keyFile, caCertFile string) (etcd.Client, error) {
	client, err := goetcd.NewTLSClient(machines, certFile, keyFile, caCertFile)
	if err != nil {
		return etcd.Client{}, err
	}
	client.SetConsistency(goetcd.STRONG_CONSISTENCY)

	return etcd.NewClient(client), nil
}
