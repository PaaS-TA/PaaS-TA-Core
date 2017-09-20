package metrics

import (
	"os"
	"time"

	"code.cloudfoundry.org/clock"
	"code.cloudfoundry.org/executor"
	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/runtimeschema/metric"
)

const (
	totalMemory     = metric.Mebibytes("CapacityTotalMemory")
	totalDisk       = metric.Mebibytes("CapacityTotalDisk")
	totalContainers = metric.Metric("CapacityTotalContainers")

	remainingMemory     = metric.Mebibytes("CapacityRemainingMemory")
	remainingDisk       = metric.Mebibytes("CapacityRemainingDisk")
	remainingContainers = metric.Metric("CapacityRemainingContainers")

	containerCount = metric.Metric("ContainerCount")
)

type ExecutorSource interface {
	RemainingResources(lager.Logger) (executor.ExecutorResources, error)
	TotalResources(lager.Logger) (executor.ExecutorResources, error)
	ListContainers(lager.Logger) ([]executor.Container, error)
}

type Reporter struct {
	Interval       time.Duration
	ExecutorSource ExecutorSource
	Clock          clock.Clock
	Logger         lager.Logger
}

func (reporter *Reporter) Run(signals <-chan os.Signal, ready chan<- struct{}) error {
	logger := reporter.Logger.Session("metrics-reporter")

	close(ready)

	timer := reporter.Clock.NewTimer(reporter.Interval)

	for {
		select {
		case <-signals:
			return nil

		case <-timer.C():
			remainingCapacity, err := reporter.ExecutorSource.RemainingResources(logger)
			if err != nil {
				reporter.Logger.Error("failed-remaining-resources", err)
				remainingCapacity.Containers = -1
				remainingCapacity.DiskMB = -1
				remainingCapacity.MemoryMB = -1
			}

			totalCapacity, err := reporter.ExecutorSource.TotalResources(logger)
			if err != nil {
				reporter.Logger.Error("failed-total-resources", err)
				totalCapacity.Containers = -1
				totalCapacity.DiskMB = -1
				totalCapacity.MemoryMB = -1
			}

			var nContainers int
			containers, err := reporter.ExecutorSource.ListContainers(logger)
			if err != nil {
				reporter.Logger.Error("failed-to-list-containers", err)
				nContainers = -1
			} else {
				nContainers = len(containers)
			}

			err = totalMemory.Send(totalCapacity.MemoryMB)
			if err != nil {
				logger.Error("failed-to-send-total-memory-metric", err)
			}
			err = totalDisk.Send(totalCapacity.DiskMB)
			if err != nil {
				logger.Error("failed-to-send-total-disk-metric", err)
			}
			err = totalContainers.Send(totalCapacity.Containers)
			if err != nil {
				logger.Error("failed-to-send-total-container-metric", err)
			}

			err = remainingMemory.Send(remainingCapacity.MemoryMB)
			if err != nil {
				logger.Error("failed-to-send-remaining-memory-metric", err)
			}
			err = remainingDisk.Send(remainingCapacity.DiskMB)
			if err != nil {
				logger.Error("failed-to-send-remaining-disk-metric", err)
			}
			err = remainingContainers.Send(remainingCapacity.Containers)
			if err != nil {
				logger.Error("failed-to-send-remaining-containers-metric", err)
			}

			err = containerCount.Send(nContainers)
			if err != nil {
				logger.Error("failed-to-send-container-count-metric", err)
			}

			timer.Reset(reporter.Interval)
		}
	}
}
