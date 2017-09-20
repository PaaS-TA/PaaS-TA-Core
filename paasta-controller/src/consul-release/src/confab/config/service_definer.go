package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"code.cloudfoundry.org/lager"
)

var createFile = os.Create
var syncFile = syncFileFn

func syncFileFn(f *os.File) error {
	return f.Sync()
}

type logger interface {
	Info(action string, data ...lager.Data)
	Error(action string, err error, data ...lager.Data)
}

type ServiceDefinition struct {
	ServiceName       string                   `json:"-"`
	Name              string                   `json:"name"`
	Check             *ServiceDefinitionCheck  `json:"check,omitempty"`
	Checks            []ServiceDefinitionCheck `json:"checks,omitempty"`
	Tags              []string                 `json:"tags,omitempty"`
	Address           string                   `json:"address,omitempty"`
	Port              int                      `json:"port,omitempty"`
	EnableTagOverride bool                     `json:"enableTagOverride,omitempty"`
	ID                string                   `json:"id,omitempty"`
	Token             string                   `json:"token,omitempty"`
}

type ServiceDefinitionCheck struct {
	Name              string `json:"name"`
	ID                string `json:"id,omitempty"`
	Script            string `json:"script,omitempty"`
	HTTP              string `json:"http,omitempty"`
	TCP               string `json:"tcp,omitempty"`
	TTL               string `json:"ttl,omitempty"`
	Interval          string `json:"interval,omitempty"`
	Timeout           string `json:"timeout,omitempty"`
	Notes             string `json:"notes,omitempty"`
	DockerContainerID string `json:"docker_container_id,omitempty"`
	Shell             string `json:"shell,omitempty"`
	Status            string `json:"status,omitempty"`
	ServiceID         string `json:"service_id,omitempty"`
}

type ServiceDefiner struct {
	Logger logger
}

func (s ServiceDefiner) GenerateDefinitions(config Config) []ServiceDefinition {
	definitions := []ServiceDefinition{}

	for name, service := range config.Consul.Agent.Services {
		s.Logger.Info("service-definer.generate-definitions.define", lager.Data{
			"service": name,
		})
		definition := ServiceDefinition{
			ServiceName: name,
			Name:        strings.Replace(name, "_", "-", -1),
			Check: &ServiceDefinitionCheck{
				Name:     "dns_health_check",
				Script:   fmt.Sprintf("/var/vcap/jobs/%s/bin/dns_health_check", name),
				Interval: "3s",
			},
			Checks:            service.Checks,
			Tags:              []string{fmt.Sprintf("%s-%d", strings.Replace(config.Node.Name, "_", "-", -1), config.Node.Index)},
			Address:           service.Address,
			Port:              service.Port,
			EnableTagOverride: service.EnableTagOverride,
			ID:                service.ID,
			Token:             service.Token,
		}

		if service.Name != "" {
			definition.Name = service.Name
		}

		if service.Check != nil {
			definition.Check = service.Check
		}

		if service.Tags != nil {
			definition.Tags = service.Tags
		}

		definitions = append(definitions, definition)
	}

	return definitions
}

func (s ServiceDefiner) WriteDefinitions(configDir string, definitions []ServiceDefinition) error {
	for _, definition := range definitions {
		path := filepath.Join(configDir, fmt.Sprintf("service-%s.json", definition.ServiceName))
		s.Logger.Info("service-definer.write-definitions.write", lager.Data{
			"path": path,
		})

		file, err := createFile(path)
		if err != nil {
			err = errors.New(err.Error())
			s.Logger.Error("service-definer.write-definitions.write.failed", err, lager.Data{
				"path": path,
			})
			return err
		}

		err = json.NewEncoder(file).Encode(map[string]ServiceDefinition{
			"service": definition,
		})
		if err != nil {
			err = errors.New(err.Error())
			s.Logger.Error("service-definer.write-definitions.write.failed", err, lager.Data{
				"path": path,
			})
			file.Close()
			return err
		}
		if err := syncFile(file); err != nil {
			file.Close()
			return err
		}

		file.Close()

		s.Logger.Info("service-definer.write-definitions.write.success", lager.Data{
			"path": path,
		})
	}
	return nil
}
