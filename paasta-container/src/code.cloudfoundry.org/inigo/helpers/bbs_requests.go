package helpers

import (
	"fmt"
	"time"

	"code.cloudfoundry.org/bbs"
	"code.cloudfoundry.org/bbs/models"
	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/routing-info/cfroutes"
	. "github.com/onsi/gomega"
)

const defaultDomain = "inigo"
const defaultLogGuid = "logGuid"

var defaultPreloadedRootFS = "preloaded:" + DefaultStack
var SecondaryPreloadedRootFS = "preloaded:" + PreloadedStacks[1]

const BogusPreloadedRootFS = "preloaded:bogus-rootfs"
const dockerRootFS = "docker:///cloudfoundry/diego-docker-app#latest"

const DefaultHost = "lrp-route"

var defaultRoutes = cfroutes.CFRoutes{{Hostnames: []string{DefaultHost}, Port: 8080}}.RoutingInfo()
var defaultPorts = []uint32{8080}

var defaultSetupFunc = func() *models.Action {
	return models.WrapAction(&models.DownloadAction{
		From: fmt.Sprintf("http://%s/v1/static/%s", addresses.FileServer, "lrp.zip"),
		To:   "/tmp/diego",
		User: "vcap",
	})
}

var defaultAction = models.WrapAction(&models.RunAction{
	User: "vcap",
	Path: "/tmp/diego/go-server",
	Env:  []*models.EnvironmentVariable{{"PORT", "8080"}},
})

var defaultMonitor = models.WrapAction(&models.RunAction{
	User: "vcap",
	Path: "nc",
	Args: []string{"-z", "localhost", "8080"},
})

var defaultDeclartiveMonitor = &models.CheckDefinition{
	Checks: []*models.Check{
		{
			TcpCheck: &models.TCPCheck{
				Port: 8080,
			},
		},
	},
}

var dockerMonitor = models.WrapAction(&models.RunAction{
	User: "vcap",
	Path: "sh",
	Args: []string{"-c", "echo bogus | nc localhost 8080"},
})

func UpsertInigoDomain(logger lager.Logger, bbsClient bbs.InternalClient) {
	err := bbsClient.UpsertDomain(logger, defaultDomain, 0)
	Expect(err).NotTo(HaveOccurred())
}

func lrpCreateRequest(
	processGuid,
	logGuid,
	rootfs string,
	numInstances int,
	placementTags []string,
	action, monitor *models.Action,
) *models.DesiredLRP {
	return &models.DesiredLRP{
		ProcessGuid: processGuid,
		Domain:      defaultDomain,
		RootFs:      rootfs,
		Instances:   int32(numInstances),

		LogGuid: logGuid,

		Routes: &defaultRoutes,
		Ports:  defaultPorts,

		Setup:         defaultSetupFunc(),
		Action:        action,
		Monitor:       monitor,
		PlacementTags: placementTags,
	}
}

func DefaultLRPCreateRequest(processGuid, logGuid string, numInstances int) *models.DesiredLRP {
	return lrpCreateRequest(processGuid, logGuid, defaultPreloadedRootFS, numInstances, nil, defaultAction, defaultMonitor)
}

func DefaultDeclaritiveHealthcheckLRPCreateRequest(processGuid, logGuid string, numInstances int) *models.DesiredLRP {
	request := lrpCreateRequest(processGuid, logGuid, defaultPreloadedRootFS, numInstances, nil, defaultAction, nil)
	request.CheckDefinition = defaultDeclartiveMonitor
	request.StartTimeoutMs = int64(time.Minute / time.Millisecond)
	return request
}

func LRPCreateRequestWithPlacementTag(processGuid string, tags []string) *models.DesiredLRP {
	return lrpCreateRequest(processGuid, defaultLogGuid, defaultPreloadedRootFS, 1, tags, defaultAction, defaultMonitor)
}

func LRPCreateRequestWithRootFS(processGuid, rootfs string) *models.DesiredLRP {
	return lrpCreateRequest(processGuid, defaultLogGuid, rootfs, 1, nil, defaultAction, defaultMonitor)
}

func DockerLRPCreateRequest(processGuid string) *models.DesiredLRP {
	action := models.WrapAction(&models.RunAction{
		User: "vcap",
		Path: "dockerapp",
		Env:  []*models.EnvironmentVariable{{"PORT", "8080"}},
	})

	return lrpCreateRequest(processGuid, defaultLogGuid, dockerRootFS, 1, nil, action, dockerMonitor)
}

func CrashingLRPCreateRequest(processGuid string) *models.DesiredLRP {
	action := models.WrapAction(&models.RunAction{User: "vcap", Path: "false"})
	return lrpCreateRequest(processGuid, defaultLogGuid, defaultPreloadedRootFS, 1, nil, action, defaultMonitor)
}

func LightweightLRPCreateRequest(processGuid string) *models.DesiredLRP {
	action := models.WrapAction(&models.RunAction{
		User: "vcap",
		Path: "sh",
		Args: []string{
			"-c",
			"while true; do sleep 1; done",
		},
	})

	monitor := models.WrapAction(&models.RunAction{
		User: "vcap",
		Path: "sh",
		Args: []string{"-c", "echo all good"},
	})

	lrp := lrpCreateRequest(processGuid, defaultLogGuid, defaultPreloadedRootFS, 1, nil, action, monitor)
	lrp.MemoryMb = 128
	lrp.DiskMb = 1024
	return lrp
}

func TaskCreateRequest(taskGuid string, action models.ActionInterface) *models.Task {
	return taskCreateRequest(taskGuid, defaultPreloadedRootFS, action, 0, 0, nil)
}

func TaskCreateRequestWithTags(taskGuid string, action models.ActionInterface, tags []string) *models.Task {
	task := taskCreateRequest(taskGuid, defaultPreloadedRootFS, action, 0, 0, nil)
	task.PlacementTags = tags
	return task
}

func TaskCreateRequestWithMemory(taskGuid string, action models.ActionInterface, memoryMB int) *models.Task {
	return taskCreateRequest(taskGuid, defaultPreloadedRootFS, action, memoryMB, 0, nil)
}

func TaskCreateRequestWithRootFS(taskGuid, rootfs string, action models.ActionInterface) *models.Task {
	return taskCreateRequest(taskGuid, rootfs, action, 0, 0, nil)
}

func TaskCreateRequestWithMemoryAndDisk(taskGuid string, action models.ActionInterface, memoryMB, diskMB int) *models.Task {
	return taskCreateRequest(taskGuid, defaultPreloadedRootFS, action, memoryMB, diskMB, nil)
}

func TaskCreateRequestWithCertificateProperties(taskGuid string, action models.ActionInterface, certificateProperties *models.CertificateProperties) *models.Task {
	return taskCreateRequest(taskGuid, defaultPreloadedRootFS, action, 0, 0, certificateProperties)
}

func taskCreateRequest(taskGuid, rootFS string, action models.ActionInterface, memoryMB, diskMB int, certificateProperties *models.CertificateProperties) *models.Task {
	return &models.Task{
		TaskGuid: taskGuid,
		Domain:   defaultDomain,

		TaskDefinition: &models.TaskDefinition{
			RootFs:                rootFS,
			MemoryMb:              int32(memoryMB),
			DiskMb:                int32(diskMB),
			Action:                models.WrapAction(action),
			CertificateProperties: certificateProperties,
		},
	}
}
