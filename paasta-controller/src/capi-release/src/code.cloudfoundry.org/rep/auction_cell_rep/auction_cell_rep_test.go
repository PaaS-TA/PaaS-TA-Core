package auction_cell_rep_test

import (
	"errors"

	"code.cloudfoundry.org/bbs/models"
	"code.cloudfoundry.org/executor"
	fake_client "code.cloudfoundry.org/executor/fakes"
	"code.cloudfoundry.org/lager/lagertest"
	"code.cloudfoundry.org/rep"
	"code.cloudfoundry.org/rep/auction_cell_rep"
	"code.cloudfoundry.org/rep/evacuation/evacuation_context/fake_evacuation_context"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("AuctionCellRep", func() {
	const (
		expectedCellID = "some-cell-id"
		linuxStack     = "linux"
		linuxPath      = "/data/rootfs/linux"
	)

	var (
		cellRep            rep.AuctionCellClient
		client             *fake_client.FakeClient
		logger             *lagertest.TestLogger
		evacuationReporter *fake_evacuation_context.FakeEvacuationReporter

		expectedGuid, linuxRootFSURL string
		commonErr, expectedGuidError error

		fakeGenerateContainerGuid func() (string, error)

		placementTags, optionalPlacementTags []string
	)

	BeforeEach(func() {
		client = new(fake_client.FakeClient)
		logger = lagertest.NewTestLogger("test")
		evacuationReporter = &fake_evacuation_context.FakeEvacuationReporter{}

		expectedGuid = "container-guid"
		expectedGuidError = nil
		fakeGenerateContainerGuid = func() (string, error) {
			return expectedGuid, expectedGuidError
		}
		linuxRootFSURL = models.PreloadedRootFS(linuxStack)

		commonErr = errors.New("Failed to fetch")
		client.HealthyReturns(true)
	})

	JustBeforeEach(func() {
		cellRep = auction_cell_rep.New(
			expectedCellID,
			rep.StackPathMap{linuxStack: linuxPath},
			[]string{"docker"},
			"the-zone",
			fakeGenerateContainerGuid,
			client,
			evacuationReporter,
			placementTags,
			optionalPlacementTags,
		)
	})

	Describe("State", func() {
		var (
			availableResources, totalResources executor.ExecutorResources
			containers                         []executor.Container
			volumeDrivers                      []string
		)

		BeforeEach(func() {
			evacuationReporter.EvacuatingReturns(true)
			totalResources = executor.ExecutorResources{
				MemoryMB:   1024,
				DiskMB:     2048,
				Containers: 4,
			}

			availableResources = executor.ExecutorResources{
				MemoryMB:   512,
				DiskMB:     256,
				Containers: 2,
			}

			containers = []executor.Container{
				{
					Guid:     "first",
					Resource: executor.NewResource(20, 10, 100, "rootfs"),
					Tags: executor.Tags{
						rep.LifecycleTag:    rep.LRPLifecycle,
						rep.ProcessGuidTag:  "the-first-app-guid",
						rep.ProcessIndexTag: "17",
						rep.DomainTag:       "domain",
					},
					State: executor.StateReserved,
				},
				{
					Guid:     "second",
					Resource: executor.NewResource(40, 30, 100, "rootfs"),
					Tags: executor.Tags{
						rep.LifecycleTag:    rep.LRPLifecycle,
						rep.ProcessGuidTag:  "the-second-app-guid",
						rep.ProcessIndexTag: "92",
						rep.DomainTag:       "domain",
					},
					State: executor.StateInitializing,
				},
				{
					Guid:     "da-task",
					Resource: executor.NewResource(40, 30, 100, "rootfs"),
					Tags: executor.Tags{
						rep.LifecycleTag: rep.TaskLifecycle,
						rep.DomainTag:    "domain",
					},
					State: executor.StateCreated,
				},
				{
					Guid:     "other-task",
					Resource: executor.NewResource(40, 30, 100, "rootfs"),
					Tags:     nil,
					State:    executor.StateRunning,
				},
			}

			volumeDrivers = []string{"lewis", "nico", "sebastian", "felipe"}
			placementTags = []string{}
			optionalPlacementTags = []string{}

			client.TotalResourcesReturns(totalResources, nil)
			client.RemainingResourcesReturns(availableResources, nil)
			client.ListContainersReturns(containers, nil)
			client.VolumeDriversReturns(volumeDrivers, nil)
		})

		It("queries the client and returns state", func() {
			state, err := cellRep.State(logger)
			Expect(err).NotTo(HaveOccurred())

			Expect(state.Evacuating).To(BeTrue())
			Expect(state.RootFSProviders).To(Equal(rep.RootFSProviders{
				models.PreloadedRootFSScheme: rep.NewFixedSetRootFSProvider("linux"),
				"docker":                     rep.ArbitraryRootFSProvider{},
			}))

			Expect(state.AvailableResources).To(Equal(rep.Resources{
				MemoryMB:   int32(availableResources.MemoryMB),
				DiskMB:     int32(availableResources.DiskMB),
				Containers: availableResources.Containers,
			}))

			Expect(state.TotalResources).To(Equal(rep.Resources{
				MemoryMB:   int32(totalResources.MemoryMB),
				DiskMB:     int32(totalResources.DiskMB),
				Containers: totalResources.Containers,
			}))

			Expect(state.LRPs).To(ConsistOf([]rep.LRP{
				rep.NewLRP(
					models.NewActualLRPKey("the-first-app-guid", 17, "domain"),
					rep.NewResource(20, 10, 0),
					rep.NewPlacementConstraint("", nil, nil),
				),
				rep.NewLRP(
					models.NewActualLRPKey("the-second-app-guid", 92, "domain"),
					rep.NewResource(40, 30, 0),
					rep.NewPlacementConstraint("", nil, nil),
				),
			}))

			Expect(state.Tasks).To(ConsistOf([]rep.Task{
				rep.NewTask(
					"da-task",
					"domain",
					rep.NewResource(40, 30, 0),
					rep.NewPlacementConstraint("", nil, nil),
				),
			}))

			Expect(state.StartingContainerCount).To(Equal(3))

			Expect(state.VolumeDrivers).To(ConsistOf(volumeDrivers))
		})

		Context("when the cell is not healthy", func() {
			BeforeEach(func() {
				client.HealthyReturns(false)
			})

			It("errors when reporting state", func() {
				_, err := cellRep.State(logger)
				Expect(err).To(MatchError(auction_cell_rep.ErrCellUnhealthy))
			})
		})

		Context("when the client fails to fetch total resources", func() {
			BeforeEach(func() {
				client.TotalResourcesReturns(executor.ExecutorResources{}, commonErr)
			})

			It("should return an error and no state", func() {
				state, err := cellRep.State(logger)
				Expect(state).To(BeZero())
				Expect(err).To(MatchError(commonErr))
			})
		})

		Context("when the client fails to fetch available resources", func() {
			BeforeEach(func() {
				client.RemainingResourcesReturns(executor.ExecutorResources{}, commonErr)
			})

			It("should return an error and no state", func() {
				state, err := cellRep.State(logger)
				Expect(state).To(BeZero())
				Expect(err).To(MatchError(commonErr))
			})
		})

		Context("when the client fails to list containers", func() {
			BeforeEach(func() {
				client.ListContainersReturns(nil, commonErr)
			})

			It("should return an error and no state", func() {
				state, err := cellRep.State(logger)
				Expect(state).To(BeZero())
				Expect(err).To(MatchError(commonErr))
			})
		})

		Context("when placement tags have been set", func() {
			BeforeEach(func() {
				placementTags = []string{"quack", "oink"}
			})

			It("returns the tags as part of the state", func() {
				state, err := cellRep.State(logger)
				Expect(err).NotTo(HaveOccurred())
				Expect(state.PlacementTags).To(ConsistOf(placementTags))
			})
		})

		Context("when optional placement tags have been set", func() {
			BeforeEach(func() {
				optionalPlacementTags = []string{"baa", "cluck"}
			})

			It("returns the tags as part of the state", func() {
				state, err := cellRep.State(logger)
				Expect(err).NotTo(HaveOccurred())
				Expect(state.OptionalPlacementTags).To(ConsistOf(optionalPlacementTags))
			})
		})
	})

	Describe("Perform", func() {
		var (
			work rep.Work
			task rep.Task

			expectedIndex = 1
		)

		Context("when evacuating", func() {
			BeforeEach(func() {
				evacuationReporter.EvacuatingReturns(true)

				lrp := rep.NewLRP(
					models.NewActualLRPKey("process-guid", int32(expectedIndex), "tests"),
					rep.NewResource(2048, 1024, 100),
					rep.NewPlacementConstraint(linuxRootFSURL, nil, []string{}),
				)

				task := rep.NewTask(
					"the-task-guid",
					"tests",
					rep.NewResource(2048, 1024, 100),
					rep.NewPlacementConstraint(linuxRootFSURL, nil, []string{}),
				)

				work = rep.Work{
					LRPs:  []rep.LRP{lrp},
					Tasks: []rep.Task{task},
				}
			})

			It("returns all work it was given", func() {
				Expect(cellRep.Perform(logger, work)).To(Equal(work))
			})
		})

		Describe("performing starts", func() {
			const (
				expectedIndexOneString = "1"
				expectedIndexTwoString = "2"
			)

			var (
				lrpAuctionOne, lrpAuctionTwo rep.LRP
				expectedGuidOne                    = "instance-guid-1"
				expectedGuidTwo                    = "instance-guid-2"
				expectedIndexOne             int32 = 1
				expectedIndexTwo             int32 = 2
			)

			BeforeEach(func() {
				guidChan := make(chan string, 2)
				guidChan <- expectedGuidOne
				guidChan <- expectedGuidTwo

				fakeGenerateContainerGuid = func() (string, error) {
					return <-guidChan, nil
				}

				lrpAuctionOne = rep.NewLRP(
					models.NewActualLRPKey("process-guid", expectedIndexOne, "tests"),
					rep.NewResource(2048, 1024, 100),
					rep.NewPlacementConstraint("rootfs", nil, []string{}),
				)
				lrpAuctionTwo = rep.NewLRP(
					models.NewActualLRPKey("process-guid", expectedIndexTwo, "tests"),
					rep.NewResource(2048, 1024, 100),
					rep.NewPlacementConstraint("rootfs", nil, []string{}),
				)
			})

			Context("when all LRP Auctions can be successfully translated to container specs", func() {
				BeforeEach(func() {
					lrpAuctionOne.RootFs = linuxRootFSURL
					lrpAuctionTwo.RootFs = "unsupported-arbitrary://still-goes-through"
				})

				It("makes the correct allocation requests for all LRP Auctions", func() {
					_, err := cellRep.Perform(logger, rep.Work{
						LRPs: []rep.LRP{lrpAuctionOne, lrpAuctionTwo},
					})
					Expect(err).NotTo(HaveOccurred())

					Expect(client.AllocateContainersCallCount()).To(Equal(1))
					_, arg := client.AllocateContainersArgsForCall(0)
					Expect(arg).To(ConsistOf(
						executor.AllocationRequest{
							Guid: rep.LRPContainerGuid(lrpAuctionOne.ProcessGuid, expectedGuidOne),
							Tags: executor.Tags{
								rep.LifecycleTag:    rep.LRPLifecycle,
								rep.DomainTag:       lrpAuctionOne.Domain,
								rep.ProcessGuidTag:  lrpAuctionOne.ProcessGuid,
								rep.InstanceGuidTag: expectedGuidOne,
								rep.ProcessIndexTag: expectedIndexOneString,
							},
							Resource: executor.NewResource(int(lrpAuctionOne.MemoryMB), int(lrpAuctionOne.DiskMB), int(lrpAuctionOne.MaxPids), linuxPath),
						},
						executor.AllocationRequest{
							Guid: rep.LRPContainerGuid(lrpAuctionTwo.ProcessGuid, expectedGuidTwo),
							Tags: executor.Tags{
								rep.LifecycleTag:    rep.LRPLifecycle,
								rep.DomainTag:       lrpAuctionTwo.Domain,
								rep.ProcessGuidTag:  lrpAuctionTwo.ProcessGuid,
								rep.InstanceGuidTag: expectedGuidTwo,
								rep.ProcessIndexTag: expectedIndexTwoString,
							},
							Resource: executor.NewResource(int(lrpAuctionTwo.MemoryMB), int(lrpAuctionTwo.DiskMB), int(lrpAuctionTwo.MaxPids), "unsupported-arbitrary://still-goes-through"),
						},
					))
				})

				Context("when all containers can be successfully allocated", func() {
					BeforeEach(func() {
						client.AllocateContainersReturns([]executor.AllocationFailure{}, nil)
					})

					It("does not mark any LRP Auctions as failed", func() {
						failedWork, err := cellRep.Perform(logger, rep.Work{LRPs: []rep.LRP{lrpAuctionOne, lrpAuctionTwo}})
						Expect(err).NotTo(HaveOccurred())
						Expect(failedWork).To(BeZero())
					})
				})

				Context("when a container fails to be allocated", func() {
					BeforeEach(func() {
						resource := executor.NewResource(int(lrpAuctionOne.MemoryMB), int(lrpAuctionOne.DiskMB), int(lrpAuctionOne.MaxPids), "rootfs")
						tags := executor.Tags{}
						tags[rep.ProcessGuidTag] = lrpAuctionOne.ProcessGuid
						allocationRequest := executor.NewAllocationRequest(
							rep.LRPContainerGuid(lrpAuctionOne.ProcessGuid, expectedGuidOne),
							&resource,
							tags,
						)
						allocationFailure := executor.NewAllocationFailure(&allocationRequest, commonErr.Error())
						client.AllocateContainersReturns([]executor.AllocationFailure{allocationFailure}, nil)
					})

					It("marks the corresponding LRP Auctions as failed", func() {
						failedWork, err := cellRep.Perform(logger, rep.Work{LRPs: []rep.LRP{lrpAuctionOne, lrpAuctionTwo}})
						Expect(err).NotTo(HaveOccurred())
						Expect(failedWork.LRPs).To(ConsistOf(lrpAuctionOne))
					})
				})
			})

			Context("when an LRP Auction specifies a preloaded RootFSes for which it cannot determine a RootFS path", func() {
				BeforeEach(func() {
					lrpAuctionOne.RootFs = linuxRootFSURL
					lrpAuctionTwo.RootFs = "preloaded:not-on-cell"
				})

				It("only makes container allocation requests for the remaining LRP Auctions", func() {
					_, err := cellRep.Perform(logger, rep.Work{LRPs: []rep.LRP{lrpAuctionOne, lrpAuctionTwo}})
					Expect(err).NotTo(HaveOccurred())

					Expect(client.AllocateContainersCallCount()).To(Equal(1))
					_, arg := client.AllocateContainersArgsForCall(0)
					Expect(arg).To(ConsistOf(
						executor.AllocationRequest{
							Guid: rep.LRPContainerGuid(lrpAuctionOne.ProcessGuid, expectedGuidOne),
							Tags: executor.Tags{
								rep.LifecycleTag:    rep.LRPLifecycle,
								rep.DomainTag:       lrpAuctionOne.Domain,
								rep.ProcessGuidTag:  lrpAuctionOne.ProcessGuid,
								rep.InstanceGuidTag: expectedGuidOne,
								rep.ProcessIndexTag: expectedIndexOneString,
							},
							Resource: executor.NewResource(int(lrpAuctionOne.MemoryMB), int(lrpAuctionOne.DiskMB), int(lrpAuctionOne.MaxPids), linuxPath),
						},
					))
				})

				It("marks the LRP Auction as failed", func() {
					failedWork, err := cellRep.Perform(logger, rep.Work{LRPs: []rep.LRP{lrpAuctionOne, lrpAuctionTwo}})
					Expect(err).NotTo(HaveOccurred())
					Expect(failedWork.LRPs).To(ContainElement(lrpAuctionTwo))
				})

				Context("when all remaining containers can be successfully allocated", func() {
					BeforeEach(func() {
						client.AllocateContainersReturns([]executor.AllocationFailure{}, nil)
					})

					It("does not mark any additional LRP Auctions as failed", func() {
						failedWork, err := cellRep.Perform(logger, rep.Work{LRPs: []rep.LRP{lrpAuctionOne, lrpAuctionTwo}})
						Expect(err).NotTo(HaveOccurred())
						Expect(failedWork.LRPs).To(ConsistOf(lrpAuctionTwo))
					})
				})

				Context("when a remaining container fails to be allocated", func() {
					BeforeEach(func() {
						resource := executor.NewResource(int(lrpAuctionOne.MemoryMB), int(lrpAuctionOne.DiskMB), int(lrpAuctionOne.MaxPids), "rootfs")
						tags := executor.Tags{}
						tags[rep.ProcessGuidTag] = lrpAuctionOne.ProcessGuid
						allocationRequest := executor.NewAllocationRequest(
							rep.LRPContainerGuid(lrpAuctionOne.ProcessGuid, expectedGuidOne),
							&resource,
							tags,
						)
						allocationFailure := executor.NewAllocationFailure(&allocationRequest, commonErr.Error())
						client.AllocateContainersReturns([]executor.AllocationFailure{allocationFailure}, nil)
					})

					It("marks the corresponding LRP Auctions as failed", func() {
						failedWork, err := cellRep.Perform(logger, rep.Work{LRPs: []rep.LRP{lrpAuctionOne, lrpAuctionTwo}})
						Expect(err).NotTo(HaveOccurred())
						Expect(failedWork.LRPs).To(ConsistOf(lrpAuctionOne, lrpAuctionTwo))
					})
				})
			})

			Context("when an LRP Auction specifies a blank RootFS URL", func() {
				BeforeEach(func() {
					lrpAuctionOne.RootFs = ""
				})

				It("makes the correct allocation request for it, passing along the blank path to the executor client", func() {
					_, err := cellRep.Perform(logger, rep.Work{LRPs: []rep.LRP{lrpAuctionOne}})
					Expect(err).NotTo(HaveOccurred())

					Expect(client.AllocateContainersCallCount()).To(Equal(1))
					_, arg := client.AllocateContainersArgsForCall(0)
					Expect(arg).To(ConsistOf(
						executor.AllocationRequest{
							Guid: rep.LRPContainerGuid(lrpAuctionOne.ProcessGuid, expectedGuidOne),
							Tags: executor.Tags{
								rep.LifecycleTag:    rep.LRPLifecycle,
								rep.DomainTag:       lrpAuctionOne.Domain,
								rep.ProcessGuidTag:  lrpAuctionOne.ProcessGuid,
								rep.InstanceGuidTag: expectedGuidOne,
								rep.ProcessIndexTag: expectedIndexOneString,
							},
							Resource: executor.NewResource(int(lrpAuctionOne.MemoryMB), int(lrpAuctionOne.DiskMB), int(lrpAuctionOne.MaxPids), ""),
						},
					))
				})
			})

			Context("when an LRP Auction specifies an invalid RootFS URL", func() {
				BeforeEach(func() {
					lrpAuctionOne.RootFs = linuxRootFSURL
					lrpAuctionTwo.RootFs = "%x"
				})

				It("only makes container allocation requests for the remaining LRP Auctions", func() {
					_, err := cellRep.Perform(logger, rep.Work{LRPs: []rep.LRP{lrpAuctionOne, lrpAuctionTwo}})
					Expect(err).NotTo(HaveOccurred())

					Expect(client.AllocateContainersCallCount()).To(Equal(1))
					_, arg := client.AllocateContainersArgsForCall(0)
					Expect(arg).To(ConsistOf(
						executor.AllocationRequest{
							Guid: rep.LRPContainerGuid(lrpAuctionOne.ProcessGuid, expectedGuidOne),
							Tags: executor.Tags{
								rep.LifecycleTag:    rep.LRPLifecycle,
								rep.DomainTag:       lrpAuctionOne.Domain,
								rep.ProcessGuidTag:  lrpAuctionOne.ProcessGuid,
								rep.InstanceGuidTag: expectedGuidOne,
								rep.ProcessIndexTag: expectedIndexOneString,
							},
							Resource: executor.NewResource(int(lrpAuctionOne.MemoryMB), int(lrpAuctionOne.DiskMB), int(lrpAuctionOne.MaxPids), linuxPath),
						},
					))
				})

				It("marks the LRP Auction as failed", func() {
					failedWork, err := cellRep.Perform(logger, rep.Work{LRPs: []rep.LRP{lrpAuctionOne, lrpAuctionTwo}})
					Expect(err).NotTo(HaveOccurred())
					Expect(failedWork.LRPs).To(ContainElement(lrpAuctionTwo))
				})

				Context("when all remaining containers can be successfully allocated", func() {
					BeforeEach(func() {
						client.AllocateContainersReturns([]executor.AllocationFailure{}, nil)
					})

					It("does not mark any additional LRP Auctions as failed", func() {
						failedWork, err := cellRep.Perform(logger, rep.Work{LRPs: []rep.LRP{lrpAuctionOne, lrpAuctionTwo}})
						Expect(err).NotTo(HaveOccurred())
						Expect(failedWork.LRPs).To(ConsistOf(lrpAuctionTwo))
					})
				})

				Context("when a remaining container fails to be allocated", func() {
					BeforeEach(func() {
						resource := executor.NewResource(int(lrpAuctionOne.MemoryMB), int(lrpAuctionOne.DiskMB), int(lrpAuctionOne.MaxPids), "rootfs")
						tags := executor.Tags{}
						tags[rep.ProcessGuidTag] = lrpAuctionOne.ProcessGuid
						allocationRequest := executor.NewAllocationRequest(
							rep.LRPContainerGuid(lrpAuctionOne.ProcessGuid, expectedGuidOne),
							&resource,
							tags,
						)
						allocationFailure := executor.NewAllocationFailure(&allocationRequest, commonErr.Error())
						client.AllocateContainersReturns([]executor.AllocationFailure{allocationFailure}, nil)
					})

					It("marks the corresponding LRP Auctions as failed", func() {
						failedWork, err := cellRep.Perform(logger, rep.Work{LRPs: []rep.LRP{lrpAuctionOne, lrpAuctionTwo}})
						Expect(err).NotTo(HaveOccurred())
						Expect(failedWork.LRPs).To(ConsistOf(lrpAuctionOne, lrpAuctionTwo))
					})
				})
			})
		})

		Describe("starting tasks", func() {
			var task1, task2 rep.Task

			BeforeEach(func() {
				resource1 := rep.NewResource(256, 512, 256)
				placement1 := rep.NewPlacementConstraint("tests", nil, []string{})
				task1 = rep.NewTask("the-task-guid-1", "tests", resource1, placement1)

				resource2 := rep.NewResource(512, 1024, 256)
				placement2 := rep.NewPlacementConstraint("linux", nil, []string{})
				task2 = rep.NewTask("the-task-guid-2", "tests", resource2, placement2)

				work = rep.Work{Tasks: []rep.Task{task}}
			})

			Context("when all Tasks can be successfully translated to container specs", func() {
				BeforeEach(func() {
					task1.RootFs = linuxRootFSURL
					task2.RootFs = "unsupported-arbitrary://still-goes-through"
				})

				It("makes the correct allocation requests for all Tasks", func() {
					_, err := cellRep.Perform(logger, rep.Work{Tasks: []rep.Task{task1, task2}})
					Expect(err).NotTo(HaveOccurred())

					Expect(client.AllocateContainersCallCount()).To(Equal(1))
					_, arg := client.AllocateContainersArgsForCall(0)
					Expect(arg).To(ConsistOf(
						allocationRequestFromTask(task1, linuxPath),
						allocationRequestFromTask(task2, task2.RootFs),
					))
				})

				Context("when all containers can be successfully allocated", func() {
					BeforeEach(func() {
						client.AllocateContainersReturns([]executor.AllocationFailure{}, nil)
					})

					It("does not mark any Tasks as failed", func() {
						failedWork, err := cellRep.Perform(logger, rep.Work{Tasks: []rep.Task{task1, task2}})
						Expect(err).NotTo(HaveOccurred())
						Expect(failedWork).To(BeZero())
					})
				})

				Context("when a container fails to be allocated", func() {
					BeforeEach(func() {
						resource := executor.NewResource(int(task1.MemoryMB), int(task1.DiskMB), int(task1.MaxPids), "linux")
						tags := executor.Tags{}
						allocationRequest := executor.NewAllocationRequest(
							task1.TaskGuid,
							&resource,
							tags,
						)
						allocationFailure := executor.NewAllocationFailure(&allocationRequest, commonErr.Error())
						client.AllocateContainersReturns([]executor.AllocationFailure{allocationFailure}, nil)
					})

					It("marks the corresponding Tasks as failed", func() {
						failedWork, err := cellRep.Perform(logger, rep.Work{Tasks: []rep.Task{task1, task2}})
						Expect(err).NotTo(HaveOccurred())
						Expect(failedWork.Tasks).To(ConsistOf(task1))
					})
				})
			})

			Context("when a Task specifies a preloaded RootFSes for which it cannot determine a RootFS path", func() {
				BeforeEach(func() {
					task1.RootFs = linuxRootFSURL
					task2.RootFs = "preloaded:not-on-cell"
				})

				It("only makes container allocation requests for the remaining Tasks", func() {
					_, err := cellRep.Perform(logger, rep.Work{Tasks: []rep.Task{task1, task2}})
					Expect(err).NotTo(HaveOccurred())

					Expect(client.AllocateContainersCallCount()).To(Equal(1))
					_, arg := client.AllocateContainersArgsForCall(0)
					Expect(arg).To(ConsistOf(
						allocationRequestFromTask(task1, linuxPath),
					))
				})

				It("marks the Task as failed", func() {
					failedWork, err := cellRep.Perform(logger, rep.Work{Tasks: []rep.Task{task1, task2}})
					Expect(err).NotTo(HaveOccurred())
					Expect(failedWork.Tasks).To(ContainElement(task2))
				})

				Context("when all remaining containers can be successfully allocated", func() {
					BeforeEach(func() {
						client.AllocateContainersReturns([]executor.AllocationFailure{}, nil)
					})

					It("does not mark any additional Tasks as failed", func() {
						failedWork, err := cellRep.Perform(logger, rep.Work{Tasks: []rep.Task{task1, task2}})
						Expect(err).NotTo(HaveOccurred())
						Expect(failedWork.Tasks).To(ConsistOf(task2))
					})
				})

				Context("when a remaining container fails to be allocated", func() {
					BeforeEach(func() {
						request := allocationRequestFromTask(task1, linuxPath)
						allocationFailure := executor.NewAllocationFailure(&request, commonErr.Error())
						client.AllocateContainersReturns([]executor.AllocationFailure{allocationFailure}, nil)
					})

					It("marks the corresponding Tasks as failed", func() {
						failedWork, err := cellRep.Perform(logger, rep.Work{Tasks: []rep.Task{task1, task2}})
						Expect(err).NotTo(HaveOccurred())
						Expect(failedWork.Tasks).To(ConsistOf(task1, task2))
					})
				})
			})

			Context("when a Task specifies a blank RootFS URL", func() {
				BeforeEach(func() {
					task1.RootFs = ""
				})

				It("makes the correct allocation request for it, passing along the blank path to the executor client", func() {
					_, err := cellRep.Perform(logger, rep.Work{Tasks: []rep.Task{task1}})
					Expect(err).NotTo(HaveOccurred())

					Expect(client.AllocateContainersCallCount()).To(Equal(1))
					_, arg := client.AllocateContainersArgsForCall(0)
					Expect(arg).To(ConsistOf(
						allocationRequestFromTask(task1, ""),
					))
				})
			})

			Context("when a Task specifies an invalid RootFS URL", func() {
				BeforeEach(func() {
					task1.RootFs = linuxRootFSURL
					task2.RootFs = "%x"
				})

				It("only makes container allocation requests for the remaining Tasks", func() {
					_, err := cellRep.Perform(logger, rep.Work{Tasks: []rep.Task{task1, task2}})
					Expect(err).NotTo(HaveOccurred())

					Expect(client.AllocateContainersCallCount()).To(Equal(1))
					_, arg := client.AllocateContainersArgsForCall(0)
					Expect(arg).To(ConsistOf(
						allocationRequestFromTask(task1, linuxPath),
					))
				})

				It("marks the Task as failed", func() {
					failedWork, err := cellRep.Perform(logger, rep.Work{Tasks: []rep.Task{task1, task2}})
					Expect(err).NotTo(HaveOccurred())
					Expect(failedWork.Tasks).To(ContainElement(task2))
				})

				Context("when all remaining containers can be successfully allocated", func() {
					BeforeEach(func() {
						client.AllocateContainersReturns([]executor.AllocationFailure{}, nil)
					})

					It("does not mark any additional LRP Auctions as failed", func() {
						failedWork, err := cellRep.Perform(logger, rep.Work{Tasks: []rep.Task{task1, task2}})
						Expect(err).NotTo(HaveOccurred())
						Expect(failedWork.Tasks).To(ConsistOf(task2))
					})
				})

				Context("when a remaining container fails to be allocated", func() {
					BeforeEach(func() {
						request := allocationRequestFromTask(task1, linuxPath)
						allocationFailure := executor.NewAllocationFailure(&request, commonErr.Error())
						client.AllocateContainersReturns([]executor.AllocationFailure{allocationFailure}, nil)
					})

					It("marks the corresponding Tasks as failed", func() {
						failedWork, err := cellRep.Perform(logger, rep.Work{Tasks: []rep.Task{task1, task2}})
						Expect(err).NotTo(HaveOccurred())
						Expect(failedWork.Tasks).To(ConsistOf(task1, task2))
					})
				})
			})
		})
	})
})

func allocationRequestFromTask(task rep.Task, rootFSPath string) executor.AllocationRequest {
	resource := executor.NewResource(int(task.MemoryMB), int(task.DiskMB), int(task.MaxPids), rootFSPath)
	return executor.NewAllocationRequest(
		task.TaskGuid,
		&resource,
		executor.Tags{
			rep.LifecycleTag: rep.TaskLifecycle,
			rep.DomainTag:    task.Domain,
		},
	)
}
