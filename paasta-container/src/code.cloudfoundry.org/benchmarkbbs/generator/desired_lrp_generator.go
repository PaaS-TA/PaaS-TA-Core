package generator

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"sync"
	"time"

	"code.cloudfoundry.org/bbs"
	"code.cloudfoundry.org/bbs/models"
	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/workpool"
	"github.com/zorkian/go-datadog-api"
)

type DesiredLRPGenerator struct {
	errorTolerance float64
	metricPrefix   string
	bbsClient      bbs.InternalClient
	datadogClient  *datadog.Client
	workPool       *workpool.WorkPool
}

func NewDesiredLRPGenerator(
	errTolerance float64,
	metricPrefix string,
	workpoolSize int,
	bbsClient bbs.InternalClient,
	datadogClient *datadog.Client,
) *DesiredLRPGenerator {
	workPool, err := workpool.NewWorkPool(workpoolSize)
	if err != nil {
		panic(err)
	}
	return &DesiredLRPGenerator{
		errorTolerance: errTolerance,
		metricPrefix:   metricPrefix,
		bbsClient:      bbsClient,
		workPool:       workPool,
		datadogClient:  datadogClient,
	}
}

type stampedError struct {
	err    error
	guid   string
	cellId string
	time.Time
}

func newStampedError(err error, guid, cellId string) *stampedError {
	return &stampedError{err, guid, cellId, time.Now()}
}

func (g *DesiredLRPGenerator) Generate(logger lager.Logger, numReps, count int) (int, map[string]int, error) {
	logger = logger.Session("generate-desired-lrp", lager.Data{"count": count})

	start := time.Now()

	var wg sync.WaitGroup

	desiredErrCh := make(chan *stampedError, count)
	actualErrCh := make(chan *stampedError, count)
	actualStartErrCh := make(chan *stampedError, count)

	logger.Info("queing-started")
	for i := 0; i < count; i++ {
		wg.Add(1)
		id := fmt.Sprintf("BENCHMARK-BBS-GUID-%06d", i)
		g.workPool.Submit(func() {
			defer wg.Done()

			desired, err := newDesiredLRP(id)
			if err != nil {
				desiredErrCh <- newStampedError(err, id, "")
				return
			}
			desiredErrCh <- newStampedError(g.bbsClient.DesireLRP(logger, desired), id, "")

			cellID := fmt.Sprintf("cell-%d", rand.Intn(numReps))
			actualLRPInstanceKey := &models.ActualLRPInstanceKey{InstanceGuid: desired.ProcessGuid + "-i", CellId: cellID}
			netInfo := models.NewActualLRPNetInfo("1.2.3.4", "2.2.2.2", models.NewPortMapping(61999, 8080))
			actualStartErrCh <- newStampedError(
				g.bbsClient.StartActualLRP(logger, &models.ActualLRPKey{Domain: desired.Domain, ProcessGuid: desired.ProcessGuid, Index: 0}, actualLRPInstanceKey, &netInfo),
				id,
				cellID,
			)
		})

		if i%10000 == 0 {
			logger.Info("queing-progress", lager.Data{"current": i, "total": count})
		}
	}

	logger.Info("queing-complete", lager.Data{"duration": time.Since(start)})

	go func() {
		wg.Wait()
		close(desiredErrCh)
		close(actualErrCh)
		close(actualStartErrCh)
	}()

	return g.processResults(logger, desiredErrCh, actualStartErrCh, numReps)
}

func (g *DesiredLRPGenerator) processResults(logger lager.Logger, desiredErrCh, actualStartErrCh chan *stampedError, numReps int) (int, map[string]int, error) {
	var totalDesiredResults, totalActualResults, errorDesiredResults, errorActualResults int
	perCellActualLRPCount := make(map[string]int)
	perCellActualLRPStartAttempts := make(map[string]int)
	for err := range desiredErrCh {
		if err.err != nil {
			newErr := fmt.Errorf("Error %v GUID %s", err, err.guid)
			logger.Error("failed-seeding-desired-lrps", newErr)
			errorDesiredResults++
		}
		totalDesiredResults++
	}

	for err := range actualStartErrCh {
		if err.err != nil {
			newErr := fmt.Errorf("Error %v GUID %s", err, err.guid)
			logger.Error("failed-starting-actual-lrps", newErr)
			errorActualResults++
		} else {
			// only increment the per cell count on successful requests
			perCellActualLRPCount[err.cellId]++
		}
		perCellActualLRPStartAttempts[err.cellId]++
		totalActualResults++
	}

	desiredErrorRate := float64(errorDesiredResults) / float64(totalDesiredResults)
	logger.Info("desireds-complete", lager.Data{
		"total-results": totalDesiredResults,
		"error-results": errorDesiredResults,
		"error-rate":    fmt.Sprintf("%.2f", desiredErrorRate),
	})

	if desiredErrorRate > g.errorTolerance {
		err := fmt.Errorf("Error rate of %.3f for desireds exceeds tolerance of %.3f", desiredErrorRate, g.errorTolerance)
		logger.Error("failed", err)
		return 0, nil, err
	}

	actualErrorRate := float64(errorActualResults) / float64(totalActualResults)
	logger.Info("actuals-complete", lager.Data{
		"total-results": totalActualResults,
		"error-results": errorActualResults,
		"error-rate":    fmt.Sprintf("%.2f", actualErrorRate),
	})

	if actualErrorRate > g.errorTolerance {
		err := fmt.Errorf("Error rate of %.3f for actuals exceeds tolerance of %.3f", actualErrorRate, g.errorTolerance)
		logger.Error("failed", err)
		return 0, nil, err
	}

	if g.datadogClient != nil {
		logger.Info("posting-datadog-metrics")
		timestamp := float64(time.Now().Unix())
		err := g.datadogClient.PostMetrics([]datadog.Metric{
			{
				Metric: fmt.Sprintf("%s.failed-desired-requests", g.metricPrefix),
				Points: []datadog.DataPoint{
					{timestamp, float64(errorDesiredResults)},
				},
			},
		})

		if err != nil {
			logger.Error("failed-posting-datadog-metrics", err)
		}

		err = g.datadogClient.PostMetrics([]datadog.Metric{
			{
				Metric: fmt.Sprintf("%s.failed-actual-requests", g.metricPrefix),
				Points: []datadog.DataPoint{
					{timestamp, float64(errorActualResults)},
				},
			},
		})
		if err != nil {
			logger.Error("failed-posting-datadog-metrics", err)
		}
	}

	for cell, lrpCount := range perCellActualLRPCount {
		attempts := perCellActualLRPStartAttempts[cell]
		errorCount := attempts - lrpCount
		errorRate := float64(errorCount) / float64(attempts)
		if errorRate > g.errorTolerance {
			err := fmt.Errorf("Error rate of %.3f for actuals exceeds tolerance of %.3f", errorRate, g.errorTolerance)
			logger.Error("failed", err)
			return 0, nil, err
		}
	}

	return totalDesiredResults - errorDesiredResults, perCellActualLRPCount, nil
}

func newDesiredLRP(guid string) (*models.DesiredLRP, error) {
	myRouterJSON := json.RawMessage(`[{"hostnames":["dora.bosh-lite.com"],"port":8080}]`)
	myRouterJSON2 := json.RawMessage(`{"container_port":2222,"host_fingerprint":"44:00:2b:21:19:1a:42:ab:54:2f:c3:9d:97:d6:c8:0f","private_key":"-----BEGIN RSA PRIVATE KEY-----\nMIICXQIBAAKBgQCu4BiQh96+AvbYHDxRhfK9Scsl5diUkb/LIbe7Hx7DZg8iTxvr\nkw+de3i1TZG3wH02bdReBnCXrN/u59q0qqsz8ge71BFqnSF0dJaSmXhWizN0NQEy\n5u4WyqM4WJTzUGFnofJxnwFArHBT6QEtDjqCJxyggjuBrF60x3HtSfp4gQIDAQAB\nAoGBAJp/SbSHFXbxz3tmlrO/j5FEHMJCqnG3wqaIB3a+K8Od60j4c0ZRCr6rUx16\nhn69BOKNbc4UCm02QjEjjcmH7u/jLflvKLR/EeEXpGpAd7i3b5bqNn98PP+KwnbS\nPxbot37KErdwLnlF8QYFZMeqHiXQG8nO1nqroiX+fVUDtipBAkEAx8nDxLet6ObJ\nWzdR/8dSQ5qeCqXlfX9PFN6JHtw/OBZjRP5jc2cfGXAAB2h7w5XBy0tak1+76v+Y\nTrdq/rqAdQJBAOAT7W0FpLAZEJusY4sXkhZJvGO0e9MaOdYx53Z2m2gUgxLuowkS\nOmKn/Oj+jqr8r1FAhnTYBDY3k5lzM9p41l0CQEXQ9j6qSXXYIIllvZv6lX7Wa2Ah\nNR8z+/i5A4XrRZReDnavxyUu5ilHgFsWYhmpHb3jKVXS4KJwi1MGubcmiXkCQQDH\nWrNG5Vhpm0MdXLeLDcNYtO04P2BSpeiC2g81Y7xLUsRyWYEPFvp+vznRCHhhQ0Gu\npht5ZJ4KplNYmBev7QW5AkA2PuQ8n7APgIhi8xBwlZW3jufnSHT8dP6JUCgvvon1\nDvUM22k/ZWRo0mUB4BdGctIqRFiGwB8Hd0WSl7gSb5oF\n-----END RSA PRIVATE KEY-----\n"}`)
	modTag := models.NewModificationTag("epoch", 0)
	desiredLRP := &models.DesiredLRP{
		ProcessGuid:          guid,
		Domain:               "benchmark-bbs",
		RootFs:               "some:rootfs",
		Instances:            1,
		EnvironmentVariables: []*models.EnvironmentVariable{{Name: "FOO", Value: "bar"}},
		Setup:                models.WrapAction(&models.RunAction{Path: "ls", User: "name"}),
		Action:               models.WrapAction(&models.RunAction{Path: "ls", User: "name"}),
		StartTimeoutMs:       15000,
		Monitor: models.WrapAction(models.EmitProgressFor(
			models.Timeout(models.Try(models.Parallel(models.Serial(&models.RunAction{Path: "ls", User: "name"}))),
				10*time.Second,
			),
			"start-message",
			"success-message",
			"failure-message",
		)),
		DiskMb:    512,
		MemoryMb:  1024,
		CpuWeight: 42,
		Routes: &models.Routes{"my-router": &myRouterJSON,
			"diego-ssh": &myRouterJSON2},
		LogSource:   "some-log-source",
		LogGuid:     "some-log-guid",
		MetricsGuid: "some-metrics-guid",
		Annotation:  "some-annotation",
		EgressRules: []*models.SecurityGroupRule{{
			Protocol:     models.TCPProtocol,
			Destinations: []string{"1.1.1.1/32", "2.2.2.2/32"},
			PortRange:    &models.PortRange{Start: 10, End: 16000},
		}},
		ModificationTag: &modTag,
	}
	err := desiredLRP.Validate()
	if err != nil {
		return nil, err
	}

	return desiredLRP, nil
}
