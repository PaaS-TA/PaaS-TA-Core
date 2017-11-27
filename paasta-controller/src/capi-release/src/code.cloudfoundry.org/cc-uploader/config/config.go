package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"strings"
	"time"

	"code.cloudfoundry.org/debugserver"
	"code.cloudfoundry.org/lager/lagerflags"
)

type Duration time.Duration

func (d *Duration) UnmarshalJSON(data []byte) error {
	var s string
	err := json.Unmarshal(data, &s)
	if err != nil {
		return err
	}

	dur, err := time.ParseDuration(s)
	if err != nil {
		return err
	}

	*d = Duration(dur)
	return nil
}

func (d Duration) MarshalJSON() ([]byte, error) {
	t := time.Duration(d)
	return []byte(fmt.Sprintf(`"%s"`, t.String())), nil
}

type MutualTLS struct {
	ListenAddress string `json:"listen_addr"`
	CACert        string `json:"ca_cert"`
	ServerCert    string `json:"server_cert"`
	ServerKey     string `json:"server_key"`
}

type UploaderConfig struct {
	ConsulCluster        string                        `json:"consul_cluster"`
	DropsondePort        int                           `json:"dropsonde_port"`
	ListenAddress        string                        `json:"listen_addr"`
	CCJobPollingInterval Duration                      `json:"job_polling_interval"`
	LagerConfig          lagerflags.LagerConfig        `json:"lager_config"`
	DebugServerConfig    debugserver.DebugServerConfig `json:"debug_server_config"`
	CCClientCert         string                        `json:"cc_client_cert"`
	CCClientKey          string                        `json:"cc_client_key"`
	CCCACert             string                        `json:"cc_ca_cert"`
	MutualTLS            MutualTLS                     `json:"mutual_tls"`
}

func DefaultUploaderConfig() UploaderConfig {
	return UploaderConfig{
		DropsondePort:        3457,
		LagerConfig:          lagerflags.DefaultLagerConfig(),
		ListenAddress:        "0.0.0.0:9090",
		CCJobPollingInterval: Duration(1 * time.Second),
	}
}

func NewUploaderConfig(configPath string) (UploaderConfig, error) {
	configFile, err := ioutil.ReadFile(configPath)
	if err != nil {
		return UploaderConfig{}, err
	}

	uploaderConfig := DefaultUploaderConfig()

	err = json.Unmarshal(configFile, &uploaderConfig)
	if err != nil {
		return UploaderConfig{}, err
	}

	err = uploaderConfig.validate()
	if err != nil {
		return UploaderConfig{}, err
	}

	return uploaderConfig, nil
}

func (uploaderConfig *UploaderConfig) validate() error {
	missingRequiredValues := make([]string, 0)

	if uploaderConfig.MutualTLS.ListenAddress == "" {
		missingRequiredValues = append(missingRequiredValues, "'mutual_tls.listen_addr'")
	}
	if uploaderConfig.MutualTLS.CACert == "" {
		missingRequiredValues = append(missingRequiredValues, "'mutual_tls.ca_cert'")
	}
	if uploaderConfig.MutualTLS.ServerCert == "" {
		missingRequiredValues = append(missingRequiredValues, "'mutual_tls.server_cert'")
	}
	if uploaderConfig.MutualTLS.ServerKey == "" {
		missingRequiredValues = append(missingRequiredValues, "'mutual_tls.server_key'")
	}

	if len(missingRequiredValues) > 0 {
		errorMsg := fmt.Sprintf("The following required config values were not provided: %s", strings.Join(missingRequiredValues, ","))
		return errors.New(errorMsg)
	}

	return nil
}
