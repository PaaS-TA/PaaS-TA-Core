// +build !windows

package main_test

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"syscall"

	. "github.com/onsi/gomega"
)

func killProcessAttachedToPort(port int) {
	cmdLine := fmt.Sprintf("lsof -i :%d | tail -1 | cut -d' ' -f4", port)
	cmd := exec.Command("bash", "-c", cmdLine)
	var buffer bytes.Buffer
	cmd.Stdout = &buffer
	Expect(cmd.Run()).To(Succeed())

	pidStr := strings.TrimSpace(buffer.String())
	if pidStr != "" {
		pid, err := strconv.Atoi(pidStr)
		Expect(err).NotTo(HaveOccurred())
		killPID(pid)
	}
}

func killPID(pid int) {
	process, err := os.FindProcess(pid)
	Expect(err).NotTo(HaveOccurred())

	process.Signal(syscall.SIGKILL)
}
