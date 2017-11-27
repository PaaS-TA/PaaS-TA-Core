package config_test

import (
	"encoding/json"
	"io/ioutil"
	"os"
	"time"

	"code.cloudfoundry.org/auctioneer/cmd/auctioneer/config"
	"code.cloudfoundry.org/debugserver"
	"code.cloudfoundry.org/durationjson"
	loggregator_v2 "code.cloudfoundry.org/go-loggregator/compatibility"
	"code.cloudfoundry.org/lager/lagerflags"
	"code.cloudfoundry.org/locket"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("AuctioneerConfig", func() {
	var configFilePath, configData string

	BeforeEach(func() {
		configData = `{
			"auction_runner_workers": 10,
			"bbs_address": "1.1.1.1:9091",
			"bbs_ca_cert_file": "/tmp/bbs_ca_cert",
			"bbs_client_cert_file": "/tmp/bbs_client_cert",
			"bbs_client_key_file": "/tmp/bbs_client_key",
			"bbs_client_session_cache_size": 100,
			"bbs_max_idle_conns_per_host": 10,
			"ca_cert_file": "/path-to-cert",
			"cell_state_timeout": "2s",
			"communication_timeout": "15s",
			"consul_cluster": "1.1.1.1",
			"debug_address": "127.0.0.1:17017",
			"dropsonde_port": 1234,
			"listen_address": "0.0.0.0:9090",
			"lock_retry_interval": "1m",
			"lock_ttl": "20s",
			"locket_address": "laksdjflksdajflkajsdf",
			"locket_ca_cert_file": "locket-ca-cert",
			"locket_client_cert_file": "locket-client-cert",
			"locket_client_key_file": "locket-client-key",
			"log_level": "debug",
			"loggregator": {
				"loggregator_use_v2_api": true,
				"loggregator_api_port": 1234,
				"loggregator_ca_path": "ca-path",
				"loggregator_cert_path": "cert-path",
				"loggregator_key_path": "key-path",
				"loggregator_job_deployment": "job-deployment",
				"loggregator_job_name": "job-name",
				"loggregator_job_index": "job-index",
				"loggregator_job_ip": "job-ip",
				"loggregator_job_origin": "job-origin"
			},
			"rep_ca_cert": "/var/vcap/jobs/auctioneer/config/rep.ca",
			"rep_client_cert": "/var/vcap/jobs/auctioneer/config/rep.crt",
			"rep_client_key": "/var/vcap/jobs/auctioneer/config/rep.key",
			"rep_client_session_cache_size": 10,
			"rep_require_tls": true,
			"server_cert_file": "/path-to-server-cert",
			"server_key_file": "/path-to-server-key",
			"skip_consul_lock": true,
			"starting_container_count_maximum": 10,
			"starting_container_weight": 0.5,
			"uuid": "bosh-boshy-bosh-bosh"
    }`
	})

	JustBeforeEach(func() {
		configFile, err := ioutil.TempFile("", "auctioneer-config-file")
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
		auctioneerConfig, err := config.NewAuctioneerConfig(configFilePath)
		Expect(err).NotTo(HaveOccurred())

		expectedConfig := config.AuctioneerConfig{
			AuctionRunnerWorkers:      10,
			BBSAddress:                "1.1.1.1:9091",
			BBSCACertFile:             "/tmp/bbs_ca_cert",
			BBSClientCertFile:         "/tmp/bbs_client_cert",
			BBSClientKeyFile:          "/tmp/bbs_client_key",
			BBSClientSessionCacheSize: 100,
			BBSMaxIdleConnsPerHost:    10,
			CACertFile:                "/path-to-cert",
			CellStateTimeout:          durationjson.Duration(2 * time.Second),
			ClientLocketConfig: locket.ClientLocketConfig{
				LocketAddress:        "laksdjflksdajflkajsdf",
				LocketCACertFile:     "locket-ca-cert",
				LocketClientCertFile: "locket-client-cert",
				LocketClientKeyFile:  "locket-client-key",
			},
			CommunicationTimeout: durationjson.Duration(15 * time.Second),
			ConsulCluster:        "1.1.1.1",
			DebugServerConfig: debugserver.DebugServerConfig{
				DebugAddress: "127.0.0.1:17017",
			},
			DropsondePort: 1234,
			LagerConfig: lagerflags.LagerConfig{
				LogLevel: "debug",
			},
			ListenAddress:     "0.0.0.0:9090",
			LockRetryInterval: durationjson.Duration(1 * time.Minute),
			LockTTL:           durationjson.Duration(20 * time.Second),
			LoggregatorConfig: loggregator_v2.Config{
				UseV2API:      true,
				APIPort:       1234,
				CACertPath:    "ca-path",
				CertPath:      "cert-path",
				KeyPath:       "key-path",
				JobDeployment: "job-deployment",
				JobName:       "job-name",
				JobIndex:      "job-index",
				JobIP:         "job-ip",
				JobOrigin:     "job-origin",
			},
			RepCACert:                     "/var/vcap/jobs/auctioneer/config/rep.ca",
			RepClientCert:                 "/var/vcap/jobs/auctioneer/config/rep.crt",
			RepClientKey:                  "/var/vcap/jobs/auctioneer/config/rep.key",
			RepClientSessionCacheSize:     10,
			RepRequireTLS:                 true,
			ServerCertFile:                "/path-to-server-cert",
			ServerKeyFile:                 "/path-to-server-key",
			SkipConsulLock:                true,
			StartingContainerCountMaximum: 10,
			StartingContainerWeight:       .5,
			UUID: "bosh-boshy-bosh-bosh",
		}

		Expect(auctioneerConfig).To(Equal(expectedConfig))
	})

	Context("when the file does not exist", func() {
		It("returns an error", func() {
			_, err := config.NewAuctioneerConfig("foobar")
			Expect(err).To(HaveOccurred())
		})
	})

	Context("when the file does not contain valid json", func() {
		BeforeEach(func() {
			configData = "{{"
		})

		It("returns an error", func() {
			_, err := config.NewAuctioneerConfig(configFilePath)
			Expect(err).To(HaveOccurred())
		})
	})

	Context("default values", func() {
		BeforeEach(func() {
			configData = `{}`
		})

		It("uses default values when they are not specified", func() {
			auctioneerConfig, err := config.NewAuctioneerConfig(configFilePath)
			Expect(err).NotTo(HaveOccurred())

			Expect(auctioneerConfig).To(Equal(config.DefaultAuctioneerConfig()))
		})

		Context("when serialized from AuctioneerConfig", func() {
			BeforeEach(func() {
				auctioneerConfig := config.AuctioneerConfig{}
				bytes, err := json.Marshal(auctioneerConfig)
				Expect(err).NotTo(HaveOccurred())
				configData = string(bytes)
			})

			It("uses default values when they are not specified", func() {
				auctioneerConfig, err := config.NewAuctioneerConfig(configFilePath)
				Expect(err).NotTo(HaveOccurred())

				Expect(auctioneerConfig).To(Equal(config.DefaultAuctioneerConfig()))
			})
		})
	})
})
