package agent_test

import (
	"bytes"
	"encoding/json"
	"io/ioutil"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"syscall"
	"testing"

	"github.com/cloudfoundry-incubator/consul-release/src/confab/agent"
	"github.com/onsi/gomega/gexec"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func TestAgent(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "agent")
}

var (
	pathToFakeProcess string
)

var _ = BeforeSuite(func() {
	var err error
	pathToFakeProcess, err = gexec.Build("github.com/cloudfoundry-incubator/consul-release/src/confab/fakes/process")
	Expect(err).NotTo(HaveOccurred())
})

var _ = AfterSuite(func() {
	gexec.CleanupBuildArtifacts()
})

type FakeAgentOutput struct {
	Args []string
	PID  int
}

func getFakeAgentOutput(runner *agent.Runner) FakeAgentOutput {
	bytes, err := ioutil.ReadFile(filepath.Join(runner.ConfigDir, "fake-output.json"))
	if err != nil {
		return FakeAgentOutput{}
	}
	var output FakeAgentOutput
	if err = json.Unmarshal(bytes, &output); err != nil {
		return FakeAgentOutput{}
	}
	return output
}

func getPID(runner *agent.Runner) (int, error) {
	pidFileContents, err := ioutil.ReadFile(runner.PIDFile)
	if err != nil {
		return 0, err
	}

	pid, err := strconv.Atoi(string(pidFileContents))
	if err != nil {
		return 0, err
	}

	return pid, nil
}

func processIsRunning(runner *agent.Runner) bool {
	pid, err := getPID(runner)
	Expect(err).NotTo(HaveOccurred())

	process, err := os.FindProcess(pid)
	Expect(err).NotTo(HaveOccurred())

	errorSendingSignal := process.Signal(syscall.Signal(0))

	return (errorSendingSignal == nil)
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
