package benchmarkbbs_test

import (
	"fmt"
	"math/rand"
	"os"
	"sync"
	"sync/atomic"
	"time"

	"code.cloudfoundry.org/bbs/models"
	"code.cloudfoundry.org/benchmarkbbs/reporter"
	"code.cloudfoundry.org/operationq"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/tedsuo/ifrit"
	"github.com/tedsuo/ifrit/ginkgomon"
)

const (
	RepBulkFetching                   = "RepBulkFetching"
	RepBulkLoop                       = "RepBulkLoop"
	RepClaimActualLRP                 = "RepClaimActualLRP"
	RepStartActualLRP                 = "RepStartActualLRP"
	NsyncBulkerFetching               = "NsyncBulkerFetching"
	ConvergenceGathering              = "ConvergenceGathering"
	FetchActualLRPsAndSchedulingInfos = "FetchActualLRPsAndSchedulingInfos"
)

var bulkCycle = 30 * time.Second
var eventCount int32 = 0
var claimCount int32 = 0

var BenchmarkTests = func(numReps, numTrials int) {
	Describe("main benchmark test", func() {

		eventCountRunner := func(signals <-chan os.Signal, ready chan<- struct{}) error {
			eventSource, err := bbsClient.SubscribeToEvents(logger)
			Expect(err).NotTo(HaveOccurred())
			close(ready)

			eventChan := make(chan models.Event)
			go func() {
				for {
					event, err := eventSource.Next()
					if err != nil {
						logger.Error("error-getting-next-event", err)
						return
					}
					if event != nil {
						eventChan <- event
					}
				}
			}()

			for {
				select {
				case <-eventChan:
					eventCount += 1

				case <-signals:
					if eventSource != nil {
						err := eventSource.Close()
						if err != nil {
							logger.Error("failed-closing-event-source", err)
						}
					}
					return nil
				}
			}
		}

		var process ifrit.Process
		BeforeEach(func() {
			process = ifrit.Invoke(ifrit.RunFunc(eventCountRunner))
		})

		AfterEach(func() {
			ginkgomon.Kill(process)
		})

		Measure("data for benchmarks", func(b Benchmarker) {
			wg := sync.WaitGroup{}

			// start nsync
			go func() {
				defer GinkgoRecover()
				logger.Info("start-nsync-bulker-loop")
				defer logger.Info("finish-nsync-bulker-loop")
				wg.Add(1)
				defer wg.Done()
				for i := 0; i < numTrials; i++ {
					sleepDuration := getSleepDuration(i, bulkCycle)
					time.Sleep(sleepDuration)
					b.Time("fetch all desired LRP scheduling info", func() {
						desireds, err := bbsClient.DesiredLRPSchedulingInfos(logger, models.DesiredLRPFilter{})
						Expect(err).NotTo(HaveOccurred())
						Expect(len(desireds)).To(BeNumerically("~", expectedLRPCount, expectedLRPVariation), "Number of DesiredLRPs retrieved in Nsync Bulk Loop")
					}, reporter.ReporterInfo{
						MetricName: NsyncBulkerFetching,
					})
				}
			}()

			// start convergence
			go func() {
				defer GinkgoRecover()
				logger.Info("start-lrp-convergence-loop")
				defer logger.Info("finish-lrp-convergence-loop")
				wg.Add(1)
				defer wg.Done()
				for i := 0; i < numTrials; i++ {
					sleepDuration := getSleepDuration(i, bulkCycle)
					time.Sleep(sleepDuration)
					cellSet := models.NewCellSet()
					for i := 0; i < numReps; i++ {
						cellID := fmt.Sprintf("cell-%d", i)
						presence := models.NewCellPresence(cellID, "earth", "http://planet-earth", "north", models.CellCapacity{}, nil, nil, nil, nil)
						cellSet.Add(&presence)
					}

					b.Time("BBS' internal gathering of LRPs", func() {
						activeDB.ConvergeLRPs(logger, cellSet)
					}, reporter.ReporterInfo{
						MetricName: ConvergenceGathering,
					})
				}
			}()

			// start route-emitter
			go func() {
				defer GinkgoRecover()
				logger.Info("start-route-emitter-loop")
				defer logger.Info("finish-route-emitter-loop")
				wg.Add(1)
				defer wg.Done()
				for i := 0; i < numTrials; i++ {
					sleepDuration := getSleepDuration(i, bulkCycle)
					time.Sleep(sleepDuration)
					b.Time("fetch all actualLRPs", func() {
						actuals, err := bbsClient.ActualLRPGroups(logger, models.ActualLRPFilter{})
						Expect(err).NotTo(HaveOccurred())
						Expect(len(actuals)).To(BeNumerically("~", expectedLRPCount, expectedLRPVariation), "Number of ActualLRPs retrieved in router-emitter")

						desireds, err := bbsClient.DesiredLRPSchedulingInfos(logger, models.DesiredLRPFilter{})
						Expect(err).NotTo(HaveOccurred())
						Expect(len(desireds)).To(BeNumerically("~", expectedLRPCount, expectedLRPVariation), "Number of DesiredLRPs retrieved in route-emitter")
					}, reporter.ReporterInfo{
						MetricName: FetchActualLRPsAndSchedulingInfos,
					})
				}
			}()

			totalRan := int32(0)
			totalQueued := int32(0)
			var err error
			queue := operationq.NewSlidingQueue(numTrials)

			// we need to make sure we don't run out of ports so limit amount of
			// active http requests to 25000
			semaphore := make(chan struct{}, 25000)

			for i := 0; i < numReps; i++ {
				cellID := fmt.Sprintf("cell-%d", i)
				wg.Add(1)

				go func(cellID string) {
					defer GinkgoRecover()
					defer wg.Done()

					for j := 0; j < numTrials; j++ {
						sleepDuration := getSleepDuration(j, bulkCycle)
						time.Sleep(sleepDuration)

						b.Time("rep bulk loop", func() {
							defer GinkgoRecover()
							var actuals []*models.ActualLRPGroup
							b.Time("rep bulk fetch", func() {
								semaphore <- struct{}{}
								actuals, err = bbsClient.ActualLRPGroups(logger, models.ActualLRPFilter{CellID: cellID})
								<-semaphore
								Expect(err).NotTo(HaveOccurred())
							}, reporter.ReporterInfo{
								MetricName: RepBulkFetching,
							})

							expectedActualLRPCount, ok := expectedActualLRPCounts[cellID]
							Expect(ok).To(BeTrue())

							expectedActualLRPVariation, ok := expectedActualLRPVariations[cellID]
							Expect(ok).To(BeTrue())

							Expect(len(actuals)).To(BeNumerically("~", expectedActualLRPCount, expectedActualLRPVariation), "Number of ActualLRPs retrieved by cell %s in rep bulk loop", cellID)

							numActuals := len(actuals)
							for k := 0; k < numActuals; k++ {
								actualLRP, _ := actuals[k].Resolve()
								atomic.AddInt32(&totalQueued, 1)
								queue.Push(&lrpOperation{actualLRP, percentWrites, b, &totalRan, &claimCount, semaphore})
							}
						}, reporter.ReporterInfo{
							MetricName: RepBulkLoop,
						})
					}
				}(cellID)
			}

			wg.Wait()

			eventTolerance := float64(claimCount) * errorTolerance
			Eventually(func() int32 { return eventCount }, 2*time.Minute).Should(BeNumerically("~", claimCount, eventTolerance), "events received")
			Eventually(func() int32 { return totalRan }, 2*time.Minute).Should(Equal(totalQueued), "should have run the same number of queued LRP operations")
		}, 1)
	})
}

type lrpOperation struct {
	actualLRP        *models.ActualLRP
	percentWrites    float64
	b                Benchmarker
	globalCount      *int32
	globalClaimCount *int32
	semaphore        chan struct{}
}

func (lo *lrpOperation) Key() string {
	return lo.actualLRP.ProcessGuid
}

func (lo *lrpOperation) Execute() {
	defer GinkgoRecover()
	defer atomic.AddInt32(lo.globalCount, 1)
	var err error
	randomNum := rand.Float64() * 100.0

	// divided by 2 because the start following the claim cause two writes.
	isClaiming := randomNum < (lo.percentWrites / 2)
	actualLRP := lo.actualLRP

	lo.b.Time("start actual LRP", func() {
		netInfo := models.NewActualLRPNetInfo("1.2.3.4", models.NewPortMapping(61999, 8080))
		lo.semaphore <- struct{}{}
		err = bbsClient.StartActualLRP(logger, &actualLRP.ActualLRPKey, &actualLRP.ActualLRPInstanceKey, &netInfo)
		<-lo.semaphore
		Expect(err).NotTo(HaveOccurred())
		if actualLRP.State == models.ActualLRPStateClaimed {
			defer atomic.AddInt32(lo.globalClaimCount, 1)
		}
	}, reporter.ReporterInfo{
		MetricName: RepStartActualLRP,
	})

	if isClaiming {
		lo.b.Time("claim actual LRP", func() {
			index := int(actualLRP.ActualLRPKey.Index)
			lo.semaphore <- struct{}{}
			err = bbsClient.ClaimActualLRP(logger, actualLRP.ActualLRPKey.ProcessGuid, index, &actualLRP.ActualLRPInstanceKey)
			<-lo.semaphore
			Expect(err).NotTo(HaveOccurred())
			defer atomic.AddInt32(lo.globalClaimCount, 1)
		}, reporter.ReporterInfo{
			MetricName: RepClaimActualLRP,
		})
	}
}

func getSleepDuration(loopCounter int, cycleTime time.Duration) time.Duration {
	sleepDuration := cycleTime
	if loopCounter == 0 {
		numMilli := rand.Intn(int(cycleTime.Nanoseconds() / 1000000))
		sleepDuration = time.Duration(numMilli) * time.Millisecond
	}
	return sleepDuration
}
