package chaperon

import (
	"github.com/cloudfoundry-incubator/consul-release/src/confab/config"
	"github.com/cloudfoundry-incubator/consul-release/src/confab/utils"
)

type controller interface {
	WriteServiceDefinitions() error
	BootAgent(utils.Timeout) error
	ConfigureServer(utils.Timeout) error
	ConfigureClient() error
	StopAgent()
}

type configWriter interface {
	Write(config.Config) error
}

type bootstrapChecker interface {
	StartInBootstrapMode() (bool, error)
}

type Server struct {
	controller       controller
	configWriter     configWriter
	bootstrapChecker bootstrapChecker
}

func NewServer(controller controller, configWriter configWriter, bootstrapChecker bootstrapChecker) Server {
	return Server{
		controller:       controller,
		configWriter:     configWriter,
		bootstrapChecker: bootstrapChecker,
	}
}

func (s Server) Start(cfg config.Config, timeout utils.Timeout) error {
	if err := s.configWriter.Write(cfg); err != nil {
		return err
	}

	if err := s.controller.WriteServiceDefinitions(); err != nil {
		return err
	}

	if err := s.controller.BootAgent(timeout); err != nil {
		return err
	}

	var err error
	cfg.Consul.Agent.Bootstrap, err = s.bootstrapChecker.StartInBootstrapMode()
	if err != nil {
		return err
	}

	if cfg.Consul.Agent.Bootstrap {
		s.Stop()
		if err := s.configWriter.Write(cfg); err != nil {
			return err
		}
		if err := s.controller.BootAgent(timeout); err != nil {
			return err
		}
	}

	if err := s.controller.ConfigureServer(timeout); err != nil {
		return err
	}

	return nil
}

func (s Server) Stop() {
	s.controller.StopAgent()
}
