package driverhttp

import (
	"code.cloudfoundry.org/voldriver"
)

//go:generate counterfeiter -o ../voldriverfakes/fake_remote_client_factory.go . RemoteClientFactory

type RemoteClientFactory interface {
	NewRemoteClient(url string, tls *voldriver.TLSConfig) (voldriver.Driver, error)
}

func NewRemoteClientFactory() RemoteClientFactory {
	return &remoteClientFactory{}
}

type remoteClientFactory struct{}

func (_ *remoteClientFactory) NewRemoteClient(url string, tls *voldriver.TLSConfig) (voldriver.Driver, error) {
	return NewRemoteClient(url, tls)
}
