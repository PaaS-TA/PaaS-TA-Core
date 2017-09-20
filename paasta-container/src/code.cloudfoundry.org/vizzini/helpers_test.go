package vizzini_test

import (
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"strconv"
	"time"

	"code.cloudfoundry.org/bbs/models"
	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/routing-info/cfroutes"

	. "code.cloudfoundry.org/vizzini/matchers"
	"github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

const HealthyCheckInterval = 30 * time.Second
const ConvergerInterval = 30 * time.Second
const CrashRestartTimeout = 30 * time.Second

const GraceTarballURL = "http://onsi-public.s3.amazonaws.com/grace.tar.gz"
const GraceTarballURLSHAChecksum = "8540eec19bd22ff3fb69d3771123a43ca34e0634"
const GraceBusyboxImageURL = "docker:///onsi/grace-busybox"

//Tasks

func TaskGetter(logger lager.Logger, guid string) func() (*models.Task, error) {
	return func() (*models.Task, error) {
		return bbsClient.TaskByGuid(logger, guid)
	}
}

func TasksByDomainGetter(logger lager.Logger, domain string) func() ([]*models.Task, error) {
	return func() ([]*models.Task, error) {
		return bbsClient.TasksByDomain(logger, domain)
	}
}

func ClearOutTasksInDomain(domain string) {
	tasks, err := bbsClient.TasksByDomain(logger, domain)
	Expect(err).NotTo(HaveOccurred())
	for _, task := range tasks {
		if task.State != models.Task_Completed {
			bbsClient.CancelTask(logger, task.TaskGuid)
			Eventually(TaskGetter(logger, task.TaskGuid)).Should(HaveTaskState(models.Task_Completed))
		}
		Expect(bbsClient.ResolvingTask(logger, task.TaskGuid)).To(Succeed())
		Expect(bbsClient.DeleteTask(logger, task.TaskGuid)).To(Succeed())
	}
	Eventually(TasksByDomainGetter(logger, domain)).Should(BeEmpty())
}

func Task() *models.TaskDefinition {
	return &models.TaskDefinition{
		Action: models.WrapAction(&models.RunAction{
			Path: "bash",
			Args: []string{"-c", "echo 'some output' > /tmp/bar"},
			User: "vcap",
		}),
		RootFs:     defaultRootFS,
		MemoryMb:   128,
		DiskMb:     128,
		CpuWeight:  100,
		LogGuid:    guid,
		LogSource:  "VIZ",
		ResultFile: "/tmp/bar",
		Annotation: "arbitrary-data",
	}
}

//LRPs

func LRPGetter(logger lager.Logger, guid string) func() (*models.DesiredLRP, error) {
	return func() (*models.DesiredLRP, error) {
		return bbsClient.DesiredLRPByProcessGuid(logger, guid)
	}
}

func ActualLRPByProcessGuidAndIndex(logger lager.Logger, guid string, index int) (models.ActualLRP, error) {
	actualLRPGroup, err := bbsClient.ActualLRPGroupByProcessGuidAndIndex(logger, guid, index)
	if err != nil {
		return models.ActualLRP{}, err
	}
	actualLRP, _ := actualLRPGroup.Resolve()
	return *actualLRP, nil
}

func ActualsByProcessGuid(logger lager.Logger, guid string) ([]models.ActualLRP, error) {
	actualLRPGroups, err := bbsClient.ActualLRPGroupsByProcessGuid(logger, guid)
	if err != nil {
		return nil, err
	}

	return resolveActuals(actualLRPGroups), nil
}

func ActualsByDomain(logger lager.Logger, domain string) ([]models.ActualLRP, error) {
	actualLRPGroups, err := bbsClient.ActualLRPGroups(logger, models.ActualLRPFilter{Domain: domain})
	if err != nil {
		return nil, err
	}

	return resolveActuals(actualLRPGroups), nil
}

func resolveActuals(actualLRPGroups []*models.ActualLRPGroup) []models.ActualLRP {
	actualLRPs := make([]models.ActualLRP, 0, len(actualLRPGroups))
	for _, actualLRPGroup := range actualLRPGroups {
		actualLRP, _ := actualLRPGroup.Resolve()
		actualLRPs = append(actualLRPs, *actualLRP)
	}

	return actualLRPs
}

func ActualGetter(logger lager.Logger, guid string, index int) func() (models.ActualLRP, error) {
	return func() (models.ActualLRP, error) {
		return ActualLRPByProcessGuidAndIndex(logger, guid, index)
	}
}

func ActualByProcessGuidGetter(logger lager.Logger, guid string) func() ([]models.ActualLRP, error) {
	return func() ([]models.ActualLRP, error) {
		return ActualsByProcessGuid(logger, guid)
	}
}

func ActualByDomainGetter(logger lager.Logger, domain string) func() ([]models.ActualLRP, error) {
	return func() ([]models.ActualLRP, error) {
		return ActualsByDomain(logger, domain)
	}
}

func ClearOutDesiredLRPsInDomain(domain string) {
	lrps, err := bbsClient.DesiredLRPs(logger, models.DesiredLRPFilter{Domain: domain})
	Expect(err).NotTo(HaveOccurred())
	for _, lrp := range lrps {
		Expect(bbsClient.RemoveDesiredLRP(logger, lrp.ProcessGuid)).To(Succeed())
	}
	// Wait enough time for the Grace app to exit if it was run with -catchTerminate
	Eventually(ActualByDomainGetter(logger, domain), timeout+time.Second).Should(BeEmpty())
}

func EndpointCurler(endpoint string) func() (int, error) {
	return func() (int, error) {
		resp, err := http.Get(endpoint)
		if err != nil {
			return 0, err
		}
		resp.Body.Close()
		return resp.StatusCode, nil
	}
}

func EndpointContentCurler(endpoint string) func() (string, error) {
	return func() (string, error) {
		resp, err := http.Get(endpoint)
		if err != nil {
			return "", err
		}
		defer resp.Body.Close()
		content, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			return "", err
		}
		return string(content), nil
	}
}

func IndexCounter(guid string, optionalHttpClient ...*http.Client) func() (int, error) {
	return IndexCounterWithAttempts(guid, 100, optionalHttpClient...)
}

func IndexCounterWithAttempts(guid string, attempts int, optionalHttpClient ...*http.Client) func() (int, error) {
	return func() (int, error) {
		counts := map[int]bool{}
		for i := 0; i < attempts; i++ {
			index, err := GetIndexFromEndpointFor(guid, optionalHttpClient...)
			if err != nil {
				return 0, err
			}
			if index == -1 {
				continue
			}
			counts[index] = true
		}
		return len(counts), nil
	}
}

func GetIndexFromEndpointFor(guid string, optionalHttpClient ...*http.Client) (int, error) {
	httpClient := http.DefaultClient
	if len(optionalHttpClient) == 1 {
		httpClient = optionalHttpClient[0]
	}
	url := "http://" + RouteForGuid(guid) + "/index"
	resp, err := httpClient.Get(url)
	if err != nil {
		return 0, err
	}
	if resp.StatusCode != http.StatusOK {
		resp.Body.Close()
		return -1, nil
	}
	content, err := ioutil.ReadAll(resp.Body)
	resp.Body.Close()
	if err != nil {
		return 0, err
	}

	return strconv.Atoi(string(content))
}

func GraceCounterGetter(guid string) func() (int, error) {
	return func() (int, error) {
		resp, err := http.Get("http://" + RouteForGuid(guid) + "/counter")
		if err != nil {
			return 0, err
		}
		defer resp.Body.Close()
		content, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			return 0, err
		}
		return strconv.Atoi(string(content))
	}
}

func StartedAtGetter(guid string) func() (int64, error) {
	url := "http://" + RouteForGuid(guid) + "/started-at"
	return func() (int64, error) {
		resp, err := http.Get(url)
		if err != nil {
			return 0, err
		}
		if resp.StatusCode != http.StatusOK {
			resp.Body.Close()
			return 0, errors.New(fmt.Sprintf("invalid status code: %d", resp.StatusCode))
		}
		content, err := ioutil.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			return 0, err
		}
		return strconv.ParseInt(string(content), 10, 64)
	}
}

func RouteForGuid(guid string) string {
	return fmt.Sprintf("%s.%s", guid, routableDomainSuffix)
}

func DirectAddressFor(guid string, index int, containerPort uint32) string {
	actualLRP, err := ActualGetter(logger, guid, index)()
	Expect(err).NotTo(HaveOccurred())
	Expect(actualLRP).NotTo(BeZero())

	for _, portMapping := range actualLRP.Ports {
		if portMapping.ContainerPort == containerPort {
			return fmt.Sprintf("%s:%d", actualLRP.Address, portMapping.HostPort)
		}
	}

	ginkgo.Fail(fmt.Sprintf("could not find port %d for ActualLRP %d with ProcessGuid %s", containerPort, index, guid))
	return ""
}

func DesiredLRPWithGuid(guid string) *models.DesiredLRP {
	routingInfo := cfroutes.CFRoutes{
		{Port: 8080, Hostnames: []string{RouteForGuid(guid)}},
	}.RoutingInfo()

	return &models.DesiredLRP{
		ProcessGuid: guid,
		Domain:      domain,
		Instances:   1,
		CachedDependencies: []*models.CachedDependency{
			&models.CachedDependency{
				From:              GraceTarballURL,
				To:                "/tmp/grace",
				CacheKey:          "grace",
				ChecksumAlgorithm: "sha1",
				ChecksumValue:     GraceTarballURLSHAChecksum,
			},
		},
		LegacyDownloadUser: "vcap",
		Action: models.WrapAction(&models.RunAction{
			Path: "/tmp/grace/grace",
			User: "vcap",
			Env:  []*models.EnvironmentVariable{{Name: "PORT", Value: "8080"}, {"ACTION_LEVEL", "COYOTE"}, {"OVERRIDE", "DAQUIRI"}},
		}),
		Monitor: models.WrapAction(&models.RunAction{
			Path: "nc",
			Args: []string{"-z", "0.0.0.0", "8080"},
			User: "vcap",
		}),
		RootFs:     defaultRootFS,
		MemoryMb:   128,
		DiskMb:     128,
		CpuWeight:  100,
		Ports:      []uint32{8080},
		Routes:     &routingInfo,
		LogGuid:    guid,
		LogSource:  "VIZ",
		Annotation: "arbitrary-data",
	}
}
