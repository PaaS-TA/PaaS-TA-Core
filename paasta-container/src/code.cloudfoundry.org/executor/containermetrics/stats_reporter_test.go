package containermetrics_test

import (
	"errors"
	"time"

	"code.cloudfoundry.org/clock/fakeclock"
	"code.cloudfoundry.org/executor"
	"code.cloudfoundry.org/executor/containermetrics"
	efakes "code.cloudfoundry.org/executor/fakes"
	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/lager/lagertest"
	msfake "github.com/cloudfoundry/dropsonde/metric_sender/fake"
	dmetrics "github.com/cloudfoundry/dropsonde/metrics"
	"github.com/cloudfoundry/sonde-go/events"
	"github.com/tedsuo/ifrit"
	"github.com/tedsuo/ifrit/ginkgomon"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

type listContainerResults struct {
	containers []executor.Container
	err        error
}

type metricsResults struct {
	metrics executor.ContainerMetrics
	err     error
}

var _ = Describe("StatsReporter", func() {
	var (
		logger *lagertest.TestLogger

		interval           time.Duration
		fakeClock          *fakeclock.FakeClock
		fakeExecutorClient *efakes.FakeClient
		fakeMetricSender   *msfake.FakeMetricSender

		metricsResults chan map[string]executor.Metrics
		process        ifrit.Process
	)

	sendResults := func() {
		metricsResults <- map[string]executor.Metrics{
			"guid-without-index": executor.Metrics{
				MetricsConfig: executor.MetricsConfig{Guid: "metrics-guid-without-index"},
				ContainerMetrics: executor.ContainerMetrics{
					MemoryUsageInBytes: 123,
					DiskUsageInBytes:   456,
					TimeSpentInCPU:     100 * time.Second,
					MemoryLimitInBytes: 789,
					DiskLimitInBytes:   1024,
				},
			},
			"guid-with-index": executor.Metrics{
				MetricsConfig: executor.MetricsConfig{Guid: "metrics-guid-with-index", Index: 1},
				ContainerMetrics: executor.ContainerMetrics{
					MemoryUsageInBytes: 321,
					DiskUsageInBytes:   654,
					TimeSpentInCPU:     100 * time.Second,
					MemoryLimitInBytes: 987,
					DiskLimitInBytes:   2048,
				},
			},
		}

		metricsResults <- map[string]executor.Metrics{
			"guid-without-index": executor.Metrics{
				MetricsConfig: executor.MetricsConfig{Guid: "metrics-guid-without-index"},
				ContainerMetrics: executor.ContainerMetrics{
					MemoryUsageInBytes: 1230,
					DiskUsageInBytes:   4560,
					TimeSpentInCPU:     105 * time.Second,
					MemoryLimitInBytes: 7890,
					DiskLimitInBytes:   4096,
				},
			},
			"guid-with-index": executor.Metrics{
				MetricsConfig: executor.MetricsConfig{Guid: "metrics-guid-with-index", Index: 1},
				ContainerMetrics: executor.ContainerMetrics{
					MemoryUsageInBytes: 3210,
					DiskUsageInBytes:   6540,
					TimeSpentInCPU:     110 * time.Second,
					MemoryLimitInBytes: 9870,
					DiskLimitInBytes:   512,
				}},
		}

		metricsResults <- map[string]executor.Metrics{
			"guid-without-index": executor.Metrics{
				MetricsConfig: executor.MetricsConfig{Guid: "metrics-guid-without-index"},
				ContainerMetrics: executor.ContainerMetrics{
					MemoryUsageInBytes: 12300,
					DiskUsageInBytes:   45600,
					TimeSpentInCPU:     107 * time.Second,
					MemoryLimitInBytes: 7890,
					DiskLimitInBytes:   234,
				},
			},
			"guid-with-index": executor.Metrics{
				MetricsConfig: executor.MetricsConfig{Guid: "metrics-guid-with-index", Index: 1},
				ContainerMetrics: executor.ContainerMetrics{
					MemoryUsageInBytes: 32100,
					DiskUsageInBytes:   65400,
					TimeSpentInCPU:     112 * time.Second,
					MemoryLimitInBytes: 98700,
					DiskLimitInBytes:   43200,
				},
			},
		}
	}

	containerMetrics := func(eventList []events.Event) []events.ContainerMetric {
		metrics := make([]events.ContainerMetric, 0)
		for _, event := range eventList {
			metrics = append(metrics, *event.(*events.ContainerMetric))
		}
		return metrics
	}

	createEventContainerMetric := func(metrics executor.Metrics, cpuPercentage float64) events.ContainerMetric {
		instanceIndex := int32(metrics.MetricsConfig.Index)
		return events.ContainerMetric{
			ApplicationId:    &metrics.MetricsConfig.Guid,
			InstanceIndex:    &instanceIndex,
			CpuPercentage:    &cpuPercentage,
			MemoryBytes:      &metrics.ContainerMetrics.MemoryUsageInBytes,
			DiskBytes:        &metrics.ContainerMetrics.DiskUsageInBytes,
			MemoryBytesQuota: &metrics.ContainerMetrics.MemoryLimitInBytes,
			DiskBytesQuota:   &metrics.ContainerMetrics.DiskLimitInBytes,
		}
	}

	BeforeEach(func() {
		logger = lagertest.NewTestLogger("test")

		interval = 10 * time.Second
		fakeClock = fakeclock.NewFakeClock(time.Now())
		fakeExecutorClient = new(efakes.FakeClient)

		fakeMetricSender = msfake.NewFakeMetricSender()
		dmetrics.Initialize(fakeMetricSender, nil)

		metricsResults = make(chan map[string]executor.Metrics, 10)

		fakeExecutorClient.GetBulkMetricsStub = func(lager.Logger) (map[string]executor.Metrics, error) {
			result, ok := <-metricsResults
			if !ok || result == nil {
				return nil, errors.New("closed")
			}
			return result, nil
		}

		process = ifrit.Invoke(containermetrics.NewStatsReporter(logger, interval, fakeClock, fakeExecutorClient))
	})

	AfterEach(func() {
		close(metricsResults)
		ginkgomon.Interrupt(process)
	})

	Context("when the interval elapses", func() {
		BeforeEach(func() {
			sendResults()

			fakeClock.WaitForWatcherAndIncrement(interval)
			Eventually(fakeExecutorClient.GetBulkMetricsCallCount).Should(Equal(1))
		})

		It("emits memory and disk usage for each container, but no CPU", func() {
			Eventually(func() []events.ContainerMetric {
				return containerMetrics(fakeMetricSender.Events())
			}).Should(ConsistOf([]events.ContainerMetric{
				createEventContainerMetric(executor.Metrics{
					MetricsConfig: executor.MetricsConfig{Guid: "metrics-guid-without-index"},
					ContainerMetrics: executor.ContainerMetrics{
						MemoryUsageInBytes: 123,
						DiskUsageInBytes:   456,
						MemoryLimitInBytes: 789,
						DiskLimitInBytes:   1024,
					},
				},
					0.0),
				createEventContainerMetric(executor.Metrics{
					MetricsConfig: executor.MetricsConfig{Guid: "metrics-guid-with-index", Index: 1},
					ContainerMetrics: executor.ContainerMetrics{
						MemoryUsageInBytes: 321,
						DiskUsageInBytes:   654,
						MemoryLimitInBytes: 987,
						DiskLimitInBytes:   2048,
					},
				},
					0.0),
			}))

		})

		It("does not emit anything for containers with no metrics guid", func() {
			Consistently(func() msfake.ContainerMetric {
				return fakeMetricSender.GetContainerMetric("")
			}).Should(BeZero())
		})

		Context("and the interval elapses again", func() {
			BeforeEach(func() {
				fakeClock.WaitForWatcherAndIncrement(interval)
				Eventually(fakeExecutorClient.GetBulkMetricsCallCount).Should(Equal(2))
			})

			It("emits the new memory and disk usage, and the computed CPU percent", func() {
				Eventually(func() []events.ContainerMetric {
					return containerMetrics(fakeMetricSender.Events())
				}).Should(ContainElement(
					createEventContainerMetric(executor.Metrics{
						MetricsConfig: executor.MetricsConfig{Guid: "metrics-guid-without-index"},
						ContainerMetrics: executor.ContainerMetrics{
							MemoryUsageInBytes: 1230,
							DiskUsageInBytes:   4560,
							MemoryLimitInBytes: 7890,
							DiskLimitInBytes:   4096,
						},
					}, 50.0)))
				Eventually(func() []events.ContainerMetric {
					return containerMetrics(fakeMetricSender.Events())
				}).Should(ContainElement(
					createEventContainerMetric(executor.Metrics{
						MetricsConfig: executor.MetricsConfig{Guid: "metrics-guid-with-index", Index: 1},
						ContainerMetrics: executor.ContainerMetrics{
							MemoryUsageInBytes: 3210,
							DiskUsageInBytes:   6540,
							MemoryLimitInBytes: 9870,
							DiskLimitInBytes:   512,
						},
					},
						100.0)))
			})

			Context("and the interval elapses again", func() {
				BeforeEach(func() {
					fakeClock.WaitForWatcherAndIncrement(interval)
					Eventually(fakeExecutorClient.GetBulkMetricsCallCount).Should(Equal(3))
				})

				It("emits the new memory and disk usage, and the computed CPU percent", func() {
					Eventually(func() []events.ContainerMetric {
						return containerMetrics(fakeMetricSender.Events())
					}).Should(ContainElement(
						createEventContainerMetric(executor.Metrics{
							MetricsConfig: executor.MetricsConfig{Guid: "metrics-guid-without-index"},
							ContainerMetrics: executor.ContainerMetrics{
								MemoryUsageInBytes: 12300,
								DiskUsageInBytes:   45600,
								MemoryLimitInBytes: 7890,
								DiskLimitInBytes:   234,
							}},
							20.0),
					))
					Eventually(func() []events.ContainerMetric {
						return containerMetrics(fakeMetricSender.Events())
					}).Should(ContainElement(
						createEventContainerMetric(executor.Metrics{
							MetricsConfig: executor.MetricsConfig{Guid: "metrics-guid-with-index", Index: 1},
							ContainerMetrics: executor.ContainerMetrics{
								MemoryUsageInBytes: 32100,
								DiskUsageInBytes:   65400,
								MemoryLimitInBytes: 98700,
								DiskLimitInBytes:   43200,
							},
						},
							20.0)))
				})
			})
		})
	})

	Context("when get all metrics fails", func() {
		BeforeEach(func() {
			metricsResults <- nil

			fakeClock.Increment(interval)
			Eventually(fakeExecutorClient.GetBulkMetricsCallCount).Should(Equal(1))
		})

		It("does not blow up", func() {
			Consistently(process.Wait()).ShouldNot(Receive())
		})

		Context("and the interval elapses again, and it works that time", func() {
			BeforeEach(func() {
				sendResults()
				fakeClock.Increment(interval)
				Eventually(fakeExecutorClient.GetBulkMetricsCallCount).Should(Equal(2))
			})

			It("processes the containers happily", func() {
				Eventually(func() []events.ContainerMetric {
					return containerMetrics(fakeMetricSender.Events())
				}).Should(ContainElement(
					createEventContainerMetric(executor.Metrics{
						MetricsConfig: executor.MetricsConfig{Guid: "metrics-guid-without-index", Index: 0},
						ContainerMetrics: executor.ContainerMetrics{
							MemoryUsageInBytes: 123,
							DiskUsageInBytes:   456,
							MemoryLimitInBytes: 789,
							DiskLimitInBytes:   1024,
						}},
						0.0),
				))
				Eventually(func() []events.ContainerMetric {
					return containerMetrics(fakeMetricSender.Events())
				}).Should(ContainElement(
					createEventContainerMetric(executor.Metrics{
						MetricsConfig: executor.MetricsConfig{Guid: "metrics-guid-with-index", Index: 1},
						ContainerMetrics: executor.ContainerMetrics{
							MemoryUsageInBytes: 321,
							DiskUsageInBytes:   654,
							MemoryLimitInBytes: 987,
							DiskLimitInBytes:   2048,
						}},
						0.0),
				))
			})
		})
	})

	Context("when a container is no longer present", func() {
		metrics := func(metricsGuid string, index int, memoryUsage, diskUsage, memoryLimit, diskLimit uint64, cpuTime time.Duration) executor.Metrics {
			return executor.Metrics{
				MetricsConfig: executor.MetricsConfig{
					Guid:  metricsGuid,
					Index: index,
				},
				ContainerMetrics: executor.ContainerMetrics{
					MemoryUsageInBytes: memoryUsage,
					DiskUsageInBytes:   diskUsage,
					MemoryLimitInBytes: memoryLimit,
					DiskLimitInBytes:   diskLimit,
					TimeSpentInCPU:     cpuTime,
				},
			}
		}

		waitForMetrics := func(id string, instance int32, cpu float64, memoryUsage, diskUsage, memoryLimit, diskLimit uint64) {
			Eventually(func() []events.ContainerMetric {
				return containerMetrics(fakeMetricSender.Events())
			}).Should(ContainElement(
				createEventContainerMetric(executor.Metrics{
					MetricsConfig: executor.MetricsConfig{Guid: id, Index: int(instance)},
					ContainerMetrics: executor.ContainerMetrics{
						MemoryUsageInBytes: memoryUsage,
						DiskUsageInBytes:   diskUsage,
						MemoryLimitInBytes: memoryLimit,
						DiskLimitInBytes:   diskLimit,
					}},
					cpu),
			))
		}

		It("only remembers the previous metrics", func() {
			metricsResults <- map[string]executor.Metrics{
				"container-guid-0": metrics("metrics-guid-0", 0, 128, 256, 512, 1024, 10*time.Second),
				"container-guid-1": metrics("metrics-guid-1", 1, 256, 512, 1024, 2048, 10*time.Second),
			}

			fakeClock.Increment(interval)

			waitForMetrics("metrics-guid-0", 0, 0, 128, 256, 512, 1024)
			waitForMetrics("metrics-guid-1", 1, 0, 256, 512, 1024, 2048)

			By("losing a container")
			metricsResults <- map[string]executor.Metrics{
				"container-guid-0": metrics("metrics-guid-0", 0, 256, 512, 1024, 2048, 30*time.Second),
			}

			fakeClock.Increment(interval)

			waitForMetrics("metrics-guid-0", 0, 200, 256, 512, 1024, 2048)
			waitForMetrics("metrics-guid-1", 1, 0, 256, 512, 1024, 2048)

			By("finding the container again")
			metricsResults <- map[string]executor.Metrics{
				"container-guid-0": metrics("metrics-guid-0", 0, 256, 512, 2, 3, 40*time.Second),
				"container-guid-1": metrics("metrics-guid-1", 1, 512, 1024, 3, 2, 10*time.Second),
			}

			fakeClock.Increment(interval)

			waitForMetrics("metrics-guid-0", 0, 100, 256, 512, 2, 3)
			waitForMetrics("metrics-guid-1", 1, 0, 512, 1024, 3, 2)
		})
	})
})
