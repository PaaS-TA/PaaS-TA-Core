package integration_test

import (
	"fmt"
	"github.com/cloudfoundry/dropsonde"
	"github.com/cloudfoundry/dropsonde/log_sender"
	"github.com/cloudfoundry/dropsonde/metric_sender"
	"github.com/cloudfoundry/dropsonde/metricbatcher"
	"github.com/cloudfoundry/dropsonde/metrics"
	"github.com/cloudfoundry/loggregatorlib/loggertesthelper"
	"github.com/cloudfoundry/sonde-go/events"
	"github.com/gogo/protobuf/proto"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"net"
	"strings"
	"sync"
	"time"
)

var (
	logLock     sync.RWMutex
	logMessages []*events.LogMessage
	udpConn     net.PacketConn
)

var _ = Describe("LogIntegration", func() {
	Context("with standard initialization", func() {
		origin := []string{"test-origin"}

		BeforeEach(func() {
			var err error
			logMessages = nil
			udpConn, err = net.ListenPacket("udp4", ":0")
			Expect(err).ToNot(HaveOccurred())

			go listenForLogs()
			udpAddr := udpConn.LocalAddr().(*net.UDPAddr)
			dropsonde.Initialize(fmt.Sprintf("localhost:%d", udpAddr.Port), origin...)
			sender := metric_sender.NewMetricSender(dropsonde.AutowiredEmitter())
			batcher := metricbatcher.New(sender, 100*time.Millisecond)
			metrics.Initialize(sender, batcher)
		})

		AfterEach(func() {
			udpConn.Close()
		})

		It("sends dropped error message for messages which are just under 64k and don't fit in UDP packet", func() {
			logSender := log_sender.NewLogSender(dropsonde.AutowiredEmitter(), time.Second, loggertesthelper.Logger())

			const length = 64*1024 - 1
			reader := strings.NewReader(strings.Repeat("s", length) + "\n")
			logSender.ScanErrorLogStream("someId", "app", "0", reader)

			Eventually(func() []*events.LogMessage {
				logLock.RLock()
				defer logLock.RUnlock()
				return logMessages
			}).Should(HaveLen(1))

			Expect(logMessages[0].MessageType).To(Equal(events.LogMessage_ERR.Enum()))
			Expect(string(logMessages[0].GetMessage())).To(ContainSubstring("message could not fit in UDP packet"))

		})

		It("sends dropped error message for messages which are over 64k", func() {
			logSender := log_sender.NewLogSender(dropsonde.AutowiredEmitter(), time.Second, loggertesthelper.Logger())

			const length = 64*1024 + 1
			reader := strings.NewReader(strings.Repeat("s", length) + "\n")
			logSender.ScanErrorLogStream("someId", "app", "0", reader)

			Eventually(func() []*events.LogMessage {
				logLock.RLock()
				defer logLock.RUnlock()
				return logMessages
			}).Should(HaveLen(2))

			Expect(logMessages[0].MessageType).To(Equal(events.LogMessage_ERR.Enum()))
			Expect(string(logMessages[0].GetMessage())).To(ContainSubstring(" message too long (>64K without a newline)"))
			Expect(string(logMessages[1].GetMessage())).To(ContainSubstring("s"))
		})
	})
})

func listenForLogs() {
	for {
		buffer := make([]byte, 1024)
		n, _, err := udpConn.ReadFrom(buffer)
		if err != nil {
			return
		}

		if n == 0 {
			panic("Received empty packet")
		}
		envelope := new(events.Envelope)
		err = proto.Unmarshal(buffer[0:n], envelope)
		if err != nil {
			panic(err)
		}

		if envelope.GetEventType() == events.Envelope_LogMessage {
			logLock.Lock()
			logMessages = append(logMessages, envelope.GetLogMessage())
			logLock.Unlock()

		}
	}
}
