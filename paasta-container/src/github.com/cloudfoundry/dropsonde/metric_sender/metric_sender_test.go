package metric_sender_test

import (
	"errors"
	"time"

	"github.com/cloudfoundry/dropsonde/emitter/fake"
	"github.com/cloudfoundry/dropsonde/metric_sender"
	"github.com/cloudfoundry/sonde-go/events"
	"github.com/gogo/protobuf/proto"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("MetricSender", func() {
	var (
		emitter *fake.FakeEventEmitter
		sender  *metric_sender.MetricSender
	)

	BeforeEach(func() {
		emitter = fake.NewFakeEventEmitter("test-origin")
		sender = metric_sender.NewMetricSender(emitter)
	})

	Describe("Value", func() {
		It("sets the required properties", func() {
			err := sender.Value("foo", 1.2, "bar").Send()
			Expect(err).ToNot(HaveOccurred())

			Expect(emitter.GetEnvelopes()).To(HaveLen(1))
			metric := emitter.GetEnvelopes()[0].ValueMetric

			Expect(metric.GetName()).To(Equal("foo"))
			Expect(metric.GetValue()).To(Equal(1.2))
			Expect(metric.GetUnit()).To(Equal("bar"))
		})

		It("can send tags", func() {
			err := sender.Value("foo", 1.2, "bar").SetTag("baz", "qux").Send()
			Expect(err).ToNot(HaveOccurred())

			Expect(emitter.GetEnvelopes()).To(HaveLen(1))
			envelope := emitter.GetEnvelopes()[0]

			Expect(envelope.GetTags()).To(HaveKeyWithValue("baz", "qux"))
		})

		It("sets origin", func() {
			err := sender.Value("foo", 1.2, "bar").Send()
			Expect(err).ToNot(HaveOccurred())

			Expect(emitter.GetEnvelopes()).To(HaveLen(1))
			envelope := emitter.GetEnvelopes()[0]

			Expect(envelope.GetOrigin()).To(Equal("test-origin"))
		})

		It("sets the timestamp", func() {
			err := sender.Value("foo", 1.2, "bar").Send()
			now := time.Now().UnixNano()
			Expect(err).ToNot(HaveOccurred())

			Expect(emitter.GetEnvelopes()).To(HaveLen(1))
			envelope := emitter.GetEnvelopes()[0]

			Expect(envelope.GetTimestamp()).To(BeNumerically("~", now, time.Second))
		})
	})

	Describe("ContainerMetric", func() {
		It("sets the required properties", func() {
			err := sender.ContainerMetric("test-app-id", 1234, 1.2, 2345, 3456).
				Send()
			Expect(err).ToNot(HaveOccurred())

			Expect(emitter.GetEnvelopes()).To(HaveLen(1))
			metric := emitter.GetEnvelopes()[0].ContainerMetric

			Expect(metric.GetApplicationId()).To(Equal("test-app-id"))
			Expect(metric.GetInstanceIndex()).To(BeEquivalentTo(1234))
			Expect(metric.GetCpuPercentage()).To(Equal(1.2))
			Expect(metric.GetMemoryBytes()).To(BeEquivalentTo(2345))
			Expect(metric.GetDiskBytes()).To(BeEquivalentTo(3456))
		})

		It("can send tags", func() {
			err := sender.ContainerMetric("test-app-id", 1234, 1.2, 2345, 3456).
				SetTag("baz", "qux").
				Send()
			Expect(err).ToNot(HaveOccurred())

			Expect(emitter.GetEnvelopes()).To(HaveLen(1))
			envelope := emitter.GetEnvelopes()[0]

			Expect(envelope.GetTags()).To(HaveKeyWithValue("baz", "qux"))
		})

		It("sets origin", func() {
			err := sender.ContainerMetric("test-app-id", 1234, 1.2, 2345, 3456).
				Send()
			Expect(err).ToNot(HaveOccurred())

			Expect(emitter.GetEnvelopes()).To(HaveLen(1))
			envelope := emitter.GetEnvelopes()[0]

			Expect(envelope.GetOrigin()).To(Equal("test-origin"))
		})

		It("sets the timestamp", func() {
			err := sender.ContainerMetric("test-app-id", 1234, 1.2, 2345, 3456).
				Send()
			now := time.Now().UnixNano()
			Expect(err).ToNot(HaveOccurred())

			Expect(emitter.GetEnvelopes()).To(HaveLen(1))
			envelope := emitter.GetEnvelopes()[0]

			Expect(envelope.GetTimestamp()).To(BeNumerically("~", now, time.Second))
		})
	})

	Describe("Counter", func() {
		It("sets the required properties", func() {
			err := sender.Counter("requests").Increment()
			Expect(err).ToNot(HaveOccurred())
			err = sender.Counter("requests").Add(3)
			Expect(err).ToNot(HaveOccurred())

			Expect(emitter.GetEnvelopes()).To(HaveLen(2))

			counter := emitter.GetEnvelopes()[0].CounterEvent
			Expect(counter.GetName()).To(Equal("requests"))
			Expect(counter.GetDelta()).To(BeEquivalentTo(1))

			counter = emitter.GetEnvelopes()[1].CounterEvent
			Expect(counter.GetName()).To(Equal("requests"))
			Expect(counter.GetDelta()).To(BeEquivalentTo(3))
		})

		It("can send tags", func() {
			err := sender.Counter("requests").
				SetTag("baz", "qux").
				Increment()
			Expect(err).ToNot(HaveOccurred())

			Expect(emitter.GetEnvelopes()).To(HaveLen(1))
			envelope := emitter.GetEnvelopes()[0]

			Expect(envelope.GetTags()).To(HaveKeyWithValue("baz", "qux"))
		})

		It("sets origin", func() {
			err := sender.Counter("requests").Increment()
			Expect(err).ToNot(HaveOccurred())

			Expect(emitter.GetEnvelopes()).To(HaveLen(1))
			envelope := emitter.GetEnvelopes()[0]

			Expect(envelope.GetOrigin()).To(Equal("test-origin"))
		})

		It("sets the timestamp", func() {
			err := sender.Counter("requests").Increment()
			now := time.Now().UnixNano()
			Expect(err).ToNot(HaveOccurred())

			Expect(emitter.GetEnvelopes()).To(HaveLen(1))
			envelope := emitter.GetEnvelopes()[0]

			Expect(envelope.GetTimestamp()).To(BeNumerically("~", now, time.Second))
		})
	})

	It("sends an event to its emitter", func() {
		err := sender.Send(&events.ValueMetric{
			Name:  proto.String("metric-name"),
			Value: proto.Float64(42),
			Unit:  proto.String("answers"),
		})
		Expect(err).NotTo(HaveOccurred())

		Expect(emitter.GetMessages()).To(HaveLen(1))
		metric := emitter.GetMessages()[0].Event.(*events.ValueMetric)
		Expect(metric.GetName()).To(Equal("metric-name"))
		Expect(metric.GetValue()).To(BeNumerically("==", 42))
		Expect(metric.GetUnit()).To(Equal("answers"))
	})

	It("errors out when sending if the emitter errors", func() {
		emitter.ReturnError = errors.New("some error")

		err := sender.Send(nil)
		Expect(emitter.GetMessages()).To(HaveLen(0))
		Expect(err.Error()).To(Equal("some error"))
	})

	It("sends a value metric to its emitter", func() {
		err := sender.SendValue("metric-name", 42, "answers")
		Expect(err).NotTo(HaveOccurred())

		Expect(emitter.GetMessages()).To(HaveLen(1))
		metric := emitter.GetMessages()[0].Event.(*events.ValueMetric)
		Expect(metric.GetName()).To(Equal("metric-name"))
		Expect(metric.GetValue()).To(BeNumerically("==", 42))
		Expect(metric.GetUnit()).To(Equal("answers"))
	})

	It("returns an error if it can't send value metric", func() {
		emitter.ReturnError = errors.New("some error")

		err := sender.SendValue("stuff", 12, "no answer")
		Expect(emitter.GetMessages()).To(HaveLen(0))
		Expect(err.Error()).To(Equal("some error"))
	})

	It("sends an update counter event to its emitter", func() {
		err := sender.IncrementCounter("counter-strike")
		Expect(err).NotTo(HaveOccurred())

		Expect(emitter.GetMessages()).To(HaveLen(1))
		counterEvent := emitter.GetMessages()[0].Event.(*events.CounterEvent)
		Expect(counterEvent.GetName()).To(Equal("counter-strike"))
		Expect(counterEvent.GetDelta()).To(Equal(uint64(1)))
	})

	It("sends an update counter event with arbitrary increment", func() {
		err := sender.AddToCounter("counter-strike", 3)
		Expect(err).NotTo(HaveOccurred())

		Expect(emitter.GetMessages()).To(HaveLen(1))
		counterEvent := emitter.GetMessages()[0].Event.(*events.CounterEvent)
		Expect(counterEvent.GetName()).To(Equal("counter-strike"))
		Expect(counterEvent.GetDelta()).To(Equal(uint64(3)))
	})

	It("returns an error if it can't increment counter", func() {
		emitter.ReturnError = errors.New("some counter event error")

		err := sender.IncrementCounter("count me in")
		Expect(emitter.GetMessages()).To(HaveLen(0))
		Expect(err.Error()).To(Equal("some counter event error"))
	})

	It("sends a container metric to its emitter", func() {
		err := sender.SendContainerMetric("some_app_guid", 0, 42.42, 1234, 123412341234)
		Expect(err).NotTo(HaveOccurred())

		Expect(emitter.GetMessages()).To(HaveLen(1))
		containerMetric := emitter.GetMessages()[0].Event.(*events.ContainerMetric)

		Expect(containerMetric.GetApplicationId()).To(Equal("some_app_guid"))
		Expect(containerMetric.GetInstanceIndex()).To(Equal(int32(0)))

		Expect(containerMetric.GetCpuPercentage()).To(BeNumerically("~", 42.42, 0.005))
		Expect(containerMetric.GetMemoryBytes()).To(Equal(uint64(1234)))
		Expect(containerMetric.GetDiskBytes()).To(Equal(uint64(123412341234)))
	})

	It("returns an error if it can't send a container metric", func() {

		emitter.ReturnError = errors.New("some container metric error")

		err := sender.SendContainerMetric("some_app_guid", 0, 42.42, 1234, 123412341234)
		Expect(emitter.GetMessages()).To(HaveLen(0))
		Expect(err.Error()).To(Equal("some container metric error"))
	})
})
