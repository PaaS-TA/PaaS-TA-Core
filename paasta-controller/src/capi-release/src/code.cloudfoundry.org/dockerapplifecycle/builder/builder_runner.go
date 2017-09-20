package main

import (
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"strconv"
	"time"

	"code.cloudfoundry.org/dockerapplifecycle/docker/nat"
	"code.cloudfoundry.org/dockerapplifecycle/helpers"
	"code.cloudfoundry.org/dockerapplifecycle/protocol"
	"code.cloudfoundry.org/dockerapplifecycle/unix_transport"
	"github.com/nu7hatch/gouuid"
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
	if builder.CacheDockerImage {
		if builder.DockerDaemonUnixSocket == "" {
			builder.DockerDaemonExecutablePath = "/var/run/docker.sock/info"
		}

		err := builder.waitForDocker(signals)
		if err != nil {
			return err
		}
	}
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

		if builder.CacheDockerImage {
			info.DockerImage, err = builder.cacheDockerImage(dockerImageURL)
			if err != nil {
				println("failed to cache image", dockerImageURL, err.Error())
				errorChan <- err
				return
			}
		}

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

func (builder *Builder) cacheDockerImage(dockerImage string) (string, error) {
	fmt.Println("Caching docker image ...")

	if builder.CacheDockerImage && len(builder.DockerUser) > 0 && len(builder.DockerPassword) > 0 && len(builder.DockerEmail) > 0 {
		fmt.Printf("Logging to %s ...\n", builder.DockerLoginServer)
		err := builder.RunDockerCommand("login", "-u", builder.DockerUser, "-p", builder.DockerPassword, "-e", builder.DockerEmail, builder.DockerLoginServer)
		if err != nil {
			return "", err
		}
		fmt.Println("Logged in.")
	}

	fmt.Printf("Pulling docker image %s ...\n", dockerImage)
	err := builder.RunDockerCommand("pull", dockerImage)
	if err != nil {
		return "", err
	}
	fmt.Println("Docker image pulled.")

	cachedDockerImage, err := builder.GenerateImageName()
	if err != nil {
		return "", err
	}
	fmt.Printf("Docker image will be cached as %s\n", cachedDockerImage)

	fmt.Printf("Tagging docker image %s as %s ...\n", dockerImage, cachedDockerImage)
	err = builder.RunDockerCommand("tag", dockerImage, cachedDockerImage)
	if err != nil {
		return "", err
	}
	fmt.Println("Docker image tagged.")

	fmt.Printf("Pushing docker image %s\n", cachedDockerImage)
	err = builder.RunDockerCommand("push", cachedDockerImage)
	if err != nil {
		return "", err
	}
	fmt.Println("Docker image pushed.")
	fmt.Println("Docker image caching completed.")

	return cachedDockerImage, nil
}

func (builder *Builder) RunDockerCommand(args ...string) error {
	cmd := exec.Command(builder.DockerDaemonExecutablePath, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Env = os.Environ()
	return cmd.Run()
}

func (builder *Builder) GenerateImageName() (string, error) {
	uuid, err := uuid.NewV4()
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%s:%d/%s", builder.DockerRegistryHost, builder.DockerRegistryPort, uuid), nil
}

func (builder *Builder) waitForDocker(signals <-chan os.Signal) error {
	giveUp := make(chan struct{})
	defer close(giveUp)

	select {
	case err := <-builder.waitForDockerDaemon(giveUp):
		if err != nil {
			return err
		}
	case <-time.After(builder.DockerDaemonTimeout):
		return errors.New("Timed out waiting for docker daemon to start")
	case signal := <-signals:
		return errors.New(signal.String())
	}

	return nil
}

func (builder *Builder) waitForDockerDaemon(giveUp <-chan struct{}) <-chan error {
	errChan := make(chan error, 1)
	client := http.Client{Transport: unix_transport.New(builder.DockerDaemonUnixSocket)}

	go builder.pingDaemonPeriodically(client, errChan, giveUp)

	return errChan
}

func (builder Builder) pingDaemonPeriodically(client http.Client, errChan chan<- error, giveUp <-chan struct{}) {
	for {
		resp, err := client.Get("unix://" + builder.DockerDaemonUnixSocket + "/info")
		if err != nil {
			select {
			case <-giveUp:
				return
			case <-time.After(100 * time.Millisecond):
			}
			continue
		} else {
			if resp.StatusCode == http.StatusOK {
				fmt.Println("Docker daemon running")
			} else {
				errChan <- fmt.Errorf("Docker daemon failed to start. Ping returned %s", resp.Status)
				return
			}
			break
		}

	}
	errChan <- nil
	return
}
