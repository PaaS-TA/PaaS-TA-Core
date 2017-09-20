package utils_test

import (
	"time"

	"github.com/cloudfoundry-incubator/consul-release/src/confab/utils"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Timeout", func() {
	Describe("Done", func() {
		It("closes the channel when the timer finishes", func() {
			timer := make(chan time.Time)
			timeout := utils.NewTimeout(timer)

			Expect(timeout.Done()).NotTo(BeClosed())

			timer <- time.Now()

			Eventually(timeout.Done).Should(BeClosed())
		})
	})
})
