package configuration

import (
	"fmt"
	"strconv"

	"code.cloudfoundry.org/executor"
	"code.cloudfoundry.org/garden"
	garden_client "code.cloudfoundry.org/garden/client"
)

const Automatic = "auto"

var (
	ErrMemoryFlagInvalid       = fmt.Errorf("memory limit must be a positive number or '%s'", Automatic)
	ErrDiskFlagInvalid         = fmt.Errorf("disk limit must be a positive number or '%s'", Automatic)
	ErrAutoDiskCapacityInvalid = fmt.Errorf("auto disk limit must result in a positive number")
)

func ConfigureCapacity(
	gardenClient garden_client.Client,
	memoryMBFlag string,
	diskMBFlag string,
	maxCacheSizeInBytes uint64,
	autoDiskMBOverhead int,
) (executor.ExecutorResources, error) {
	gardenCapacity, err := gardenClient.Capacity()
	if err != nil {
		return executor.ExecutorResources{}, err
	}

	memory, err := memoryInMB(gardenCapacity, memoryMBFlag)
	if err != nil {
		return executor.ExecutorResources{}, err
	}

	disk, err := diskInMB(gardenCapacity, diskMBFlag, maxCacheSizeInBytes, autoDiskMBOverhead)
	if err != nil {
		return executor.ExecutorResources{}, err
	}

	return executor.ExecutorResources{
		MemoryMB:   memory,
		DiskMB:     disk,
		Containers: int(gardenCapacity.MaxContainers) - 1,
	}, nil
}

func memoryInMB(capacity garden.Capacity, memoryMBFlag string) (int, error) {
	if memoryMBFlag == Automatic {
		return int(capacity.MemoryInBytes / (1024 * 1024)), nil
	} else {
		memoryMB, err := strconv.Atoi(memoryMBFlag)
		if err != nil || memoryMB <= 0 {
			return 0, ErrMemoryFlagInvalid
		}
		return memoryMB, nil
	}
}

func diskInMB(capacity garden.Capacity, diskMBFlag string, maxCacheSizeInBytes uint64, autoDiskMBOverhead int) (int, error) {
	if diskMBFlag == Automatic {
		diskMB := ((int(capacity.DiskInBytes) - int(maxCacheSizeInBytes)) / (1024 * 1024)) - autoDiskMBOverhead
		if diskMB <= 0 {
			return 0, ErrAutoDiskCapacityInvalid
		}
		return diskMB, nil
	} else {
		diskMB, err := strconv.Atoi(diskMBFlag)
		if err != nil || diskMB <= 0 {
			return 0, ErrDiskFlagInvalid
		}
		return diskMB, nil
	}
}
