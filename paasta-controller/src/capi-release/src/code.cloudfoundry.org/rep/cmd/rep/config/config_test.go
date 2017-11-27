package config_test

import (
	"io/ioutil"
	"os"
	"time"

	"code.cloudfoundry.org/debugserver"
	"code.cloudfoundry.org/durationjson"
	executorinit "code.cloudfoundry.org/executor/initializer"
	"code.cloudfoundry.org/executor/initializer/configuration"
	"code.cloudfoundry.org/lager/lagerflags"
	"code.cloudfoundry.org/locket"
	"code.cloudfoundry.org/rep/cmd/rep/config"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("RepConfig", func() {
	var configFilePath, configData string

	BeforeEach(func() {
		configData = `{
			"advertise_domain": "test-domain",
			"bbs_address": "1.1.1.1:9091",
			"bbs_ca_cert_file": "/tmp/bbs_ca_cert",
			"bbs_client_cert_file": "/tmp/bbs_client_cert",
			"bbs_client_key_file": "/tmp/bbs_client_key",
			"bbs_client_session_cache_size": 100,
			"bbs_max_idle_conns_per_host": 10,
			"ca_cert_file": "/tmp/ca_cert",
			"cache_path": "/tmp/cache",
			"cell_id" : "cell_z1/10",
			"communication_timeout": "11s",
			"consul_ca_cert": "/tmp/consul_ca_cert",
			"consul_client_cert": "/tmp/consul_client_cert",
			"consul_client_key": "/tmp/consul_client_key",
			"consul_cluster": "test cluster",
			"container_inode_limit": 1000,
			"container_max_cpu_shares": 4,
			"container_metrics_report_interval": "16s",
			"container_owner_name": "vcap",
			"container_reap_interval": "11s",
			"create_work_pool_size": 15,
			"debug_address": "5.5.5.5:9090",
			"delete_work_pool_size": 10,
			"disk_mb": "20000",
			"dropsonde_port": 8082,
			"enable_legacy_api_endpoints": true,
			"evacuation_polling_interval" : "13s",
			"evacuation_timeout" : "12s",
			"export_network_env_vars": false,
			"garden_addr": "100.0.0.1",
			"garden_healthcheck_command_retry_pause": "15s",
			"garden_healthcheck_emission_interval": "13s",
			"garden_healthcheck_interval": "12s",
			"garden_healthcheck_process_args": ["arg1", "arg2"],
			"garden_healthcheck_process_dir": "/tmp/process",
			"garden_healthcheck_process_env": ["env1", "env2"],
			"garden_healthcheck_process_path": "/tmp/healthcheck-process",
			"garden_healthcheck_process_user": "vcap_health",
			"garden_healthcheck_timeout": "14s",
			"garden_network": "test-network",
			"healthcheck_container_owner_name": "vcap_health",
			"healthcheck_work_pool_size": 10,
			"healthy_monitoring_interval": "5s",
			"healthy_monitoring_interval": "5s",
			"listen_addr": "0.0.0.0:8080",
			"listen_addr_admin": "0.0.0.1:8081",
			"listen_addr_securable": "0.0.0.0:8081",
			"lock_retry_interval": "5s",
			"lock_ttl": "5s",
			"locket_address": "0.0.0.0:909090909",
			"locket_ca_cert_file": "locket-ca-cert",
			"locket_client_cert_file": "locket-client-cert",
			"locket_client_key_file": "locket-client-key",
			"log_level": "debug",
			"max_cache_size_in_bytes": 101,
			"max_concurrent_downloads": 11,
			"memory_mb": "1000",
			"metrics_work_pool_size": 5,
			"optional_placement_tags": ["otag1", "otag2"],
			"path_to_ca_certs_for_downloads": "/tmp/ca-certs",
			"placement_tags": ["tag1", "tag2"],
			"polling_interval": "10s",
			"post_setup_hook": "post_setup_hook",
			"post_setup_user": "post_setup_user",
			"preloaded_root_fs": ["test:value", "test2:value2"],
			"read_work_pool_size": 15,
			"require_tls": true,
			"reserved_expiration_time": "10s",
			"server_cert_file": "/tmp/server_cert",
			"server_key_file": "/tmp/server_key",
			"session_name": "test",
			"skip_cert_verify": true,
			"supported_providers": ["provider1", "provider2"],
			"temp_dir": "/tmp/test",
			"trusted_system_certificates_path": "/tmp/trusted",
			"unhealthy_monitoring_interval": "10s",
			"volman_driver_paths": "/tmp/volman1:/tmp/volman2",
			"zone": "test-zone"
		}`
	})

	JustBeforeEach(func() {
		configFile, err := ioutil.TempFile("", "config-file")
		Expect(err).NotTo(HaveOccurred())

		n, err := configFile.WriteString(configData)
		Expect(err).NotTo(HaveOccurred())
		Expect(n).To(Equal(len(configData)))

		configFilePath = configFile.Name()
	})

	AfterEach(func() {
		err := os.RemoveAll(configFilePath)
		Expect(err).NotTo(HaveOccurred())
	})

	It("correctly parses the config file", func() {
		repConfig, err := config.NewRepConfig(configFilePath)
		Expect(err).NotTo(HaveOccurred())

		Expect(repConfig).To(Equal(config.RepConfig{
			AdvertiseDomain:           "test-domain",
			BBSAddress:                "1.1.1.1:9091",
			BBSCACertFile:             "/tmp/bbs_ca_cert",
			BBSClientCertFile:         "/tmp/bbs_client_cert",
			BBSClientKeyFile:          "/tmp/bbs_client_key",
			BBSClientSessionCacheSize: 100,
			BBSMaxIdleConnsPerHost:    10,
			CaCertFile:                "/tmp/ca_cert",
			CellID:                    "cell_z1/10",
			ClientLocketConfig: locket.ClientLocketConfig{
				LocketAddress:        "0.0.0.0:909090909",
				LocketCACertFile:     "locket-ca-cert",
				LocketClientCertFile: "locket-client-cert",
				LocketClientKeyFile:  "locket-client-key",
			},
			CommunicationTimeout: durationjson.Duration(11 * time.Second),
			ConsulCACert:         "/tmp/consul_ca_cert",
			ConsulClientCert:     "/tmp/consul_client_cert",
			ConsulClientKey:      "/tmp/consul_client_key",
			ConsulCluster:        "test cluster",
			DebugServerConfig: debugserver.DebugServerConfig{
				DebugAddress: "5.5.5.5:9090",
			},
			DropsondePort:             8082,
			EnableLegacyAPIServer:     true,
			EvacuationPollingInterval: durationjson.Duration(13 * time.Second),
			EvacuationTimeout:         durationjson.Duration(12 * time.Second),
			ExecutorConfig: executorinit.ExecutorConfig{
				CachePath:                          "/tmp/cache",
				ContainerInodeLimit:                1000,
				ContainerMaxCpuShares:              4,
				ContainerMetricsReportInterval:     16000000000,
				ContainerOwnerName:                 "vcap",
				ContainerReapInterval:              11000000000,
				CreateWorkPoolSize:                 15,
				DeleteWorkPoolSize:                 10,
				DiskMB:                             "20000",
				ExportNetworkEnvVars:               false,
				GardenAddr:                         "100.0.0.1",
				GardenHealthcheckCommandRetryPause: 15000000000,
				GardenHealthcheckEmissionInterval:  13000000000,
				GardenHealthcheckInterval:          12000000000,
				GardenHealthcheckProcessArgs:       []string{"arg1", "arg2"},
				GardenHealthcheckProcessDir:        "/tmp/process",
				GardenHealthcheckProcessEnv:        []string{"env1", "env2"},
				GardenHealthcheckProcessPath:       "/tmp/healthcheck-process",
				GardenHealthcheckProcessUser:       "vcap_health",
				GardenHealthcheckTimeout:           14000000000,
				GardenNetwork:                      "test-network",
				HealthCheckContainerOwnerName:      "vcap_health",
				HealthCheckWorkPoolSize:            10,
				HealthyMonitoringInterval:          5000000000,
				MaxCacheSizeInBytes:                101,
				MaxConcurrentDownloads:             11,
				MemoryMB:                           "1000",
				MetricsWorkPoolSize:                5,
				PathToCACertsForDownloads:          "/tmp/ca-certs",
				PostSetupHook:                      "post_setup_hook",
				PostSetupUser:                      "post_setup_user",
				ReadWorkPoolSize:                   15,
				ReservedExpirationTime:             10000000000,
				SkipCertVerify:                     true,
				TempDir:                            "/tmp/test",
				TrustedSystemCertificatesPath: "/tmp/trusted",
				UnhealthyMonitoringInterval:   10000000000,
				VolmanDriverPaths:             "/tmp/volman1:/tmp/volman2",
			},
			LagerConfig: lagerflags.LagerConfig{
				LogLevel: lagerflags.DEBUG,
			},
			ListenAddr:            "0.0.0.0:8080",
			ListenAddrAdmin:       "0.0.0.1:8081",
			ListenAddrSecurable:   "0.0.0.0:8081",
			LockRetryInterval:     durationjson.Duration(5 * time.Second),
			LockTTL:               durationjson.Duration(5 * time.Second),
			OptionalPlacementTags: []string{"otag1", "otag2"},
			PlacementTags:         []string{"tag1", "tag2"},
			PollingInterval:       durationjson.Duration(10 * time.Second),
			PreloadedRootFS:       map[string]string{"test": "value", "test2": "value2"},
			RequireTLS:            true,
			ServerCertFile:        "/tmp/server_cert",
			ServerKeyFile:         "/tmp/server_key",
			SessionName:           "test",
			SupportedProviders:    []string{"provider1", "provider2"},
			Zone:                  "test-zone",
		}))
	})

	Context("when the file does not exist", func() {
		It("returns an error", func() {
			_, err := config.NewRepConfig("foobar")
			Expect(err).To(HaveOccurred())
		})
	})

	Context("when the file does not contain valid json", func() {
		BeforeEach(func() {
			configData = "{{"
		})

		It("returns an error", func() {
			_, err := config.NewRepConfig(configFilePath)
			Expect(err).To(HaveOccurred())
		})

		Context("because the communication_timeout is not valid", func() {
			BeforeEach(func() {
				configData = `{"communication_timeout": 4234342342}`
			})

			It("returns an error", func() {
				_, err := config.NewRepConfig(configFilePath)
				Expect(err).To(HaveOccurred())
			})
		})
	})

	Context("default values", func() {
		BeforeEach(func() {
			configData = `{}`
		})

		It("uses default values when they are not specified", func() {
			repConfig, err := config.NewRepConfig(configFilePath)
			Expect(err).NotTo(HaveOccurred())

			Expect(repConfig).To(Equal(config.RepConfig{
				SessionName:               "rep",
				LockTTL:                   durationjson.Duration(locket.DefaultSessionTTL),
				LockRetryInterval:         durationjson.Duration(locket.RetryInterval),
				ListenAddr:                "0.0.0.0:1800",
				ListenAddrSecurable:       "0.0.0.0:1801",
				RequireTLS:                true,
				PollingInterval:           durationjson.Duration(30 * time.Second),
				DropsondePort:             3457,
				CommunicationTimeout:      durationjson.Duration(10 * time.Second),
				EvacuationPollingInterval: durationjson.Duration(10 * time.Second),
				AdvertiseDomain:           "cell.service.cf.internal",
				EnableLegacyAPIServer:     true,
				BBSClientSessionCacheSize: 0,
				EvacuationTimeout:         durationjson.Duration(10 * time.Minute),
				LagerConfig:               lagerflags.DefaultLagerConfig(),
				ExecutorConfig: executorinit.ExecutorConfig{
					GardenNetwork:                      "unix",
					GardenAddr:                         "/tmp/garden.sock",
					MemoryMB:                           configuration.Automatic,
					DiskMB:                             configuration.Automatic,
					TempDir:                            "/tmp",
					ReservedExpirationTime:             durationjson.Duration(time.Minute),
					ContainerReapInterval:              durationjson.Duration(time.Minute),
					ContainerInodeLimit:                200000,
					ContainerMaxCpuShares:              0,
					CachePath:                          "/tmp/cache",
					MaxCacheSizeInBytes:                10 * 1024 * 1024 * 1024,
					SkipCertVerify:                     false,
					HealthyMonitoringInterval:          durationjson.Duration(30 * time.Second),
					UnhealthyMonitoringInterval:        durationjson.Duration(500 * time.Millisecond),
					ExportNetworkEnvVars:               false,
					ContainerOwnerName:                 "executor",
					HealthCheckContainerOwnerName:      "executor-health-check",
					CreateWorkPoolSize:                 32,
					DeleteWorkPoolSize:                 32,
					ReadWorkPoolSize:                   64,
					MetricsWorkPoolSize:                8,
					HealthCheckWorkPoolSize:            64,
					MaxConcurrentDownloads:             5,
					GardenHealthcheckInterval:          durationjson.Duration(10 * time.Minute),
					GardenHealthcheckEmissionInterval:  durationjson.Duration(30 * time.Second),
					GardenHealthcheckTimeout:           durationjson.Duration(10 * time.Minute),
					GardenHealthcheckCommandRetryPause: durationjson.Duration(1 * time.Second),
					GardenHealthcheckProcessArgs:       []string{},
					GardenHealthcheckProcessEnv:        []string{},
					ContainerMetricsReportInterval:     durationjson.Duration(15 * time.Second),
				},
			}))
		})
	})
})
