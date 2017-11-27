package backoff_test

import (
	"code.cloudfoundry.org/clock/fakeclock"
	"code.cloudfoundry.org/voldriver/backoff"
	"context"
	"errors"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"sync"
)

var _ = Describe("Backoff", func() {

	var (
		err error

		now time.Time

		backerOffer backoff.ExponentialBackoff

		ctx       context.Context
		fakeClock *fakeclock.FakeClock

		op   func(context.Context) error
		done *sync.Mutex
	)

	JustBeforeEach(func() {
		backerOffer = backoff.NewExponentialBackOff(ctx, fakeClock)

		err = nil
		done = &sync.Mutex{}
		done.Lock()

		go func() {
			defer done.Unlock()
			err = backerOffer.Retry(op)
		}()
	})

	BeforeEach(func() {
		now = time.Now()
		fakeClock = fakeclock.NewFakeClock(now)

		ctx = context.TODO()

		op = func(context.Context) error {
			return nil
		}
	})

	It("should succeed", func() {
		done.Lock()
		defer done.Unlock()
		Expect(err).NotTo(HaveOccurred())
	})

	Context("when operation fails and backoff has expired", func() {
		BeforeEach(func() {
			op = func(context.Context) error {
				return errors.New("badness")
			}
		})

		JustBeforeEach(func() {
			fakeClock.WaitForWatcherAndIncrement(time.Second * 31)
			done.Lock()
			defer done.Unlock()
		})

		It("Retry should return an error", func() {
			Expect(err).To(HaveOccurred())
		})
	})

	Context("when operation fails", func() {
		var count int
		BeforeEach(func() {
			ctx, _ = context.WithDeadline(ctx, now.Add(30*time.Second))

			op = func(context.Context) error {
				count++
				return errors.New("badness")
			}

		})

		JustBeforeEach(func() {
			fakeClock.WaitForWatcherAndIncrement(time.Second * 15)
			fakeClock.WaitForWatcherAndIncrement(time.Second * 20)
			done.Lock()
			defer done.Unlock()
		})

		It("Retry should retry and eventually return an error", func() {
			Expect(err).To(HaveOccurred())
			Expect(count).To(Equal(3))
		})
	})

	Context("when operation fails and then operation is cancelled", func() {
		var (
			count     int
			canceller func()
		)
		BeforeEach(func() {
			ctx, canceller = context.WithDeadline(ctx, now.Add(30*time.Second))

			op = func(context.Context) error {
				count++
				canceller()
				return errors.New("badness")
			}

		})

		JustBeforeEach(func() {
			fakeClock.WaitForWatcherAndIncrement(time.Second * 15)
			done.Lock()
			defer done.Unlock()
		})

		It("Retry should not retry", func() {
			Expect(err).To(HaveOccurred())
			Expect(count).To(Equal(1))
			Expect(err).To(Equal(context.Canceled))
		})
	})

	Context("when operation fails and then succeeds", func() {
		var (
			count     int
			canceller func()
		)

		BeforeEach(func() {
			ctx, canceller = context.WithDeadline(ctx, now.Add(30*time.Second))

			op = func(context.Context) error {
				count++
				if count == 1 {
					return errors.New("badness")

				} else {
					return nil
				}
			}
		})

		JustBeforeEach(func() {
			fakeClock.WaitForWatcherAndIncrement(time.Second * 15)
			done.Lock()
			defer done.Unlock()
		})

		It("should succeed", func() {
			Expect(count).To(Equal(2))
			Expect(err).NotTo(HaveOccurred())
		})
	})

	Context("when a nil context is used", func() {
		var count int
		BeforeEach(func() {
			ctx = nil

			op = func(context.Context) error {
				count++
				return errors.New("badness")
			}
		})

		JustBeforeEach(func() {
			fakeClock.WaitForWatcherAndIncrement(time.Second * 15)
			fakeClock.WaitForWatcherAndIncrement(time.Second * 20)
			done.Lock()
			defer done.Unlock()
		})

		It("Retry should retry and eventually return an error", func() {
			Expect(err).To(HaveOccurred())
			Expect(count).To(Equal(3))
		})
	})
})
