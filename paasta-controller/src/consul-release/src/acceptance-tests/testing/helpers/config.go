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
	BOSH           ConfigBOSH `json:"bosh"`
	ParallelNodes  int        `json:"parallel_nodes"`
	WindowsClients bool       `json:"windows_clients"`
}

type ConfigBOSH struct {
	Target         string `json:"target"`
	Host           string
	Username       string `json:"username"`
	Password       string `json:"password"`
	DirectorCACert string `json:"director_ca_cert"`
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
		return Config{}, errors.New("missing `bosh.target` - e.g. 'https://192.168.50.4:25555'")
	}

	config.BOSH.Host, err = addBOSHHost(config.BOSH.Target)
	if err != nil {
		return Config{}, err
	}

	if config.BOSH.DirectorCACert == "" {
		return Config{}, errors.New("missing `bosh.director_ca_cert` - specify CA cert for BOSH director validation")
	}

	if config.BOSH.Username == "" {
		return Config{}, errors.New("missing `bosh.username` - specify username for authenticating with BOSH")
	}

	if config.BOSH.Password == "" {
		return Config{}, errors.New("missing `bosh.password` - specify password for authenticating with BOSH")
	}

	if config.ParallelNodes == 0 {
		config.ParallelNodes = 1
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
