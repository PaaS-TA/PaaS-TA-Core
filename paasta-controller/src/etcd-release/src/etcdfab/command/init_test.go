package command_test

import (
	"bytes"
	"sync"
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func TestCommand(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "command")
}

type concurrentSafeBuffer struct {
	sync.Mutex
	buffer *bytes.Buffer
}

func newConcurrentSafeBuffer() *concurrentSafeBuffer {
	return &concurrentSafeBuffer{
		buffer: bytes.NewBuffer([]byte{}),
	}
}

func (c *concurrentSafeBuffer) Write(b []byte) (int, error) {
	c.Lock()
	defer c.Unlock()

	n, err := c.buffer.Write(b)
	return n, err
}

func (c *concurrentSafeBuffer) String() string {
	c.Lock()
	defer c.Unlock()

	s := c.buffer.String()
	return s
}
