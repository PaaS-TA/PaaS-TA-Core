package main

import (
	"errors"
	"flag"
	"fmt"
	"net"
	"net/url"
	"os"

	"github.com/cloudfoundry/dropsonde"
	"github.com/hashicorp/consul/api"
	"github.com/tedsuo/ifrit"
	"github.com/tedsuo/ifrit/grouper"
	"github.com/tedsuo/ifrit/http_server"
	"github.com/tedsuo/ifrit/sigmon"

	"code.cloudfoundry.org/bbs"
	"code.cloudfoundry.org/cflager"
	"code.cloudfoundry.org/clock"
	"code.cloudfoundry.org/consuladapter"
	"code.cloudfoundry.org/debugserver"
	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/locket"
	"code.cloudfoundry.org/runtimeschema/cc_messages"
	"code.cloudfoundry.org/runtimeschema/cc_messages/flags"
	"code.cloudfoundry.org/stager/backend"
	"code.cloudfoundry.org/stager/cc_client"
	"code.cloudfoundry.org/stager/handlers"
	"code.cloudfoundry.org/stager/vars"
)

var ccBaseURL = flag.String(
	"ccBaseURL",
	"",
	"URI to acccess the Cloud Controller",
)

var ccUsername = flag.String(
	"ccUsername",
	"",
	"Basic auth username for CC internal API",
)

var ccPassword = flag.String(
	"ccPassword",
	"",
	"Basic auth password for CC internal API",
)

var privilegedContainers = flag.Bool(
	"privilegedContainers",
	false,
	"Whether or not to use privileged containers for  buildpack based LRPs and tasks. Containers with a docker-image-based rootfs will continue to always be unprivileged and cannot be changed.",
)

var skipCertVerify = flag.Bool(
	"skipCertVerify",
	false,
	"skip SSL certificate verification",
)

var dropsondePort = flag.Int(
	"dropsondePort",
	3457,
	"port the local metron agent is listening on",
)

var bbsAddress = flag.String(
	"bbsAddress",
	"",
	"Address to the BBS Server",
)

var listenAddress = flag.String(
	"listenAddress",
	"",
	"Address from which the Stager serves requests",
)

var stagingTaskCallbackURL = flag.String(
	"stagingTaskCallbackURL",
	"",
	"URL for staging task callbacks",
)

var fileServerURL = flag.String(
	"fileServerURL",
	"",
	"URL of the file server",
)

var ccUploaderURL = flag.String(
	"ccUploaderURL",
	"",
	"URL of the cc uploader",
)

var dockerRegistryAddress = flag.String(
	"dockerRegistryAddress",
	"",
	"Address (host:port) of the docker registry",
)

var consulCluster = flag.String(
	"consulCluster",
	"",
	"Consul Agent URL",
)

var dockerStagingStack = flag.String(
	"dockerStagingStack",
	"",
	"Stack to use for staging Docker applications",
)

var bbsCACert = flag.String(
	"bbsCACert",
	"",
	"path to certificate authority cert used for mutually authenticated TLS BBS communication",
)

var bbsClientCert = flag.String(
	"bbsClientCert",
	"",
	"path to client cert used for mutually authenticated TLS BBS communication",
)

var bbsClientKey = flag.String(
	"bbsClientKey",
	"",
	"path to client key used for mutually authenticated TLS BBS communication",
)

var bbsClientSessionCacheSize = flag.Int(
	"bbsClientSessionCacheSize",
	0,
	"Capacity of the ClientSessionCache option on the TLS configuration. If zero, golang's default will be used",
)

var bbsMaxIdleConnsPerHost = flag.Int(
	"bbsMaxIdleConnsPerHost",
	0,
	"Controls the maximum number of idle (keep-alive) connctions per host. If zero, golang's default will be used",
)

var insecureDockerRegistries = make(vars.StringList)

const (
	dropsondeOrigin = "stager"
)

func main() {
	debugserver.AddFlags(flag.CommandLine)
	cflager.AddFlags(flag.CommandLine)

	flag.Var(
		&insecureDockerRegistries,
		"insecureDockerRegistry",
		"Docker registry to allow connecting to even if not secure. (Can be specified multiple times to allow insecure connection to multiple repositories)",
	)

	lifecycles := flags.LifecycleMap{}
	flag.Var(&lifecycles, "lifecycle", "app lifecycle binary bundle mapping (lifecycle[/stack]:bundle-filepath-in-fileserver)")
	flag.Parse()

	logger, reconfigurableSink := cflager.New("stager")
	initializeDropsonde(logger)

	ccClient := cc_client.NewCcClient(*ccBaseURL, *ccUsername, *ccPassword, *skipCertVerify)

	backends := initializeBackends(logger, lifecycles)

	handler := handlers.New(logger, ccClient, initializeBBSClient(logger), backends, clock.NewClock())

	clock := clock.NewClock()
	consulClient, err := consuladapter.NewClientFromUrl(*consulCluster)
	if err != nil {
		logger.Fatal("new-client-failed", err)
	}

	_, portString, err := net.SplitHostPort(*listenAddress)
	if err != nil {
		logger.Fatal("failed-invalid-listen-address", err)
	}
	portNum, err := net.LookupPort("tcp", portString)
	if err != nil {
		logger.Fatal("failed-invalid-listen-port", err)
	}

	registrationRunner := initializeRegistrationRunner(logger, consulClient, portNum, clock)

	members := grouper.Members{
		{"server", http_server.New(*listenAddress, handler)},
		{"registration-runner", registrationRunner},
	}

	if dbgAddr := debugserver.DebugAddress(flag.CommandLine); dbgAddr != "" {
		members = append(grouper.Members{
			{"debug-server", debugserver.Runner(dbgAddr, reconfigurableSink)},
		}, members...)
	}

	logger.Info("starting")

	group := grouper.NewOrdered(os.Interrupt, members)

	process := ifrit.Invoke(sigmon.New(group))

	logger.Info("Listening for staging requests!")

	err = <-process.Wait()
	if err != nil {
		logger.Fatal("Stager exited with error", err)
	}

	logger.Info("stopped")
}

func initializeDropsonde(logger lager.Logger) {
	dropsondeDestination := fmt.Sprint("localhost:", *dropsondePort)
	err := dropsonde.Initialize(dropsondeDestination, dropsondeOrigin)
	if err != nil {
		logger.Error("failed to initialize dropsonde: %v", err)
	}
}

func initializeBackends(logger lager.Logger, lifecycles flags.LifecycleMap) map[string]backend.Backend {
	_, err := url.Parse(*stagingTaskCallbackURL)
	if err != nil {
		logger.Fatal("Invalid staging task callback url", err)
	}
	if *dockerStagingStack == "" {
		logger.Fatal("Invalid Docker staging stack", errors.New("dockerStagingStack cannot be blank"))
	}

	_, err = url.Parse(*consulCluster)
	if err != nil {
		logger.Fatal("Error parsing consul agent URL", err)
	}
	_, err = url.Parse(*dockerRegistryAddress)
	if err != nil {
		logger.Fatal("Error parsing Docker Registry address", err)
	}

	config := backend.Config{
		TaskDomain:               cc_messages.StagingTaskDomain,
		StagerURL:                *stagingTaskCallbackURL,
		FileServerURL:            *fileServerURL,
		CCUploaderURL:            *ccUploaderURL,
		Lifecycles:               lifecycles,
		DockerRegistryAddress:    *dockerRegistryAddress,
		InsecureDockerRegistries: insecureDockerRegistries.Values(),
		ConsulCluster:            *consulCluster,
		SkipCertVerify:           *skipCertVerify,
		PrivilegedContainers:     *privilegedContainers,
		Sanitizer:                backend.SanitizeErrorMessage,
		DockerStagingStack:       *dockerStagingStack,
	}

	return map[string]backend.Backend{
		"buildpack": backend.NewTraditionalBackend(config, logger),
		"docker":    backend.NewDockerBackend(config, logger),
	}
}

func initializeBBSClient(logger lager.Logger) bbs.Client {
	bbsURL, err := url.Parse(*bbsAddress)
	if err != nil {
		logger.Fatal("Invalid BBS URL", err)
	}

	if bbsURL.Scheme != "https" {
		return bbs.NewClient(*bbsAddress)
	}

	bbsClient, err := bbs.NewSecureClient(*bbsAddress, *bbsCACert, *bbsClientCert, *bbsClientKey, *bbsClientSessionCacheSize, *bbsMaxIdleConnsPerHost)
	if err != nil {
		logger.Fatal("Failed to configure secure BBS client", err)
	}
	return bbsClient
}

func initializeRegistrationRunner(logger lager.Logger, consulClient consuladapter.Client, port int, clock clock.Clock) ifrit.Runner {
	registration := &api.AgentServiceRegistration{
		Name: "stager",
		Port: port,
		Check: &api.AgentServiceCheck{
			TTL: "3s",
		},
	}
	return locket.NewRegistrationRunner(logger, registration, consulClient, locket.RetryInterval, clock)
}
