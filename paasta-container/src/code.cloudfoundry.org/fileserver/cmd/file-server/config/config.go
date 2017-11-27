package config

import (
	"encoding/json"
	"os"

	"code.cloudfoundry.org/debugserver"
	"code.cloudfoundry.org/lager/lagerflags"
)

type FileServerConfig struct {
	ServerAddress   string `json:"server_address,omitempty"`
	StaticDirectory string `json:"static_directory,omitempty"`
	DropsondePort   int    `json:"dropsonde_port,omitempty"`
	ConsulCluster   string `json:"consul_cluster,omitempty"`

	debugserver.DebugServerConfig
	lagerflags.LagerConfig
}

func DefaultFileServerConfig() FileServerConfig {
	return FileServerConfig{
		ServerAddress: "0.0.0.0:8080",
		DropsondePort: 3457,
		LagerConfig:   lagerflags.DefaultLagerConfig(),
	}
}

func NewFileServerConfig(configPath string) (FileServerConfig, error) {
	fileServerConfig := DefaultFileServerConfig()

	configFile, err := os.Open(configPath)
	if err != nil {
		return FileServerConfig{}, err
	}

	defer configFile.Close()

	decoder := json.NewDecoder(configFile)
	err = decoder.Decode(&fileServerConfig)
	if err != nil {
		return FileServerConfig{}, err
	}

	return fileServerConfig, nil
}
