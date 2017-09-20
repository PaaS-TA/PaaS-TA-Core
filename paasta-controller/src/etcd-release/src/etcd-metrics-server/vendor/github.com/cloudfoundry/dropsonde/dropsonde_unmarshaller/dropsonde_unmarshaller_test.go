package dropsonde_unmarshaller_test

import (
	"github.com/cloudfoundry/dropsonde/dropsonde_unmarshaller"
	"github.com/cloudfoundry/dropsonde/factories"
	"github.com/cloudfoundry/loggregatorlib/cfcomponent/instrumentation"
	"github.com/cloudfoundry/loggregatorlib/cfcomponent/instrumentation/testhelpers"
	"github.com/cloudfoundry/loggregatorlib/loggertesthelper"
	"github.com/cloudfoundry/sonde-go/events"
	"github.com/gogo/protobuf/proto"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("DropsondeUnmarshaller", func() {
	var (
		inputChan    chan []byte
		outputChan   chan *events.Envelope
		runComplete  chan struct{}
		unmarshaller dropsonde_unmarshaller.DropsondeUnmarshaller
	)

	BeforeEach(func() {
		fakeEventEmitter.Reset()
		metricBatcher.Reset()
	})

	Context("UnmarshallMessage", func() {
		BeforeEach(func() {
			unmarshaller = dropsonde_unmarshaller.NewDropsondeUnmarshaller(loggertesthelper.Logger())
		})

		It("unmarshalls bytes", func() {
			input := &events.Envelope{
				Origin:      proto.String("fake-origin-3"),
				EventType:   events.Envelope_ValueMetric.Enum(),
				ValueMetric: factories.NewValueMetric("value-name", 1.0, "units"),
			}
			message, _ := proto.Marshal(input)

			output, _ := unmarshaller.UnmarshallMessage(message)

			Expect(output).To(Equal(input))
		})

		It("handles bad input gracefully", func() {
			output, err := unmarshaller.UnmarshallMessage(make([]byte, 4))
			Expect(output).To(BeNil())
			Expect(err).To(HaveOccurred())
		})
	})

	Context("Run", func() {
		BeforeEach(func() {
			inputChan = make(chan []byte, 10)
			outputChan = make(chan *events.Envelope, 10)
			runComplete = make(chan struct{})
			unmarshaller = dropsonde_unmarshaller.NewDropsondeUnmarshaller(loggertesthelper.Logger())

			go func() {
				unmarshaller.Run(inputChan, outputChan)
				close(runComplete)
			}()
		})

		AfterEach(func() {
			close(inputChan)
			Eventually(runComplete).Should(BeClosed())
		})

		It("unmarshals bytes into envelopes", func() {
			envelope := &events.Envelope{
				Origin:      proto.String("fake-origin-3"),
				EventType:   events.Envelope_ValueMetric.Enum(),
				ValueMetric: factories.NewValueMetric("value-name", 1.0, "units"),
			}
			message, _ := proto.Marshal(envelope)

			inputChan <- message
			outputEnvelope := <-outputChan
			Expect(outputEnvelope).To(Equal(envelope))
		})
	})

	Context("metrics", func() {
		BeforeEach(func() {
			inputChan = make(chan []byte, 1000)
			outputChan = make(chan *events.Envelope, 1000)
			runComplete = make(chan struct{})
			unmarshaller = dropsonde_unmarshaller.NewDropsondeUnmarshaller(loggertesthelper.Logger())

			go func() {
				unmarshaller.Run(inputChan, outputChan)
				close(runComplete)
			}()
		})

		AfterEach(func() {
			close(inputChan)
			Eventually(runComplete).Should(BeClosed())
		})

		It("emits the correct metrics context", func() {
			Expect(unmarshaller.Emit().Name).To(Equal("dropsondeUnmarshaller"))
		})

		It("emits a value metric counter", func() {
			envelope := &events.Envelope{
				Origin:      proto.String("fake-origin-3"),
				EventType:   events.Envelope_ValueMetric.Enum(),
				ValueMetric: factories.NewValueMetric("value-name", 1.0, "units"),
			}
			message, _ := proto.Marshal(envelope)

			inputChan <- message
			testhelpers.EventuallyExpectMetric(unmarshaller, "valueMetricReceived", 1)

			Eventually(fakeEventEmitter.GetMessages).Should(HaveLen(1))
			Expect(fakeEventEmitter.GetMessages()[0].Event.(*events.CounterEvent)).To(Equal(&events.CounterEvent{
				Name:  proto.String("dropsondeUnmarshaller.valueMetricReceived"),
				Delta: proto.Uint64(1),
			}))
		})

		It("emits a total log message counter", func() {
			envelope1 := &events.Envelope{
				Origin:     proto.String("fake-origin-3"),
				EventType:  events.Envelope_LogMessage.Enum(),
				LogMessage: factories.NewLogMessage(events.LogMessage_OUT, "test log message 1", "fake-app-id-1", "DEA"),
			}

			envelope2 := &events.Envelope{
				Origin:     proto.String("fake-origin-3"),
				EventType:  events.Envelope_LogMessage.Enum(),
				LogMessage: factories.NewLogMessage(events.LogMessage_OUT, "test log message 2", "fake-app-id-2", "DEA"),
			}

			message1, _ := proto.Marshal(envelope1)
			message2, _ := proto.Marshal(envelope2)

			inputChan <- message1
			inputChan <- message1
			inputChan <- message2

			Eventually(func() uint64 {
				return getTotalLogMessageCount(unmarshaller)
			}).Should(BeNumerically("==", 3))

			Eventually(fakeEventEmitter.GetMessages).Should(HaveLen(1))
			Expect(fakeEventEmitter.GetMessages()[0].Event.(*events.CounterEvent)).To(Equal(&events.CounterEvent{
				Name:  proto.String("dropsondeUnmarshaller.logMessageTotal"),
				Delta: proto.Uint64(3),
			}))
		})

		It("has consistency between total log message counter and per-app counters", func() {
			envelope1 := &events.Envelope{
				Origin:     proto.String("fake-origin-3"),
				EventType:  events.Envelope_LogMessage.Enum(),
				LogMessage: factories.NewLogMessage(events.LogMessage_OUT, "test log message 1", "fake-app-id-1", "DEA"),
			}

			envelope2 := &events.Envelope{
				Origin:     proto.String("fake-origin-3"),
				EventType:  events.Envelope_LogMessage.Enum(),
				LogMessage: factories.NewLogMessage(events.LogMessage_OUT, "test log message 2", "fake-app-id-2", "DEA"),
			}

			message1, _ := proto.Marshal(envelope1)
			message2, _ := proto.Marshal(envelope2)

			inputChan <- message1
			inputChan <- message1
			inputChan <- message2

			Eventually(func() uint64 {
				return getTotalLogMessageCount(unmarshaller)
			}).Should(BeNumerically("==", 3))

			Eventually(fakeEventEmitter.GetMessages).Should(HaveLen(1))
			Expect(fakeEventEmitter.GetMessages()[0].Event.(*events.CounterEvent)).To(Equal(&events.CounterEvent{
				Name:  proto.String("dropsondeUnmarshaller.logMessageTotal"),
				Delta: proto.Uint64(3),
			}))
		})

		It("emits an unmarshal error counter", func() {
			inputChan <- []byte{1, 2, 3}
			testhelpers.EventuallyExpectMetric(unmarshaller, "unmarshalErrors", 1)

			Eventually(fakeEventEmitter.GetMessages).Should(HaveLen(1))
			Expect(fakeEventEmitter.GetMessages()[0].Event.(*events.CounterEvent)).To(Equal(&events.CounterEvent{
				Name:  proto.String("dropsondeUnmarshaller.unmarshalErrors"),
				Delta: proto.Uint64(1),
			}))
		})

		It("counts unknown message types", func() {
			unexpectedMessageType := events.Envelope_EventType(1)
			envelope1 := &events.Envelope{
				Origin:     proto.String("fake-origin-3"),
				EventType:  &unexpectedMessageType,
				LogMessage: factories.NewLogMessage(events.LogMessage_OUT, "test log message 1", "fake-app-id-1", "DEA"),
			}
			message1, err := proto.Marshal(envelope1)
			Expect(err).NotTo(HaveOccurred())

			inputChan <- message1

			Eventually(fakeEventEmitter.GetMessages).Should(HaveLen(1))
			Expect(fakeEventEmitter.GetMessages()[0].Event.(*events.CounterEvent)).To(Equal(&events.CounterEvent{
				Name:  proto.String("dropsondeUnmarshaller.unknownEventTypeReceived"),
				Delta: proto.Uint64(1),
			}))
		})

		Context("when a http start stop message is received", func() {
			It("emits a counter message with a delta value of 1", func() {
				envelope := &events.Envelope{
					Origin:        proto.String("fake-origin-1"),
					EventType:     events.Envelope_HttpStartStop.Enum(),
					HttpStartStop: getHTTPStartStopEvent(),
				}

				message, _ := proto.Marshal(envelope)
				inputChan <- message

				Eventually(func() uint64 {
					return getHTTPStartStopCount(unmarshaller)
				}).Should(BeNumerically("==", 1))

				Eventually(fakeEventEmitter.GetMessages).Should(HaveLen(1))
				Expect(fakeEventEmitter.GetMessages()[0].Event.(*events.CounterEvent)).To(Equal(&events.CounterEvent{
					Name:  proto.String("dropsondeUnmarshaller.httpStartStopReceived"),
					Delta: proto.Uint64(1),
				}))
			})
		})

		Context("when multiple http start stop message is received", func() {
			It("emits one counter message with the right delta value", func() {
				const totalMessages = 100
				for i := 0; i < totalMessages; i++ {
					envelope := &events.Envelope{
						Origin:        proto.String("fake-origin-1"),
						EventType:     events.Envelope_HttpStartStop.Enum(),
						HttpStartStop: getHTTPStartStopEvent(),
					}

					message, _ := proto.Marshal(envelope)
					inputChan <- message
				}

				Eventually(func() uint64 {
					return getHTTPStartStopCount(unmarshaller)
				}).Should(BeNumerically("==", totalMessages))

				Eventually(fakeEventEmitter.GetMessages).Should(HaveLen(1))
				Expect(fakeEventEmitter.GetMessages()[0].Event.(*events.CounterEvent)).To(Equal(&events.CounterEvent{
					Name:  proto.String("dropsondeUnmarshaller.httpStartStopReceived"),
					Delta: proto.Uint64(totalMessages),
				}))
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

func getTotalLogMessageCount(instrumentable instrumentation.Instrumentable) uint64 {
	for _, metric := range instrumentable.Emit().Metrics {
		if metric.Name == "logMessageTotal" {
			return metric.Value.(uint64)
		}
	}
	return uint64(0)
}

func getHTTPStartStopCount(instrumentable instrumentation.Instrumentable) uint64 {
	for _, metric := range instrumentable.Emit().Metrics {
		if metric.Name == "httpStartStopReceived" {
			return metric.Value.(uint64)
		}
	}
	return uint64(0)
}
