package config

import (
	"encoding/json"
	"io/ioutil"

	"code.cloudfoundry.org/debugserver"
	"code.cloudfoundry.org/lager/lagerflags"
)

type StagerConfig struct {
	BBSAddress                string                        `json:"bbs_api_url"`
	BBSCACert                 string                        `json:"bbs_ca_cert"`
	BBSClientCert             string                        `json:"bbs_client_cert"`
	BBSClientKey              string                        `json:"bbs_client_key"`
	BBSClientSessionCacheSize int                           `json:"bbs_client_cache_size"`
	BBSMaxIdleConnsPerHost    int                           `json:"bbs_max_idle_conns_per_host"`
	CCBaseUrl                 string                        `json:"cc_base_url"`
	CCPassword                string                        `json:"cc_basic_auth_password"`
	CCUploaderURL             string                        `json:"cc_uploader_url"`
	CCUsername                string                        `json:"cc_basic_auth_username"`
	ConsulCluster             string                        `json:"consul_cluster"`
	DebugServerConfig         debugserver.DebugServerConfig `json:"debug_server_config"`
	DockerStagingStack        string                        `json:"docker_staging_stack"`
	DropsondePort             int                           `json:"dropsonde_port"`
	InsecureDockerRegistries  []string                      `json:"insecure_docker_registries"`
	FileServerUrl             string                        `json:"file_server_url"`
	LagerConfig               lagerflags.LagerConfig        `json:"lager_config"`
	Lifecycles                []string                      `json:"lifecycles"`
	ListenAddress             string                        `json:"stager_listen_addr"`
	PrivilegedContainers      bool                          `json:"diego_privileged_containers"`
	SkipCertVerify            bool                          `json:"skip_cert_verify"`
	StagingTaskCallbackURL    string                        `json:"staging_task_callback_url"`
}

func DefaultStagerConfig() StagerConfig {
	return StagerConfig{
		BBSClientSessionCacheSize: 0,
		BBSMaxIdleConnsPerHost:    0,
		DropsondePort:             3457,
		LagerConfig:               lagerflags.DefaultLagerConfig(),
		PrivilegedContainers:      false,
		SkipCertVerify:            false,
	}
}

func NewStagerConfig(configPath string) (StagerConfig, error) {
	configFile, err := ioutil.ReadFile(configPath)
	if err != nil {
		return StagerConfig{}, err
	}

	stagerConfig := DefaultStagerConfig()

	err = json.Unmarshal(configFile, &stagerConfig)
	if err != nil {
		return StagerConfig{}, err
	}

	return stagerConfig, nil
}
