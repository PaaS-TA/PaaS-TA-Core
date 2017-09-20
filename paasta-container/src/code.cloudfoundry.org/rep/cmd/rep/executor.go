package main

import (
	"flag"

	"path/filepath"

	executorinit "code.cloudfoundry.org/executor/initializer"
)

var gardenNetwork = flag.String(
	"gardenNetwork",
	executorinit.DefaultConfiguration.GardenNetwork,
	"network mode for garden server (tcp, unix)",
)

var gardenAddr = flag.String(
	"gardenAddr",
	executorinit.DefaultConfiguration.GardenAddr,
	"network address for garden server",
)

var memoryMBFlag = flag.String(
	"memoryMB",
	executorinit.DefaultConfiguration.MemoryMB,
	"the amount of memory the executor has available in megabytes",
)

var diskMBFlag = flag.String(
	"diskMB",
	executorinit.DefaultConfiguration.DiskMB,
	"the amount of disk the executor has available in megabytes",
)

var tempDir = flag.String(
	"tempDir",
	executorinit.DefaultConfiguration.TempDir,
	"location to store temporary assets",
)

var reservedExpirationTime = flag.Duration(
	"reservedExpirationTime",
	executorinit.DefaultConfiguration.ReservedExpirationTime,
	"amount of time during which a container can remain in the allocated state",
)

var containerReapInterval = flag.Duration(
	"containerReapInterval",
	executorinit.DefaultConfiguration.ContainerReapInterval,
	"interval at which the executor reaps extra/missing containers",
)

var containerMetricsReportInterval = flag.Duration(
	"containerMetricsReportInterval",
	executorinit.DefaultConfiguration.ContainerMetricsReportInterval,
	"interval at which the executor reports metrics from containers",
)

var containerInodeLimit = flag.Uint64(
	"containerInodeLimit",
	executorinit.DefaultConfiguration.ContainerInodeLimit,
	"max number of inodes per container",
)

var containerMaxCpuShares = flag.Uint64(
	"containerMaxCpuShares",
	executorinit.DefaultConfiguration.ContainerMaxCpuShares,
	"cpu shares allocatable to a container",
)

var cachePath = flag.String(
	"cachePath",
	executorinit.DefaultConfiguration.CachePath,
	"location to cache assets",
)

var maxCacheSizeInBytes = flag.Uint64(
	"maxCacheSizeInBytes",
	executorinit.DefaultConfiguration.MaxCacheSizeInBytes,
	"maximum size of the cache (in bytes) - you should include a healthy amount of overhead",
)

var skipCertVerify = flag.Bool(
	"skipCertVerify",
	executorinit.DefaultConfiguration.SkipCertVerify,
	"skip SSL certificate verification",
)

var pathToCACertsForDownloads = flag.String(
	"caCertsForDownloads",
	"",
	"path to CA certificate bundle to be used when downloading assets",
)

var healthyMonitoringInterval = flag.Duration(
	"healthyMonitoringInterval",
	executorinit.DefaultConfiguration.HealthyMonitoringInterval,
	"interval on which to check healthy containers",
)

var unhealthyMonitoringInterval = flag.Duration(
	"unhealthyMonitoringInterval",
	executorinit.DefaultConfiguration.UnhealthyMonitoringInterval,
	"interval on which to check unhealthy containers",
)

var exportNetworkEnvVars = flag.Bool(
	"exportNetworkEnvVars",
	executorinit.DefaultConfiguration.ExportNetworkEnvVars,
	"export network environment variables into container (e.g. CF_INSTANCE_IP, CF_INSTANCE_PORT)",
)

var containerOwnerName = flag.String(
	"containerOwnerName",
	executorinit.DefaultConfiguration.ContainerOwnerName,
	"owner name with which to tag containers",
)

var createWorkPoolSize = flag.Int(
	"createWorkPoolSize",
	executorinit.DefaultConfiguration.CreateWorkPoolSize,
	"Number of concurrent create operations in garden",
)

var deleteWorkPoolSize = flag.Int(
	"deleteWorkPoolSize",
	executorinit.DefaultConfiguration.DeleteWorkPoolSize,
	"Number of concurrent delete operations in garden",
)

var readWorkPoolSize = flag.Int(
	"readWorkPoolSize",
	executorinit.DefaultConfiguration.ReadWorkPoolSize,
	"Number of concurrent read operations in garden",
)

var metricsWorkPoolSize = flag.Int(
	"metricsWorkPoolSize",
	executorinit.DefaultConfiguration.MetricsWorkPoolSize,
	"Number of concurrent metrics operations in garden",
)

var healthCheckWorkPoolSize = flag.Int(
	"healthCheckWorkPoolSize",
	executorinit.DefaultConfiguration.HealthCheckWorkPoolSize,
	"Number of concurrent ping operations in garden",
)

var maxConcurrentDownloads = flag.Int(
	"maxConcurrentDownloads",
	executorinit.DefaultConfiguration.MaxConcurrentDownloads,
	"Number of concurrent download steps",
)

var gardenHealthcheckInterval = flag.Duration(
	"gardenHealthcheckInterval",
	executorinit.DefaultConfiguration.GardenHealthcheckInterval,
	"Frequency for healthchecking garden",
)

var gardenHealthcheckEmissionInterval = flag.Duration(
	"gardenHealthcheckEmissionInterval",
	executorinit.DefaultConfiguration.GardenHealthcheckEmissionInterval,
	"Frequency for emitting UnhealthyCell metric",
)

var gardenHealthcheckTimeout = flag.Duration(
	"gardenHealthcheckTimeout",
	executorinit.DefaultConfiguration.GardenHealthcheckTimeout,
	"Maximum allowed time for garden healthcheck",
)

var gardenHealthcheckCommandRetryPause = flag.Duration(
	"gardenHealthcheckCommandRetryPause",
	executorinit.DefaultConfiguration.GardenHealthcheckCommandRetryPause,
	"Time to wait between retrying garden commands",
)

var gardenHealthcheckProcessPath = flag.String(
	"gardenHealthcheckProcessPath",
	executorinit.DefaultConfiguration.GardenHealthcheckProcessPath,
	"Path of the command to run to perform a container healthcheck",
)

var gardenHealthcheckProcessUser = flag.String(
	"gardenHealthcheckProcessUser",
	executorinit.DefaultConfiguration.GardenHealthcheckProcessUser,
	"User to use while performing a container healthcheck",
)

var gardenHealthcheckProcessDir = flag.String(
	"gardenHealthcheckProcessDir",
	executorinit.DefaultConfiguration.GardenHealthcheckProcessDir,
	"Directory to run the healthcheck process from",
)

var postSetupHook = flag.String(
	"postSetupHook",
	"",
	"Set of commands to run after the setup action",
)

var postSetupUser = flag.String(
	"postSetupUser",
	"root",
	"User to run the post setup hook",
)

var trustedSystemCertificatesPath = flag.String(
	"trustedSystemCertificatesPath",
	"",
	"path to directory containing trusted system ca certs.",
)

var volmanDriverPaths = flag.String(
	"volmanDriverPaths",
	"",
	"path to directories that the volume manager uses to discover drivers.  May be an OS-specific path-separated set of paths; e.g. /path/to/a:/path/to/b",
)

func executorConfig(caCertsForDownloads []byte, gardenHealthcheckRootFS string, gardenHealthcheckArgs, gardenHealthcheckEnv []string) executorinit.Configuration {
	return executorinit.Configuration{
		GardenNetwork:                      *gardenNetwork,
		GardenAddr:                         *gardenAddr,
		ContainerOwnerName:                 *containerOwnerName,
		TempDir:                            *tempDir,
		CachePath:                          *cachePath,
		MaxCacheSizeInBytes:                *maxCacheSizeInBytes,
		SkipCertVerify:                     *skipCertVerify,
		CACertsForDownloads:                caCertsForDownloads,
		ExportNetworkEnvVars:               *exportNetworkEnvVars,
		ContainerMaxCpuShares:              *containerMaxCpuShares,
		ContainerInodeLimit:                *containerInodeLimit,
		HealthyMonitoringInterval:          *healthyMonitoringInterval,
		UnhealthyMonitoringInterval:        *unhealthyMonitoringInterval,
		HealthCheckWorkPoolSize:            *healthCheckWorkPoolSize,
		CreateWorkPoolSize:                 *createWorkPoolSize,
		DeleteWorkPoolSize:                 *deleteWorkPoolSize,
		ReadWorkPoolSize:                   *readWorkPoolSize,
		MetricsWorkPoolSize:                *metricsWorkPoolSize,
		ReservedExpirationTime:             *reservedExpirationTime,
		ContainerReapInterval:              *containerReapInterval,
		MemoryMB:                           *memoryMBFlag,
		DiskMB:                             *diskMBFlag,
		MaxConcurrentDownloads:             *maxConcurrentDownloads,
		GardenHealthcheckInterval:          *gardenHealthcheckInterval,
		GardenHealthcheckEmissionInterval:  *gardenHealthcheckEmissionInterval,
		GardenHealthcheckTimeout:           *gardenHealthcheckTimeout,
		GardenHealthcheckCommandRetryPause: *gardenHealthcheckCommandRetryPause,
		GardenHealthcheckRootFS:            gardenHealthcheckRootFS,
		GardenHealthcheckProcessPath:       *gardenHealthcheckProcessPath,
		GardenHealthcheckProcessUser:       *gardenHealthcheckProcessUser,
		GardenHealthcheckProcessDir:        *gardenHealthcheckProcessDir,
		GardenHealthcheckProcessArgs:       gardenHealthcheckArgs,
		GardenHealthcheckProcessEnv:        gardenHealthcheckEnv,
		PostSetupHook:                      *postSetupHook,
		PostSetupUser:                      *postSetupUser,
		TrustedSystemCertificatesPath:      *trustedSystemCertificatesPath,
		VolmanDriverPaths:                  filepath.SplitList(*volmanDriverPaths),
		ContainerMetricsReportInterval:     *containerMetricsReportInterval,
	}
}
