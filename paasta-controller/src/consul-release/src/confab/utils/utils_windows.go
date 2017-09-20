// +build windows

package utils

import "os"

func IsPIDRunning(pid int) bool {
	// On Windows FindProcess returns
	// an error if the PID is invalid.
	process, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	process.Release()
	return true
}
