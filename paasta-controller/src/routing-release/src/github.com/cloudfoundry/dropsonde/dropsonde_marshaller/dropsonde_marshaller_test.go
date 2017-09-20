package dropsonde_marshaller_test

import (
	"time"

	"github.com/cloudfoundry/dropsonde/dropsonde_marshaller"
	"github.com/cloudfoundry/dropsonde/factories"
	"github.com/cloudfoundry/dropsonde/metric_sender/fake"
	"github.com/cloudfoundry/dropsonde/metricbatcher"
	"github.com/cloudfoundry/dropsonde/metrics"
	"github.com/cloudfoundry/loggregatorlib/loggertesthelper"
	"github.com/cloudfoundry/sonde-go/events"
	"github.com/gogo/protobuf/proto"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("DropsondeMarshaller", func() {
	var (
		inputChan   chan *events.Envelope
		outputChan  chan []byte
		runComplete chan struct{}
		marshaller  *dropsonde_marshaller.DropsondeMarshaller
		fakeSender  *fake.FakeMetricSender
	)

	BeforeEach(func() {
		inputChan = make(chan *events.Envelope, 100)
		outputChan = make(chan []byte, 10)
		runComplete = make(chan struct{})
		marshaller = dropsonde_marshaller.NewDropsondeMarshaller(loggertesthelper.Logger())
		fakeSender = fake.NewFakeMetricSender()
		batcher := metricbatcher.New(fakeSender, 200*time.Millisecond)
		metrics.Initialize(fakeSender, batcher)

		go func() {
			marshaller.Run(inputChan, outputChan)
			close(runComplete)
		}()
	})

	AfterEach(func() {
		close(inputChan)
		Eventually(runComplete).Should(BeClosed())
	})

	It("marshals envelopes into bytes", func() {
		envelope := &events.Envelope{
			Origin:     proto.String("fake-origin-1"),
			EventType:  events.Envelope_LogMessage.Enum(),
			LogMessage: factories.NewLogMessage(events.LogMessage_OUT, "message", "appid", "sourceType"),
		}
		message, _ := proto.Marshal(envelope)

		inputChan <- envelope
		outputMessage := <-outputChan
		Expect(outputMessage).To(Equal(message))
	})

	Context("metrics", func() {
		var eventuallyExpectCounter = func(name string, value uint64) {
			Eventually(func() uint64 { return fakeSender.GetCounter(name) }).Should(BeEquivalentTo(value))
		}

		It("emits a marshal error counter", func() {
			envelope := &events.Envelope{}

			inputChan <- envelope
			eventuallyExpectCounter("dropsondeMarshaller.marshalErrors", 1)
		})

		It("emits a value metric counter", func() {
			envelope := &events.Envelope{
				Origin:      proto.String("fake-origin-3"),
				EventType:   events.Envelope_ValueMetric.Enum(),
				ValueMetric: factories.NewValueMetric("value-name", 1.0, "units"),
			}

			inputChan <- envelope

			eventuallyExpectCounter("dropsondeMarshaller.valueMetricReceived", 1)
		})

		It("counts unknown message types", func() {
			unexpectedMessageType := events.Envelope_EventType(1)
			envelope1 := &events.Envelope{
				Origin:     proto.String("fake-origin-3"),
				EventType:  &unexpectedMessageType,
				LogMessage: factories.NewLogMessage(events.LogMessage_OUT, "test log message 1", "fake-app-id-1", "DEA"),
			}

			inputChan <- envelope1

			eventuallyExpectCounter("dropsondeMarshaller.unknownEventTypeReceived", 1)
		})

		Context("when a http start stop message is received", func() {
			It("emits a counter message with a delta value of 1", func() {
				envelope := &events.Envelope{
					Origin:        proto.String("fake-origin-1"),
					EventType:     events.Envelope_HttpStartStop.Enum(),
					HttpStartStop: getHTTPStartStopEvent(),
				}

				inputChan <- envelope

				eventuallyExpectCounter("dropsondeMarshaller.httpStartStopReceived", 1)
			})
		})

		Context("when multiple http start stop message is received", func() {
			It("emits one counter message with the right delta value", func() {
				const totalMessages = 10
				for i := 0; i < totalMessages; i++ {
					envelope := &events.Envelope{
						Origin:        proto.String("fake-origin-1"),
						EventType:     events.Envelope_HttpStartStop.Enum(),
						HttpStartStop: getHTTPStartStopEvent(),
					}

					inputChan <- envelope
				}

				eventuallyExpectCounter("dropsondeMarshaller.httpStartStopReceived", totalMessages)
			})
		})
	})
})

func getHTTPStartStopEvent() *events.HttpStartStop {
	return &events.HttpStartStop{
		StartTimestamp: proto.Int64(200),
		StopTimestamp:  proto.Int64(500),
		RequestId: &events.UUID{
			Low:  proto.Uint64(200),
			High: proto.Uint64(300),
		},
		PeerType:      events.PeerType_Client.Enum(),
		Method:        events.Method_GET.Enum(),
		Uri:           proto.String("http://some.example.com"),
		RemoteAddress: proto.String("http://remote.address"),
		UserAgent:     proto.String("some user agent"),
		ContentLength: proto.Int64(200),
		StatusCode:    proto.Int32(200),
	}
}
