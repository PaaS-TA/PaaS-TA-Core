package helpers

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os"
	"strings"
)

type Config struct {
	BOSH                  ConfigBOSH     `json:"bosh"`
	AWS                   ConfigAWS      `json:"aws"`
	Registry              ConfigRegistry `json:"registry"`
	TurbulenceReleaseName string
	IPTablesAgent         bool
	CF                    ConfigCF `json:"cf"`
}

type ConfigBOSH struct {
	Target             string `json:"target"`
	Host               string `json:"host"`
	Username           string `json:"username"`
	Password           string `json:"password"`
	DirectorCACert     string `json:"director_ca_cert"`
	DeploymentVarsPath string `json:"deployment_vars_path"`
}

type ConfigAWS struct {
	Subnet                string   `json:"subnet"`
	AccessKeyID           string   `json:"access_key_id"`
	SecretAccessKey       string   `json:"secret_access_key"`
	DefaultKeyName        string   `json:"default_key_name"`
	DefaultSecurityGroups []string `json:"default_security_groups"`
	Region                string   `json:"region"`
}

type ConfigRegistry struct {
	Host     string `json:"host"`
	Port     int    `json:"port"`
	Username string `json:"username"`
	Password string `json:"password"`
}

type ConfigCF struct {
	Domain string `json:"domain"`
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

	config.BOSH.Host, err = addBOSHHost(config.BOSH.Target)
	if err != nil {
		return Config{}, err
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
		config.AWS.Region = "us-east-1"
	}

	return config, nil
}

func addBOSHHost(target string) (string, error) {
	u, err := url.Parse(target)
	if err != nil {
		return "", err
	}

	return u.Hostname(), nil
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

func ConfigPath() (string, error) {
	path := os.Getenv("EATS_CONFIG")
	if path == "" || !strings.HasPrefix(path, "/") {
		return "", fmt.Errorf("$EATS_CONFIG %q does not specify an absolute path to test config file", path)
	}

	return path, nil
}

func EtcdDevReleaseVersion() string {
	version := os.Getenv("ETCD_RELEASE_VERSION")
	if version == "" {
		version = "latest"
	}

	return version
}
