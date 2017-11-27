package utils

import (
	"os"
	"os/exec"
)

type MonitWrapper struct {
	command string
}

func NewMonitWrapper(command string) MonitWrapper {
	return MonitWrapper{
		command: command,
	}
}

func (m MonitWrapper) Run(args []string) error {
	cmd := exec.Command(m.command, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	err := cmd.Run()
	if err != nil {
		return err
	}

	return nil
}

func (m MonitWrapper) Output(args []string) (string, error) {
	cmd := exec.Command(m.command, args...)
	cmd.Stderr = os.Stderr
	output, err := cmd.Output()
	if err != nil {
		return "", err
	}

	return string(output), nil
}
