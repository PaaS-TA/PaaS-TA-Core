package loggregator_test

import (
	"errors"
	"time"

	"code.cloudfoundry.org/go-loggregator"
	"code.cloudfoundry.org/go-loggregator/rpc/loggregator_v2"
	"code.cloudfoundry.org/go-loggregator/runtimeemitter"
	"code.cloudfoundry.org/go-loggregator/testhelpers"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
)

var _ = Describe("IngressClient", func() {
	var (
		client    *loggregator.IngressClient
		receivers chan loggregator_v2.Ingress_BatchSenderServer
		server    *testhelpers.TestIngressServer
	)

	BeforeEach(func() {
		var err error
		server, err = testhelpers.NewTestIngressServer(fixture("metron.crt"), fixture("metron.key"), fixture("CA.crt"))
		Expect(err).NotTo(HaveOccurred())

		err = server.Start()
		Expect(err).NotTo(HaveOccurred())

		receivers = server.Receivers()

		tlsConfig, err := loggregator.NewTLSConfig(
			fixture("CA.crt"),
			fixture("client.crt"),
			fixture("client.key"),
		)
		Expect(err).NotTo(HaveOccurred())

		client, err = loggregator.NewIngressClient(
			tlsConfig,
			loggregator.WithAddr(server.Addr()),
			loggregator.WithBatchFlushInterval(50*time.Millisecond),
			loggregator.WithStringTag("string", "client-string-tag"),
			loggregator.WithDecimalTag("decimal", 1.234),
			loggregator.WithIntegerTag("integer", 42),
		)
	})

	AfterEach(func() {
		server.Stop()
	})

	It("sends in batches", func() {
		for i := 0; i < 10; i++ {
			client.EmitLog(
				"message",
				loggregator.WithAppInfo("app-id", "source-type", "source-instance"),
				loggregator.WithStdout(),
			)
			time.Sleep(10 * time.Millisecond)
		}

		Eventually(func() int {
			b, err := getBatch(receivers)
			if err != nil {
				return 0
			}

			return len(b.Batch)
		}).Should(BeNumerically(">", 1))
	})

	It("sends app logs", func() {
		client.EmitLog(
			"message",
			loggregator.WithAppInfo("app-id", "source-type", "source-instance"),
			loggregator.WithStdout(),
		)
		env, err := getEnvelopeAt(receivers, 0)
		Expect(err).NotTo(HaveOccurred())

		Expect(env.Tags["source_instance"].GetText()).To(Equal("source-instance"))
		Expect(env.SourceId).To(Equal("app-id"))
		Expect(env.InstanceId).To(Equal("source-instance"))

		ts := time.Unix(0, env.Timestamp)
		Expect(ts).Should(BeTemporally("~", time.Now(), time.Second))
		log := env.GetLog()
		Expect(log).NotTo(BeNil())
		Expect(log.Payload).To(Equal([]byte("message")))
		Expect(log.Type).To(Equal(loggregator_v2.Log_OUT))
	})

	It("sends app error logs", func() {
		client.EmitLog(
			"message",
			loggregator.WithAppInfo("app-id", "source-type", "source-instance"),
		)

		env, err := getEnvelopeAt(receivers, 0)
		Expect(err).NotTo(HaveOccurred())

		Expect(env.Tags["source_instance"].GetText()).To(Equal("source-instance"))
		Expect(env.SourceId).To(Equal("app-id"))
		Expect(env.InstanceId).To(Equal("source-instance"))

		ts := time.Unix(0, env.Timestamp)
		Expect(ts).Should(BeTemporally("~", time.Now(), time.Second))
		log := env.GetLog()
		Expect(log).NotTo(BeNil())
		Expect(log.Payload).To(Equal([]byte("message")))
		Expect(log.Type).To(Equal(loggregator_v2.Log_ERR))
	})

	It("sends app metrics", func() {
		client.EmitGauge(
			loggregator.WithGaugeValue("name-a", 1, "unit-a"),
			loggregator.WithGaugeValue("name-b", 2, "unit-b"),
			loggregator.WithGaugeTags(map[string]string{"some-tag": "some-tag-value"}),
			loggregator.WithGaugeAppInfo("app-id"),
		)

		env, err := getEnvelopeAt(receivers, 0)
		Expect(err).NotTo(HaveOccurred())

		ts := time.Unix(0, env.Timestamp)
		Expect(ts).Should(BeTemporally("~", time.Now(), time.Second))
		metrics := env.GetGauge()
		Expect(metrics).NotTo(BeNil())
		Expect(env.SourceId).To(Equal("app-id"))
		Expect(metrics.GetMetrics()).To(HaveLen(2))
		Expect(metrics.GetMetrics()["name-a"].Value).To(Equal(1.0))
		Expect(metrics.GetMetrics()["name-b"].Value).To(Equal(2.0))
		Expect(env.Tags["some-tag"].GetText()).To(Equal("some-tag-value"))
	})

	It("reconnects when the server goes away and comes back", func() {
		client.EmitLog(
			"message",
			loggregator.WithAppInfo("app-id", "source-type", "source-instance"),
		)

		envBatch, err := getBatch(receivers)
		Expect(err).NotTo(HaveOccurred())
		Expect(envBatch.Batch).To(HaveLen(1))

		server.Stop()
		Eventually(server.Start).Should(Succeed())

		Consistently(receivers).Should(BeEmpty())

		for i := 0; i < 200; i++ {
			client.EmitLog(
				"message",
				loggregator.WithAppInfo("app-id", "source-type", "source-instance"),
			)
		}

		envBatch, err = getBatch(receivers)
		Expect(err).NotTo(HaveOccurred())
		Expect(envBatch.Batch).ToNot(BeEmpty())
	})

	It("works with the runtime emitter", func() {
		// This test is to ensure that the v2 client satisfies the
		// runtimeemitter.Sender interface. If it does not satisfy the
		// runtimeemitter.Sender interface this test will force a compile time
		// error.
		runtimeemitter.New(client)
	})

	DescribeTable("tagging", func(emit func()) {
		emit()

		env, err := getEnvelopeAt(receivers, 0)
		Expect(err).NotTo(HaveOccurred())

		Expect(env.Tags["string"].GetText()).To(Equal("client-string-tag"), "The client tag for string was not set properly")
		Expect(env.Tags["decimal"].GetDecimal()).To(Equal(1.234), "The client tag for decimal was not set properly")
		Expect(env.Tags["integer"].GetInteger()).To(Equal(int64(42)), "The client tag for integer was not set properly")

		Expect(env.Tags["envelope-string"].GetText()).To(Equal("envelope-string-tag"), "The envelope tag for string was not set properly")
		Expect(env.Tags["envelope-decimal"].GetDecimal()).To(Equal(1.234), "The envelope tag for decimal was not set properly")
		Expect(env.Tags["envelope-integer"].GetInteger()).To(Equal(int64(42)), "The envelope tag for integer was not set properly")
	},
		Entry("logs", func() {
			client.EmitLog(
				"message",
				loggregator.WithEnvelopeStringTag("envelope-string", "envelope-string-tag"),
				loggregator.WithEnvelopeDecimalTag("envelope-decimal", 1.234),
				loggregator.WithEnvelopeIntegerTag("envelope-integer", 42),
			)
		}),
		Entry("gauge", func() {
			client.EmitGauge(
				loggregator.WithGaugeValue("gauge-name", 123.4, "some-unit"),
				loggregator.WithEnvelopeStringTag("envelope-string", "envelope-string-tag"),
				loggregator.WithEnvelopeDecimalTag("envelope-decimal", 1.234),
				loggregator.WithEnvelopeIntegerTag("envelope-integer", 42),
			)
		}),
		Entry("counter", func() {
			client.EmitCounter(
				"foo",
				loggregator.WithEnvelopeStringTag("envelope-string", "envelope-string-tag"),
				loggregator.WithEnvelopeDecimalTag("envelope-decimal", 1.234),
				loggregator.WithEnvelopeIntegerTag("envelope-integer", 42),
			)
		}),
	)

	Describe("With functions", func() {
		Describe("WithDelta()", func() {
			It("sets the counter's delta to the given value", func() {
				e := &loggregator_v2.Envelope{
					Message: &loggregator_v2.Envelope_Counter{
						Counter: &loggregator_v2.Counter{},
					},
				}
				loggregator.WithDelta(99)(e)
				Expect(e.GetCounter().GetDelta()).To(Equal(uint64(99)))
			})
		})
	})
})

func getBatch(receivers chan loggregator_v2.Ingress_BatchSenderServer) (*loggregator_v2.EnvelopeBatch, error) {
	var recv loggregator_v2.Ingress_BatchSenderServer
	Eventually(receivers, 10).Should(Receive(&recv))

	return recv.Recv()
}

func getEnvelopeAt(receivers chan loggregator_v2.Ingress_BatchSenderServer, idx int) (*loggregator_v2.Envelope, error) {
	envBatch, err := getBatch(receivers)
	if err != nil {
		return nil, err
	}

	if len(envBatch.Batch) < 1 {
		return nil, errors.New("no envelopes")
	}

	return envBatch.Batch[idx], nil
}
