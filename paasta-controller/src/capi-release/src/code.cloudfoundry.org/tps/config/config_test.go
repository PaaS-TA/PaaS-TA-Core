package config_test

import (
	"time"

	. "code.cloudfoundry.org/tps/config"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Config", func() {
	Context("Listener config", func() {
		It("generates a config with the default values", func() {
			listenerConfig, err := NewListenerConfig("../fixtures/empty_config.json")
			Expect(err).ToNot(HaveOccurred())

			Expect(listenerConfig.BBSMaxIdleConnsPerHost).To(Equal(0))
			Expect(listenerConfig.BulkLRPStatusWorkers).To(Equal(15))
			Expect(listenerConfig.DropsondePort).To(Equal(3457))
			Expect(listenerConfig.LagerConfig.LogLevel).To(Equal("info"))
			Expect(listenerConfig.ListenAddress).To(Equal("0.0.0.0:1518"))
			Expect(listenerConfig.MaxInFlightRequests).To(Equal(200))
			Expect(listenerConfig.SkipCertVerify).To(Equal(true))
		})

		It("reads from the config file and populates the config", func() {
			listenerConfig, err := NewListenerConfig("../fixtures/listener_config.json")
			Expect(err).ToNot(HaveOccurred())

			Expect(listenerConfig.BBSAddress).To(Equal("https://foobar.com"))
			Expect(listenerConfig.BBSCACert).To(Equal("/path/to/cert"))
			Expect(listenerConfig.BBSClientCert).To(Equal("/path/to/another/cert"))
			Expect(listenerConfig.BBSClientKey).To(Equal("/path/to/key"))
			Expect(listenerConfig.BBSMaxIdleConnsPerHost).To(Equal(10))
			Expect(listenerConfig.BulkLRPStatusWorkers).To(Equal(99))
			Expect(listenerConfig.ConsulCluster).To(Equal("https://consul.com"))
			Expect(listenerConfig.DebugServerConfig.DebugAddress).To(Equal("https://debugger.com"))
			Expect(listenerConfig.DropsondePort).To(Equal(666))
			Expect(listenerConfig.LagerConfig.LogLevel).To(Equal("debug"))
			Expect(listenerConfig.ListenAddress).To(Equal("https://tps.com/listen"))
			Expect(listenerConfig.MaxInFlightRequests).To(Equal(33))
			Expect(listenerConfig.SkipCertVerify).To(Equal(false))
			Expect(listenerConfig.TrafficControllerURL).To(Equal("https://trafficcontroller.com"))
		})
	})

	Context("Watcher config", func() {
		It("generates a config with the default values", func() {
			watcherConfig, err := NewWatcherConfig("../fixtures/empty_config.json")
			Expect(err).ToNot(HaveOccurred())

			Expect(watcherConfig.BBSClientSessionCacheSize).To(Equal(0))
			Expect(watcherConfig.BBSMaxIdleConnsPerHost).To(Equal(0))
			Expect(watcherConfig.DropsondePort).To(Equal(3457))
			Expect(watcherConfig.LagerConfig.LogLevel).To(Equal("info"))
			Expect(watcherConfig.MaxEventHandlingWorkers).To(Equal(500))
			Expect(watcherConfig.SkipConsulLock).To(Equal(false))
		})

		It("reads from the config file and populates the config", func() {
			watcherConfig, err := NewWatcherConfig("../fixtures/watcher_config.json")
			Expect(err).ToNot(HaveOccurred())

			Expect(watcherConfig.BBSAddress).To(Equal("https://foobar.com"))
			Expect(watcherConfig.BBSCACert).To(Equal("/path/to/cert"))
			Expect(watcherConfig.BBSClientCert).To(Equal("/path/to/another/cert"))
			Expect(watcherConfig.BBSClientKey).To(Equal("/path/to/key"))
			Expect(watcherConfig.BBSClientSessionCacheSize).To(Equal(1234))
			Expect(watcherConfig.BBSMaxIdleConnsPerHost).To(Equal(10))
			Expect(watcherConfig.ConsulCluster).To(Equal("https://consul.com"))
			Expect(watcherConfig.CCBaseUrl).To(Equal("https://cloudcontroller.com"))
			Expect(watcherConfig.ConsulCluster).To(Equal("https://consul.com"))
			Expect(watcherConfig.DebugServerConfig.DebugAddress).To(Equal("https://debugger.com"))
			Expect(watcherConfig.DropsondePort).To(Equal(666))
			Expect(watcherConfig.LagerConfig.LogLevel).To(Equal("debug"))
			Expect(watcherConfig.LockRetryInterval).To(Equal(Duration(100 * time.Second)))
			Expect(watcherConfig.LockTTL).To(Equal(Duration(200 * time.Second)))
			Expect(watcherConfig.MaxEventHandlingWorkers).To(Equal(33))
			Expect(watcherConfig.CCClientCert).To(Equal("/path/to/server.cert"))
			Expect(watcherConfig.CCClientKey).To(Equal("/path/to/server.key"))
			Expect(watcherConfig.CCCACert).To(Equal("/path/to/server-ca.cert"))
			Expect(watcherConfig.SkipConsulLock).To(Equal(true))
			Expect(watcherConfig.LocketAddress).To(Equal("https://locket.com"))
			Expect(watcherConfig.LocketCACertFile).To(Equal("/path/to/locket/ca-cert"))
			Expect(watcherConfig.LocketClientCertFile).To(Equal("/path/to/locket/cert"))
			Expect(watcherConfig.LocketClientKeyFile).To(Equal("/path/to/locket/key"))
		})
	})
})
