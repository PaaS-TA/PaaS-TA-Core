package main

import (
	"errors"
	"fmt"
	"net/url"
	"os"
	"strconv"
	"time"

	"code.cloudfoundry.org/dockerapplifecycle/docker/nat"
	"code.cloudfoundry.org/dockerapplifecycle/helpers"
	"code.cloudfoundry.org/dockerapplifecycle/protocol"
)

type Builder struct {
	RegistryURL                string
	RepoName                   string
	Tag                        string
	InsecureDockerRegistries   []string
	OutputFilename             string
	DockerDaemonExecutablePath string
	DockerDaemonUnixSocket     string
	DockerDaemonTimeout        time.Duration
	CacheDockerImage           bool
	DockerRegistryIPs          []string
	DockerRegistryHost         string
	DockerRegistryPort         int
	DockerRegistryRequireTLS   bool
	DockerLoginServer          string
	DockerUser                 string
	DockerPassword             string
	DockerEmail                string
}

func (builder *Builder) Run(signals <-chan os.Signal, ready chan<- struct{}) error {
	close(ready)

	select {
	case err := <-builder.build():
		if err != nil {
			return err
		}
	case signal := <-signals:
		return errors.New(signal.String())
	}

	return nil
}

type basicCredentialStore struct {
	username string
	password string
}

func (bcs basicCredentialStore) Basic(*url.URL) (string, string) {
	return bcs.username, bcs.password
}

func (builder Builder) build() <-chan error {
	errorChan := make(chan error, 1)

	go func() {
		defer close(errorChan)

		credentials := basicCredentialStore{builder.DockerUser, builder.DockerPassword}

		img, err := helpers.FetchMetadata(builder.RegistryURL, builder.RepoName, builder.Tag, builder.InsecureDockerRegistries, credentials, os.Stderr)
		if err != nil {
			errorChan <- fmt.Errorf(
				"failed to fetch metadata from [%s] with tag [%s] and insecure registries %s due to %s",
				builder.RepoName,
				builder.Tag,
				builder.InsecureDockerRegistries,
				err.Error(),
			)
			return
		}

		info := protocol.DockerImageMetadata{}
		if img.Config != nil {
			info.ExecutionMetadata.Cmd = img.Config.Cmd
			info.ExecutionMetadata.Entrypoint = img.Config.Entrypoint
			info.ExecutionMetadata.Workdir = img.Config.WorkingDir
			info.ExecutionMetadata.User = img.Config.User
			info.ExecutionMetadata.ExposedPorts, err = extractPorts(img.Config.ExposedPorts)
			if err != nil {
				portDetails := fmt.Sprintf("%v", img.Config.ExposedPorts)
				println("failed to parse image ports", portDetails, err.Error())
				errorChan <- err
				return
			}
		}

		dockerImageURL := builder.RepoName
		if builder.RegistryURL != helpers.DockerHubHostname {
			dockerImageURL = builder.RegistryURL + "/" + dockerImageURL
		}
		if len(builder.Tag) > 0 {
			dockerImageURL = dockerImageURL + ":" + builder.Tag
		}
		info.DockerImage = dockerImageURL

		if err := helpers.SaveMetadata(builder.OutputFilename, &info); err != nil {
			errorChan <- fmt.Errorf(
				"failed to save metadata to [%s] due to %s",
				builder.OutputFilename,
				err.Error(),
			)
			return
		}

		errorChan <- nil
	}()

	return errorChan
}

func extractPorts(dockerPorts map[nat.Port]struct{}) (exposedPorts []protocol.Port, err error) {
	sortedPorts := sortPorts(dockerPorts)
	for _, port := range sortedPorts {
		exposedPort, err := strconv.ParseUint(port.Port(), 10, 16)
		if err != nil {
			return []protocol.Port{}, err
		}
		exposedPorts = append(exposedPorts, protocol.Port{Port: uint16(exposedPort), Protocol: port.Proto()})
	}
	return exposedPorts, nil
}

func sortPorts(dockerPorts map[nat.Port]struct{}) []nat.Port {
	var dockerPortsSlice []nat.Port
	for port := range dockerPorts {
		dockerPortsSlice = append(dockerPortsSlice, port)
	}
	nat.Sort(dockerPortsSlice, func(ip, jp nat.Port) bool {
		return ip.Int() < jp.Int() || (ip.Int() == jp.Int() && ip.Proto() == "tcp")
	})
	return dockerPortsSlice
}
