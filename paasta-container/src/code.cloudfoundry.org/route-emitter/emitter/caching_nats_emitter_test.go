package emitter_test

import (
	"errors"

	"code.cloudfoundry.org/lager/lagertest"
	"code.cloudfoundry.org/route-emitter/emitter"
	"code.cloudfoundry.org/route-emitter/emitter/fakes"
	"code.cloudfoundry.org/route-emitter/routingtable"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
)

var _ = Describe("NatsEmitter", func() {
	var (
		natsEmitter    *fakes.FakeNATSEmitter
		cachingEmitter *emitter.CachingNATSEmitter
		logger         *lagertest.TestLogger
	)

	messagesToEmit := routingtable.MessagesToEmit{
		RegistrationMessages: []routingtable.RegistryMessage{
			{URIs: []string{"foo.com", "bar.com"}, Host: "1.1.1.1", Port: 11},
			{URIs: []string{"baz.com"}, Host: "2.2.2.2", Port: 22},
		},
		UnregistrationMessages: []routingtable.RegistryMessage{
			{URIs: []string{"wibble.com"}, Host: "1.1.1.1", Port: 11},
			{URIs: []string{"baz.com"}, Host: "3.3.3.3", Port: 33},
		},
	}

	BeforeEach(func() {
		natsEmitter = &fakes.FakeNATSEmitter{}
		cachingEmitter = emitter.NewCachingNATSEmitter(natsEmitter)
		logger = lagertest.NewTestLogger("caching-nats-emitter")
	})

	Describe("Emit", func() {
		It("does not emit messages", func() {
			err := cachingEmitter.Emit(logger, messagesToEmit)
			Expect(err).NotTo(HaveOccurred())
			Expect(natsEmitter.EmitCallCount()).To(Equal(0))
		})

		It("caches messages", func() {
			err := cachingEmitter.Emit(logger, messagesToEmit)
			Expect(err).NotTo(HaveOccurred())
			msgs := cachingEmitter.Cache()
			Expect(msgs).To(Equal(messagesToEmit))
		})

		It("logs the caching event", func() {
			err := cachingEmitter.Emit(logger, messagesToEmit)
			Expect(err).NotTo(HaveOccurred())

			Expect(logger).To(gbytes.Say("caching-nats-events"))
		})
	})

	Describe("EmitCached", func() {
		BeforeEach(func() {
			err := cachingEmitter.Emit(logger, messagesToEmit)
			Expect(err).NotTo(HaveOccurred())
		})

		It("emits any cached messages", func() {
			err := cachingEmitter.EmitCached()
			Expect(err).NotTo(HaveOccurred())

			Expect(natsEmitter.EmitCallCount()).To(Equal(1))
			Expect(natsEmitter.EmitArgsForCall(0)).To(Equal(messagesToEmit))
		})

		It("removes emitted messages from its cache", func() {
			err := cachingEmitter.EmitCached()
			Expect(err).NotTo(HaveOccurred())

			Expect(cachingEmitter.Cache()).To(Equal(routingtable.MessagesToEmit{}))
		})

		Context("when nats emitter errors", func() {
			BeforeEach(func() {
				natsEmitter.EmitReturns(errors.New("blam"))
			})

			It("returns an error", func() {
				err := cachingEmitter.EmitCached()
				Expect(err).To(HaveOccurred())
			})

			It("removes cached events", func() {
				err := cachingEmitter.EmitCached()
				Expect(err).To(HaveOccurred())
				Expect(cachingEmitter.Cache()).To(Equal(routingtable.MessagesToEmit{}))
			})
		})
	})
})
