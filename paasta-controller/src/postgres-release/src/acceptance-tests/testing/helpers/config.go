package helpers

import (
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"strings"

	"gopkg.in/yaml.v2"
)

const MissingCertificateMsg = "missing `director_ca_cert` - specify BOSH director CA certificate"
const IncorrectEnvMsg = "$PGATS_CONFIG %q does not specify an absolute path to test config file"

type PgatsConfig struct {
	Bosh              BOSHConfig      `yaml:"bosh"`
	BoshCC            BOSHCloudConfig `yaml:"cloud_configs"`
	PGReleaseVersion  string          `yaml:"postgres_release_version"`
	PostgreSQLVersion string          `yaml:"postgresql_version"`
	VersionsFile      string          `yaml:"versions_file"`
}

var DefaultPgatsConfig = PgatsConfig{
	Bosh:              DefaultBOSHConfig,
	BoshCC:            DefaultCloudConfig,
	PGReleaseVersion:  "latest",
	PostgreSQLVersion: "current",
	VersionsFile:      "",
}

func LoadConfig(configFilePath string) (PgatsConfig, error) {
	var config PgatsConfig
	config = DefaultPgatsConfig

	configFile, err := ioutil.ReadFile(configFilePath)
	if err != nil {
		return PgatsConfig{}, err
	}
	if err := yaml.Unmarshal(configFile, &config); err != nil {
		return PgatsConfig{}, err
	}

	if config.Bosh.DirectorCACert == "" {
		return PgatsConfig{}, errors.New(MissingCertificateMsg)
	}
	return config, nil
}

func ConfigPath() (string, error) {
	path := os.Getenv("PGATS_CONFIG")
	if path == "" || !strings.HasPrefix(path, "/") {
		return "", fmt.Errorf(IncorrectEnvMsg, path)
	}

	return path, nil
}
