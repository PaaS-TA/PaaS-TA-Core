package dropsonde_marshaller_test

import (
	"github.com/cloudfoundry/dropsonde/dropsonde_marshaller"
	"github.com/cloudfoundry/dropsonde/factories"
	"github.com/cloudfoundry/loggregatorlib/cfcomponent/instrumentation/testhelpers"
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
		marshaller  dropsonde_marshaller.DropsondeMarshaller
	)

	BeforeEach(func() {
		inputChan = make(chan *events.Envelope, 10)
		outputChan = make(chan []byte, 10)
		runComplete = make(chan struct{})
		marshaller = dropsonde_marshaller.NewDropsondeMarshaller(loggertesthelper.Logger())

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
		It("emits the correct metrics context", func() {
			Expect(marshaller.Emit().Name).To(Equal("dropsondeMarshaller"))
		})

		It("emits a marshal error counter", func() {
			envelope := &events.Envelope{}

			inputChan <- envelope
			testhelpers.EventuallyExpectMetric(marshaller, "marshalErrors", 1)
		})
	})
})
