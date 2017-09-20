package fakes

import "github.com/cloudfoundry-incubator/consul-release/src/confab/config"

type ServiceDefiner struct {
	GenerateDefinitionsCall struct {
		Receives struct {
			Config config.Config
		}
		Returns struct {
			Definitions []config.ServiceDefinition
		}
	}

	WriteDefinitionsCall struct {
		Receives struct {
			Definitions []config.ServiceDefinition
			ConfigDir   string
		}
		Returns struct {
			Error error
		}
	}
}

func (d *ServiceDefiner) WriteDefinitions(configDir string, definitions []config.ServiceDefinition) error {
	d.WriteDefinitionsCall.Receives.Definitions = definitions
	d.WriteDefinitionsCall.Receives.ConfigDir = configDir
	return d.WriteDefinitionsCall.Returns.Error
}

func (d *ServiceDefiner) GenerateDefinitions(config config.Config) []config.ServiceDefinition {
	d.GenerateDefinitionsCall.Receives.Config = config
	return d.GenerateDefinitionsCall.Returns.Definitions
}
