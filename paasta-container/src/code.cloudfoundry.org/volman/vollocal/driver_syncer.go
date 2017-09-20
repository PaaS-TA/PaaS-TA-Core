package vollocal

import (
	"time"

	"sync"

	"os"

	"fmt"
	"path/filepath"
	"regexp"

	"context"

	"code.cloudfoundry.org/clock"
	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/voldriver"
	"code.cloudfoundry.org/voldriver/driverhttp"
	"github.com/tedsuo/ifrit"
)

type DriverSyncer interface {
	Runner() ifrit.Runner
	Discover(logger lager.Logger) (map[string]voldriver.Driver, error)
}

type driverSyncer struct {
	sync.RWMutex
	logger        lager.Logger
	driverFactory DriverFactory
	scanInterval  time.Duration
	clock         clock.Clock

	driverRegistry DriverRegistry
	driverPaths    []string
}

func NewDriverSyncer(logger lager.Logger, driverRegistry DriverRegistry, driverPaths []string, scanInterval time.Duration, clock clock.Clock) *driverSyncer {
	return &driverSyncer{
		logger:        logger,
		driverFactory: NewDriverFactory(),
		scanInterval:  scanInterval,
		clock:         clock,

		driverRegistry: driverRegistry,
		driverPaths:    driverPaths,
	}
}

func NewDriverSyncerWithDriverFactory(logger lager.Logger, driverRegistry DriverRegistry, driverPaths []string, scanInterval time.Duration, clock clock.Clock, factory DriverFactory) *driverSyncer {
	return &driverSyncer{
		logger:        logger,
		driverFactory: factory,
		scanInterval:  scanInterval,
		clock:         clock,

		driverRegistry: driverRegistry,
		driverPaths:    driverPaths,
	}
}

func (d *driverSyncer) Runner() ifrit.Runner {
	return d
}

func (r *driverSyncer) Run(signals <-chan os.Signal, ready chan<- struct{}) error {
	logger := r.logger.Session("sync-drivers")
	logger.Info("start")
	defer logger.Info("end")

	timer := r.clock.NewTimer(r.scanInterval)
	defer timer.Stop()

	drivers, err := r.Discover(logger)
	if err != nil {
		return err
	}
	r.setDrivers(drivers)

	close(ready)

	newDriverCh := make(chan map[string]voldriver.Driver, 1)

	for {
		select {
		case <-timer.C():
			go func() {
				drivers, err := r.Discover(logger)
				if err != nil {
					logger.Error("volman-driver-discovery-failed", err)
					newDriverCh <- nil
				} else {
					newDriverCh <- drivers
				}
			}()

		case drivers := <-newDriverCh:
			r.setDrivers(drivers)
			timer.Reset(r.scanInterval)

		case signal := <-signals:
			logger.Info("received-signal", lager.Data{"signal": signal.String()})
			return nil
		}
	}
}

func (r *driverSyncer) setDrivers(drivers map[string]voldriver.Driver) {
	r.driverRegistry.Set(drivers)
}

func (r *driverSyncer) Discover(logger lager.Logger) (map[string]voldriver.Driver, error) {
	logger = logger.Session("discover")
	logger.Debug("start")
	logger.Info(fmt.Sprintf("Discovering drivers in %s", r.driverPaths))
	defer logger.Debug("end")

	endpoints := make(map[string]voldriver.Driver)
	for _, driverPath := range r.driverPaths {
		//precedence order: sock -> spec -> json
		spec_types := [3]string{"sock", "spec", "json"}
		for _, spec_type := range spec_types {
			matchingDriverSpecs, err := r.getMatchingDriverSpecs(logger, driverPath, spec_type)

			if err != nil {
				// untestable on linux, does glob work differently on windows???
				return map[string]voldriver.Driver{}, fmt.Errorf("Volman configured with an invalid driver path '%s', error occured list files (%s)", driverPath, err.Error())
			}
			if len(matchingDriverSpecs) > 0 {
				logger.Debug("driver-specs", lager.Data{"drivers": matchingDriverSpecs})
				endpoints = r.insertIfAliveAndNotFound(logger, endpoints, driverPath, matchingDriverSpecs)
			}
		}
	}
	return endpoints, nil
}

func (r *driverSyncer) getMatchingDriverSpecs(logger lager.Logger, path string, pattern string) ([]string, error) {
	logger.Debug("binaries", lager.Data{"path": path, "pattern": pattern})
	matchingDriverSpecs, err := filepath.Glob(path + "/*." + pattern)
	if err != nil { // untestable on linux, does glob work differently on windows???
		return nil, fmt.Errorf("Volman configured with an invalid driver path '%s', error occured list files (%s)", path, err.Error())
	}
	return matchingDriverSpecs, nil

}

func (r *driverSyncer) insertIfAliveAndNotFound(logger lager.Logger, endpoints map[string]voldriver.Driver, driverPath string, specs []string) map[string]voldriver.Driver {
	logger = logger.Session("insert-if-not-found")
	logger.Debug("start")
	defer logger.Debug("end")

	for _, spec := range specs {
		re := regexp.MustCompile("([^/]*/)?([^/]*)\\.(sock|spec|json)$")

		segs2 := re.FindAllStringSubmatch(spec, 1)
		if len(segs2) <= 0 {
			continue
		}
		specName := segs2[0][2]
		specFile := segs2[0][2] + "." + segs2[0][3]
		logger.Debug("insert-unique-spec", lager.Data{"specname": specName})
		_, ok := endpoints[specName]
		if ok == false {
			driver, err := r.driverFactory.Driver(logger, specName, driverPath, specFile)
			if err != nil {
				logger.Error("error-creating-driver", err)
				continue
			}

			env := driverhttp.NewHttpDriverEnv(logger, context.TODO())

			resp := driver.Activate(env)
			if resp.Err != "" {
				logger.Info("skipping-non-responsive-driver", lager.Data{"specname": specName})
			} else {
				driverImplementsErr := fmt.Errorf("driver-implements: %#v", resp.Implements)
				if len(resp.Implements) == 0 {
					logger.Error("driver-incorrect", driverImplementsErr)
					continue
				}

				if !driverImplements("VolumeDriver", resp.Implements) {
					logger.Error("driver-incorrect", driverImplementsErr)
					continue
				}
				endpoints[specName] = driver
			}
		}
	}
	return endpoints
}
