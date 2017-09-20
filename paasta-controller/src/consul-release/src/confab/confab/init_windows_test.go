package main_test

import (
	"bytes"
	"errors"
	"os"
	"os/exec"
	"strconv"
	"syscall"
	"time"

	"github.com/cloudfoundry-incubator/consul-release/src/confab/utils"
	. "github.com/onsi/gomega"
)

func killProcessAttachedToPort(kill int) {
	cmd := exec.Command("netstat.exe", "-a", "-n", "-o")
	out, err := cmd.CombinedOutput()
	Expect(err).To(BeNil())

	lines := bytes.Split(out, []byte("\r\n"))
	Expect(lines).ToNot(HaveLen(0))
	lines = lines[1:]

	for _, b := range lines {
		fields := bytes.Fields(bytes.TrimSpace(b))
		if len(fields) != 5 {
			continue
		}
		port, pid, err := parseNetstatLine(fields)
		if err != nil {
			continue
		}
		if port == kill {
			killPID(pid)
			return
		}
	}
}

func parseNetstatLine(line [][]byte) (port, pid int, err error) {
	n := bytes.LastIndexByte(line[1], ':')
	if n < 0 {
		return port, pid, errors.New("parsing port")
	}
	port, err = strconv.Atoi(string(line[1][n+1:]))
	if err != nil {
		return
	}
	pid, err = strconv.Atoi(string(line[4]))
	return
}

func killPID(pid int) {
	process, err := os.FindProcess(pid)
	if err != nil {
		return
	}
	process.Signal(syscall.SIGKILL)
	process.Release()
	Eventually(func() bool {
		return utils.IsPIDRunning(pid)
	}, COMMAND_TIMEOUT, time.Millisecond*250).Should(BeFalse())
}
