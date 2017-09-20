package main

import (
	"bytes"
	"errors"
	"flag"
	"fmt"
	"io/ioutil"
	"net"
	"net/url"
	"os"
	"strings"
	"time"

	"code.cloudfoundry.org/bbs"
	"code.cloudfoundry.org/cfhttp"
	"code.cloudfoundry.org/clock"
	"code.cloudfoundry.org/consuladapter"
	"code.cloudfoundry.org/debugserver"
	"code.cloudfoundry.org/executor"
	executorinit "code.cloudfoundry.org/executor/initializer"
	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/lager/lagerflags"
	"code.cloudfoundry.org/localip"
	"code.cloudfoundry.org/locket"
	"code.cloudfoundry.org/operationq"
	"code.cloudfoundry.org/rep"
	"code.cloudfoundry.org/rep/auction_cell_rep"
	"code.cloudfoundry.org/rep/evacuation"
	"code.cloudfoundry.org/rep/evacuation/evacuation_context"
	"code.cloudfoundry.org/rep/generator"
	"code.cloudfoundry.org/rep/handlers"
	"code.cloudfoundry.org/rep/harmonizer"
	"code.cloudfoundry.org/rep/maintain"
	"github.com/cloudfoundry/dropsonde"
	"github.com/hashicorp/consul/api"
	"github.com/nu7hatch/gouuid"
	"github.com/tedsuo/ifrit"
	"github.com/tedsuo/ifrit/grouper"
	"github.com/tedsuo/ifrit/http_server"
	"github.com/tedsuo/ifrit/sigmon"
	"github.com/tedsuo/rata"

	GardenClient "code.cloudfoundry.org/garden/client"
)

const (
	dropsondeOrigin = "rep"
	bbsPingTimeout  = 5 * time.Minute
)

func main() {
	debugserver.AddFlags(flag.CommandLine)
	lagerflags.AddFlags(flag.CommandLine)

	stackMap := stackPathMap{}
	flag.Var(&stackMap, "preloadedRootFS", "List of preloaded RootFSes")

	supportedProviders := multiArgList{}
	flag.Var(&supportedProviders, "rootFSProvider", "List of RootFS providers")

	gardenHealthcheckEnv := commaSeparatedArgList{}
	flag.Var(&gardenHealthcheckEnv, "gardenHealthcheckProcessEnv", "Environment variables to use when running the garden health check")

	gardenHealthcheckArgs := commaSeparatedArgList{}
	flag.Var(&gardenHealthcheckArgs, "gardenHealthcheckProcessArgs", "List of command line args to pass to the garden health check process")

	placementTags := multiArgList{}
	flag.Var(&placementTags, "placementTag", "Placement tags used for scheduling Tasks and LRPs")

	optionalPlacementTags := multiArgList{}
	flag.Var(&optionalPlacementTags, "optionalPlacementTag", "Optional Placement tags used for scheduling Tasks and LRPs")

	flag.Parse()

	preloadedRootFSes := []string{}
	for k := range stackMap {
		preloadedRootFSes = append(preloadedRootFSes, k)
	}

	cfhttp.Initialize(*communicationTimeout)

	clock := clock.NewClock()
	logger, reconfigurableSink := lagerflags.New(*sessionName)

	var (
		executorConfiguration   executorinit.Configuration
		gardenHealthcheckRootFS string
		certBytes               []byte
		err                     error
	)

	if len(preloadedRootFSes) == 0 {
		gardenHealthcheckRootFS = ""
	} else {
		gardenHealthcheckRootFS = stackMap[preloadedRootFSes[0]]
	}

	if *pathToCACertsForDownloads != "" {
		certBytes, err = ioutil.ReadFile(*pathToCACertsForDownloads)
		if err != nil {
			logger.Error("failed-to-open-ca-cert-file", err)
			os.Exit(1)
		}

		certBytes = bytes.TrimSpace(certBytes)
	}

	executorConfiguration = executorConfig(certBytes, gardenHealthcheckRootFS, gardenHealthcheckArgs, gardenHealthcheckEnv)
	if !executorConfiguration.Validate(logger) {
		logger.Fatal("", errors.New("failed-to-configure-executor"))
	}

	initializeDropsonde(logger)

	if *cellID == "" {
		logger.Error("invalid-cell-id", errors.New("-cellID must be specified"))
		os.Exit(1)
	}

	//==================================================================================================================
	// gardenClient added
	//executorClient, executorMembers, err := executorinit.Initialize(logger, executorConfiguration, clock)
	executorClient, executorMembers, gardenClient, err := executorinit.Initialize(logger, executorConfiguration, clock)
	//==================================================================================================================

	if err != nil {
		logger.Error("failed-to-initialize-executor", err)
		os.Exit(1)
	}
	defer executorClient.Cleanup(logger)

	if err := validateBBSAddress(); err != nil {
		logger.Error("invalid-bbs-address", err)
		os.Exit(1)
	}

	consulClient, err := consuladapter.NewClientFromUrl(*consulCluster)
	if err != nil {
		logger.Fatal("new-client-failed", err)
	}

	serviceClient := bbs.NewServiceClient(consulClient, clock)

	evacuatable, evacuationReporter, evacuationNotifier := evacuation_context.New()

	// only one outstanding operation per container is necessary
	queue := operationq.NewSlidingQueue(1)

	evacuator := evacuation.NewEvacuator(
		logger,
		clock,
		executorClient,
		evacuationNotifier,
		*cellID,
		*evacuationTimeout,
		*evacuationPollingInterval,
	)

	bbsClient := initializeBBSClient(logger)

	//==================================================================================================================
	//gardenClient Added
	//httpServer, address := initializeServer(bbsClient, executorClient, evacuatable, evacuationReporter, logger, rep.StackPathMap(stackMap), supportedProviders, placementTags, optionalPlacementTags, false)
	//httpsServer, _ := initializeServer(bbsClient, executorClient, evacuatable, evacuationReporter, logger, rep.StackPathMap(stackMap), supportedProviders, placementTags, optionalPlacementTags, true)
	httpServer, address := initializeServer(bbsClient, executorClient, gardenClient, evacuatable, evacuationReporter, logger, rep.StackPathMap(stackMap), supportedProviders, placementTags, optionalPlacementTags, false)
	httpsServer, _ := initializeServer(bbsClient, executorClient, gardenClient, evacuatable, evacuationReporter, logger, rep.StackPathMap(stackMap), supportedProviders, placementTags, optionalPlacementTags, true)
	//==================================================================================================================
	opGenerator := generator.New(*cellID, bbsClient, executorClient, evacuationReporter, uint64(evacuationTimeout.Seconds()))
	cleanup := evacuation.NewEvacuationCleanup(logger, *cellID, bbsClient, executorClient, clock)

	_, portString, err := net.SplitHostPort(*listenAddr)
	if err != nil {
		logger.Fatal("failed-invalid-server-address", err)
	}
	portNum, err := net.LookupPort("tcp", portString)
	if err != nil {
		logger.Fatal("failed-invalid-server-port", err)
	}

	registrationRunner := initializeRegistrationRunner(logger, consulClient, portNum, clock)

	members := grouper.Members{
		{"presence", initializeCellPresence(address, serviceClient, executorClient, logger, supportedProviders, preloadedRootFSes, placementTags, optionalPlacementTags, true)},
		{"http_server", httpServer},
		{"https_server", httpsServer},
		{"evacuation-cleanup", cleanup},
		{"bulker", harmonizer.NewBulker(logger, *pollingInterval, *evacuationPollingInterval, evacuationNotifier, clock, opGenerator, queue)},
		{"event-consumer", harmonizer.NewEventConsumer(logger, opGenerator, queue)},
		{"evacuator", evacuator},
		{"registration-runner", registrationRunner},
	}

	members = append(executorMembers, members...)

	if dbgAddr := debugserver.DebugAddress(flag.CommandLine); dbgAddr != "" {
		members = append(grouper.Members{
			{"debug-server", debugserver.Runner(dbgAddr, reconfigurableSink)},
		}, members...)
	}

	group := grouper.NewOrdered(os.Interrupt, members)

	monitor := ifrit.Invoke(sigmon.New(group))

	logger.Info("started", lager.Data{"cell-id": *cellID})

	err = <-monitor.Wait()
	if err != nil {
		logger.Error("exited-with-failure", err)
		os.Exit(1)
	}

	logger.Info("exited")
}

func initializeDropsonde(logger lager.Logger) {
	dropsondeDestination := fmt.Sprint("localhost:", *dropsondePort)
	err := dropsonde.Initialize(dropsondeDestination, dropsondeOrigin)
	if err != nil {
		logger.Error("failed to initialize dropsonde: %v", err)
	}
}

func initializeCellPresence(
	address string,
	serviceClient bbs.ServiceClient,
	executorClient executor.Client,
	logger lager.Logger,
	rootFSProviders,
	preloadedRootFSes,
	placementTags []string,
	optionalPlacementTags []string,
	secure bool,
) ifrit.Runner {

	var repUrl string
	port := strings.Split(*listenAddrSecurable, ":")[1]
	if secure && *requireTLS {
		repUrl = fmt.Sprintf("https://%s:%s", repURL(), port)
	} else {
		repUrl = fmt.Sprintf("http://%s:%s", repURL(), port)
	}

	config := maintain.Config{
		CellID:                *cellID,
		RepAddress:            address,
		RepUrl:                repUrl,
		Zone:                  *zone,
		RetryInterval:         *lockRetryInterval,
		RootFSProviders:       rootFSProviders,
		PreloadedRootFSes:     preloadedRootFSes,
		PlacementTags:         placementTags,
		OptionalPlacementTags: optionalPlacementTags,
	}

	return maintain.New(logger, config, executorClient, serviceClient, *lockTTL, clock.NewClock())
}

//=================================================
// Add - parameter gardenClient GardenClient.Client
//=================================================
func initializeServer(
	bbsClient bbs.InternalClient,
	executorClient executor.Client,
        gardenClient GardenClient.Client,
	evacuatable evacuation_context.Evacuatable,
	evacuationReporter evacuation_context.EvacuationReporter,
	logger lager.Logger,
	stackMap rep.StackPathMap,
	supportedProviders []string,
	placementTags []string,
	optionalPlacementTags []string,
	secure bool,
) (ifrit.Runner, string) {
	auctionCellRep := auction_cell_rep.New(*cellID, stackMap, supportedProviders, *zone, generateGuid, executorClient, evacuationReporter, placementTags, optionalPlacementTags)

	//============================================================================
	//gardenClient Added
	//handlers := getHandlers(auctionCellRep, executorClient, evacuatable, logger, secure)
	handlers := getHandlers(auctionCellRep, executorClient, gardenClient, evacuatable, logger, secure)
	//============================================================================

	routes := getRoutes(secure)
	router, err := rata.NewRouter(routes, handlers)

	if err != nil {
		logger.Fatal("failed-to-construct-router", err)
	}

	ip, err := localip.LocalIP()
	if err != nil {
		logger.Fatal("failed-to-fetch-ip", err)
	}

	listenAddress := *listenAddr
	if secure {
		listenAddress = *listenAddrSecurable
	}
	port := strings.Split(listenAddress, ":")[1]
	address := fmt.Sprintf("http://%s:%s", ip, port)

	if secure && *requireTLS {
		tlsConfig, err := cfhttp.NewTLSConfig(*certFile, *keyFile, *caFile)
		if err != nil {
			logger.Fatal("tls-configuration-failed", err)
		}
		address = fmt.Sprintf("https://%s:%s", ip, port)
		return http_server.NewTLSServer(listenAddress, router, tlsConfig), address
	}

	return http_server.New(listenAddress, router), address
}

func validateBBSAddress() error {
	if *bbsAddress == "" {
		return errors.New("bbsAddress is required")
	}
	return nil
}

func generateGuid() (string, error) {
	guid, err := uuid.NewV4()
	if err != nil {
		return "", err
	}
	return guid.String(), nil
}

//=================================================
// Add - parameter gardenClient GardenClient.Client
//=================================================
func getHandlers(
	auctionCellRep rep.AuctionCellClient,
	executorClient executor.Client,
	gardenClient GardenClient.Client,
	evacuatable evacuation_context.Evacuatable,
	logger lager.Logger,
	isSecureServer bool) rata.Handlers {

	if *enableLegacyApiServer && !isSecureServer {
		//return handlers.NewLegacy(auctionCellRep, executorClient, evacuatable, logger)
		return handlers.NewLegacy(auctionCellRep, executorClient, gardenClient, evacuatable, logger)
	}
	//return handlers.New(auctionCellRep, executorClient, evacuatable, logger, isSecureServer)
	return handlers.New(auctionCellRep, executorClient, gardenClient, evacuatable, logger, isSecureServer)
}

func getRoutes(isSecureServer bool) rata.Routes {
	if *enableLegacyApiServer && !isSecureServer {
		return rep.Routes
	}
	return rep.NewRoutes(isSecureServer)
}

func initializeBBSClient(logger lager.Logger) bbs.InternalClient {
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

func repHost() string {
	return strings.Replace(*cellID, "_", "-", -1)
}

func repBaseHostName() string {
	return strings.Split(*advertiseDomain, ".")[0]
}

func repURL() string {
	return repHost() + "." + *advertiseDomain
}

func initializeRegistrationRunner(
	logger lager.Logger,
	consulClient consuladapter.Client,
	port int,
	clock clock.Clock) ifrit.Runner {
	registration := &api.AgentServiceRegistration{
		Name: repBaseHostName(),
		Port: port,
		Check: &api.AgentServiceCheck{
			TTL: "3s",
		},
		Tags: []string{repHost()},
	}
	return locket.NewRegistrationRunner(logger, registration, consulClient, locket.RetryInterval, clock)
}
