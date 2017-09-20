package consuldownchecker_test

import (
	"errors"
	"os"
	"time"

	"code.cloudfoundry.org/clock/fakeclock"
	"code.cloudfoundry.org/consuladapter/fakes"
	"code.cloudfoundry.org/lager/lagertest"
	"code.cloudfoundry.org/route-emitter/consuldownchecker"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
)

var _ = Describe("ConsulDownChecker", func() {
	var (
		logger        *lagertest.TestLogger
		clock         *fakeclock.FakeClock
		consulClient  *fakes.FakeClient
		statusClient  *fakes.FakeStatus
		retryInterval time.Duration
		signals       chan os.Signal
		ready         chan struct{}
		runErrCh      chan error

		consulDownChecker *consuldownchecker.ConsulDownChecker
	)

	BeforeEach(func() {
		clock = fakeclock.NewFakeClock(time.Now())
		logger = lagertest.NewTestLogger("test")
		retryInterval = 100 * time.Millisecond
		consulClient = new(fakes.FakeClient)
		statusClient = new(fakes.FakeStatus)
		consulClient.StatusReturns(statusClient)
		signals = make(chan os.Signal)
		ready = make(chan struct{})
		runErrCh = make(chan error)

		consulDownChecker = consuldownchecker.NewConsulDownChecker(logger, clock, consulClient, retryInterval)
	})

	JustBeforeEach(func() {
		go func() {
			defer GinkgoRecover()
			runErrCh <- consulDownChecker.Run(signals, ready)
		}()
	})

	Context("when consul has a leader", func() {
		BeforeEach(func() {
			statusClient.LeaderReturns("Pompeius", nil)
		})

		It("exits after checking 3 times", func() {
			clock.WaitForWatcherAndIncrement(retryInterval)
			clock.WaitForWatcherAndIncrement(retryInterval)
			var err error
			Eventually(runErrCh, 5).Should(Receive(&err))
			Expect(err).NotTo(HaveOccurred())
			Expect(ready).NotTo(BeClosed())
			Eventually(statusClient.LeaderCallCount).Should(Equal(3))
		})
	})

	Context("when consul agent is unreachable", func() {
		BeforeEach(func() {
			statusClient.LeaderReturns("", errors.New("not a five hundred"))
		})

		It("exits quickly with error", func() {
			var err error
			Eventually(runErrCh, 5).Should(Receive(&err))
			Expect(err).To(HaveOccurred())
			Expect(ready).NotTo(BeClosed())
		})
	})

	Context("when consul does not have leader", func() {
		BeforeEach(func() {
			statusClient.LeaderReturns("", errors.New("Unexpected response code: 500 (rpc error: No cluster leader)"))
		})

		JustBeforeEach(func() {
			clock.WaitForWatcherAndIncrement(retryInterval)
			clock.WaitForWatcherAndIncrement(retryInterval)
			Eventually(ready).Should(BeClosed())
		})

		It("continuously checks for the leader", func() {
			Eventually(logger).Should(gbytes.Say("still-down"))
			clock.WaitForWatcherAndIncrement(retryInterval)
			Eventually(logger).Should(gbytes.Say("still-down"))
		})

		It("exits gracefully when interrupted", func() {
			signals <- os.Interrupt
			Eventually(logger).Should(gbytes.Say("received-signal"))
			var err error
			Eventually(runErrCh).Should(Receive(&err))
			Expect(err).NotTo(HaveOccurred())
		})

		It("exits when there is a leader", func() {
			statusClient.LeaderReturns("Ceasar", nil)
			clock.WaitForWatcherAndIncrement(retryInterval)
			Eventually(logger).Should(gbytes.Say("consul-has-leader"))
			clock.WaitForWatcherAndIncrement(retryInterval)
			Eventually(logger).Should(gbytes.Say("consul-has-leader"))
			clock.WaitForWatcherAndIncrement(retryInterval)
			Eventually(logger).Should(gbytes.Say("consul-has-leader"))
			var err error
			Eventually(runErrCh).Should(Receive(&err))
			Expect(err).NotTo(HaveOccurred())
		})

		It("keeps check if consul is flapping", func() {
			Eventually(logger).Should(gbytes.Say("still-down"))
			statusClient.LeaderReturns("Ceasar", nil)

			clock.WaitForWatcherAndIncrement(retryInterval)
			Eventually(logger).Should(gbytes.Say("consul-has-leader"))

			statusClient.LeaderReturns("", nil)
			clock.WaitForWatcherAndIncrement(retryInterval)
			Eventually(logger).Should(gbytes.Say("still-down"))

			statusClient.LeaderReturns("Ceasar", nil)
			clock.WaitForWatcherAndIncrement(retryInterval)
			Eventually(logger).Should(gbytes.Say("consul-has-leader"))
			clock.WaitForWatcherAndIncrement(retryInterval)
			Eventually(logger).Should(gbytes.Say("consul-has-leader"))
			clock.WaitForWatcherAndIncrement(retryInterval)
			Eventually(logger).Should(gbytes.Say("consul-has-leader"))
			var err error
			Eventually(runErrCh).Should(Receive(&err))
			Expect(err).NotTo(HaveOccurred())
		})

		It("exits with error when consul agent is unreachable", func() {
			statusClient.LeaderReturns("", errors.New("not a five hundred"))
			clock.WaitForWatcherAndIncrement(retryInterval)
			Eventually(logger).Should(gbytes.Say("failed-getting-leader"))
			var err error
			Eventually(runErrCh).Should(Receive(&err))
			Expect(err).To(HaveOccurred())
		})
	})
})
