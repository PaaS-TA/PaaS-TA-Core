package command

import (
	"io"
	"os/exec"
)

type Wrapper struct {
}

func NewWrapper() Wrapper {
	return Wrapper{}
}

func (w Wrapper) Start(commandPath string, commandArgs []string, outWriter, errWriter io.Writer) (int, error) {
	cmd := exec.Command(commandPath, commandArgs...)

	cmd.Stdout = outWriter
	cmd.Stderr = errWriter

	err := cmd.Start()
	if err != nil {
		return 0, err
	}

	return cmd.Process.Pid, nil
}
