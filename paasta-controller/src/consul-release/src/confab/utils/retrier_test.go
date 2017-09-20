package utils_test

import (
	"errors"
	"time"

	"github.com/cloudfoundry-incubator/consul-release/src/confab/fakes"
	"github.com/cloudfoundry-incubator/consul-release/src/confab/utils"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("TryUntil", func() {
	var (
		clock      *fakes.Clock
		retrier    utils.Retrier
		retryDelay time.Duration
		timeout    utils.Timeout
	)
	BeforeEach(func() {
		clock = &fakes.Clock{}
		retryDelay = 0
		timeout = utils.NewTimeout(make(chan time.Time))
		retrier = utils.NewRetrier(clock, retryDelay)
	})

	It("retries till the function is succesful within given timeout", func() {
		callCount := 0
		errorProneFunction := func() error {
			callCount++
			if callCount < 10 {
				return errors.New("some error occurred")
			}
			return nil
		}

		err := retrier.TryUntil(timeout, errorProneFunction)
		Expect(err).NotTo(HaveOccurred())

		Expect(callCount).To(Equal(10))
		Expect(clock.SleepCall.CallCount).To(Equal(9))
	})

	Context("failure cases", func() {
		It("returns an error if the function doesn't succeed", func() {
			timeout := utils.NewTimeout(time.After(1 * time.Millisecond))

			callCount := 0
			errorProneFunction := func() error {
				callCount++
				return errors.New("some error occurred")
			}

			err := retrier.TryUntil(timeout, errorProneFunction)
			Expect(err).To(MatchError(`timeout exceeded: "some error occurred"`))
			Expect(callCount).NotTo(Equal(0))
		})
	})
})
