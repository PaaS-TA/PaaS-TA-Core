package main

import (
	"flag"
	"fmt"
	"net/url"
	"os"
	"time"

	"code.cloudfoundry.org/bbs"
	"code.cloudfoundry.org/cfhttp"
	"code.cloudfoundry.org/cflager"
	"code.cloudfoundry.org/clock"
	"code.cloudfoundry.org/consuladapter"
	"code.cloudfoundry.org/debugserver"
	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/locket"
	route_emitter "code.cloudfoundry.org/route-emitter"
	"code.cloudfoundry.org/route-emitter/consuldownchecker"
	"code.cloudfoundry.org/route-emitter/consuldownmodenotifier"
	"code.cloudfoundry.org/route-emitter/diegonats"
	"code.cloudfoundry.org/route-emitter/nats_emitter"
	"code.cloudfoundry.org/route-emitter/routing_table"
	"code.cloudfoundry.org/route-emitter/syncer"
	"code.cloudfoundry.org/route-emitter/watcher"
	"code.cloudfoundry.org/workpool"
	"github.com/cloudfoundry/dropsonde"
	"github.com/nu7hatch/gouuid"
	"github.com/tedsuo/ifrit"
	"github.com/tedsuo/ifrit/grouper"
	"github.com/tedsuo/ifrit/sigmon"
)

var bbsAddress = flag.String(
	"bbsAddress",
	"",
	"Address of the BBS API Server",
)

var sessionName = flag.String(
	"sessionName",
	"route-emitter",
	"consul session name",
)

var consulCluster = flag.String(
	"consulCluster",
	"",
	"comma-separated list of consul server URLs (scheme://ip:port)",
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

var natsAddresses = flag.String(
	"natsAddresses",
	"127.0.0.1:4222",
	"comma-separated list of NATS addresses (ip:port)",
)

var natsUsername = flag.String(
	"natsUsername",
	"nats",
	"Username to connect to nats",
)

var natsPassword = flag.String(
	"natsPassword",
	"nats",
	"Password for nats user",
)

var syncInterval = flag.Duration(
	"syncInterval",
	time.Minute,
	"the interval between syncs of the routing table from etcd",
)

var consulDownModeNotificationInterval = flag.Duration(
	"consulDownModeNotificationInterval",
	time.Minute,
	"the interval between emission of the ConsulDownMode metric",
)

var dropsondePort = flag.Int(
	"dropsondePort",
	3457,
	"port the local metron agent is listening on",
)

var communicationTimeout = flag.Duration(
	"communicationTimeout",
	30*time.Second,
	"Timeout applied to all HTTP requests.",
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

var routeEmittingWorkers = flag.Int(
	"routeEmittingWorkers",
	20,
	"Max concurrency for sending route messages",
)

const (
	dropsondeOrigin = "route_emitter"
)

func main() {
	debugserver.AddFlags(flag.CommandLine)
	cflager.AddFlags(flag.CommandLine)
	flag.Parse()

	cfhttp.Initialize(*communicationTimeout)

	logger, reconfigurableSink := cflager.New(*sessionName)
	natsClient := diegonats.NewClient()

	natsPingduration := 20 * time.Second
	logger.Info("setting-nats-ping-interval", lager.Data{"duration-in-seconds": natsPingduration.Seconds()})
	natsClient.SetPingInterval(natsPingduration)

	clock := clock.NewClock()
	syncer := syncer.NewSyncer(clock, *syncInterval, natsClient, logger)

	initializeDropsonde(logger)

	natsClientRunner := diegonats.NewClientRunner(*natsAddresses, *natsUsername, *natsPassword, logger, natsClient)

	table := initializeRoutingTable(logger)
	emitter := initializeNatsEmitter(natsClient, logger)
	watcher := ifrit.RunFunc(func(signals <-chan os.Signal, ready chan<- struct{}) error {
		return watcher.NewWatcher(initializeBBSClient(logger), clock, table, emitter, syncer.Events(), logger).Run(signals, ready)
	})

	syncRunner := ifrit.RunFunc(func(signals <-chan os.Signal, ready chan<- struct{}) error {
		return syncer.Run(signals, ready)
	})

	consulClient := initializeConsulClient(logger, *consulCluster)

	lockMaintainer := initializeLockMaintainer(logger, consulClient, *sessionName, *lockTTL, *lockRetryInterval, clock)
	consulDownModeNotifier := consuldownmodenotifier.NewConsulDownModeNotifier(logger, 0, clock, *consulDownModeNotificationInterval)

	members := grouper.Members{
		{"lock-maintainer", lockMaintainer},
		{"consul-down-mode-notifier", consulDownModeNotifier},
		{"nats-client", natsClientRunner},
		{"watcher", watcher},
		{"syncer", syncRunner},
	}

	if dbgAddr := debugserver.DebugAddress(flag.CommandLine); dbgAddr != "" {
		members = append(grouper.Members{
			{"debug-server", debugserver.Runner(dbgAddr, reconfigurableSink)},
		}, members...)
	}

	group := grouper.NewOrdered(os.Interrupt, members)

	monitor := ifrit.Invoke(sigmon.New(group))

	logger.Info("started")

	err := <-monitor.Wait()
	if err != nil {
		logger.Error("finished-with-failure", err)
	} else {
		logger.Info("finished")
	}

	// ConsulDown mode

	logger = logger.Session("consul-down-mode")

	consulDownChecker := consuldownchecker.NewConsulDownChecker(logger, clock, consulClient, *lockRetryInterval)

	consulDownModeNotifier = consuldownmodenotifier.NewConsulDownModeNotifier(logger, 1, clock, *consulDownModeNotificationInterval)

	members = grouper.Members{
		{"consul-down-checker", consulDownChecker},
		{"consul-down-mode-notifier", consulDownModeNotifier},
		{"nats-client", natsClientRunner},
		{"watcher", watcher},
		{"syncer", syncRunner},
	}

	group = grouper.NewOrdered(os.Interrupt, members)

	logger.Info("starting")

	monitor = ifrit.Invoke(sigmon.New(group))

	logger.Info("started")

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

func initializeNatsEmitter(natsClient diegonats.NATSClient, logger lager.Logger) nats_emitter.NATSEmitter {
	workPool, err := workpool.NewWorkPool(*routeEmittingWorkers)
	if err != nil {
		logger.Fatal("failed-to-construct-nats-emitter-workpool", err, lager.Data{"num-workers": *routeEmittingWorkers}) // should never happen
	}

	return nats_emitter.New(natsClient, workPool, logger)
}

func initializeRoutingTable(logger lager.Logger) routing_table.RoutingTable {
	return routing_table.NewTable(logger)
}

func initializeConsulClient(logger lager.Logger, consulCluster string) consuladapter.Client {
	consulClient, err := consuladapter.NewClientFromUrl(consulCluster)
	if err != nil {
		logger.Fatal("new-client-failed", err)
	}
	return consulClient
}

func initializeLockMaintainer(
	logger lager.Logger,
	consulClient consuladapter.Client,
	sessionName string,
	lockTTL, lockRetryInterval time.Duration,
	clock clock.Clock,
) ifrit.Runner {
	uuid, err := uuid.NewV4()
	if err != nil {
		logger.Fatal("Couldn't generate uuid", err)
	}

	serviceClient := route_emitter.NewServiceClient(consulClient, clock)

	return serviceClient.NewRouteEmitterLockRunner(logger, uuid.String(), lockRetryInterval, lockTTL)
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
