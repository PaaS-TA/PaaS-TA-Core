package helpers

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
)

type Config struct {
	BOSH                  ConfigBOSH     `json:"bosh"`
	AWS                   ConfigAWS      `json:"aws"`
	Registry              ConfigRegistry `json:"registry"`
	ParallelNodes         int            `json:"parallel_nodes"`
	TurbulenceReleaseName string
	TurbulenceHost        string
	WindowsClients        bool `json:"windows_clients"`
}

type ConfigBOSH struct {
	Target         string       `json:"target"`
	Username       string       `json:"username"`
	Password       string       `json:"password"`
	DirectorCACert string       `json:"director_ca_cert"`
	Errand         ConfigErrand `json:"errand"`
}

type ConfigAWS struct {
	Subnets               []ConfigSubnet `json:"subnets"`
	CloudConfigSubnets    []ConfigSubnet `json:"cloud_config_subnets"`
	AccessKeyID           string         `json:"access_key_id"`
	SecretAccessKey       string         `json:"secret_access_key"`
	DefaultKeyName        string         `json:"default_key_name"`
	DefaultSecurityGroups []string       `json:"default_security_groups"`
	Region                string         `json:"region"`
}

type ConfigErrand struct {
	DefaultVMType             string `json:"default_vm_type"`
	DefaultPersistentDiskType string `json:"default_persistent_disk_type"`
}

type ConfigSubnet struct {
	ID            string `json:"id"`
	Range         string `json:"range"`
	AZ            string `json:"az"`
	SecurityGroup string `json:"security_group"`
}

type ConfigRegistry struct {
	Host     string `json:"host"`
	Port     int    `json:"port"`
	Username string `json:"username"`
	Password string `json:"password"`
}

func checkAbsolutePath(configValue, jsonKey string) error {
	if !strings.HasPrefix(configValue, "/") {
		return fmt.Errorf("invalid `%s` %q - must be an absolute path", jsonKey, configValue)
	}
	return nil
}

func LoadConfig(configFilePath string) (Config, error) {
	config, err := loadConfigJsonFromPath(configFilePath)
	if err != nil {
		return Config{}, err
	}

	if config.BOSH.Target == "" {
		return Config{}, errors.New("missing `bosh.target` - e.g. 'lite' or '192.168.50.4'")
	}

	if config.BOSH.Username == "" {
		return Config{}, errors.New("missing `bosh.username` - specify username for authenticating with BOSH")
	}

	if config.BOSH.Password == "" {
		return Config{}, errors.New("missing `bosh.password` - specify password for authenticating with BOSH")
	}

	config.TurbulenceReleaseName = "turbulence"

	if config.AWS.DefaultKeyName == "" {
		config.AWS.DefaultKeyName = "bosh"
	}

	if config.AWS.Region == "" {
		config.AWS.Region = "us-west-2"
	}

	if config.ParallelNodes == 0 {
		config.ParallelNodes = 1
	}

	return config, nil
}

func loadConfigJsonFromPath(configFilePath string) (Config, error) {
	configFile, err := os.Open(configFilePath)
	if err != nil {
		return Config{}, err
	}

	var config Config
	if err := json.NewDecoder(configFile).Decode(&config); err != nil {
		return Config{}, err
	}

	return config, nil
}

func ConsulReleaseVersion() string {
	version := os.Getenv("CONSUL_RELEASE_VERSION")
	if version == "" {
		version = "latest"
	}

	return version
}

func ConfigPath() (string, error) {
	path := os.Getenv("CONSATS_CONFIG")
	if path == "" || !strings.HasPrefix(path, "/") {
		return "", fmt.Errorf("$CONSATS_CONFIG %q does not specify an absolute path to test config file", path)
	}

	return path, nil
}
