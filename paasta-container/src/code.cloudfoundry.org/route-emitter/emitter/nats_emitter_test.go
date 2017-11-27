package emitter_test

import (
	"errors"

	"code.cloudfoundry.org/lager/lagertest"
	"code.cloudfoundry.org/route-emitter/diegonats"
	"code.cloudfoundry.org/route-emitter/emitter"
	"code.cloudfoundry.org/route-emitter/routingtable"
	"code.cloudfoundry.org/workpool"
	fake_metrics_sender "github.com/cloudfoundry/dropsonde/metric_sender/fake"
	"github.com/cloudfoundry/dropsonde/metrics"
	"github.com/nats-io/nats"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("NatsEmitter", func() {
	var natsEmitter emitter.NATSEmitter
	var natsClient *diegonats.FakeNATSClient
	var fakeMetricSender *fake_metrics_sender.FakeMetricSender

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
		natsClient = diegonats.NewFakeClient()
		logger := lagertest.NewTestLogger("test")
		workPool, err := workpool.NewWorkPool(1)
		Expect(err).NotTo(HaveOccurred())
		natsEmitter = emitter.NewNATSEmitter(natsClient, workPool, logger)
		fakeMetricSender = fake_metrics_sender.NewFakeMetricSender()
		metrics.Initialize(fakeMetricSender, nil)
	})

	Describe("Emitting", func() {
		It("should emit register and unregister messages", func() {
			err := natsEmitter.Emit(messagesToEmit)
			Expect(err).NotTo(HaveOccurred())

			Expect(natsClient.PublishedMessages("router.register")).To(HaveLen(2))
			Expect(natsClient.PublishedMessages("router.unregister")).To(HaveLen(2))

			registeredPayloads := [][]byte{
				natsClient.PublishedMessages("router.register")[0].Data,
				natsClient.PublishedMessages("router.register")[1].Data,
			}

			unregisteredPayloads := [][]byte{
				natsClient.PublishedMessages("router.unregister")[0].Data,
				natsClient.PublishedMessages("router.unregister")[1].Data,
			}

			Expect(registeredPayloads).To(ContainElement(MatchJSON(`
        {
          "uris":["foo.com", "bar.com"],
          "host":"1.1.1.1",
          "port":11
        }
      `)))

			Expect(registeredPayloads).To(ContainElement(MatchJSON(`
        {
          "uris":["baz.com"],
          "host":"2.2.2.2",
          "port":22
        }
      `)))

			Expect(unregisteredPayloads).To(ContainElement(MatchJSON(`
        {
          "uris":["wibble.com"],
          "host":"1.1.1.1",
          "port":11
        }
      `)))

			Expect(unregisteredPayloads).To(ContainElement(MatchJSON(`
        {
          "uris":["baz.com"],
          "host":"3.3.3.3",
          "port":33
        }
      `)))

			Expect(fakeMetricSender.GetCounter("MessagesEmitted")).To(BeEquivalentTo(4))
		})

		Context("when the nats client errors", func() {
			BeforeEach(func() {
				natsClient.WhenPublishing("router.register", func(*nats.Msg) error {
					return errors.New("bam")
				})
			})

			It("should error", func() {
				Expect(natsEmitter.Emit(messagesToEmit)).To(MatchError(errors.New("bam")))
			})
		})
	})
})
