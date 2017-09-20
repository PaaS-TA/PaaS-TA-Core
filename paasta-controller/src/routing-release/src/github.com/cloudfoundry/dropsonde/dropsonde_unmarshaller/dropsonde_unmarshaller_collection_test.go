package dropsonde_unmarshaller_test

import (
	"github.com/cloudfoundry/dropsonde/dropsonde_unmarshaller"
	"github.com/cloudfoundry/loggregatorlib/loggertesthelper"
	"github.com/gogo/protobuf/proto"

	"sync"

	"github.com/cloudfoundry/sonde-go/events"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("DropsondeUnmarshallerCollection", func() {
	var (
		inputChan  chan []byte
		outputChan chan *events.Envelope
		collection *dropsonde_unmarshaller.DropsondeUnmarshallerCollection
		waitGroup  *sync.WaitGroup
	)

	BeforeEach(func() {
		inputChan = make(chan []byte)
		outputChan = make(chan *events.Envelope)
		collection = dropsonde_unmarshaller.NewDropsondeUnmarshallerCollection(loggertesthelper.Logger(), 5)
		waitGroup = &sync.WaitGroup{}
	})

	Context("DropsondeUnmarshallerCollection", func() {
		It("creates the right number of unmarshallers", func() {
			Expect(collection.Size()).To(Equal(5))
		})
	})

	Context("Run", func() {
		It("doesn't block while there are unmarshallers idle", func() {
			collection.Run(inputChan, outputChan, waitGroup)
			env := &events.Envelope{
				Origin:    proto.String("foo"),
				EventType: events.Envelope_LogMessage.Enum(),
			}
			bytes, err := proto.Marshal(env)
			Expect(err).ToNot(HaveOccurred())
			done := make(chan struct{})
			go func() {
				defer close(done)
				for i := 0; i < 5; i++ {
					inputChan <- bytes
				}
			}()
			Eventually(done).Should(BeClosed())
			done = make(chan struct{})
			go func() {
				defer close(done)
				inputChan <- bytes
			}()
			Consistently(done).ShouldNot(BeClosed())
			<-outputChan
			Eventually(done).Should(BeClosed())
		})
	})
})
