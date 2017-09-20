package execshim

import "io"

//go:generate counterfeiter -o exec_fake/fake_cmd.go . Cmd

type Cmd interface {
	Start() error
	StdoutPipe() (io.ReadCloser, error)
	StderrPipe() (io.ReadCloser, error)
	Wait() error
}