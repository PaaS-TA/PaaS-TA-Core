package utils

import (
	"io/ioutil"
	"strconv"
)

func IsRunningProcess(pidFilePath string) bool {
	pidFileContents, err := ioutil.ReadFile(pidFilePath)
	if err != nil {
		return false
	}

	pid, err := strconv.Atoi(string(pidFileContents))
	if err != nil {
		return false
	}

	return IsPIDRunning(pid)
}
