package expiration_test

import (
	"errors"
	"sync/atomic"
	"time"

	"code.cloudfoundry.org/clock/fakeclock"
	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/lager/lagertest"
	"code.cloudfoundry.org/locket/db"
	"code.cloudfoundry.org/locket/db/dbfakes"
	"code.cloudfoundry.org/locket/expiration"
	"code.cloudfoundry.org/locket/models"

	"github.com/cloudfoundry/dropsonde/metric_sender/fake"
	"github.com/cloudfoundry/dropsonde/metrics"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
)

var _ = Describe("LockPick", func() {
	var (
		lockPick expiration.LockPick

		logger     *lagertest.TestLogger
		fakeLockDB *dbfakes.FakeLockDB
		fakeClock  *fakeclock.FakeClock

		ttl time.Duration

		sender         *fake.FakeMetricSender
		lock, presence *db.Lock
	)

	BeforeEach(func() {
		lock = &db.Lock{
			Resource: &models.Resource{
				Key:   "funky",
				Owner: "town",
				Value: "won't you take me to",
				Type:  models.LockType,
			},
			TtlInSeconds:  25,
			ModifiedIndex: 6,
			ModifiedId:    "guid",
		}

		presence = &db.Lock{
			Resource: &models.Resource{
				Key:   "funky-presence",
				Owner: "town-presence",
				Value: "please dont take me",
				Type:  models.PresenceType,
			},
			TtlInSeconds:  25,
			ModifiedIndex: 6,
			ModifiedId:    "guid",
		}

		ttl = time.Duration(lock.TtlInSeconds) * time.Second

		fakeClock = fakeclock.NewFakeClock(time.Now())
		logger = lagertest.NewTestLogger("lock-pick")
		fakeLockDB = &dbfakes.FakeLockDB{}

		sender = fake.NewFakeMetricSender()
		metrics.Initialize(sender, nil)

		lockPick = expiration.NewLockPick(fakeLockDB, fakeClock)
	})

	Context("RegisterTTL", func() {
		BeforeEach(func() {
			fakeLockDB.FetchReturns(lock, nil)
		})

		It("checks that the lock expires after the ttl", func() {
			lockPick.RegisterTTL(logger, lock)

			fakeClock.WaitForWatcherAndIncrement(ttl)

			Eventually(fakeLockDB.FetchCallCount).Should(Equal(1))
			_, key := fakeLockDB.FetchArgsForCall(0)
			Expect(key).To(Equal(lock.Key))

			Eventually(fakeLockDB.ReleaseCallCount).Should(Equal(1))
			_, resource := fakeLockDB.ReleaseArgsForCall(0)
			Expect(resource).To(Equal(lock.Resource))
		})

		It("emits a counter metric for lock expiration", func() {
			lockPick.RegisterTTL(logger, lock)
			lockPick.RegisterTTL(logger, presence)

			fakeClock.WaitForNWatchersAndIncrement(ttl, 2)

			Eventually(func() uint64 {
				return sender.GetCounter("LocksExpired")
			}).Should(BeEquivalentTo(1))
		})

		It("emits a counter metric for presence expiration", func() {
			lockPick.RegisterTTL(logger, lock)
			lockPick.RegisterTTL(logger, presence)

			fakeClock.WaitForNWatchersAndIncrement(ttl, 2)

			Eventually(func() uint64 {
				return sender.GetCounter("PresenceExpired")
			}).Should(BeEquivalentTo(1))
		})

		It("logs the type of the lock", func() {
			lockPick.RegisterTTL(logger, lock)
			Eventually(logger.Buffer()).Should(gbytes.Say("\"type\":\"lock\""))
		})

		It("logs the type of the presence", func() {
			lockPick.RegisterTTL(logger, presence)
			Eventually(logger.Buffer()).Should(gbytes.Say("\"type\":\"presence\""))
		})

		Context("when the modified index has been incremented", func() {
			var returnedLock *db.Lock
			BeforeEach(func() {
				returnedLock = &db.Lock{
					Resource: &models.Resource{
						Key:   "funky",
						Owner: "town",
						Value: "won't you take me to",
					},
					TtlInSeconds:  25,
					ModifiedIndex: 7,
				}

				fakeLockDB.FetchReturns(returnedLock, nil)
			})

			It("does not release the lock", func() {
				lockPick.RegisterTTL(logger, lock)

				fakeClock.WaitForWatcherAndIncrement(ttl)

				Eventually(fakeLockDB.FetchCallCount).Should(Equal(1))
				_, key := fakeLockDB.FetchArgsForCall(0)
				Expect(key).To(Equal(lock.Key))

				Consistently(fakeLockDB.ReleaseCallCount).Should(Equal(0))
			})
		})

		Context("when the modified id has been changed", func() {
			var returnedLock *db.Lock
			BeforeEach(func() {
				returnedLock = &db.Lock{
					Resource: &models.Resource{
						Key:   "funky",
						Owner: "town",
						Value: "won't you take me to",
					},
					TtlInSeconds:  25,
					ModifiedIndex: 6,
					ModifiedId:    "new-guid",
				}

				fakeLockDB.FetchReturns(returnedLock, nil)
			})

			It("does not release the lock", func() {
				lockPick.RegisterTTL(logger, lock)

				fakeClock.WaitForWatcherAndIncrement(ttl)

				Eventually(fakeLockDB.FetchCallCount).Should(Equal(1))
				_, key := fakeLockDB.FetchArgsForCall(0)
				Expect(key).To(Equal(lock.Key))

				Consistently(fakeLockDB.ReleaseCallCount).Should(Equal(0))
			})
		})

		Context("when fetching the lock fails", func() {
			BeforeEach(func() {
				fakeLockDB.FetchReturns(nil, errors.New("failed-to-fetch-lock"))
			})

			It("does not release the lock", func() {
				lockPick.RegisterTTL(logger, lock)

				fakeClock.WaitForWatcherAndIncrement(ttl)

				Eventually(fakeLockDB.FetchCallCount).Should(Equal(1))
				Consistently(fakeLockDB.ReleaseCallCount).Should(Equal(0))
			})
		})

		Context("when releasing the lock fails", func() {
			BeforeEach(func() {
				fakeLockDB.ReleaseReturns(errors.New("failed-to-release-lock"))
			})

			It("logs the error", func() {
				lockPick.RegisterTTL(logger, lock)

				fakeClock.WaitForWatcherAndIncrement(ttl)

				Eventually(fakeLockDB.ReleaseCallCount).Should(Equal(1))
			})
		})

		Context("when there is already a check process running", func() {
			BeforeEach(func() {
				lockPick.RegisterTTL(logger, lock)
				Eventually(fakeClock.WatcherCount).Should(Equal(1))
			})

			Context("and the lock id is the same", func() {
				Context("and the lock index is incremented", func() {
					var returnedLock *db.Lock
					BeforeEach(func() {
						returnedLock = &db.Lock{
							Resource: &models.Resource{
								Key:   "funky",
								Owner: "town",
								Value: "won't you take me to",
							},
							TtlInSeconds:  lock.TtlInSeconds,
							ModifiedIndex: 7,
							ModifiedId:    "guid",
						}

						fakeLockDB.FetchReturns(returnedLock, nil)
					})

					It("cancels the existing check and adds a new one", func() {
						lockPick.RegisterTTL(logger, returnedLock)

						Eventually(fakeClock.WatcherCount).Should(Equal(2))
						Consistently(fakeClock.WatcherCount).Should(Equal(2))
						fakeClock.WaitForWatcherAndIncrement(ttl)

						Eventually(logger).Should(gbytes.Say("cancelling-old-check"))

						Eventually(fakeLockDB.FetchCallCount).Should(Equal(1))
						_, key := fakeLockDB.FetchArgsForCall(0)
						Expect(key).To(Equal(returnedLock.Key))

						Eventually(fakeLockDB.ReleaseCallCount).Should(Equal(1))
						Consistently(fakeLockDB.ReleaseCallCount).Should(Equal(1))
					})
				})

				Context("and competes with a newer lock on checking expiry", func() {
					var thirdLock db.Lock
					var trigger uint32

					BeforeEach(func() {
						newLock := *lock
						newLock.ModifiedIndex += 1

						thirdLock = newLock
						thirdLock.ModifiedIndex += 1

						trigger = 1
						fakeLockDB.FetchStub = func(logger lager.Logger, key string) (*db.Lock, error) {
							if atomic.LoadUint32(&trigger) != 0 {
								// second expiry goroutine
								lockPick.RegisterTTL(logger, &newLock)
							}
							atomic.StoreUint32(&trigger, 0)

							return &thirdLock, nil
						}
					})

					It("checks the expiration of the lock", func() {
						// first expiry goroutine proceeds into timer case statement
						fakeClock.WaitForWatcherAndIncrement(ttl)
						Eventually(fakeLockDB.FetchCallCount).Should(Equal(1))
						Eventually(func() uint32 {
							return atomic.LoadUint32(&trigger)
						}).Should(BeEquivalentTo(0))

						// third expiry goroutine, cancels the second expiry goroutine
						lockPick.RegisterTTL(logger, &thirdLock)

						Eventually(fakeClock.WatcherCount).Should(Equal(2))
						fakeClock.WaitForWatcherAndIncrement(ttl)

						Eventually(fakeLockDB.FetchCallCount).Should(Equal(2))
						Consistently(fakeLockDB.FetchCallCount).Should(Equal(2))

						Eventually(fakeLockDB.ReleaseCallCount).Should(Equal(1))
						_, resource := fakeLockDB.ReleaseArgsForCall(0)
						Expect(resource).To(Equal(thirdLock.Resource))
					})
				})

				Context("when registering same lock", func() {
					It("does nothing", func() {
						lockPick.RegisterTTL(logger, lock)
						Eventually(logger).Should(gbytes.Say("found-expiration-goroutine"))
					})
				})

				Context("when registering an older lock", func() {
					var oldLock db.Lock

					BeforeEach(func() {
						oldLock = *lock
						oldLock.ModifiedIndex -= 1
					})

					It("does nothing", func() {
						l := oldLock
						lockPick.RegisterTTL(logger, &l)
						Eventually(logger).Should(gbytes.Say("found-expiration-goroutine"))
					})

					Context("and the previous lock has already expired", func() {
						BeforeEach(func() {
							fakeClock.WaitForWatcherAndIncrement(ttl)
							Eventually(fakeLockDB.ReleaseCallCount).Should(Equal(1))
						})

						It("checks the expiration of the lock", func() {
							l := oldLock
							lockPick.RegisterTTL(logger, &l)
							Eventually(fakeClock.WatcherCount).Should(Equal(1))
							fakeClock.WaitForWatcherAndIncrement(ttl)

							Eventually(fakeLockDB.FetchCallCount).Should(Equal(2))
							_, key := fakeLockDB.FetchArgsForCall(0)
							Expect(key).To(Equal(l.Key))
						})
					})
				})
			})

			Context("when the same lock is registered with a different id", func() {
				var newLock db.Lock

				BeforeEach(func() {
					newLock = *lock
					newLock.ModifiedId = "new-guid"
					fakeLockDB.FetchReturns(&newLock, nil)
				})

				It("does not effect the other check goroutines", func() {
					lockPick.RegisterTTL(logger, &newLock)

					Eventually(fakeClock.WatcherCount).Should(Equal(2))
					Consistently(fakeClock.WatcherCount).Should(Equal(2))

					fakeClock.WaitForWatcherAndIncrement(ttl)

					Eventually(fakeLockDB.FetchCallCount).Should(Equal(2))
					_, key1 := fakeLockDB.FetchArgsForCall(0)
					_, key2 := fakeLockDB.FetchArgsForCall(1)
					Expect(key1).To(Equal(newLock.Key))
					Expect(key2).To(Equal(newLock.Key))

					Eventually(fakeLockDB.ReleaseCallCount).Should(Equal(1))
					Consistently(fakeLockDB.ReleaseCallCount).Should(Equal(1))
				})
			})

			Context("when another lock is registered", func() {
				var anotherLock, newLock db.Lock
				BeforeEach(func() {
					anotherLock = db.Lock{
						Resource: &models.Resource{
							Key:   "another",
							Owner: "myself",
							Value: "hi",
						},
						TtlInSeconds:  lock.TtlInSeconds,
						ModifiedIndex: 9,
					}

					newLock = *lock
					newLock.ModifiedIndex += 1

					fakeLockDB.FetchStub = func(logger lager.Logger, key string) (*db.Lock, error) {
						switch {
						case key == newLock.Key:
							return &newLock, nil
						case key == anotherLock.Key:
							return &anotherLock, nil
						default:
							return nil, errors.New("unknown lock")
						}
					}
				})

				It("does not effect the other check goroutines", func() {
					lockPick.RegisterTTL(logger, &anotherLock)

					Eventually(fakeClock.WatcherCount).Should(Equal(2))
					Consistently(fakeClock.WatcherCount).Should(Equal(2))

					lockPick.RegisterTTL(logger, &newLock)

					Eventually(fakeClock.WatcherCount).Should(Equal(3))
					fakeClock.WaitForWatcherAndIncrement(ttl)

					Eventually(fakeLockDB.FetchCallCount).Should(Equal(2))
					_, key1 := fakeLockDB.FetchArgsForCall(0)
					_, key2 := fakeLockDB.FetchArgsForCall(1)
					Expect([]string{key1, key2}).To(ContainElement(newLock.Key))
					Expect([]string{key1, key2}).To(ContainElement(anotherLock.Key))

					Eventually(fakeLockDB.ReleaseCallCount).Should(Equal(2))
					Consistently(fakeLockDB.ReleaseCallCount).Should(Equal(2))
				})
			})

			Context("and the check process finishes", func() {
				BeforeEach(func() {
					fakeClock.WaitForWatcherAndIncrement(ttl)

					Eventually(fakeLockDB.FetchCallCount).Should(Equal(1))
					_, key := fakeLockDB.FetchArgsForCall(0)
					Expect(key).To(Equal(lock.Key))

					Eventually(fakeLockDB.ReleaseCallCount).Should(Equal(1))
					Consistently(fakeLockDB.ReleaseCallCount).Should(Equal(1))
				})

				It("performs the expiration check", func() {
					lockPick.RegisterTTL(logger, lock)

					Eventually(fakeClock.WatcherCount).Should(Equal(1))
					fakeClock.WaitForWatcherAndIncrement(ttl)

					Eventually(fakeLockDB.FetchCallCount).Should(Equal(2))
					_, key := fakeLockDB.FetchArgsForCall(0)
					Expect(key).To(Equal(lock.Key))

					Eventually(fakeLockDB.ReleaseCallCount).Should(Equal(2))
					Consistently(fakeLockDB.ReleaseCallCount).Should(Equal(2))
				})
			})
		})
	})
})
