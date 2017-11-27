package chaperon

import (
	"github.com/cloudfoundry-incubator/consul-release/src/confab/config"
	"github.com/cloudfoundry-incubator/consul-release/src/confab/utils"
)

type Client struct {
	controller     controller
	keyringRemover keyringRemover
	configWriter   configWriter
}

type keyringRemover interface {
	Execute() error
}

func NewClient(controller controller, keyringRemover keyringRemover, configWriter configWriter) Client {
	return Client{
		controller:     controller,
		keyringRemover: keyringRemover,
		configWriter:   configWriter,
	}
}

func (c Client) Start(cfg config.Config, timeout utils.Timeout) error {
	if err := c.configWriter.Write(cfg); err != nil {
		return err
	}

	if err := c.controller.WriteServiceDefinitions(); err != nil {
		return err
	}

	if err := c.keyringRemover.Execute(); err != nil {
		return err
	}

	if err := c.controller.BootAgent(timeout); err != nil {
		return err
	}

	if err := c.controller.ConfigureClient(); err != nil {
		return err
	}

	return nil
}

func (c Client) Stop() {
	c.controller.StopAgent()
}
