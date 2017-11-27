package helpers

import (
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"path"
	"strings"
	"time"

	"golang.org/x/net/context"

	"code.cloudfoundry.org/dockerapplifecycle"
	"code.cloudfoundry.org/dockerapplifecycle/docker/nat"
	"code.cloudfoundry.org/dockerapplifecycle/protocol"
	"github.com/docker/distribution/registry/client"
	"github.com/docker/distribution/registry/client/auth"
	"github.com/docker/distribution/registry/client/transport"
)

const (
	DockerHubHostname    = "registry-1.docker.io"
	DockerHubLoginServer = "https://index.docker.io/v1/"
)

type Config struct {
	User         string
	ExposedPorts map[nat.Port]struct{}
	Cmd          []string
	WorkingDir   string
	Entrypoint   []string
}

type Image struct {
	Config *Config `json:"config,omitempty"`
}

// borrowed from docker/docker
func splitReposName(reposName string) (string, string) {
	nameParts := strings.SplitN(reposName, "/", 2)
	var hostname, repoName string
	if len(nameParts) == 1 || (!strings.Contains(nameParts[0], ".") &&
		!strings.Contains(nameParts[0], ":") && nameParts[0] != "localhost") {
		// This is a Docker Index repos (ex: samalba/hipache or ubuntu)
		// 'docker.io' in docker/docker codebase, but they use indices...
		hostname = DockerHubHostname
		repoName = reposName
	} else {
		hostname = nameParts[0]
		repoName = nameParts[1]
	}
	return hostname, repoName
}

// For standard docker image references expressed as a protocol-less string
// returns RegistryURL, repoName, tag|digest
func ParseDockerRef(dockerRef string) (string, string, string) {
	remote, tag := ParseRepositoryTag(dockerRef)
	hostname, repoName := splitReposName(remote)

	if hostname == DockerHubHostname && !strings.Contains(repoName, "/") {
		repoName = "library/" + repoName
	}

	if len(tag) == 0 {
		tag = "latest"
	}
	return hostname, repoName, tag
}

// stolen from docker/docker
// Get a repos name and returns the right reposName + tag
// The tag can be confusing because of a port in a repository name.
//     Ex: localhost.localdomain:5000/samalba/hipache:latest
func ParseRepositoryTag(repos string) (string, string) {
	n := strings.LastIndex(repos, ":")
	if n < 0 {
		return repos, ""
	}
	if tag := repos[n+1:]; !strings.Contains(tag, "/") {
		return repos[:n], tag
	}
	return repos, ""
}

func FetchMetadata(registryURL string, repoName string, tag string, insecureRegistries []string, credentials auth.CredentialStore, stderr io.Writer) (*Image, error) {
	scheme := "https"
	var err error
	transport, err := makeTransport(scheme, registryURL, repoName, insecureRegistries, credentials, stderr)
	if err != nil {
		scheme = "http"
		transport, err = makeTransport(scheme, registryURL, repoName, insecureRegistries, credentials, stderr)
		if err != nil {
			return nil, err
		}
	}

	repoClient, err := client.NewRepository(context.TODO(), repoName, scheme+"://"+registryURL, transport)
	if err != nil {
		fmt.Fprintln(stderr, "Failed making docker repository client:", err)
		return nil, err
	}

	manifestService, err := repoClient.Manifests(context.TODO())
	if err != nil {
		fmt.Fprintln(stderr, "Failed getting docker image manifests:", err)
		return nil, err
	}

	// var manifest *manifest.SignedManifest
	// var err2 error
	manifest, err := manifestService.GetByTag(tag)
	for i := 0; i <= 3; i++ {
		if err != nil {
			if i < 3 {
				fmt.Fprintln(stderr, "Failed getting docker image by tag:", err, " Going to retry attempt:", i+1)
				manifest, err = manifestService.GetByTag(tag)
				continue
			}
			fmt.Fprintln(stderr, "Failed getting docker image by tag:", err)
			return nil, err
		} else {
			break
		}
	}

	var image Image
	err = json.Unmarshal([]byte(manifest.History[0].V1Compatibility), &image)
	if err != nil {
		fmt.Fprintln(stderr, "Failed parsing docker image JSON:", err)
		return nil, err
	}

	return &image, nil
}

func SaveMetadata(filename string, metadata *protocol.DockerImageMetadata) error {
	err := os.MkdirAll(path.Dir(filename), 0755)
	if err != nil {
		return err
	}

	executionMetadataJSON, err := json.Marshal(metadata.ExecutionMetadata)
	if err != nil {
		return err
	}

	resultFile, err := os.Create(filename)
	if err != nil {
		return err
	}

	defer resultFile.Close()

	startCommand := strings.Join(metadata.ExecutionMetadata.Cmd, " ")
	if len(metadata.ExecutionMetadata.Entrypoint) > 0 {
		startCommand = strings.Join([]string{strings.Join(metadata.ExecutionMetadata.Entrypoint, " "), startCommand}, " ")
	}

	err = json.NewEncoder(resultFile).Encode(dockerapplifecycle.NewStagingResult(
		dockerapplifecycle.ProcessTypes{
			"web": startCommand,
		},
		dockerapplifecycle.LifecycleMetadata{
			DockerImage: metadata.DockerImage,
		},
		string(executionMetadataJSON),
	))
	if err != nil {
		return err
	}

	return nil
}

func makeTransport(scheme, registryURL, repository string, insecureRegistries []string, credentialStore auth.CredentialStore, stderr io.Writer) (http.RoundTripper, error) {
	baseTransport := &http.Transport{
		Proxy: http.ProxyFromEnvironment,
		Dial: (&net.Dialer{
			Timeout:   30 * time.Second,
			KeepAlive: 30 * time.Second,
			DualStack: true,
		}).Dial,
		DisableKeepAlives: true,
	}

	if scheme == "https" {
		secure := true
		for _, insecureRegistry := range insecureRegistries {
			if registryURL == insecureRegistry {
				secure = false
				break
			}
		}

		if !secure {
			baseTransport.TLSClientConfig = &tls.Config{InsecureSkipVerify: true}
		}
	}

	authTransport := transport.NewTransport(baseTransport)

	pingClient := &http.Client{
		Transport: authTransport,
		Timeout:   5 * time.Second,
	}

	req, err := http.NewRequest("GET", scheme+"://"+registryURL+"/v2/", nil)
	if err != nil {
		return nil, err
	}
	challengeManager := auth.NewSimpleChallengeManager()

	resp, err := pingClient.Do(req)
	if err != nil {
		fmt.Fprintln(stderr, "Failed to talk to docker registry:", err)
		return nil, err
	} else {
		defer resp.Body.Close()
		if err := challengeManager.AddResponse(resp); err != nil {
			fmt.Fprintln(stderr, "Failed to talk to docker registry:", err)
			return nil, err
		}
	}

	tokenHandler := auth.NewTokenHandler(authTransport, credentialStore, repository, "pull")
	basicHandler := auth.NewBasicHandler(credentialStore)
	authorizer := auth.NewAuthorizer(challengeManager, tokenHandler, basicHandler)

	return transport.NewTransport(baseTransport, authorizer), nil
}
