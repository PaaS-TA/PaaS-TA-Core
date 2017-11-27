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
	"code.cloudfoundry.org/clock"
	"code.cloudfoundry.org/consuladapter"
	"code.cloudfoundry.org/debugserver"
	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/lager/lagerflags"
	"code.cloudfoundry.org/locket"
	"code.cloudfoundry.org/runtimeschema/cc_messages"
	"code.cloudfoundry.org/runtimeschema/cc_messages/flags"
	"code.cloudfoundry.org/stager/backend"
	"code.cloudfoundry.org/stager/cc_client"
	"code.cloudfoundry.org/stager/config"
	"code.cloudfoundry.org/stager/handlers"
	"code.cloudfoundry.org/stager/vars"
)

var configPath = flag.String(
	"configPath",
	"",
	"path to the stager configuration file",
)

var insecureDockerRegistries = make(vars.StringList)

const (
	dropsondeOrigin = "stager"
)

func main() {
	flag.Parse()

	stagerConfig, err := config.NewStagerConfig(*configPath)
	if err != nil {
		panic(err.Error())
	}
	lifecycles := flags.LifecycleMap{}
	for _, value := range stagerConfig.Lifecycles {
		if err := lifecycles.Set(value); err != nil {
			panic(err.Error())
		}
	}

	logger, reconfigurableSink := lagerflags.NewFromConfig("stager", stagerConfig.LagerConfig)

	initializeDropsonde(logger, stagerConfig)

	ccClient := cc_client.NewCcClient(stagerConfig.CCBaseUrl, stagerConfig.CCUsername, stagerConfig.CCPassword, stagerConfig.SkipCertVerify)
	backends := initializeBackends(logger, lifecycles, stagerConfig)

	handler := handlers.New(logger, ccClient, initializeBBSClient(logger, stagerConfig), backends, clock.NewClock())

	clock := clock.NewClock()
	consulClient, err := consuladapter.NewClientFromUrl(stagerConfig.ConsulCluster)
	if err != nil {
		logger.Fatal("new-client-failed", err)
	}

	_, portString, err := net.SplitHostPort(stagerConfig.ListenAddress)
	if err != nil {
		logger.Fatal("failed-invalid-listen-address", err)
	}
	portNum, err := net.LookupPort("tcp", portString)
	if err != nil {
		logger.Fatal("failed-invalid-listen-port", err)
	}

	registrationRunner := initializeRegistrationRunner(logger, consulClient, portNum, clock)

	members := grouper.Members{
		{"server", http_server.New(stagerConfig.ListenAddress, handler)},
		{"registration-runner", registrationRunner},
	}

	if dbgAddr := stagerConfig.DebugServerConfig.DebugAddress; dbgAddr != "" {
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

func initializeDropsonde(logger lager.Logger, stagerConfig config.StagerConfig) {
	dropsondeDestination := fmt.Sprint("localhost:", stagerConfig.DropsondePort)
	err := dropsonde.Initialize(dropsondeDestination, dropsondeOrigin)
	if err != nil {
		logger.Error("failed to initialize dropsonde: %v", err)
	}
}

func initializeBackends(logger lager.Logger, lifecycles flags.LifecycleMap, stagerConfig config.StagerConfig) map[string]backend.Backend {
	_, err := url.Parse(stagerConfig.StagingTaskCallbackURL)
	if err != nil {
		logger.Fatal("Invalid staging task callback url", err)
	}
	if stagerConfig.DockerStagingStack == "" {
		logger.Fatal("Invalid Docker staging stack", errors.New("dockerStagingStack cannot be blank"))
	}

	_, err = url.Parse(stagerConfig.ConsulCluster)
	if err != nil {
		logger.Fatal("Error parsing consul agent URL", err)
	}
	config := backend.Config{
		TaskDomain:               cc_messages.StagingTaskDomain,
		StagerURL:                stagerConfig.StagingTaskCallbackURL,
		FileServerURL:            stagerConfig.FileServerUrl,
		CCUploaderURL:            stagerConfig.CCUploaderURL,
		Lifecycles:               lifecycles,
		InsecureDockerRegistries: insecureDockerRegistries.Values(),
		ConsulCluster:            stagerConfig.ConsulCluster,
		SkipCertVerify:           stagerConfig.SkipCertVerify,
		PrivilegedContainers:     stagerConfig.PrivilegedContainers,
		Sanitizer:                backend.SanitizeErrorMessage,
		DockerStagingStack:       stagerConfig.DockerStagingStack,
	}

	return map[string]backend.Backend{
		"buildpack": backend.NewTraditionalBackend(config, logger),
		"docker":    backend.NewDockerBackend(config, logger),
	}
}

func initializeBBSClient(logger lager.Logger, stagerConfig config.StagerConfig) bbs.Client {
	bbsURL, err := url.Parse(stagerConfig.BBSAddress)
	if err != nil {
		logger.Fatal("Invalid BBS URL", err)
	}

	if bbsURL.Scheme != "https" {
		return bbs.NewClient(stagerConfig.BBSAddress)
	}

	bbsClient, err := bbs.NewSecureClient(stagerConfig.BBSAddress, stagerConfig.BBSCACert, stagerConfig.BBSClientCert, stagerConfig.BBSClientKey, stagerConfig.BBSClientSessionCacheSize, stagerConfig.BBSMaxIdleConnsPerHost)
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
			TTL: "20s",
		},
	}
	return locket.NewRegistrationRunner(logger, registration, consulClient, locket.RetryInterval, clock)
}
