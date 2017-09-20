package main

import (
	"errors"
	"flag"
	"fmt"
	"net/url"
	"os"
	"strings"
	"time"

	"code.cloudfoundry.org/dockerapplifecycle/helpers"
	"github.com/tedsuo/ifrit"
	"github.com/tedsuo/ifrit/grouper"
	"github.com/tedsuo/ifrit/sigmon"
)

type registries []string
type ips []string

func main() {
	var insecureDockerRegistries registries
	var dockerRegistryIPs ips

	flagSet := flag.NewFlagSet("builder", flag.ExitOnError)

	dockerRef := flagSet.String(
		"dockerRef",
		"",
		"docker image reference in standard docker string format",
	)

	outputFilename := flagSet.String(
		"outputMetadataJSONFilename",
		"/tmp/result/result.json",
		"filename in which to write the app metadata",
	)

	flagSet.Var(
		&insecureDockerRegistries,
		"insecureDockerRegistries",
		"insecure Docker Registry addresses (host:port)",
	)

	dockerDaemonExecutablePath := flagSet.String(
		"dockerDaemonExecutablePath",
		"/tmp/docker_app_lifecycle/docker",
		"path to the 'docker' executable",
	)

	dockerDaemonUnixSocket := flagSet.String(
		"dockerDaemonUnixSocket",
		"/var/run/docker.sock/info",
		"path to the unix socket the docker daemon is listening to",
	)

	cacheDockerImage := flagSet.Bool(
		"cacheDockerImage",
		false,
		"Caches Docker images to private docker registry",
	)

	flagSet.Var(
		&dockerRegistryIPs,
		"dockerRegistryIPs",
		"Docker Registry IPs",
	)

	dockerRegistryHost := flagSet.String(
		"dockerRegistryHost",
		"",
		"Docker Registry host",
	)

	dockerRegistryPort := flagSet.Int(
		"dockerRegistryPort",
		8080,
		"Docker Registry port",
	)

	dockerRegistryRequireTLS := flagSet.Bool(
		"dockerRegistryRequireTLS",
		false,
		"Use HTTPS when contacting the Docker Registry",
	)

	dockerLoginServer := flagSet.String(
		"dockerLoginServer",
		helpers.DockerHubLoginServer,
		"Docker Login server address",
	)

	dockerUser := flagSet.String(
		"dockerUser",
		"",
		"User for pulling from docker registry",
	)

	dockerPassword := flagSet.String(
		"dockerPassword",
		"",
		"Password for pulling from docker registry",
	)

	dockerEmail := flagSet.String(
		"dockerEmail",
		"",
		"Email for pulling from docker registry",
	)

	if err := flagSet.Parse(os.Args[1:len(os.Args)]); err != nil {
		println(err.Error())
		os.Exit(1)
	}

	var registryURL, repoName, tag string
	if len(*dockerRef) > 0 {
		registryURL, repoName, tag = helpers.ParseDockerRef(*dockerRef)
	} else {
		println("missing flag: dockerRef required")
		flagSet.PrintDefaults()
		os.Exit(1)
	}

	if *cacheDockerImage {
		if len(dockerRegistryIPs) == 0 {
			println("missing flag: dockerRegistryIPs required")
			os.Exit(1)
		}
		if len(*dockerRegistryHost) == 0 {
			println("missing flag: dockerRegistryHost required")
			os.Exit(1)
		}
		if strings.Contains(*dockerRegistryHost, ":") {
			println("invalid host format", *dockerRegistryHost)
			os.Exit(1)
		}
		if *dockerRegistryPort < 0 {
			println("negative port number", *dockerRegistryPort)
			os.Exit(1)
		}
		if *dockerRegistryPort > 65535 {
			println("port number too big", *dockerRegistryPort)
			os.Exit(1)
		}
		if !*dockerRegistryRequireTLS {
			insecureDockerRegistries = append(insecureDockerRegistries, fmt.Sprintf("%s:%d", *dockerRegistryHost, *dockerRegistryPort))
		}
	}

	builder := Builder{
		RegistryURL:                registryURL,
		RepoName:                   repoName,
		Tag:                        tag,
		OutputFilename:             *outputFilename,
		DockerDaemonExecutablePath: *dockerDaemonExecutablePath,
		InsecureDockerRegistries:   insecureDockerRegistries,
		DockerDaemonTimeout:        10 * time.Second,
		DockerDaemonUnixSocket:     *dockerDaemonUnixSocket,
		CacheDockerImage:           *cacheDockerImage,
		DockerRegistryIPs:          dockerRegistryIPs,
		DockerRegistryHost:         *dockerRegistryHost,
		DockerRegistryPort:         *dockerRegistryPort,
		DockerRegistryRequireTLS:   *dockerRegistryRequireTLS,
		DockerLoginServer:          *dockerLoginServer,
		DockerUser:                 *dockerUser,
		DockerPassword:             *dockerPassword,
		DockerEmail:                *dockerEmail,
	}

	members := grouper.Members{
		{"builder", ifrit.RunFunc(builder.Run)},
	}

	if *cacheDockerImage {
		validateCredentials(*dockerLoginServer, *dockerUser, *dockerPassword, *dockerEmail)

		if _, err := os.Stat(*dockerDaemonExecutablePath); err != nil {
			println("docker daemon not found in", *dockerDaemonExecutablePath)
			os.Exit(1)
		}

		dockerDaemon := DockerDaemon{
			DockerDaemonPath:         *dockerDaemonExecutablePath,
			InsecureDockerRegistries: insecureDockerRegistries,
			DockerRegistryIPs:        dockerRegistryIPs,
			DockerRegistryHost:       *dockerRegistryHost,
		}
		members = append(members, grouper.Member{"docker_daemon", ifrit.RunFunc(dockerDaemon.Run)})
	}

	group := grouper.NewParallel(os.Interrupt, members)
	process := ifrit.Invoke(sigmon.New(group))

	fmt.Println("Staging process started ...")

	err := <-process.Wait()
	if err != nil {
		println("Staging process failed:", err.Error())
		os.Exit(2)
	}

	fmt.Println("Staging process finished")
}

func validateCredentials(server, user, password, email string) {
	missing := len(user) == 0 && len(password) == 0 && len(email) == 0
	present := len(user) > 0 && len(password) > 0 && len(email) > 0

	if missing {
		return
	}

	if !present {
		println("missing flags: dockerUser, dockerPassword and dockerEmail required simultaneously")
		os.Exit(1)
	}

	if !(strings.Contains(email, "@") && strings.Contains(email, ".")) {
		println(fmt.Sprintf("invalid dockerEmail [%s]", email))
		os.Exit(1)
	}

	if len(server) > 0 {
		_, err := url.Parse(server)
		if err != nil {
			println(fmt.Sprintf("invalid dockerLoginServer [%s]", server))
			os.Exit(1)
		}
	}
}

func (r *registries) String() string {
	return fmt.Sprint(*r)
}

func (r *registries) Set(value string) error {
	if len(*r) > 0 {
		return errors.New("Insecure Docker Registries flag already set")
	}
	for _, reg := range strings.Split(value, ",") {
		registry := strings.TrimSpace(reg)
		if strings.Contains(registry, "://") {
			return errors.New("no scheme allowed for Docker Registry [" + registry + "]")
		}
		if !strings.Contains(registry, ":") {
			return errors.New("ip:port expected for Docker Registry [" + registry + "]")
		}
		*r = append(*r, registry)
	}
	return nil
}

func (r *ips) String() string {
	return fmt.Sprint(*r)
}

func (r *ips) Set(value string) error {
	if len(*r) > 0 {
		return errors.New("Docker Registry IPs flag already set")
	}
	for _, el := range strings.Split(value, ",") {
		element := strings.TrimSpace(el)
		if strings.Contains(element, ":") {
			return errors.New("unexpected format for [" + element + "]")
		}
		*r = append(*r, element)
	}
	return nil
}
