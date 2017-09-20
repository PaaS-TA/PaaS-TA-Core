package main

import (
	"fmt"
	"math/rand"
	"os"
	"os/exec"
	"path"
	"strings"
	"time"
)

var DockerArgs []string = []string{"daemon", "--log-level=error", "--iptables=false", "--ipv6=false"}

type DockerDaemon struct {
	DockerDaemonPath         string
	DockerRegistryIPs        []string
	InsecureDockerRegistries []string
	DockerRegistryHost       string
}

func (daemon *DockerDaemon) Run(signals <-chan os.Signal, ready chan<- struct{}) error {
	addHost(daemon.DockerRegistryHost, daemon.DockerRegistryIPs)

	daemonProcess, errorChannel := launchDockerDaemon(daemon.DockerDaemonPath, daemon.InsecureDockerRegistries)
	close(ready)

	select {
	case err := <-errorChannel:
		if err != nil {
			return err
		}
	case signal := <-signals:
		err := daemonProcess.Signal(signal)
		if err != nil {
			println("failed to send signal", signal.String(), "to Docker daemon:", err.Error())
		}
	}

	return nil
}

func launchDockerDaemon(daemonPath string, insecureDockerRegistriesList []string) (*os.Process, <-chan error) {
	chanError := make(chan error, 1)

	args := DockerArgs
	if len(insecureDockerRegistriesList) > 0 {
		args = append(args, "--insecure-registry="+strings.Join(insecureDockerRegistriesList, ","))
	}

	cmd := exec.Command(daemonPath, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Env = os.Environ()

	err := cmd.Start()
	if err != nil {
		chanError <- fmt.Errorf(
			"failed to start Docker daemon [%s %s]: %s",
			path.Clean(daemonPath),
			strings.Join(args, " "),
			err,
		)
		return nil, chanError
	}

	go func() {
		defer close(chanError)

		err := cmd.Wait()
		if err != nil {
			chanError <- err
			println("Docker daemon failed with", err.Error())
		}

		chanError <- nil
	}()

	return cmd.Process, chanError
}

func addHost(dockerRegistryHost string, dockerRegistryIPs []string) error {
	return addLineToFile("/etc/hosts", fmt.Sprintf("%s %s\n", getRegistryIP(dockerRegistryIPs), dockerRegistryHost))
}

func getRegistryIP(registryIPs []string) string {
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	return registryIPs[r.Intn(len(registryIPs))]
}

func addLineToFile(filePath string, line string) error {
	hostsFile, err := os.OpenFile(filePath, os.O_APPEND|os.O_WRONLY, 0600)
	if err != nil {
		return err
	}

	defer hostsFile.Close()

	if _, err = hostsFile.WriteString(line); err != nil {
		return err
	}

	return nil
}
