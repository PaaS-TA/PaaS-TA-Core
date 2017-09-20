package emitter

import (
	"net"

	"github.com/cloudfoundry/gosteno"
	"github.com/cloudfoundry/loggregatorlib/logmessage"
	"github.com/gogo/protobuf/proto"

	"strings"
	"time"
)

var (
	MAX_MESSAGE_BYTE_SIZE = (9 * 1024) - 512
	TRUNCATED_BYTES       = []byte("TRUNCATED")
	TRUNCATED_OFFSET      = MAX_MESSAGE_BYTE_SIZE - len(TRUNCATED_BYTES)
)

type Emitter interface {
	Emit(string, string)
	EmitError(string, string)
	EmitLogMessage(*logmessage.LogMessage)
}

type LoggregatorEmitter struct {
	conn         net.PacketConn
	addr         *net.UDPAddr
	sourceName   string
	sourceID     string
	sharedSecret string
	logger       *gosteno.Logger
}

func isEmpty(s string) bool {
	return len(strings.TrimSpace(s)) == 0
}

func splitMessage(message string) []string {
	return strings.FieldsFunc(message, func(r rune) bool {
		return r == '\n' || r == '\r'
	})
}

func (e *LoggregatorEmitter) Emit(appid, message string) {
	e.emit(appid, message, logmessage.LogMessage_OUT)
}

func (e *LoggregatorEmitter) EmitError(appid, message string) {
	e.emit(appid, message, logmessage.LogMessage_ERR)
}

func (e *LoggregatorEmitter) emit(appid, message string, messageType logmessage.LogMessage_MessageType) {
	if isEmpty(appid) || isEmpty(message) {
		return
	}
	logMessage := e.newLogMessage(appid, message, messageType)
	e.logger.Debugf("Logging message from %s of type %s with appid %s", *logMessage.SourceName, logMessage.MessageType, *logMessage.AppId)

	e.EmitLogMessage(logMessage)
}

func (e *LoggregatorEmitter) EmitLogMessage(logMessage *logmessage.LogMessage) {
	messages := splitMessage(string(logMessage.GetMessage()))

	for _, message := range messages {
		if isEmpty(message) {
			continue
		}

		if len(message) > MAX_MESSAGE_BYTE_SIZE {
			logMessage.Message = append([]byte(message)[0:TRUNCATED_OFFSET], TRUNCATED_BYTES...)
		} else {
			logMessage.Message = []byte(message)
		}
		if e.sharedSecret == "" {
			marshalledLogMessage, err := proto.Marshal(logMessage)
			if err != nil {
				e.logger.Errorf("Error marshalling message: %s", err)
				return
			}
			e.write(marshalledLogMessage)
		} else {
			logEnvelope, err := e.newLogEnvelope(*logMessage.AppId, logMessage)
			if err != nil {
				e.logger.Errorf("Error creating envelope: %s", err)
				return
			}
			marshalledLogEnvelope, err := proto.Marshal(logEnvelope)
			if err != nil {
				e.logger.Errorf("Error marshalling envelope: %s", err)
				return
			}
			e.write(marshalledLogEnvelope)
		}
	}
}

func NewEmitter(loggregatorServer, sourceName, sourceId, sharedSecret string, logger *gosteno.Logger) (*LoggregatorEmitter, error) {
	conn, err := net.ListenPacket("udp", "")
	if err != nil {
		return nil, err
	}

	return New(loggregatorServer, sourceName, sourceId, sharedSecret, conn, logger)
}

func New(loggregatorServer, sourceName, sourceId, sharedSecret string, conn net.PacketConn, logger *gosteno.Logger) (*LoggregatorEmitter, error) {
	if logger == nil {
		logger = gosteno.NewLogger("loggregatorlib.emitter")
	}

	addr, err := net.ResolveUDPAddr("udp", loggregatorServer)
	if err != nil {
		return nil, err
	}

	e := &LoggregatorEmitter{
		sharedSecret: sharedSecret,
		sourceName:   sourceName,
		sourceID:     sourceId,
		conn:         conn,
		addr:         addr,
		logger:       logger,
	}

	e.logger.Debugf("Created new loggregator emitter with sourceName: %s and sourceID: %s", e.sourceName, e.sourceID)
	return e, nil
}

func (e *LoggregatorEmitter) write(data []byte) (int, error) {
	if len(data) == 0 {
		return 0, nil
	}

	writeCount, err := e.conn.WriteTo(data, e.addr)
	if err != nil {
		e.logger.Errorf("Write to %s failed %s", e.addr.String(), err.Error())
		return writeCount, err
	}
	e.logger.Debugf("Wrote %d bytes to %s", writeCount, e.addr.String())

	return writeCount, err
}

func (e *LoggregatorEmitter) newLogMessage(appId, message string, mt logmessage.LogMessage_MessageType) *logmessage.LogMessage {
	currentTime := time.Now()

	return &logmessage.LogMessage{
		Message:     []byte(message),
		AppId:       proto.String(appId),
		MessageType: &mt,
		SourceId:    &e.sourceID,
		Timestamp:   proto.Int64(currentTime.UnixNano()),
		SourceName:  &e.sourceName,
	}
}

func (e *LoggregatorEmitter) newLogEnvelope(appId string, message *logmessage.LogMessage) (*logmessage.LogEnvelope, error) {
	envelope := &logmessage.LogEnvelope{
		LogMessage: message,
		RoutingKey: proto.String(appId),
		Signature:  []byte{},
	}
	err := envelope.SignEnvelope(e.sharedSecret)

	return envelope, err
}
