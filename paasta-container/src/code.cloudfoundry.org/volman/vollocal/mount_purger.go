package vollocal

import (
	"errors"
	"os"

	"context"

	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/voldriver"
	"code.cloudfoundry.org/voldriver/driverhttp"
	"github.com/tedsuo/ifrit"
)

type MountPurger interface {
	Runner() ifrit.Runner
	PurgeMounts(logger lager.Logger) error
}

type mountPurger struct {
	logger   lager.Logger
	registry DriverRegistry
}

func NewMountPurger(logger lager.Logger, registry DriverRegistry) MountPurger {
	return &mountPurger{
		logger,
		registry,
	}
}

func (p *mountPurger) Runner() ifrit.Runner {
	return p
}

func (p *mountPurger) Run(signals <-chan os.Signal, ready chan<- struct{}) error {

	if err := p.PurgeMounts(p.logger); err != nil {
		return err
	}

	close(ready)
	<-signals
	return nil
}

func (p *mountPurger) PurgeMounts(logger lager.Logger) error {
	logger = logger.Session("purge-mounts")
	logger.Info("start")
	defer logger.Info("end")

	drivers := p.registry.Drivers()

	for _, driver := range drivers {
		env := driverhttp.NewHttpDriverEnv(logger, context.TODO())
		listResponse := driver.List(env)
		for _, mount := range listResponse.Volumes {
			env = driverhttp.NewHttpDriverEnv(logger, context.TODO())
			errorResponse := driver.Unmount(env, voldriver.UnmountRequest{Name: mount.Name})
			if errorResponse.Err != "" {
				logger.Error("failed-purging-volume-mount", errors.New(errorResponse.Err))
			}
		}
	}

	return nil
}
