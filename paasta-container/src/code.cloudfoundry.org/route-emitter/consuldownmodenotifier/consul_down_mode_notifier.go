package consuldownmodenotifier

import (
	"os"
	"time"

	"code.cloudfoundry.org/clock"
	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/runtimeschema/metric"
)

type ConsulDownModeNofitier struct {
	logger   lager.Logger
	value    int
	clock    clock.Clock
	interval time.Duration
}

func NewConsulDownModeNotifier(
	logger lager.Logger,
	value int,
	clock clock.Clock,
	interval time.Duration,
) *ConsulDownModeNofitier {
	return &ConsulDownModeNofitier{
		logger: logger, value: value, clock: clock, interval: interval,
	}
}

func (p *ConsulDownModeNofitier) Run(signals <-chan os.Signal, ready chan<- struct{}) error {
	logger := p.logger.Session("consul-down-mode-notifier")
	logger.Info("starting")
	defer logger.Info("finished")
	retryTimer := p.clock.NewTimer(0)
	var consulDownMetric = metric.Metric("ConsulDownMode")

	close(ready)

	for {
		select {
		case <-signals:
			logger.Info("received-signal")
			return nil
		case <-retryTimer.C():
			consulDownMetric.Send(p.value)
			retryTimer.Reset(p.interval)
		}
	}
}
