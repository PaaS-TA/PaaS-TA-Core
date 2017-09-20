package eachers_test

import (
	"sync"
	"time"

	. "github.com/apoydence/eachers"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Each", func() {

	var (
		expectedChannel chan int
	)

	BeforeEach(func() {
		expectedChannel = make(chan int, 100)
	})

	Describe("Expect", func() {
		Context("channel has 0-4", func() {

			BeforeEach(func() {
				for i := 0; i < 5; i++ {
					expectedChannel <- i
				}
			})

			It("returns false if it does not match", func() {
				Expect(expectedChannel).ToNot(Each(Equal, 0, 1, 2, 3, 4, 5))
			})

			It("returns true for matching values", func() {
				Expect(expectedChannel).To(Each(BeEquivalentTo, 0, uint(1), 2, 3, 4))
			})

		})
	})

	Describe("Eventually/Consistently", func() {
		var (
			wg sync.WaitGroup
		)
		BeforeEach(func() {
			wg.Add(1)
			go func() {
				defer wg.Done()
				for i := 0; i < 5; i++ {
					expectedChannel <- i
					time.Sleep(10 * time.Millisecond)
				}
			}()
		})

		AfterEach(func() {
			wg.Wait()
		})

		It("returns false if it does not match", func() {
			Consistently(expectedChannel).ShouldNot(Each(Equal, 1, 2, 3, 4, 5))
		})

		It("returns true for matching values", func() {
			Eventually(expectedChannel).Should(Each(Equal, 0, 1, 2, 3, 4))
		})
	})
})
