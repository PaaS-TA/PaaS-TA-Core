package emitter_test

import (
	"net"
	"strings"

	. "github.com/cloudfoundry/loggregatorlib/emitter"
	"github.com/cloudfoundry/loggregatorlib/emitter/fakes"
	"github.com/cloudfoundry/loggregatorlib/logmessage"
	"github.com/cloudfoundry/loggregatorlib/logmessage/testhelpers"
	"github.com/gogo/protobuf/proto"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Testing with Ginkgo", func() {
	var (
		emitter *LoggregatorEmitter
		conn    *fakes.FakePacketConn
	)

	BeforeEach(func() {
		conn = &fakes.FakePacketConn{}

		var err error
		emitter, err = New("127.0.0.1:3456", "ROUTER", "42", "secret", conn, nil)
		Expect(err).ToNot(HaveOccurred())
	})

	It("should emit stdout", func() {
		emitter.Emit("appid", "foo")
		receivedMessage := extractLogMessage(conn.WriteToArgsForCall(0))

		Expect(receivedMessage.GetMessage()).To(Equal([]byte("foo")))
		Expect(receivedMessage.GetAppId()).To(Equal("appid"))
		Expect(receivedMessage.GetSourceId()).To(Equal("42"))
		Expect(receivedMessage.GetMessageType()).To(Equal(logmessage.LogMessage_OUT))
	})

	It("should emit stderr", func() {
		emitter.EmitError("appid", "foo")
		receivedMessage := extractLogMessage(conn.WriteToArgsForCall(0))

		Expect(receivedMessage.GetMessage()).To(Equal([]byte("foo")))
		Expect(receivedMessage.GetAppId()).To(Equal("appid"))
		Expect(receivedMessage.GetSourceId()).To(Equal("42"))
		Expect(receivedMessage.GetMessageType()).To(Equal(logmessage.LogMessage_ERR))
	})

	It("should emit fully formed log messages", func() {
		logMessage := testhelpers.NewLogMessage("test_msg", "test_app_id")
		logMessage.SourceId = proto.String("src_id")

		emitter.EmitLogMessage(logMessage)
		receivedMessage := extractLogMessage(conn.WriteToArgsForCall(0))

		Expect(receivedMessage.GetMessage()).To(Equal([]byte("test_msg")))
		Expect(receivedMessage.GetAppId()).To(Equal("test_app_id"))
		Expect(receivedMessage.GetSourceId()).To(Equal("src_id"))
	})

	It("should truncate long messages", func() {
		longMessage := strings.Repeat("7", MAX_MESSAGE_BYTE_SIZE*2)
		logMessage := testhelpers.NewLogMessage(longMessage, "test_app_id")

		emitter.EmitLogMessage(logMessage)

		receivedMessage := extractLogMessage(conn.WriteToArgsForCall(0))
		receivedMessageText := receivedMessage.GetMessage()

		truncatedOffset := len(receivedMessageText) - len(TRUNCATED_BYTES)
		expectedBytes := append([]byte(receivedMessageText)[:truncatedOffset], TRUNCATED_BYTES...)

		Expect(receivedMessageText).To(Equal(expectedBytes))
		Expect(receivedMessageText).To(HaveLen(MAX_MESSAGE_BYTE_SIZE))
	})

	It("should split messages on new lines", func() {
		message := "message1\n\rmessage2\nmessage3\r\nmessage4\r"
		logMessage := testhelpers.NewLogMessage(message, "test_app_id")

		emitter.EmitLogMessage(logMessage)
		Expect(conn.WriteToCallCount()).To(Equal(4))

		for i, expectedMessage := range []string{"message1", "message2", "message3", "message4"} {
			receivedMessage := extractLogMessage(conn.WriteToArgsForCall(i))
			Expect(receivedMessage.GetMessage()).To(Equal([]byte(expectedMessage)))
		}
	})

	It("should build the log envelope correctly", func() {
		emitter.Emit("appid", "foo")
		receivedEnvelope := extractLogEnvelope(conn.WriteToArgsForCall(0))

		Expect(receivedEnvelope.GetLogMessage().GetMessage()).To(Equal([]byte("foo")))
		Expect(receivedEnvelope.GetLogMessage().GetAppId()).To(Equal("appid"))
		Expect(receivedEnvelope.GetRoutingKey()).To(Equal("appid"))
		Expect(receivedEnvelope.GetLogMessage().GetSourceId()).To(Equal("42"))
	})

	It("should sign the log message correctly", func() {
		emitter.Emit("appid", "foo")
		receivedEnvelope := extractLogEnvelope(conn.WriteToArgsForCall(0))
		Expect(receivedEnvelope.VerifySignature("secret")).To(BeTrue(), "Expected envelope to be signed with the correct secret key")
	})

	Context("when missing an app id", func() {
		It("should not emit", func() {
			emitter.Emit("", "foo")
			Expect(conn.WriteToCallCount()).To(Equal(0))

			emitter.Emit("    ", "foo")
			Expect(conn.WriteToCallCount()).To(Equal(0))
		})
	})

	Context("with a server", func() {
		var udpListener *net.UDPConn

		BeforeEach(func() {
			udpAddr, err := net.ResolveUDPAddr("udp", "127.0.0.1:0")
			Expect(err).NotTo(HaveOccurred())
			udpListener, err = net.ListenUDP("udp", udpAddr)
			Expect(err).NotTo(HaveOccurred())
		})

		AfterEach(func() {
			udpListener.Close()
		})

		It("sends the messages", func() {
			var err error
			emitter, err = NewEmitter(udpListener.LocalAddr().String(), "ROUTER", "42", "secret", nil)
			Expect(err).ToNot(HaveOccurred())

			emitter.Emit("appid", "foo")

			buffer := make([]byte, 4096)
			readCount, _, err := udpListener.ReadFromUDP(buffer)
			Expect(err).NotTo(HaveOccurred())

			var env logmessage.LogEnvelope
			err = proto.Unmarshal(buffer[:readCount], &env)
			Expect(err).NotTo(HaveOccurred())

			msg := env.GetLogMessage()
			Expect(msg.GetMessage()).To(Equal([]byte("foo")))
			Expect(msg.GetAppId()).To(Equal("appid"))
			Expect(msg.GetSourceId()).To(Equal("42"))
			Expect(msg.GetMessageType()).To(Equal(logmessage.LogMessage_OUT))
		})
	})
})

func extractLogEnvelope(data []byte, addr net.Addr) *logmessage.LogEnvelope {
	receivedEnvelope := &logmessage.LogEnvelope{}

	err := proto.Unmarshal(data, receivedEnvelope)
	Expect(err).ToNot(HaveOccurred())

	return receivedEnvelope
}

func extractLogMessage(data []byte, addr net.Addr) *logmessage.LogMessage {
	envelope := extractLogEnvelope(data, addr)

	return envelope.GetLogMessage()
}
