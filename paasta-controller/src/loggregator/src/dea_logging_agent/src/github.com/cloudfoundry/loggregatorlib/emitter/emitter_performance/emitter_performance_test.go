package emitter_performance

import (
	"strings"
	"testing"
	"time"

	"github.com/cloudfoundry/loggregatorlib/emitter"
	"github.com/cloudfoundry/loggregatorlib/emitter/fakes"
)

const (
	SECOND = float64(1 * time.Second)
)

type messageFixture struct {
	name                string
	message             string
	logMessageExpected  float64
	logEnvelopeExpected float64
}

func (mf *messageFixture) getExpected(isEnvelope bool) float64 {
	if isEnvelope {
		return mf.logEnvelopeExpected
	}
	return mf.logMessageExpected
}

var messageFixtures = []*messageFixture{
	{"long message", longMessage(), 1 * SECOND, 2 * SECOND},
	{"message with newlines", messageWithNewlines(), 3 * SECOND, 5 * SECOND},
	{"message worst case", longMessage() + "\n", 1 * SECOND, 1 * SECOND},
}

func longMessage() string {
	return strings.Repeat("a", emitter.MAX_MESSAGE_BYTE_SIZE*2)
}

func messageWithNewlines() string {
	return strings.Repeat(strings.Repeat("a", 6*1024)+"\n", 10)
}

func BenchmarkLogEnvelopeEmit(b *testing.B) {
	conn := &fakes.FakePacketConn{}
	e, _ := emitter.New("127.0.0.1:3456", "ROUTER", "42", "secret", conn, nil)

	testEmitHelper(b, e, true)
}

func testEmitHelper(b *testing.B, e emitter.Emitter, isEnvelope bool) {
	for _, fixture := range messageFixtures {
		startTime := time.Now().UnixNano()

		for i := 0; i < b.N; i++ {
			e.Emit("appid", fixture.message)
		}
		elapsedTime := float64(time.Now().UnixNano() - startTime)

		expected := fixture.getExpected(isEnvelope)
		if elapsedTime > expected {
			b.Errorf("Elapsed time for %s should have been below %vs, but was %vs", fixture.name, expected/SECOND, float64(elapsedTime)/SECOND)
		}
	}
}
