package bosh

import (
	"bytes"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

var tempDir func(string, string) (string, error) = ioutil.TempDir
var writeFile func(string, []byte, os.FileMode) error = ioutil.WriteFile
var stderr io.Writer = os.Stderr

type CLI struct{}

func (c CLI) Interpolate(manifest, opsYAML string) (string, error) {
	tempDir, err := tempDir("", "")
	if err != nil {
		return "", err
	}

	manifestPath := filepath.Join(tempDir, "manifest.yml")
	opsFilePath := filepath.Join(tempDir, "ops.yml")

	err = writeFile(manifestPath, []byte(manifest), os.ModePerm)
	if err != nil {
		return "", err
	}

	err = writeFile(opsFilePath, []byte(opsYAML), os.ModePerm)
	if err != nil {
		return "", err
	}

	args := []string{"interpolate", manifestPath, "-o", opsFilePath}
	buffer := bytes.NewBuffer([]byte{})
	command := exec.Command("bosh", args...)
	command.Dir = tempDir
	command.Stdout = buffer
	command.Stderr = stderr

	err = command.Run()
	if err != nil {
		return "", err
	}

	return strings.TrimSuffix(buffer.String(), "\n"), nil
}
