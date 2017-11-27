package main

import (
	"crypto/tls"
	"flag"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"

	"code.cloudfoundry.org/bbs"
	"code.cloudfoundry.org/clock"
	"code.cloudfoundry.org/consuladapter"
	"code.cloudfoundry.org/debugserver"
	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/lager/lagerflags"
	"code.cloudfoundry.org/locket"
	"code.cloudfoundry.org/tps/config"
	"code.cloudfoundry.org/tps/handler"
	"github.com/cloudfoundry/dropsonde"
	"github.com/cloudfoundry/noaa/consumer"
	"github.com/hashicorp/consul/api"
	"github.com/tedsuo/ifrit"
	"github.com/tedsuo/ifrit/grouper"
	"github.com/tedsuo/ifrit/http_server"
	"github.com/tedsuo/ifrit/sigmon"
)

var configPath = flag.String(
	"configPath",
	"",
	"path to config",
)

const (
	dropsondeOrigin = "tps_listener"
)

func main() {
	flag.Parse()

	listenerConfig, err := config.NewListenerConfig(*configPath)
	if err != nil {
		panic(err.Error())
	}

	logger, reconfigurableSink := lagerflags.NewFromConfig("tps-listener", listenerConfig.LagerConfig)

	initializeDropsonde(logger, listenerConfig)
	noaaClient := consumer.New(listenerConfig.TrafficControllerURL, &tls.Config{InsecureSkipVerify: listenerConfig.SkipCertVerify}, nil)
	defer noaaClient.Close()
	apiHandler := initializeHandler(logger, noaaClient, listenerConfig.MaxInFlightRequests, initializeBBSClient(logger, listenerConfig), listenerConfig)

	consulClient, err := consuladapter.NewClientFromUrl(listenerConfig.ConsulCluster)
	if err != nil {
		logger.Fatal("new-client-failed", err)
	}

	registrationRunner := initializeRegistrationRunner(logger, consulClient, listenerConfig.ListenAddress, clock.NewClock())

	members := grouper.Members{
		{"api", http_server.New(listenerConfig.ListenAddress, apiHandler)},
		{"registration-runner", registrationRunner},
	}

	if dbgAddr := listenerConfig.DebugServerConfig.DebugAddress; dbgAddr != "" {
		members = append(grouper.Members{
			{"debug-server", debugserver.Runner(dbgAddr, reconfigurableSink)},
		}, members...)
	}

	group := grouper.NewOrdered(os.Interrupt, members)

	monitor := ifrit.Invoke(sigmon.New(group))

	logger.Info("started")

	err = <-monitor.Wait()
	if err != nil {
		logger.Error("exited-with-failure", err)
		os.Exit(1)
	}

	logger.Info("exited")
}

func initializeDropsonde(logger lager.Logger, listenerConfig config.ListenerConfig) {
	dropsondeDestination := fmt.Sprint("localhost:", listenerConfig.DropsondePort)
	err := dropsonde.Initialize(dropsondeDestination, dropsondeOrigin)
	if err != nil {
		logger.Error("failed to initialize dropsonde: %v", err)
	}
}

func initializeHandler(logger lager.Logger, noaaClient *consumer.Consumer, maxInFlight int, apiClient bbs.Client, listenerConfig config.ListenerConfig) http.Handler {
	apiHandler, err := handler.New(apiClient, noaaClient, maxInFlight, listenerConfig.BulkLRPStatusWorkers, logger)
	if err != nil {
		logger.Fatal("initialize-handler.failed", err)
	}

	return apiHandler
}

func initializeBBSClient(logger lager.Logger, listenerConfig config.ListenerConfig) bbs.Client {
	bbsURL, err := url.Parse(listenerConfig.BBSAddress)
	if err != nil {
		logger.Fatal("Invalid BBS URL", err)
	}

	if bbsURL.Scheme != "https" {
		return bbs.NewClient(listenerConfig.BBSAddress)
	}

	bbsClient, err := bbs.NewSecureClient(
		listenerConfig.BBSAddress,
		listenerConfig.BBSCACert,
		listenerConfig.BBSClientCert,
		listenerConfig.BBSClientKey,
		0,
		listenerConfig.BBSMaxIdleConnsPerHost,
	)
	if err != nil {
		logger.Fatal("Failed to configure secure BBS client", err)
	}
	return bbsClient
}

func initializeRegistrationRunner(logger lager.Logger, consulClient consuladapter.Client, listenAddress string, clock clock.Clock) ifrit.Runner {
	_, portString, err := net.SplitHostPort(listenAddress)
	if err != nil {
		logger.Fatal("failed-invalid-listen-address", err)
	}
	portNum, err := net.LookupPort("tcp", portString)
	if err != nil {
		logger.Fatal("failed-invalid-listen-port", err)
	}

	registration := &api.AgentServiceRegistration{
		Name: "tps",
		Port: portNum,
		Check: &api.AgentServiceCheck{
			TTL: "20s",
		},
	}

	return locket.NewRegistrationRunner(logger, registration, consulClient, locket.RetryInterval, clock)
}
