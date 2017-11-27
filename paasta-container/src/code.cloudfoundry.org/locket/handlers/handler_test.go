package handlers_test

import (
	"context"
	"errors"

	"code.cloudfoundry.org/bbs/db/sqldb/helpers"
	"code.cloudfoundry.org/lager/lagertest"
	"code.cloudfoundry.org/locket/db"
	"code.cloudfoundry.org/locket/db/dbfakes"
	"code.cloudfoundry.org/locket/expiration/expirationfakes"
	"code.cloudfoundry.org/locket/handlers"
	"code.cloudfoundry.org/locket/models"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
)

var _ = Describe("Lock", func() {
	var (
		fakeLockDB    *dbfakes.FakeLockDB
		fakeLockPick  *expirationfakes.FakeLockPick
		logger        *lagertest.TestLogger
		locketHandler models.LocketServer
		resource      *models.Resource
		exitCh        chan struct{}
	)

	BeforeEach(func() {
		fakeLockDB = &dbfakes.FakeLockDB{}
		fakeLockPick = &expirationfakes.FakeLockPick{}
		logger = lagertest.NewTestLogger("locket-handler")
		exitCh = make(chan struct{}, 1)

		resource = &models.Resource{
			Key:   "test",
			Value: "test-value",
			Owner: "myself",
			Type:  "lock",
		}

		locketHandler = handlers.NewLocketHandler(logger, fakeLockDB, fakeLockPick, exitCh)
	})

	Context("Lock", func() {
		var (
			request      *models.LockRequest
			expectedLock *db.Lock
		)

		BeforeEach(func() {
			request = &models.LockRequest{
				Resource:     resource,
				TtlInSeconds: 10,
			}

			expectedLock = &db.Lock{
				Resource:      resource,
				TtlInSeconds:  10,
				ModifiedIndex: 2,
			}

			fakeLockDB.LockReturns(expectedLock, nil)
		})

		It("reserves the lock in the database", func() {
			_, err := locketHandler.Lock(context.Background(), request)
			Expect(err).NotTo(HaveOccurred())

			Expect(fakeLockDB.LockCallCount()).Should(Equal(1))
			_, actualResource, ttl := fakeLockDB.LockArgsForCall(0)
			Expect(actualResource).To(Equal(resource))
			Expect(ttl).To(BeEquivalentTo(10))
		})

		It("registers the lock and ttl with the lock pick", func() {
			_, err := locketHandler.Lock(context.Background(), request)
			Expect(err).NotTo(HaveOccurred())

			Expect(fakeLockPick.RegisterTTLCallCount()).To(Equal(1))
			_, lock := fakeLockPick.RegisterTTLArgsForCall(0)
			Expect(lock).To(Equal(expectedLock))
		})

		Context("validate lock type", func() {
			Context("when type string is set", func() {
				It("should be invalid with type not set to presence/lock", func() {
					request.Resource.Type = "random"
					_, err := locketHandler.Lock(context.Background(), request)
					Expect(err).To(HaveOccurred())
				})

				It("should be valid with type set to presence", func() {
					request.Resource.Type = "presence"
					_, err := locketHandler.Lock(context.Background(), request)
					Expect(err).NotTo(HaveOccurred())
				})

				It("should be valid with type set to lock", func() {
					request.Resource.Type = "lock"
					_, err := locketHandler.Lock(context.Background(), request)
					Expect(err).NotTo(HaveOccurred())
				})
			})

			Context("when type_code is set", func() {
				It("should ba invalid when mismatching non-empty type", func() {
					request.Resource.Type = "lock"
					request.Resource.TypeCode = models.PRESENCE
					_, err := locketHandler.Lock(context.Background(), request)
					Expect(err).To(HaveOccurred())

					request.Resource.Type = "presence"
					request.Resource.TypeCode = models.LOCK
					_, err = locketHandler.Lock(context.Background(), request)
					Expect(err).To(HaveOccurred())
				})

				It("should be valid when type and type code match", func() {
					request.Resource.Type = "lock"
					request.Resource.TypeCode = models.LOCK
					_, err := locketHandler.Lock(context.Background(), request)
					Expect(err).NotTo(HaveOccurred())

					request.Resource.Type = "presence"
					request.Resource.TypeCode = models.PRESENCE
					_, err = locketHandler.Lock(context.Background(), request)
					Expect(err).NotTo(HaveOccurred())
				})

				It("should be valid on a valid type code and empty type", func() {
					request.Resource.Type = ""
					request.Resource.TypeCode = models.LOCK
					_, err := locketHandler.Lock(context.Background(), request)
					Expect(err).NotTo(HaveOccurred())
				})

				It("should be invalid on an UNKNOWN type code and empty type", func() {
					request.Resource.Type = ""
					request.Resource.TypeCode = models.UNKNOWN
					_, err := locketHandler.Lock(context.Background(), request)
					Expect(err).To(HaveOccurred())
				})

				It("should be invalid on an non-existent type code", func() {
					request.Resource.Type = ""
					request.Resource.TypeCode = 4
					_, err := locketHandler.Lock(context.Background(), request)
					Expect(err).To(HaveOccurred())
				})
			})
		})

		Context("when request does not have TTL", func() {
			BeforeEach(func() {
				request = &models.LockRequest{
					Resource: resource,
				}
			})

			It("returns a validation error", func() {
				_, err := locketHandler.Lock(context.Background(), request)
				Expect(err).To(HaveOccurred())
				Expect(err).To(Equal(models.ErrInvalidTTL))
				Expect(logger).To(gbytes.Say(models.ErrInvalidTTL.Error()))
				Expect(logger).To(gbytes.Say("\"key\":"))
				Expect(logger).To(gbytes.Say("\"owner\":"))
			})
		})

		Context("when the request does not have an owner", func() {
			BeforeEach(func() {
				resource.Owner = ""
			})

			It("returns a validation error", func() {
				_, err := locketHandler.Lock(context.Background(), request)
				Expect(err).To(HaveOccurred())
				Expect(err).To(Equal(models.ErrInvalidOwner))
				Expect(logger).To(gbytes.Say(models.ErrInvalidOwner.Error()))
				Expect(logger).To(gbytes.Say("\"key\":"))
				Expect(logger).To(gbytes.Say("\"owner\":"))
			})
		})

		Context("when locking errors", func() {
			var (
				err error
			)

			BeforeEach(func() {
				fakeLockDB.LockReturns(nil, errors.New("Boom."))
			})

			JustBeforeEach(func() {
				_, err = locketHandler.Lock(context.Background(), request)
			})

			It("returns the error", func() {
				Expect(err).To(HaveOccurred())
			})

			It("logs the error with identifying information", func() {
				Expect(logger).To(gbytes.Say("Boom."))
				Expect(logger).To(gbytes.Say("\"key\":"))
				Expect(logger).To(gbytes.Say("\"owner\":"))
			})

			Context("when lock collision error occurs", func() {
				BeforeEach(func() {
					fakeLockDB.LockReturns(nil, models.ErrLockCollision)
				})

				It("does not log the error", func() {
					Expect(logger).NotTo(gbytes.Say("lock-collision"))
				})
			})
		})

		Context("when an unrecoverable error is returned", func() {
			BeforeEach(func() {
				fakeLockDB.LockReturns(nil, helpers.ErrUnrecoverableError)
			})

			It("logs and writes to the exit channel", func() {
				locketHandler.Lock(context.Background(), request)
				Expect(logger).To(gbytes.Say("unrecoverable-error"))
				Expect(exitCh).To(Receive())
			})
		})
	})

	Context("Release", func() {
		It("releases the lock in the database", func() {
			_, err := locketHandler.Release(context.Background(), &models.ReleaseRequest{Resource: resource})
			Expect(err).NotTo(HaveOccurred())

			Expect(fakeLockDB.ReleaseCallCount()).Should(Equal(1))
			_, actualResource := fakeLockDB.ReleaseArgsForCall(0)
			Expect(actualResource).To(Equal(resource))
		})

		Context("when releasing errors", func() {
			BeforeEach(func() {
				fakeLockDB.ReleaseReturns(errors.New("Boom."))
			})

			It("returns the error", func() {
				_, err := locketHandler.Release(context.Background(), &models.ReleaseRequest{Resource: resource})
				Expect(err).To(HaveOccurred())
			})
		})

		Context("when an unrecoverable error is returned", func() {
			BeforeEach(func() {
				fakeLockDB.ReleaseReturns(helpers.ErrUnrecoverableError)
			})

			It("logs and writes to the exit channel", func() {
				locketHandler.Release(context.Background(), &models.ReleaseRequest{Resource: resource})
				Expect(logger).To(gbytes.Say("unrecoverable-error"))
				Expect(exitCh).To(Receive())
			})
		})
	})

	Context("Fetch", func() {
		BeforeEach(func() {
			fakeLockDB.FetchReturns(&db.Lock{Resource: resource}, nil)
		})

		It("fetches the lock in the database", func() {
			fetchResp, err := locketHandler.Fetch(context.Background(), &models.FetchRequest{Key: "test-fetch"})
			Expect(err).NotTo(HaveOccurred())
			Expect(fetchResp.Resource).To(Equal(resource))

			Expect(fakeLockDB.FetchCallCount()).Should(Equal(1))
			_, key := fakeLockDB.FetchArgsForCall(0)
			Expect(key).To(Equal("test-fetch"))
		})

		Context("when fetching errors", func() {
			BeforeEach(func() {
				fakeLockDB.FetchReturns(nil, errors.New("boom"))
			})

			It("returns the error", func() {
				_, err := locketHandler.Fetch(context.Background(), &models.FetchRequest{Key: "test-fetch"})
				Expect(err).To(HaveOccurred())
			})
		})

		Context("when an unrecoverable error is returned", func() {
			BeforeEach(func() {
				fakeLockDB.FetchReturns(nil, helpers.ErrUnrecoverableError)
			})

			It("logs and writes to the exit channel", func() {
				locketHandler.Fetch(context.Background(), &models.FetchRequest{Key: "test-fetch"})
				Expect(logger).To(gbytes.Say("unrecoverable-error"))
				Expect(exitCh).To(Receive())
			})
		})
	})

	Context("FetchAll", func() {
		var expectedResources []*models.Resource
		BeforeEach(func() {
			expectedResources = []*models.Resource{
				resource,
				&models.Resource{Key: "cell", Owner: "cell-1", Value: "{}"},
			}

			var locks []*db.Lock
			for _, r := range expectedResources {
				locks = append(locks, &db.Lock{Resource: r})
			}
			fakeLockDB.FetchAllReturns(locks, nil)
		})

		Context("validate lock type", func() {
			Context("when type string is set and the type code is not set", func() {
				It("should be invalid with type not set to presence/lock", func() {
					_, err := locketHandler.FetchAll(context.Background(), &models.FetchAllRequest{Type: "random"})
					Expect(err).To(HaveOccurred())
				})

				It("should be valid with type set to presence", func() {
					_, err := locketHandler.FetchAll(context.Background(), &models.FetchAllRequest{Type: "presence"})
					Expect(err).NotTo(HaveOccurred())
				})

				It("should be valid with type set to lock", func() {
					_, err := locketHandler.FetchAll(context.Background(), &models.FetchAllRequest{Type: "lock"})
					Expect(err).NotTo(HaveOccurred())
				})
			})

			Context("when type_code is set", func() {
				It("should be invalid when mismatching a non-empty type", func() {
					_, err := locketHandler.FetchAll(context.Background(), &models.FetchAllRequest{Type: "lock", TypeCode: models.PRESENCE})
					Expect(err).To(HaveOccurred())

					_, err = locketHandler.FetchAll(context.Background(), &models.FetchAllRequest{Type: "presence", TypeCode: models.LOCK})
					Expect(err).To(HaveOccurred())
				})

				It("should be valid when type and type code match", func() {
					_, err := locketHandler.FetchAll(context.Background(), &models.FetchAllRequest{Type: "lock", TypeCode: models.LOCK})
					Expect(err).NotTo(HaveOccurred())

					_, err = locketHandler.FetchAll(context.Background(), &models.FetchAllRequest{Type: "presence", TypeCode: models.PRESENCE})
					Expect(err).NotTo(HaveOccurred())
				})

				It("should be valid on a valid type code and empty type", func() {
					_, err := locketHandler.FetchAll(context.Background(), &models.FetchAllRequest{TypeCode: models.LOCK})
					Expect(err).NotTo(HaveOccurred())

					_, err = locketHandler.FetchAll(context.Background(), &models.FetchAllRequest{TypeCode: models.PRESENCE})
					Expect(err).NotTo(HaveOccurred())
				})

				It("should be invalid on an UNKNOWN type code and empty type", func() {
					_, err := locketHandler.FetchAll(context.Background(), &models.FetchAllRequest{})
					Expect(err).To(HaveOccurred())
				})
			})
		})

		Context("when the type is valid", func() {
			It("fetches all the presence locks in the database by type", func() {
				fetchResp, err := locketHandler.FetchAll(context.Background(), &models.FetchAllRequest{Type: models.PresenceType})
				Expect(err).NotTo(HaveOccurred())
				Expect(fetchResp.Resources).To(Equal(expectedResources))
				Expect(fakeLockDB.FetchAllCallCount()).Should(Equal(1))
				_, lockType := fakeLockDB.FetchAllArgsForCall(0)
				Expect(lockType).To(Equal("presence"))
			})

			It("fetches all the lock locks in the database by type", func() {
				fetchResp, err := locketHandler.FetchAll(context.Background(), &models.FetchAllRequest{Type: models.LockType})
				Expect(err).NotTo(HaveOccurred())
				Expect(fetchResp.Resources).To(Equal(expectedResources))
				Expect(fakeLockDB.FetchAllCallCount()).Should(Equal(1))
				_, lockType := fakeLockDB.FetchAllArgsForCall(0)
				Expect(lockType).To(Equal("lock"))
			})

			It("fetches all the presence locks in the database by type code", func() {
				fetchResp, err := locketHandler.FetchAll(context.Background(), &models.FetchAllRequest{TypeCode: models.PRESENCE})
				Expect(err).NotTo(HaveOccurred())
				Expect(fetchResp.Resources).To(Equal(expectedResources))
				Expect(fakeLockDB.FetchAllCallCount()).Should(Equal(1))
				_, lockType := fakeLockDB.FetchAllArgsForCall(0)
				Expect(lockType).To(Equal("presence"))
			})

			It("fetches all the lock locks in the database by type code", func() {
				fetchResp, err := locketHandler.FetchAll(context.Background(), &models.FetchAllRequest{TypeCode: models.LOCK})
				Expect(err).NotTo(HaveOccurred())
				Expect(fetchResp.Resources).To(Equal(expectedResources))
				Expect(fakeLockDB.FetchAllCallCount()).Should(Equal(1))
				_, lockType := fakeLockDB.FetchAllArgsForCall(0)
				Expect(lockType).To(Equal("lock"))
			})
		})

		Context("when the type is invalid", func() {
			It("returns an invalid type error", func() {
				_, err := locketHandler.FetchAll(context.Background(), &models.FetchAllRequest{Type: "dawg"})
				Expect(err).To(HaveOccurred())
			})
		})

		Context("when the type code is UNKNOWN", func() {
			It("returns an invalid type error", func() {
				_, err := locketHandler.FetchAll(context.Background(), &models.FetchAllRequest{TypeCode: models.UNKNOWN})
				Expect(err).To(HaveOccurred())
			})
		})

		Context("when fetching errors", func() {
			BeforeEach(func() {
				fakeLockDB.FetchAllReturns(nil, errors.New("boom"))
			})

			It("returns the error", func() {
				_, err := locketHandler.FetchAll(context.Background(), &models.FetchAllRequest{})
				Expect(err).To(HaveOccurred())
			})
		})

		Context("when an unrecoverable error is returned", func() {
			BeforeEach(func() {
				fakeLockDB.FetchAllReturns(nil, helpers.ErrUnrecoverableError)
			})

			It("logs and writes to the exit channel", func() {
				locketHandler.FetchAll(context.Background(), &models.FetchAllRequest{Type: models.PresenceType})
				Expect(logger).To(gbytes.Say("unrecoverable-error"))
				Expect(exitCh).To(Receive())
			})
		})
	})
})
