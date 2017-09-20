package main_test

import (
	"encoding/json"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"testing"

	"github.com/onsi/gomega/gexec"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

const Windows = runtime.GOOS == "windows"

func TestMain(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "confab/confab")
}

var (
	pathToFakeAgent string
	pathToConfab    string
)

var _ = BeforeSuite(func() {
	var err error
	pathToFakeAgent, err = gexec.Build("github.com/cloudfoundry-incubator/consul-release/src/confab/fakes/agent")
	Expect(err).NotTo(HaveOccurred())

	pathToConfab, err = gexec.Build("github.com/cloudfoundry-incubator/consul-release/src/confab/confab")
	Expect(err).NotTo(HaveOccurred())

	if !Windows {
		Expect(exec.Command("which", "lsof").Run()).To(Succeed())
	}
})

var _ = AfterSuite(func() {
	gexec.CleanupBuildArtifacts()
})

type FakeAgentOutputData struct {
	Args                []string
	PID                 int
	LeaveCallCount      int
	UseKeyCallCount     int
	InstallKeyCallCount int
	ConsulConfig        ConsulConfig
	StatsCallCount      int
}

type ConsulConfig struct {
	Server    bool
	Bootstrap bool
}

func killProcessWithPIDFile(pidFilePath string) {
	pidFileContents, err := ioutil.ReadFile(pidFilePath)
	if err != nil {
		return
	}

	pid, err := strconv.Atoi(string(pidFileContents))
	Expect(err).NotTo(HaveOccurred())

	killPID(pid)
}

func getPID(pidFilePath string) (int, error) {
	pidFileContents, err := ioutil.ReadFile(pidFilePath)
	if err != nil {
		return 0, err
	}

	return strconv.Atoi(string(pidFileContents))
}

func fakeAgentOutputFromFile(configDir, fileName string) (FakeAgentOutputData, error) {
	var decodedFakeOutput FakeAgentOutputData

	fakeOutput, err := ioutil.ReadFile(filepath.Join(configDir, fileName))
	if err != nil {
		return decodedFakeOutput, err
	}

	err = json.Unmarshal(fakeOutput, &decodedFakeOutput)
	if err != nil {
		return decodedFakeOutput, err
	}

	return decodedFakeOutput, nil
}

func writeConfigurationFile(filename string, configuration map[string]interface{}) {
	configData, err := json.Marshal(configuration)
	Expect(err).NotTo(HaveOccurred())

	err = ioutil.WriteFile(filename, configData, os.ModePerm)
	Expect(err).NotTo(HaveOccurred())
}
