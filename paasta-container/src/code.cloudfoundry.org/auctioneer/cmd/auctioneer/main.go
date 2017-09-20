package main

import (
	"errors"
	"flag"
	"fmt"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/hashicorp/consul/api"
	"github.com/nu7hatch/gouuid"

	"code.cloudfoundry.org/auctioneer"
	"code.cloudfoundry.org/auctioneer/auctionmetricemitterdelegate"
	"code.cloudfoundry.org/auctioneer/auctionrunnerdelegate"
	"code.cloudfoundry.org/auctioneer/handlers"
	"code.cloudfoundry.org/bbs"
	"code.cloudfoundry.org/cfhttp"
	"code.cloudfoundry.org/consuladapter"
	"code.cloudfoundry.org/debugserver"
	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/lager/lagerflags"
	"code.cloudfoundry.org/localip"
	"code.cloudfoundry.org/locket"
	"code.cloudfoundry.org/rep"

	"code.cloudfoundry.org/auction/auctionrunner"
	"code.cloudfoundry.org/auction/auctiontypes"
	"code.cloudfoundry.org/clock"
	"code.cloudfoundry.org/workpool"
	"github.com/cloudfoundry/dropsonde"
	"github.com/tedsuo/ifrit"
	"github.com/tedsuo/ifrit/grouper"
	"github.com/tedsuo/ifrit/http_server"
	"github.com/tedsuo/ifrit/sigmon"
)

var communicationTimeout = flag.Duration(
	"communicationTimeout",
	10*time.Second,
	"Timeout applied to all HTTP requests.",
)

var cellStateTimeout = flag.Duration(
	"cellStateTimeout",
	1*time.Second,
	"Timeout applied to HTTP requests to the Cell State endpoint.",
)

var consulCluster = flag.String(
	"consulCluster",
	"",
	"comma-separated list of consul server addresses (ip:port)",
)

var dropsondePort = flag.Int(
	"dropsondePort",
	3457,
	"port the local metron agent is listening on",
)

var lockTTL = flag.Duration(
	"lockTTL",
	locket.DefaultSessionTTL,
	"TTL for service lock",
)

var lockRetryInterval = flag.Duration(
	"lockRetryInterval",
	locket.RetryInterval,
	"interval to wait before retrying a failed lock acquisition",
)

var listenAddr = flag.String(
	"listenAddr",
	"0.0.0.0:9016",
	"host:port to serve auction and LRP stop requests on",
)

var bbsAddress = flag.String(
	"bbsAddress",
	"",
	"Address to the BBS Server",
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

var repRequireTLS = flag.Bool(
	"repRequireTLS",
	false,
	"whether tls connection to the rep is required or preferred",
)

var repCACert = flag.String(
	"repCACert",
	"",
	"path to certificate authority cert used for mutually authenticated TLS REP communication",
)

var repClientCert = flag.String(
	"repClientCert",
	"",
	"path to client cert used for mutually authenticated TLS REP communication",
)

var repClientKey = flag.String(
	"repClientKey",
	"",
	"path to client key used for mutually authenticated TLS REP communication",
)

var repClientSessionCacheSize = flag.Int(
	"repClientSessionCacheSize",
	0,
	"Capacity of the ClientSessionCache option on the TLS configuration. If zero, golang's default will be used",
)

var bbsMaxIdleConnsPerHost = flag.Int(
	"bbsMaxIdleConnsPerHost",
	0,
	"Controls the maximum number of idle (keep-alive) connctions per host. If zero, golang's default will be used",
)

var auctionRunnerWorkers = flag.Int(
	"auctionRunnerWorkers",
	1000,
	"Max concurrency for cell operations in the auction runner",
)

var startingContainerWeight = flag.Float64(
	"startingContainerWeight",
	0.25,
	"Factor to bias against cells with starting containers (0.0 - 1.0)",
)

const (
	auctionRunnerTimeout = 10 * time.Second
	dropsondeOrigin      = "auctioneer"
	serverProtocol       = "http"
)

func main() {
	debugserver.AddFlags(flag.CommandLine)
	lagerflags.AddFlags(flag.CommandLine)
	flag.Parse()

	cfhttp.Initialize(*communicationTimeout)

	logger, reconfigurableSink := lagerflags.New("auctioneer")
	initializeDropsonde(logger)

	if err := validateBBSAddress(); err != nil {
		logger.Fatal("invalid-bbs-address", err)
	}

	consulClient, err := consuladapter.NewClientFromUrl(*consulCluster)
	if err != nil {
		logger.Fatal("new-client-failed", err)
	}

	port, err := strconv.Atoi(strings.Split(*listenAddr, ":")[1])
	if err != nil {
		logger.Fatal("invalid-port", err)
	}

	clock := clock.NewClock()
	auctioneerServiceClient := auctioneer.NewServiceClient(consulClient, clock)

	auctionRunner := initializeAuctionRunner(logger, *cellStateTimeout,
		initializeBBSClient(logger), *startingContainerWeight)
	auctionServer := initializeAuctionServer(logger, auctionRunner)
	lockMaintainer := initializeLockMaintainer(logger, auctioneerServiceClient, port)
	registrationRunner := initializeRegistrationRunner(logger, consulClient, clock, port)

	members := grouper.Members{
		{"lock-maintainer", lockMaintainer},
		{"auction-runner", auctionRunner},
		{"auction-server", auctionServer},
		{"registration-runner", registrationRunner},
	}

	if dbgAddr := debugserver.DebugAddress(flag.CommandLine); dbgAddr != "" {
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

func initializeAuctionRunner(logger lager.Logger, cellStateTimeout time.Duration, bbsClient bbs.InternalClient, startingContainerWeight float64) auctiontypes.AuctionRunner {
	httpClient := cfhttp.NewClient()
	stateClient := cfhttp.NewCustomTimeoutClient(cellStateTimeout)
	repTLSConfig := &rep.TLSConfig{
		RequireTLS:      *repRequireTLS,
		CaCertFile:      *repCACert,
		CertFile:        *repClientCert,
		KeyFile:         *repClientKey,
		ClientCacheSize: *repClientSessionCacheSize,
	}
	repClientFactory, err := rep.NewClientFactory(httpClient, stateClient, repTLSConfig)
	if err != nil {
		logger.Fatal("new-rep-client-factory-failed", err)
	}

	delegate := auctionrunnerdelegate.New(repClientFactory, bbsClient, logger)
	metricEmitter := auctionmetricemitterdelegate.New()
	workPool, err := workpool.NewWorkPool(*auctionRunnerWorkers)
	if err != nil {
		logger.Fatal("failed-to-construct-auction-runner-workpool", err, lager.Data{"num-workers": *auctionRunnerWorkers}) // should never happen
	}

	return auctionrunner.New(
		logger,
		delegate,
		metricEmitter,
		clock.NewClock(),
		workPool,
		startingContainerWeight,
	)
}

func initializeDropsonde(logger lager.Logger) {
	dropsondeDestination := fmt.Sprint("localhost:", *dropsondePort)
	err := dropsonde.Initialize(dropsondeDestination, dropsondeOrigin)
	if err != nil {
		logger.Error("failed to initialize dropsonde: %v", err)
	}
}

func initializeAuctionServer(logger lager.Logger, runner auctiontypes.AuctionRunner) ifrit.Runner {
	return http_server.New(*listenAddr, handlers.New(runner, logger))
}

func initializeRegistrationRunner(logger lager.Logger, consulClient consuladapter.Client, clock clock.Clock, port int) ifrit.Runner {
	registration := &api.AgentServiceRegistration{
		Name: "auctioneer",
		Port: port,
		Check: &api.AgentServiceCheck{
			TTL: "3s",
		},
	}
	return locket.NewRegistrationRunner(logger, registration, consulClient, locket.RetryInterval, clock)
}

func initializeLockMaintainer(logger lager.Logger, serviceClient auctioneer.ServiceClient, port int) ifrit.Runner {
	uuid, err := uuid.NewV4()
	if err != nil {
		logger.Fatal("Couldn't generate uuid", err)
	}

	localIP, err := localip.LocalIP()
	if err != nil {
		logger.Fatal("Couldn't determine local IP", err)
	}

	address := fmt.Sprintf("%s://%s:%d", serverProtocol, localIP, port)
	auctioneerPresence := auctioneer.NewPresence(uuid.String(), address)

	lockMaintainer, err := serviceClient.NewAuctioneerLockRunner(logger, auctioneerPresence, *lockRetryInterval, *lockTTL)
	if err != nil {
		logger.Fatal("Couldn't create lock maintainer", err)
	}

	return lockMaintainer
}

func validateBBSAddress() error {
	if *bbsAddress == "" {
		return errors.New("bbsAddress is required")
	}
	return nil
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
