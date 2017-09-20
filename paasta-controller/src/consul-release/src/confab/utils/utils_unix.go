// +build !windows

package utils

import (
	"os"
	"syscall"
)

func IsPIDRunning(pid int) bool {
	// On Unix FindProcess always returns
	// a non-nil Process and a nil error.
	process, _ := os.FindProcess(pid)
	return process.Signal(syscall.Signal(0)) == nil
}
