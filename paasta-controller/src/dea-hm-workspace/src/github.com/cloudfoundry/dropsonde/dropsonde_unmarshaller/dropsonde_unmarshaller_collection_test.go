package dropsonde_unmarshaller_test

import (
	"github.com/cloudfoundry/dropsonde/dropsonde_unmarshaller"
	"github.com/cloudfoundry/loggregatorlib/loggertesthelper"

	"runtime"
	"sync"

	"github.com/cloudfoundry/sonde-go/events"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("DropsondeUnmarshallerCollection", func() {
	var (
		inputChan  chan []byte
		outputChan chan *events.Envelope
		collection dropsonde_unmarshaller.DropsondeUnmarshallerCollection
		waitGroup  *sync.WaitGroup
	)
	BeforeEach(func() {
		inputChan = make(chan []byte, 10)
		outputChan = make(chan *events.Envelope, 10)
		collection = dropsonde_unmarshaller.NewDropsondeUnmarshallerCollection(loggertesthelper.Logger(), 5)
		waitGroup = &sync.WaitGroup{}
	})

	Context("DropsondeUnmarshallerCollection", func() {
		It("creates the right number of unmarshallers", func() {
			Expect(collection.Size()).To(Equal(5))
		})

	})

	Context("Run", func() {
		It("runs its collection of unmarshallers in separate go routines", func() {
			startingCountGoroutines := runtime.NumGoroutine()
			collection.Run(inputChan, outputChan, waitGroup)
			Expect(startingCountGoroutines + 5).To(Equal(runtime.NumGoroutine()))
		})
	})
})
