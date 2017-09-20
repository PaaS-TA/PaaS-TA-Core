package evacuation_test

import (
	"errors"
	"os"
	"time"

	"code.cloudfoundry.org/bbs/fake_bbs"
	"code.cloudfoundry.org/bbs/models"
	"code.cloudfoundry.org/bbs/models/test/model_helpers"
	"code.cloudfoundry.org/clock/fakeclock"
	"code.cloudfoundry.org/executor"
	"code.cloudfoundry.org/executor/fakes"
	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/lager/lagertest"
	"code.cloudfoundry.org/rep/evacuation"
	fake_metrics_sender "github.com/cloudfoundry/dropsonde/metric_sender/fake"
	"github.com/cloudfoundry/dropsonde/metrics"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/tedsuo/ifrit"
	"github.com/tedsuo/ifrit/ginkgomon"
)

var _ = Describe("EvacuationCleanup", func() {
	var (
		logger *lagertest.TestLogger
		cellID string

		fakeClock          *fakeclock.FakeClock
		fakeBBSClient      *fake_bbs.FakeInternalClient
		fakeExecutorClient *fakes.FakeClient
		fakeMetricsSender  *fake_metrics_sender.FakeMetricSender

		cleanup        *evacuation.EvacuationCleanup
		cleanupProcess ifrit.Process

		errCh  chan error
		doneCh chan struct{}
	)

	BeforeEach(func() {
		cellID = "the-cell-id"
		logger = lagertest.NewTestLogger("cleanup")

		fakeClock = fakeclock.NewFakeClock(time.Now())
		fakeBBSClient = &fake_bbs.FakeInternalClient{}
		fakeExecutorClient = &fakes.FakeClient{}
		fakeMetricsSender = fake_metrics_sender.NewFakeMetricSender()
		metrics.Initialize(fakeMetricsSender, nil)

		errCh = make(chan error, 1)
		doneCh = make(chan struct{})
		cleanup = evacuation.NewEvacuationCleanup(logger, cellID, fakeBBSClient, fakeExecutorClient, fakeClock)
	})

	JustBeforeEach(func() {
		cleanupProcess = ginkgomon.Invoke(cleanup)
		go func() {
			err := <-cleanupProcess.Wait()
			errCh <- err
			close(doneCh)
		}()
	})

	AfterEach(func() {
		cleanupProcess.Signal(os.Interrupt)
		fakeClock.Increment(15 * time.Second)
		Eventually(doneCh).Should(BeClosed())
	})

	It("does not exit", func() {
		Consistently(errCh).ShouldNot(Receive())
	})

	Context("when the process is signalled", func() {
		var (
			actualLRPGroups                          []*models.ActualLRPGroup
			actualLRPGroup, evacuatingActualLRPGroup *models.ActualLRPGroup
		)

		BeforeEach(func() {
			runningActualLRPGroup := &models.ActualLRPGroup{
				Instance: model_helpers.NewValidActualLRP("running-process-guid", 0),
			}
			evacuatingActualLRPGroup = &models.ActualLRPGroup{
				Evacuating: model_helpers.NewValidActualLRP("evacuating-process-guid", 0),
			}
			actualLRPGroup = &models.ActualLRPGroup{
				Instance:   model_helpers.NewValidActualLRP("process-guid", 0),
				Evacuating: model_helpers.NewValidActualLRP("process-guid", 0),
			}

			actualLRPGroups = []*models.ActualLRPGroup{
				runningActualLRPGroup,
				evacuatingActualLRPGroup,
				actualLRPGroup,
			}

			fakeBBSClient.ActualLRPGroupsReturns(actualLRPGroups, nil)

			fakeExecutorClient.ListContainersStub = func(lager.Logger) ([]executor.Container, error) {
				if fakeExecutorClient.ListContainersCallCount() == 1 {
					return []executor.Container{
						{Guid: "container1", State: executor.StateRunning},
						{Guid: "container2", State: executor.StateRunning},
					}, nil
				}

				return []executor.Container{
					{Guid: "container1", State: executor.StateCompleted},
					{Guid: "container2", State: executor.StateCompleted},
				}, nil
			}
		})

		JustBeforeEach(func() {
			cleanupProcess.Signal(os.Kill)
		})

		It("removes all evacuating actual lrps associated with the cell", func() {
			Eventually(errCh).Should(Receive(nil))
			Expect(fakeBBSClient.ActualLRPGroupsCallCount()).To(Equal(1))
			_, filter := fakeBBSClient.ActualLRPGroupsArgsForCall(0)
			Expect(filter).To(Equal(models.ActualLRPFilter{CellID: cellID}))

			Expect(fakeBBSClient.RemoveEvacuatingActualLRPCallCount()).To(Equal(2))

			_, lrpKey, lrpInstanceKey := fakeBBSClient.RemoveEvacuatingActualLRPArgsForCall(0)
			Expect(*lrpKey).To(Equal(evacuatingActualLRPGroup.Evacuating.ActualLRPKey))
			Expect(*lrpInstanceKey).To(Equal(evacuatingActualLRPGroup.Evacuating.ActualLRPInstanceKey))

			_, lrpKey, lrpInstanceKey = fakeBBSClient.RemoveEvacuatingActualLRPArgsForCall(1)
			Expect(*lrpKey).To(Equal(actualLRPGroup.Evacuating.ActualLRPKey))
			Expect(*lrpInstanceKey).To(Equal(actualLRPGroup.Evacuating.ActualLRPInstanceKey))
		})

		It("emits a metric for the number of stranded evacuating actual lrps", func() {
			Eventually(errCh).Should(Receive(nil))
			Expect(fakeMetricsSender.GetValue("StrandedEvacuatingActualLRPs").Value).To(BeEquivalentTo(2))
		})

		Describe("stopping running containers", func() {
			It("should stop all of the containers that still running", func() {
				Eventually(errCh).Should(Receive(nil))
				Expect(fakeExecutorClient.ListContainersCallCount()).To(Equal(2))
				Expect(fakeExecutorClient.StopContainerCallCount()).To(Equal(2))

				_, guid := fakeExecutorClient.StopContainerArgsForCall(0)
				Expect(guid).To(Equal("container1"))

				_, guid = fakeExecutorClient.StopContainerArgsForCall(1)
				Expect(guid).To(Equal("container2"))
			})

			// https://www.pivotaltracker.com/story/show/133061923
			Describe("when StopContainer hangs", func() {
				BeforeEach(func() {
					fakeExecutorClient.StopContainerStub = func(lager.Logger, string) error {
						time.Sleep(time.Minute)
						return nil
					}
				})

				It("gives up after 15 seconds", func() {
					fakeClock.WaitForNWatchersAndIncrement(15*time.Second, 2)
					Eventually(doneCh).Should(BeClosed())
				})
			})

			Describe("when ListContainers fails the first time", func() {
				BeforeEach(func() {
					fakeExecutorClient.ListContainersStub = func(lager.Logger) ([]executor.Container, error) {
						return nil, errors.New("cannot talk to garden")
					}
				})

				It("should exit immediately", func() {
					Eventually(doneCh).Should(BeClosed())
				})

				It("should logs the error", func() {
					Eventually(logger.Buffer()).Should(gbytes.Say("cannot talk to garden"))
				})
			})

			Describe("when ListContainers fails while listing containers", func() {
				BeforeEach(func() {
					fakeExecutorClient.ListContainersStub = func(lager.Logger) ([]executor.Container, error) {
						if fakeExecutorClient.ListContainersCallCount() == 1 {
							return []executor.Container{
								{Guid: "container1", State: executor.StateRunning},
								{Guid: "container2", State: executor.StateRunning},
							}, nil
						}

						return nil, errors.New("cannot talk to garden")
					}
				})

				It("should exit immediately", func() {
					Eventually(doneCh).Should(BeClosed())
				})

				It("should logs the error", func() {
					Eventually(logger.Buffer()).Should(gbytes.Say("cannot talk to garden"))
				})
			})

			Context("when the containers do not stop in time", func() {
				BeforeEach(func() {
					fakeExecutorClient.ListContainersStub = func(lager.Logger) ([]executor.Container, error) {
						return []executor.Container{
							{Guid: "container1", State: executor.StateRunning},
							{Guid: "container2", State: executor.StateRunning},
						}, nil
					}
				})

				It("gives up after 15 seconds", func() {
					Eventually(fakeExecutorClient.ListContainersCallCount).Should(Equal(2))
					Expect(fakeExecutorClient.StopContainerCallCount()).To(Equal(2))
					Consistently(errCh).ShouldNot(Receive())

					for i := 0; i < 14; i++ {
						fakeClock.WaitForNWatchersAndIncrement(1*time.Second, 2)
						Eventually(fakeExecutorClient.ListContainersCallCount).Should(Equal(i + 3))
					}

					Consistently(errCh).ShouldNot(Receive())
					fakeClock.WaitForNWatchersAndIncrement(1*time.Second, 2)
					Eventually(errCh).Should(Receive(HaveOccurred()))
				})
			})
		})

		Describe("when fetching the actual lrp groups fails", func() {
			BeforeEach(func() {
				fakeBBSClient.ActualLRPGroupsReturns(nil, errors.New("failed"))
			})

			It("exits with an error", func() {
				var err error
				Eventually(errCh).Should(Receive(&err))
				Expect(err).To(Equal(errors.New("failed")))
			})
		})

		Describe("when removing the evacuating actual lrp fails", func() {
			BeforeEach(func() {
				fakeBBSClient.RemoveEvacuatingActualLRPReturns(errors.New("failed"))
			})

			It("continues removing evacuating actual lrps", func() {
				Eventually(errCh).Should(Receive(nil))
				Expect(fakeBBSClient.RemoveEvacuatingActualLRPCallCount()).To(Equal(2))
			})
		})
	})
})
