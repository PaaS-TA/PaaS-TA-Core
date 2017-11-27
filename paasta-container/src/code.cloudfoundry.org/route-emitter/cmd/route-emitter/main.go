package main

import (
	"errors"
	"flag"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"time"

	"code.cloudfoundry.org/bbs"
	"code.cloudfoundry.org/cfhttp"
	"code.cloudfoundry.org/clock"
	"code.cloudfoundry.org/consuladapter"
	"code.cloudfoundry.org/debugserver"
	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/lager/lagerflags"
	route_emitter "code.cloudfoundry.org/route-emitter"
	"code.cloudfoundry.org/route-emitter/cmd/route-emitter/config"
	"code.cloudfoundry.org/route-emitter/consuldownchecker"
	"code.cloudfoundry.org/route-emitter/consuldownmodenotifier"
	"code.cloudfoundry.org/route-emitter/diegonats"
	"code.cloudfoundry.org/route-emitter/emitter"
	"code.cloudfoundry.org/route-emitter/routehandlers"
	"code.cloudfoundry.org/route-emitter/routingtable"
	"code.cloudfoundry.org/route-emitter/syncer"
	"code.cloudfoundry.org/route-emitter/watcher"
	"code.cloudfoundry.org/routing-api"
	uaaclient "code.cloudfoundry.org/uaa-go-client"
	uaaconfig "code.cloudfoundry.org/uaa-go-client/config"
	"code.cloudfoundry.org/workpool"
	"github.com/cloudfoundry/dropsonde"
	"github.com/nu7hatch/gouuid"
	"github.com/tedsuo/ifrit"
	"github.com/tedsuo/ifrit/grouper"
	"github.com/tedsuo/ifrit/http_server"
	"github.com/tedsuo/ifrit/sigmon"
)

var configFilePath = flag.String(
	"config",
	"",
	"Path to JSON configuration file",
)

const (
	dropsondeOrigin = "route_emitter"
)

func main() {
	flag.Parse()

	cfg, err := config.NewRouteEmitterConfig(*configFilePath)
	if err != nil {
		logger, _ := lagerflags.NewFromConfig("route-emitter", lagerflags.DefaultLagerConfig())
		logger.Fatal("failed-to-parse-config", err)
	}

	cfhttp.Initialize(time.Duration(cfg.CommunicationTimeout))

	logger, reconfigurableSink := lagerflags.NewFromConfig(cfg.ConsulSessionName, cfg.LagerConfig)
	natsClient := diegonats.NewClient()

	natsPingDuration := 20 * time.Second
	logger.Info("setting-nats-ping-interval", lager.Data{"duration-in-seconds": natsPingDuration.Seconds()})
	natsClient.SetPingInterval(natsPingDuration)

	clock := clock.NewClock()
	syncer := syncer.NewSyncer(clock, time.Duration(cfg.SyncInterval), natsClient, logger)

	initializeDropsonde(logger, cfg.DropsondePort)

	natsClientRunner := diegonats.NewClientRunner(cfg.NATSAddresses, cfg.NATSUsername, cfg.NATSPassword, logger, natsClient)

	bbsClient := initializeBBSClient(logger, cfg)

	localMode := cfg.CellID != ""
	table := routingtable.NewRoutingTable(logger, cfg.RegisterDirectInstanceRoutes)
	natsEmitter := initializeNatsEmitter(logger, natsClient, cfg.RouteEmittingWorkers)

	routeTTL := time.Duration(cfg.TCPRouteTTL)
	if routeTTL.Seconds() > 65535 {
		logger.Fatal("invalid-route-ttl", errors.New("route TTL value too large"), lager.Data{"ttl": routeTTL.Seconds()})
	}

	var routingAPIEmitter emitter.RoutingAPIEmitter
	if cfg.EnableTCPEmitter {
		tcpLogger := logger.Session("tcp")
		uaaClient := newUaaClient(tcpLogger, &cfg, clock)
		routingAPIAddress := fmt.Sprintf("%s:%d", cfg.RoutingAPI.URL, cfg.RoutingAPI.Port)
		logger.Debug("creating-routing-api-client", lager.Data{"api-location": routingAPIAddress})
		routingAPIClient := routing_api.NewClient(routingAPIAddress, false)
		routingAPIEmitter = emitter.NewRoutingAPIEmitter(tcpLogger, routingAPIClient, uaaClient, int(routeTTL.Seconds()))
	}

	handler := routehandlers.NewHandler(table, natsEmitter, routingAPIEmitter, localMode)

	watcher := watcher.NewWatcher(
		cfg.CellID,
		bbsClient,
		clock,
		handler,
		syncer.Events(),
		logger,
	)

	healthHandler := func(resp http.ResponseWriter, req *http.Request) {
		resp.WriteHeader(http.StatusOK)
	}
	healthCheckServer := http_server.New(cfg.HealthCheckAddress, http.HandlerFunc(healthHandler))
	members := grouper.Members{
		{"nats-client", natsClientRunner},
		{"healthcheck", healthCheckServer},
	}

	var consulClient consuladapter.Client
	var consulDownModeNotifier *consuldownmodenotifier.ConsulDownModeNotifier
	if cfg.CellID == "" {
		consulClient = initializeConsulClient(logger, cfg.ConsulCluster)

		lockMaintainer := initializeLockMaintainer(
			logger,
			consulClient,
			cfg.ConsulSessionName,
			time.Duration(cfg.LockTTL),
			time.Duration(cfg.LockRetryInterval),
			clock,
		)

		consulDownModeNotifier = consuldownmodenotifier.NewConsulDownModeNotifier(
			logger,
			0,
			clock,
			time.Duration(cfg.ConsulDownModeNotificationInterval),
		)

		// we are running in global mode
		members = append(members, grouper.Member{"lock-maintainer", lockMaintainer})
		members = append(members, grouper.Member{"consul-down-mode-notifier", consulDownModeNotifier})
	}

	members = append(members,
		grouper.Member{"watcher", watcher},
		grouper.Member{"syncer", syncer},
	)

	if cfg.DebugAddress != "" {
		members = append(grouper.Members{
			{"debug-server", debugserver.Runner(cfg.DebugAddress, reconfigurableSink)},
		}, members...)
	}

	group := grouper.NewOrdered(os.Interrupt, members)

	monitor := ifrit.Invoke(sigmon.New(group))

	logger.Info("started")

	err = <-monitor.Wait()
	if err != nil {
		logger.Error("finished-with-failure", err)
	} else {
		logger.Info("finished")
	}

	if cfg.CellID == "" {
		// ConsulDown mode
		logger = logger.Session("consul-down-mode")

		consulDownChecker := consuldownchecker.NewConsulDownChecker(
			logger,
			clock,
			consulClient,
			time.Duration(cfg.LockRetryInterval),
		)

		consulDownModeNotifier = consuldownmodenotifier.NewConsulDownModeNotifier(
			logger,
			1,
			clock,
			time.Duration(cfg.ConsulDownModeNotificationInterval),
		)
		// we are running in global mode
		members = grouper.Members{
			{"nats-client", natsClientRunner},
			{"consul-down-checker", consulDownChecker},
			{"consul-down-mode-notifier", consulDownModeNotifier},
			{"watcher", watcher},
			{"syncer", syncer},
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
	}

	logger.Info("exited")
}

func newUaaClient(logger lager.Logger, c *config.RouteEmitterConfig, klok clock.Clock) uaaclient.Client {
	if !c.RoutingAPI.AuthEnabled {
		logger.Debug("creating-noop-uaa-client")
		client := uaaclient.NewNoOpUaaClient()
		return client
	}

	logger.Debug("creating-uaa-client")
	cfg := uaaconfig.Config{
		UaaEndpoint:      c.OAuth.UaaURL,
		ClientName:       c.OAuth.ClientName,
		ClientSecret:     c.OAuth.ClientSecret,
		SkipVerification: c.OAuth.SkipCertVerify,
		CACerts:          c.OAuth.CACerts,
	}
	uaaClient, err := uaaclient.NewClient(logger, &cfg, klok)
	if err != nil {
		logger.Fatal("initialize-token-fetcher-error", err)
	}

	_, err = uaaClient.FetchKey()
	if err != nil {
		logger.Fatal("failed-fetching-uaa-key", err)
	}

	return uaaClient
}

func initializeDropsonde(logger lager.Logger, dropsondePort int) {
	dropsondeDestination := fmt.Sprint("localhost:", dropsondePort)
	err := dropsonde.Initialize(dropsondeDestination, dropsondeOrigin)
	if err != nil {
		logger.Error("failed to initialize dropsonde: %v", err)
	}
}

func initializeNatsEmitter(
	logger lager.Logger,
	natsClient diegonats.NATSClient,
	routeEmittingWorkers int,
) emitter.NATSEmitter {
	workPool, err := workpool.NewWorkPool(routeEmittingWorkers)
	if err != nil {
		logger.Fatal("failed-to-construct-nats-emitter-workpool", err, lager.Data{"num-workers": routeEmittingWorkers}) // should never happen
	}

	return emitter.NewNATSEmitter(natsClient, workPool, logger)
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

func initializeBBSClient(
	logger lager.Logger,
	cfg config.RouteEmitterConfig,
) bbs.Client {
	bbsURL, err := url.Parse(cfg.BBSAddress)
	if err != nil {
		logger.Fatal("Invalid BBS URL", err)
	}

	if bbsURL.Scheme != "https" {
		return bbs.NewClient(cfg.BBSAddress)
	}

	bbsClient, err := bbs.NewSecureClient(
		cfg.BBSAddress,
		cfg.BBSCACertFile,
		cfg.BBSClientCertFile,
		cfg.BBSClientKeyFile,
		cfg.BBSClientSessionCacheSize,
		cfg.BBSMaxIdleConnsPerHost,
	)
	if err != nil {
		logger.Fatal("Failed to configure secure BBS client", err)
	}
	return bbsClient
}
