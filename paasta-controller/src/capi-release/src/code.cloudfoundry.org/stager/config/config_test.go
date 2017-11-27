package config_test

import (
	. "code.cloudfoundry.org/stager/config"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Config", func() {
	Context("Stager config", func() {
		It("generates a config with the default values", func() {
			stagerConfig, err := NewStagerConfig("../fixtures/empty_config.json")
			Expect(err).ToNot(HaveOccurred())

			Expect(stagerConfig.BBSClientSessionCacheSize).To(Equal(0))
			Expect(stagerConfig.BBSMaxIdleConnsPerHost).To(Equal(0))
			Expect(stagerConfig.DropsondePort).To(Equal(3457))
			Expect(stagerConfig.PrivilegedContainers).NotTo(BeTrue())
			Expect(stagerConfig.SkipCertVerify).NotTo(BeTrue())
			Expect(stagerConfig.BBSMaxIdleConnsPerHost).To(Equal(0))
			Expect(stagerConfig.LagerConfig.LogLevel).To(Equal("info"))
		})

		It("reads from the config file and populates the config", func() {
			stagerConfig, err := NewStagerConfig("../fixtures/stager_config.json")
			Expect(err).ToNot(HaveOccurred())
			Expect(stagerConfig.BBSAddress).To(Equal("http://bbs.example.com"))
			Expect(stagerConfig.BBSCACert).To(Equal("bbs-ca-cert"))
			Expect(stagerConfig.BBSClientCert).To(Equal("bbs-client-cert"))
			Expect(stagerConfig.BBSClientKey).To(Equal("bbs-client-key"))
			Expect(stagerConfig.BBSClientSessionCacheSize).To(Equal(10))
			Expect(stagerConfig.BBSMaxIdleConnsPerHost).To(Equal(11))
			Expect(stagerConfig.CCBaseUrl).To(Equal("cc_base_url"))
			Expect(stagerConfig.CCPassword).To(Equal("cc_basic_auth_password"))
			Expect(stagerConfig.CCUploaderURL).To(Equal("cc_uploader_url"))
			Expect(stagerConfig.CCUsername).To(Equal("cc_basic_auth_username"))
			Expect(stagerConfig.ConsulCluster).To(Equal("consul_cluster"))
			Expect(stagerConfig.DebugServerConfig.DebugAddress).To(Equal("debug_address"))
			Expect(stagerConfig.DockerStagingStack).To(Equal("docker_staging_stack"))
			Expect(stagerConfig.DropsondePort).To(Equal(12))
			Expect(stagerConfig.InsecureDockerRegistries).To(Equal([]string{"insecure_docker_registries"}))
			Expect(stagerConfig.FileServerUrl).To(Equal("file_server_url"))
			Expect(stagerConfig.LagerConfig.LogLevel).To(Equal("fatal"))
			Expect(stagerConfig.Lifecycles).To(Equal([]string{"lifecycles"}))
			Expect(stagerConfig.ListenAddress).To(Equal("stager_listen_addr"))
			Expect(stagerConfig.PrivilegedContainers).To(BeTrue())
			Expect(stagerConfig.SkipCertVerify).NotTo(BeTrue())
			Expect(stagerConfig.StagingTaskCallbackURL).To(Equal("staging_task_callback_url"))
		})
	})
})
