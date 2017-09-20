package etcd_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/pivotal-cf-experimental/destiny/consul"
	"github.com/pivotal-cf-experimental/destiny/etcd"
)

var _ = Describe("Config", func() {
	Describe("NewConfigWithDefaults", func() {
		It("applies the default values for fields that are missing", func() {
			config := etcd.NewConfigWithDefaults(etcd.Config{})
			Expect(config.Secrets.Consul.CACert).To(Equal(consul.CACert))
			Expect(config.Secrets.Consul.EncryptKey).To(Equal(consul.EncryptKey))
			Expect(config.Secrets.Consul.AgentKey).To(Equal(consul.DC1AgentKey))
			Expect(config.Secrets.Consul.AgentCert).To(Equal(consul.DC1AgentCert))
			Expect(config.Secrets.Consul.ServerKey).To(Equal(consul.DC1ServerKey))
			Expect(config.Secrets.Consul.ServerCert).To(Equal(consul.DC1ServerCert))

			Expect(config.Secrets.Etcd.CACert).To(Equal(etcd.CACert))
			Expect(config.Secrets.Etcd.ClientCert).To(Equal(etcd.ClientCert))
			Expect(config.Secrets.Etcd.ClientKey).To(Equal(etcd.ClientKey))
			Expect(config.Secrets.Etcd.PeerCACert).To(Equal(etcd.PeerCACert))
			Expect(config.Secrets.Etcd.PeerCert).To(Equal(etcd.PeerCert))
			Expect(config.Secrets.Etcd.PeerKey).To(Equal(etcd.PeerKey))
			Expect(config.Secrets.Etcd.ServerCert).To(Equal(etcd.ServerCert))
			Expect(config.Secrets.Etcd.ServerKey).To(Equal(etcd.ServerKey))
		})
	})
})
