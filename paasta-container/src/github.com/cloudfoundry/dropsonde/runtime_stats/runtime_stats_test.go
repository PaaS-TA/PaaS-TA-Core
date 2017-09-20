package runtime_stats_test

import (
	"github.com/cloudfoundry/dropsonde/runtime_stats"

	"errors"
	"log"
	"runtime"
	"time"

	"github.com/cloudfoundry/dropsonde/emitter/fake"
	"github.com/cloudfoundry/sonde-go/events"
	"github.com/gogo/protobuf/proto"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("RuntimeStats", func() {
	var (
		fakeEventEmitter  *fake.FakeEventEmitter
		runtimeStats      *runtime_stats.RuntimeStats
		stopChan, runDone chan struct{}
	)

	BeforeEach(func() {
		fakeEventEmitter = fake.NewFakeEventEmitter("fake-origin")
		runtimeStats = runtime_stats.NewRuntimeStats(fakeEventEmitter, 30*time.Second)
		stopChan = make(chan struct{})
		runDone = make(chan struct{})
	})

	AfterEach(func() {
		close(stopChan)
		Eventually(runDone).Should(BeClosed())
	})

	var perform = func() {
		go func() {
			runtimeStats.Run(stopChan)
			close(runDone)
		}()
	}

	var getMetricNames = func() []string {
		var names []string
		for _, event := range fakeEventEmitter.GetEvents() {
			names = append(names, event.(*events.ValueMetric).GetName())
		}
		return names
	}

	It("periodically emits events", func() {
		perform()

		Eventually(func() int { return len(fakeEventEmitter.GetMessages()) }).Should(BeNumerically(">=", 2))
	})

	It("emits a NumCpu metric", func() {
		perform()

		Eventually(fakeEventEmitter.GetEvents).Should(ContainElement(&events.ValueMetric{
			Name:  proto.String("numCPUS"),
			Value: proto.Float64(float64(runtime.NumCPU())),
			Unit:  proto.String("count"),
		}))
	})

	It("emits a NumGoRoutines metric", func() {
		perform()

		Eventually(getMetricNames).Should(ContainElement("numGoRoutines"))
	})

	It("emits all memoryStats metrics", func() {
		perform()

		/*Eventually(getMetricNames).Should(ContainElement("memoryStats.numBytesAllocatedHeap"))
		Eventually(getMetricNames).Should(ContainElement("memoryStats.numBytesAllocatedStack"))
		Eventually(getMetricNames).Should(ContainElement("memoryStats.numBytesAllocated"))
		Eventually(getMetricNames).Should(ContainElement("memoryStats.numMallocs"))
		Eventually(getMetricNames).Should(ContainElement("memoryStats.numFrees"))
		Eventually(getMetricNames).Should(ContainElement("memoryStats.lastGCPauseTimeNS"))*/

		//##### Modified at 2017-04-14 #####
		Eventually(getMetricNames).Should(ContainElement("memoryStats.TotalMemory"))
		Eventually(getMetricNames).Should(ContainElement("memoryStats.AvailableMemory"))
		Eventually(getMetricNames).Should(ContainElement("memoryStats.UsedMemory"))
		Eventually(getMetricNames).Should(ContainElement("memoryStats.UsedPercent"))
	})

	//##### Newly Added at 2017-04-14 #####
	It("emits all cpuLoadStats metrics", func() {
		perform()

		Eventually(getMetricNames).Should(ContainElement("loadavg1"))
		Eventually(getMetricNames).Should(ContainElement("loadavg5"))
		Eventually(getMetricNames).Should(ContainElement("loadavg15"))
	})

	//##### Newly Added at 2017-04-14 #####
	It("emits all diskStats metrics", func() {
		perform()

		Eventually(getMetricNames).Should(ContainElement("diskStats.Total"))
		Eventually(getMetricNames).Should(ContainElement("diskStats.Used"))
		Eventually(getMetricNames).Should(ContainElement("diskStats.Available"))
		Eventually(getMetricNames).Should(ContainElement("diskStats.Usage"))
	})

	//##### Newly Added at 2017-04-14 #####
	It("emits all diskIOStats metrics", func() {
		perform()

		Eventually(getMetricNames).Should(ContainElement(ContainSubstring("readCount")))
		Eventually(getMetricNames).Should(ContainElement(ContainSubstring("writeCount")))
		Eventually(getMetricNames).Should(ContainElement(ContainSubstring("readBytes")))
		Eventually(getMetricNames).Should(ContainElement(ContainSubstring("writeBytes")))
		Eventually(getMetricNames).Should(ContainElement(ContainSubstring("readTime")))
		Eventually(getMetricNames).Should(ContainElement(ContainSubstring("writeTime")))
		Eventually(getMetricNames).Should(ContainElement(ContainSubstring("ioTime")))
	})


	//##### Newly Added at 2017-04-14 #####
	It("emits all networkIOStats metrics", func() {
		perform()

		Eventually(getMetricNames).Should(ContainElement(ContainSubstring("MTU")))
		Eventually(getMetricNames).Should(ContainElement(ContainSubstring("bytesRecv")))
		Eventually(getMetricNames).Should(ContainElement(ContainSubstring("bytesSent")))
		Eventually(getMetricNames).Should(ContainElement(ContainSubstring("packetRecv")))
		Eventually(getMetricNames).Should(ContainElement(ContainSubstring("packetSent")))
		Eventually(getMetricNames).Should(ContainElement(ContainSubstring("dropIn")))
		Eventually(getMetricNames).Should(ContainElement(ContainSubstring("dropOut")))
		Eventually(getMetricNames).Should(ContainElement(ContainSubstring("errIn")))
		Eventually(getMetricNames).Should(ContainElement(ContainSubstring("errOut")))
	})

	//##### Newly Added at 2017-04-14 #####
	It("emits all processes metrics", func() {
		perform()

		Eventually(getMetricNames).Should(ContainElement(ContainSubstring("pid")))
		Eventually(getMetricNames).Should(ContainElement(ContainSubstring("ppid")))
		Eventually(getMetricNames).Should(ContainElement(ContainSubstring("startTime")))
		Eventually(getMetricNames).Should(ContainElement(ContainSubstring("memUsage")))
	})


	It("logs an error if emitting fails", func() {
		fakeEventEmitter.ReturnError = errors.New("fake error")
		fakeLogWriter := &fakeLogWriter{make(chan []byte)}
		log.SetOutput(fakeLogWriter)
		perform()
		Eventually(fakeLogWriter.writeChan).Should(Receive(ContainSubstring("fake error")))
	})
})

type fakeLogWriter struct {
	writeChan chan []byte
}

func (w *fakeLogWriter) Write(p []byte) (int, error) {
	w.writeChan <- p
	return len(p), nil
}
