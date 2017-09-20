package models_test

import (
	"time"

	"code.cloudfoundry.org/cf-tcp-router/models"
	"code.cloudfoundry.org/cf-tcp-router/testutil"
	"code.cloudfoundry.org/lager/lagertest"
	routing_api_models "code.cloudfoundry.org/routing-api/models"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
)

var _ = Describe("RoutingTable", func() {
	var (
		backendServerKey models.BackendServerKey
		routingTable     models.RoutingTable
		modificationTag  routing_api_models.ModificationTag
		logger           = lagertest.NewTestLogger("routing-table-test")
	)

	BeforeEach(func() {
		routingTable = models.NewRoutingTable(logger)
		modificationTag = routing_api_models.ModificationTag{Guid: "abc", Index: 1}
	})

	Describe("Set", func() {
		var (
			routingKey           models.RoutingKey
			routingTableEntry    models.RoutingTableEntry
			backendServerDetails models.BackendServerDetails
			now                  time.Time
		)

		BeforeEach(func() {
			routingKey = models.RoutingKey{Port: 12}
			backendServerKey = models.BackendServerKey{Address: "some-ip-1", Port: 1234}
			now = time.Now()
			backendServerDetails = models.BackendServerDetails{ModificationTag: modificationTag, TTL: 120, UpdatedTime: now}
			backends := map[models.BackendServerKey]models.BackendServerDetails{
				backendServerKey: backendServerDetails,
			}
			routingTableEntry = models.RoutingTableEntry{Backends: backends}
		})

		Context("when a new entry is added", func() {
			It("adds the entry", func() {
				ok := routingTable.Set(routingKey, routingTableEntry)
				Expect(ok).To(BeTrue())
				Expect(routingTable.Get(routingKey)).To(Equal(routingTableEntry))
				Expect(routingTable.Size()).To(Equal(1))
			})
		})

		Context("when setting pre-existing routing key", func() {
			var (
				existingRoutingTableEntry models.RoutingTableEntry
				newBackendServerKey       models.BackendServerKey
			)

			BeforeEach(func() {
				newBackendServerKey = models.BackendServerKey{
					Address: "some-ip-2",
					Port:    1234,
				}
				existingRoutingTableEntry = models.RoutingTableEntry{
					Backends: map[models.BackendServerKey]models.BackendServerDetails{
						backendServerKey:    backendServerDetails,
						newBackendServerKey: models.BackendServerDetails{ModificationTag: modificationTag, TTL: 120, UpdatedTime: now},
					},
				}
				ok := routingTable.Set(routingKey, existingRoutingTableEntry)
				Expect(ok).To(BeTrue())
				Expect(routingTable.Size()).To(Equal(1))
			})

			Context("with different value", func() {
				verifyChangedValue := func(routingTableEntry models.RoutingTableEntry) {
					ok := routingTable.Set(routingKey, routingTableEntry)
					Expect(ok).To(BeTrue())
					Expect(routingTable.Get(routingKey)).Should(Equal(routingTableEntry))
				}

				Context("when number of backends are different", func() {
					It("overwrites the value", func() {
						routingTableEntry := models.RoutingTableEntry{
							Backends: map[models.BackendServerKey]models.BackendServerDetails{
								models.BackendServerKey{
									Address: "some-ip-1",
									Port:    1234,
								}: models.BackendServerDetails{ModificationTag: modificationTag, UpdatedTime: now},
							},
						}
						verifyChangedValue(routingTableEntry)
					})
				})

				Context("when at least one backend server info is different", func() {
					It("overwrites the value", func() {
						routingTableEntry := models.RoutingTableEntry{
							Backends: map[models.BackendServerKey]models.BackendServerDetails{
								models.BackendServerKey{Address: "some-ip-1", Port: 1234}: models.BackendServerDetails{ModificationTag: modificationTag, UpdatedTime: now},
								models.BackendServerKey{Address: "some-ip-2", Port: 2345}: models.BackendServerDetails{ModificationTag: modificationTag, UpdatedTime: now},
							},
						}
						verifyChangedValue(routingTableEntry)
					})
				})

				Context("when all backend servers info are different", func() {
					It("overwrites the value", func() {
						routingTableEntry := models.RoutingTableEntry{
							Backends: map[models.BackendServerKey]models.BackendServerDetails{
								models.BackendServerKey{Address: "some-ip-1", Port: 3456}: models.BackendServerDetails{ModificationTag: modificationTag, UpdatedTime: now},
								models.BackendServerKey{Address: "some-ip-2", Port: 2345}: models.BackendServerDetails{ModificationTag: modificationTag, UpdatedTime: now},
							},
						}
						verifyChangedValue(routingTableEntry)
					})
				})

				Context("when modificationTag is different", func() {
					It("overwrites the value", func() {
						routingTableEntry := models.RoutingTableEntry{
							Backends: map[models.BackendServerKey]models.BackendServerDetails{
								models.BackendServerKey{Address: "some-ip-1", Port: 1234}: models.BackendServerDetails{ModificationTag: routing_api_models.ModificationTag{Guid: "different-guid"}, UpdatedTime: now},
								models.BackendServerKey{Address: "some-ip-2", Port: 1234}: models.BackendServerDetails{ModificationTag: routing_api_models.ModificationTag{Guid: "different-guid"}, UpdatedTime: now},
							},
						}
						verifyChangedValue(routingTableEntry)
					})
				})

				Context("when TTL is different", func() {
					It("overwrites the value", func() {
						routingTableEntry := models.RoutingTableEntry{
							Backends: map[models.BackendServerKey]models.BackendServerDetails{
								models.BackendServerKey{Address: "some-ip-1", Port: 1234}: models.BackendServerDetails{ModificationTag: modificationTag, TTL: 110, UpdatedTime: now},
								models.BackendServerKey{Address: "some-ip-2", Port: 1234}: models.BackendServerDetails{ModificationTag: modificationTag, TTL: 110, UpdatedTime: now},
							},
						}
						verifyChangedValue(routingTableEntry)
					})
				})
			})

			Context("with same value", func() {
				It("returns false", func() {
					routingTableEntry := models.RoutingTableEntry{
						Backends: map[models.BackendServerKey]models.BackendServerDetails{
							backendServerKey:    models.BackendServerDetails{ModificationTag: modificationTag, TTL: 120, UpdatedTime: now},
							newBackendServerKey: models.BackendServerDetails{ModificationTag: modificationTag, TTL: 120, UpdatedTime: now},
						},
					}
					ok := routingTable.Set(routingKey, routingTableEntry)
					Expect(ok).To(BeFalse())
					testutil.RoutingTableEntryMatches(routingTable.Get(routingKey), existingRoutingTableEntry)
				})
			})
		})
	})

	Describe("UpsertBackendServerKey", func() {
		var (
			routingKey models.RoutingKey
		)

		BeforeEach(func() {
			routingKey = models.RoutingKey{Port: 12}
			routingTable = models.NewRoutingTable(logger)
			modificationTag = routing_api_models.ModificationTag{Guid: "abc", Index: 5}
		})

		Context("when the routing key does not exist", func() {
			var (
				routingTableEntry models.RoutingTableEntry
				backendServerInfo models.BackendServerInfo
			)

			BeforeEach(func() {
				backendServerInfo = createBackendServerInfo("some-ip", 1234, modificationTag)
				routingTableEntry = models.NewRoutingTableEntry([]models.BackendServerInfo{backendServerInfo})
			})

			It("inserts the routing key with its backends", func() {
				updated := routingTable.UpsertBackendServerKey(routingKey, backendServerInfo)
				Expect(updated).To(BeTrue())
				Expect(routingTable.Size()).To(Equal(1))
				testutil.RoutingTableEntryMatches(routingTable.Get(routingKey), routingTableEntry)
			})
		})

		Context("when the routing key does exist", func() {
			var backendServerInfo models.BackendServerInfo

			BeforeEach(func() {
				backendServerInfo = createBackendServerInfo("some-ip", 1234, modificationTag)
				existingRoutingTableEntry := models.NewRoutingTableEntry([]models.BackendServerInfo{backendServerInfo})
				updated := routingTable.Set(routingKey, existingRoutingTableEntry)
				Expect(updated).To(BeTrue())
			})

			Context("when current entry is succeeded by new entry", func() {
				BeforeEach(func() {
					modificationTag.Increment()
				})

				It("updates the routing entry", func() {
					sameBackendServerInfo := createBackendServerInfo("some-ip", 1234, modificationTag)
					expectedRoutingTableEntry := models.NewRoutingTableEntry([]models.BackendServerInfo{sameBackendServerInfo})
					routingTable.UpsertBackendServerKey(routingKey, sameBackendServerInfo)
					testutil.RoutingTableEntryMatches(routingTable.Get(routingKey), expectedRoutingTableEntry)
					Expect(logger).To(gbytes.Say("applying-change-to-table"))
				})

				It("does not update routing configuration", func() {
					sameBackendServerInfo := createBackendServerInfo("some-ip", 1234, modificationTag)
					updated := routingTable.UpsertBackendServerKey(routingKey, sameBackendServerInfo)
					Expect(updated).To(BeFalse())
				})
			})

			Context("and a new backend is provided", func() {
				It("updates the routing entry's backends", func() {
					anotherModificationTag := routing_api_models.ModificationTag{Guid: "def", Index: 0}
					differentBackendServerInfo := createBackendServerInfo("some-other-ip", 1234, anotherModificationTag)
					expectedRoutingTableEntry := models.NewRoutingTableEntry([]models.BackendServerInfo{backendServerInfo, differentBackendServerInfo})
					updated := routingTable.UpsertBackendServerKey(routingKey, differentBackendServerInfo)
					Expect(updated).To(BeTrue())
					testutil.RoutingTableEntryMatches(routingTable.Get(routingKey), expectedRoutingTableEntry)
					actualDetails := routingTable.Get(routingKey).Backends[models.BackendServerKey{Address: "some-other-ip", Port: 1234}]
					expectedDetails := expectedRoutingTableEntry.Backends[models.BackendServerKey{Address: "some-other-ip", Port: 1234}]
					Expect(actualDetails.UpdatedTime.After(expectedDetails.UpdatedTime)).To(BeTrue())
				})
			})

			Context("when current entry is fresher than incoming entry", func() {

				var existingRoutingTableEntry models.RoutingTableEntry

				BeforeEach(func() {
					existingRoutingTableEntry = models.NewRoutingTableEntry([]models.BackendServerInfo{createBackendServerInfo("some-ip", 1234, modificationTag)})
					modificationTag.Index--
				})

				It("should not update routing table", func() {
					newBackendServerInfo := createBackendServerInfo("some-ip", 1234, modificationTag)
					updated := routingTable.UpsertBackendServerKey(routingKey, newBackendServerInfo)
					Expect(updated).To(BeFalse())
					Expect(logger).To(gbytes.Say("skipping-stale-event"))
					testutil.RoutingTableEntryMatches(routingTable.Get(routingKey), existingRoutingTableEntry)
				})
			})
		})
	})

	Describe("DeleteBackendServerKey", func() {
		var (
			routingKey                models.RoutingKey
			existingRoutingTableEntry models.RoutingTableEntry
			backendServerInfo1        models.BackendServerInfo
			backendServerInfo2        models.BackendServerInfo
		)
		BeforeEach(func() {
			routingKey = models.RoutingKey{Port: 12}
			backendServerInfo1 = createBackendServerInfo("some-ip", 1234, modificationTag)
		})

		Context("when the routing key does not exist", func() {
			It("it does not causes any changes or errors", func() {
				updated := routingTable.DeleteBackendServerKey(routingKey, backendServerInfo1)
				Expect(updated).To(BeFalse())
			})
		})

		Context("when the routing key does exist", func() {
			BeforeEach(func() {
				backendServerInfo2 = createBackendServerInfo("some-other-ip", 1235, modificationTag)
				existingRoutingTableEntry = models.NewRoutingTableEntry([]models.BackendServerInfo{backendServerInfo1, backendServerInfo2})
				updated := routingTable.Set(routingKey, existingRoutingTableEntry)
				Expect(updated).To(BeTrue())
			})

			Context("and the backend does not exist ", func() {
				It("does not causes any changes or errors", func() {
					backendServerInfo1 = createBackendServerInfo("some-missing-ip", 1236, modificationTag)
					ok := routingTable.DeleteBackendServerKey(routingKey, backendServerInfo1)
					Expect(ok).To(BeFalse())
					Expect(routingTable.Get(routingKey)).Should(Equal(existingRoutingTableEntry))
				})
			})

			Context("and the backend does exist", func() {
				It("deletes the backend", func() {
					updated := routingTable.DeleteBackendServerKey(routingKey, backendServerInfo1)
					Expect(updated).To(BeTrue())
					Expect(logger).To(gbytes.Say("removing-from-table"))
					expectedRoutingTableEntry := models.NewRoutingTableEntry([]models.BackendServerInfo{backendServerInfo2})
					testutil.RoutingTableEntryMatches(routingTable.Get(routingKey), expectedRoutingTableEntry)
				})

				Context("when a modification tag has the same guid but current index is greater", func() {
					BeforeEach(func() {
						backendServerInfo1.ModificationTag.Index--
					})

					It("does not deletes the backend", func() {
						updated := routingTable.DeleteBackendServerKey(routingKey, backendServerInfo1)
						Expect(updated).To(BeFalse())
						Expect(logger).To(gbytes.Say("skipping-stale-event"))
						Expect(routingTable.Get(routingKey)).Should(Equal(existingRoutingTableEntry))
					})
				})

				Context("when a modification tag has different guid", func() {
					var expectedRoutingTableEntry models.RoutingTableEntry

					BeforeEach(func() {
						expectedRoutingTableEntry = models.NewRoutingTableEntry([]models.BackendServerInfo{backendServerInfo2})
						backendServerInfo1.ModificationTag = routing_api_models.ModificationTag{Guid: "def"}
					})

					It("deletes the backend", func() {
						updated := routingTable.DeleteBackendServerKey(routingKey, backendServerInfo1)
						Expect(updated).To(BeTrue())
						Expect(logger).To(gbytes.Say("removing-from-table"))
						testutil.RoutingTableEntryMatches(routingTable.Get(routingKey), expectedRoutingTableEntry)
					})
				})

				Context("when there are no more backends left", func() {
					BeforeEach(func() {
						updated := routingTable.DeleteBackendServerKey(routingKey, backendServerInfo1)
						Expect(updated).To(BeTrue())
					})

					It("deletes the entry", func() {
						updated := routingTable.DeleteBackendServerKey(routingKey, backendServerInfo2)
						Expect(updated).To(BeTrue())
						Expect(routingTable.Size()).Should(Equal(0))
					})
				})
			})
		})
	})

	Describe("PruneEntries", func() {
		var (
			defaultTTL  int
			routingKey1 models.RoutingKey
			routingKey2 models.RoutingKey
		)
		BeforeEach(func() {
			routingKey1 = models.RoutingKey{Port: 12}
			backendServerKey := models.BackendServerKey{Address: "some-ip-1", Port: 1234}
			backendServerDetails := models.BackendServerDetails{ModificationTag: modificationTag, UpdatedTime: time.Now().Add(-10 * time.Second)}
			backendServerKey2 := models.BackendServerKey{Address: "some-ip-2", Port: 1235}
			backendServerDetails2 := models.BackendServerDetails{ModificationTag: modificationTag, UpdatedTime: time.Now().Add(-3 * time.Second)}
			backends := map[models.BackendServerKey]models.BackendServerDetails{
				backendServerKey:  backendServerDetails,
				backendServerKey2: backendServerDetails2,
			}
			routingTableEntry := models.RoutingTableEntry{Backends: backends}
			updated := routingTable.Set(routingKey1, routingTableEntry)
			Expect(updated).To(BeTrue())

			routingKey2 = models.RoutingKey{Port: 13}
			backendServerKey = models.BackendServerKey{Address: "some-ip-3", Port: 1234}
			backendServerDetails = models.BackendServerDetails{ModificationTag: modificationTag, UpdatedTime: time.Now().Add(-10 * time.Second)}
			backendServerKey2 = models.BackendServerKey{Address: "some-ip-4", Port: 1235}
			backendServerDetails2 = models.BackendServerDetails{ModificationTag: modificationTag, UpdatedTime: time.Now()}
			backends = map[models.BackendServerKey]models.BackendServerDetails{
				backendServerKey:  backendServerDetails,
				backendServerKey2: backendServerDetails2,
			}
			routingTableEntry = models.RoutingTableEntry{Backends: backends}
			updated = routingTable.Set(routingKey2, routingTableEntry)
			Expect(updated).To(BeTrue())
		})

		JustBeforeEach(func() {
			routingTable.PruneEntries(defaultTTL)
		})

		Context("when it has expired entries", func() {
			BeforeEach(func() {
				defaultTTL = 5
			})

			It("prunes the expired entries", func() {
				Expect(routingTable.Entries).To(HaveLen(2))
				Expect(routingTable.Get(routingKey1).Backends).To(HaveLen(1))
				Expect(routingTable.Get(routingKey2).Backends).To(HaveLen(1))
			})

			Context("when all the backends expire for given routing key", func() {
				BeforeEach(func() {
					defaultTTL = 2
				})

				It("prunes the expired entries and deletes the routing key", func() {
					Expect(routingTable.Entries).To(HaveLen(1))
					Expect(routingTable.Get(routingKey2).Backends).To(HaveLen(1))
				})
			})
		})

		Context("when it has no expired entries", func() {
			BeforeEach(func() {
				defaultTTL = 20
			})

			It("does not prune entries", func() {
				Expect(routingTable.Entries).To(HaveLen(2))
				Expect(routingTable.Get(routingKey1).Backends).To(HaveLen(2))
				Expect(routingTable.Get(routingKey2).Backends).To(HaveLen(2))
			})
		})
	})

	Describe("BackendServerDetails", func() {
		var (
			now        = time.Now()
			defaultTTL = 20
		)

		Context("when backend details have TTL", func() {
			It("returns true if updated time is past expiration time", func() {
				backendDetails := models.BackendServerDetails{TTL: 1, UpdatedTime: now.Add(-2 * time.Second)}
				Expect(backendDetails.Expired(defaultTTL)).To(BeTrue())
			})

			It("returns false if updated time is not past expiration time", func() {
				backendDetails := models.BackendServerDetails{TTL: 1, UpdatedTime: now}
				Expect(backendDetails.Expired(defaultTTL)).To(BeFalse())
			})
		})

		Context("when backend details do not have TTL", func() {
			It("returns true if updated time is past expiration time", func() {
				backendDetails := models.BackendServerDetails{TTL: 0, UpdatedTime: now.Add(-25 * time.Second)}
				Expect(backendDetails.Expired(defaultTTL)).To(BeTrue())
			})

			It("returns false if updated time is not past expiration time", func() {
				backendDetails := models.BackendServerDetails{TTL: 0, UpdatedTime: now}
				Expect(backendDetails.Expired(defaultTTL)).To(BeFalse())
			})
		})
	})

	Describe("RoutingTableEntry", func() {
		var (
			routingTableEntry models.RoutingTableEntry
			defaultTTL        int
		)

		BeforeEach(func() {
			backendServerKey := models.BackendServerKey{Address: "some-ip-1", Port: 1234}
			backendServerDetails := models.BackendServerDetails{ModificationTag: modificationTag, UpdatedTime: time.Now().Add(-10 * time.Second)}
			backendServerKey2 := models.BackendServerKey{Address: "some-ip-2", Port: 1235}
			backendServerDetails2 := models.BackendServerDetails{ModificationTag: modificationTag, UpdatedTime: time.Now()}
			backends := map[models.BackendServerKey]models.BackendServerDetails{
				backendServerKey:  backendServerDetails,
				backendServerKey2: backendServerDetails2,
			}
			routingTableEntry = models.RoutingTableEntry{Backends: backends}
		})

		JustBeforeEach(func() {
			routingTableEntry.PruneBackends(defaultTTL)
		})

		Context("when it has expired backends", func() {
			BeforeEach(func() {
				defaultTTL = 5
			})

			It("prunes expired backends", func() {
				Expect(routingTableEntry.Backends).To(HaveLen(1))
			})
		})

		Context("when it does not have any expired backends", func() {
			BeforeEach(func() {
				defaultTTL = 15
			})

			It("prunes expired backends", func() {
				Expect(routingTableEntry.Backends).To(HaveLen(2))
			})
		})
	})
})

func createBackendServerInfo(address string, port uint16, tag routing_api_models.ModificationTag) models.BackendServerInfo {
	return models.BackendServerInfo{Address: address, Port: port, ModificationTag: tag}

}
