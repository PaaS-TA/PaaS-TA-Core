package eventhub_test

import (
	"code.cloudfoundry.org/eventhub"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

type fakeEvent struct {
	Token int `json:"token"`
}

func (fakeEvent) EventType() string {
	return "fake"
}

var _ = Describe("Hub", func() {
	var (
		consumerBufferSize int

		hub eventhub.Hub
	)

	BeforeEach(func() {
		consumerBufferSize = 5

		hub = eventhub.NewNonBlocking(consumerBufferSize)
	})

	It("fans-out events emitted to it to all subscribers", func() {
		source1, err := hub.Subscribe()
		Expect(err).NotTo(HaveOccurred())
		source2, err := hub.Subscribe()
		Expect(err).NotTo(HaveOccurred())

		hub.Emit(fakeEvent{Token: 1})
		Expect(source1.Next()).To(Equal(fakeEvent{Token: 1}))
		Expect(source2.Next()).To(Equal(fakeEvent{Token: 1}))

		hub.Emit(fakeEvent{Token: 2})
		Expect(source1.Next()).To(Equal(fakeEvent{Token: 2}))
		Expect(source2.Next()).To(Equal(fakeEvent{Token: 2}))
	})

	It("closes slow consumers after N missed events", func() {
		slowConsumer, err := hub.Subscribe()
		Expect(err).NotTo(HaveOccurred())

		By("filling the 'buffer'")
		for eventToken := 0; eventToken < consumerBufferSize; eventToken++ {
			hub.Emit(fakeEvent{Token: eventToken})
		}

		By("reading 2 events off")
		ev, err := slowConsumer.Next()
		Expect(err).NotTo(HaveOccurred())
		Expect(ev).To(Equal(fakeEvent{Token: 0}))

		ev, err = slowConsumer.Next()
		Expect(err).NotTo(HaveOccurred())
		Expect(ev).To(Equal(fakeEvent{Token: 1}))

		By("putting 3 more events on, 'overflowing the buffer' and making the consumer 'slow'")
		for eventToken := consumerBufferSize; eventToken < consumerBufferSize+3; eventToken++ {
			hub.Emit(fakeEvent{Token: eventToken})
		}

		By("reading off all the 'buffered' events")
		for eventToken := 2; eventToken < consumerBufferSize+2; eventToken++ {
			ev, err = slowConsumer.Next()
			Expect(err).NotTo(HaveOccurred())
			Expect(ev).To(Equal(fakeEvent{Token: eventToken}))
		}

		By("trying to read more out of the source")
		_, err = slowConsumer.Next()
		Expect(err).To(Equal(eventhub.ErrReadFromClosedSource))
	})

	Describe("closing an event source", func() {
		It("prevents current events from propagating to the source", func() {
			source, err := hub.Subscribe()
			Expect(err).NotTo(HaveOccurred())

			hub.Emit(fakeEvent{Token: 1})
			Expect(source.Next()).To(Equal(fakeEvent{Token: 1}))

			err = source.Close()
			Expect(err).NotTo(HaveOccurred())

			_, err = source.Next()
			Expect(err).To(Equal(eventhub.ErrReadFromClosedSource))
		})

		It("prevents future events from propagating to the source", func() {
			source, err := hub.Subscribe()
			Expect(err).NotTo(HaveOccurred())

			err = source.Close()
			Expect(err).NotTo(HaveOccurred())

			hub.Emit(fakeEvent{Token: 1})

			_, err = source.Next()
			Expect(err).To(Equal(eventhub.ErrReadFromClosedSource))
		})

		Context("when the source is already closed", func() {
			It("errors", func() {
				source, err := hub.Subscribe()
				Expect(err).NotTo(HaveOccurred())

				err = source.Close()
				Expect(err).NotTo(HaveOccurred())

				err = source.Close()
				Expect(err).To(Equal(eventhub.ErrSourceAlreadyClosed))
			})
		})
	})

	Describe("closing the hub", func() {
		It("all subscribers receive errors", func() {
			source, err := hub.Subscribe()
			Expect(err).NotTo(HaveOccurred())

			err = hub.Close()
			Expect(err).NotTo(HaveOccurred())

			_, err = source.Next()
			Expect(err).To(Equal(eventhub.ErrReadFromClosedSource))
		})

		It("does not accept new subscribers", func() {
			err := hub.Close()
			Expect(err).NotTo(HaveOccurred())

			_, err = hub.Subscribe()
			Expect(err).To(Equal(eventhub.ErrSubscribedToClosedHub))
		})

		Context("when the hub is already closed", func() {
			BeforeEach(func() {
				err := hub.Close()
				Expect(err).NotTo(HaveOccurred())
			})

			It("errors", func() {
				err := hub.Close()
				Expect(err).To(Equal(eventhub.ErrHubAlreadyClosed))
			})
		})
	})
})
