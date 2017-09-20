package main

import (
	"errors"
	"flag"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"time"

	"code.cloudfoundry.org/debugserver"
	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/routing-api"
	"code.cloudfoundry.org/routing-api/config"
	"code.cloudfoundry.org/routing-api/db"
	"code.cloudfoundry.org/routing-api/handlers"
	"code.cloudfoundry.org/routing-api/helpers"
	"code.cloudfoundry.org/routing-api/metrics"
	"code.cloudfoundry.org/routing-api/models"
	uaaclient "code.cloudfoundry.org/uaa-go-client"
	uaaconfig "code.cloudfoundry.org/uaa-go-client/config"
	"github.com/cactus/go-statsd-client/statsd"
	"github.com/cloudfoundry/dropsonde"
	"github.com/nu7hatch/gouuid"

	cf_lager "code.cloudfoundry.org/cflager"
	"code.cloudfoundry.org/clock"
	"github.com/tedsuo/ifrit"
	"github.com/tedsuo/ifrit/grouper"
	"github.com/tedsuo/ifrit/http_server"
	"github.com/tedsuo/ifrit/sigmon"

	"github.com/tedsuo/rata"
)

const DEFAULT_ETCD_WORKERS = 25

var port = flag.Uint("port", 8080, "Port to run rounting-api server on")
var configPath = flag.String("config", "", "Configuration for routing-api")
var devMode = flag.Bool("devMode", false, "Disable authentication for easier development iteration")
var ip = flag.String("ip", "", "The public ip of the routing api")

func route(f func(w http.ResponseWriter, r *http.Request)) http.Handler {
	return http.HandlerFunc(f)
}

func main() {
	cf_lager.AddFlags(flag.CommandLine)
	flag.Parse()

	logger, reconfigurableSink := cf_lager.New("routing-api")

	err := checkFlags()
	if err != nil {
		logger.Error("failed-check-flags", err)
		os.Exit(1)
	}

	cfg, err := config.NewConfigFromFile(*configPath, *devMode)
	if err != nil {
		logger.Error("failed-load-config", err)
		os.Exit(1)
	}

	err = dropsonde.Initialize(cfg.MetronConfig.Address+":"+cfg.MetronConfig.Port, cfg.LogGuid)
	if err != nil {
		logger.Error("failed-initialize-dropsonde", err)
		os.Exit(1)
	}

	if cfg.DebugAddress != "" {
		_, err := debugserver.Run(cfg.DebugAddress, reconfigurableSink)
		if err != nil {
			logger.Error("failed-debug-server", err, lager.Data{"debug_address": cfg.DebugAddress})
		}
	}

	var sqlDatabase db.DB
	var etcdDatabase db.DB

	// Use SQL database if available, otherwise use ETCD
	if cfg.SqlDB.Host != "" && cfg.SqlDB.Port > 0 && cfg.SqlDB.Schema != "" {
		data := lager.Data{"host": cfg.SqlDB.Host, "port": cfg.SqlDB.Port}
		logger.Info("sql-database", data)
		sqlDatabase, err = db.NewSqlDB(&cfg.SqlDB)
		if err != nil {
			logger.Error("failed-initialize-sql-connection", err, data)
		}
	}

	logger.Info("database", lager.Data{"etcd-addresses": cfg.Etcd.NodeURLS})
	etcdDatabase, err = db.NewETCD(cfg.Etcd)
	if err != nil {
		logger.Error("failed-initialize-etcd-db", err)
		os.Exit(1)
	}

	err = etcdDatabase.Connect()
	if err != nil {
		logger.Error("failed-etcd-connection", err)
		os.Exit(1)
	}
	defer etcdDatabase.CancelWatches()

	database, err := db.NewJointDB(etcdDatabase, sqlDatabase)
	if err != nil {
		logger.Error("failed-initialize-database", err)
		os.Exit(1)
	}
	seedRouterGroups(cfg, logger, database)

	prefix := "routing_api"
	statsdClient, err := statsd.NewBufferedClient(cfg.StatsdEndpoint, prefix, cfg.StatsdClientFlushInterval, 512)
	if err != nil {
		logger.Error("failed to create a statsd client", err)
		os.Exit(1)
	}
	defer statsdClient.Close()

	apiServer := constructApiServer(cfg, database, statsdClient, logger.Session("api-server"))
	stopper := constructStopper(database)

	routerRegister := constructRouteRegister(
		cfg.LogGuid,
		cfg.SystemDomain,
		cfg.MaxTTL,
		database,
		logger.Session("route-register"),
	)

	metricsTicker := time.NewTicker(cfg.MetricsReportingInterval)
	metricsReporter := metrics.NewMetricsReporter(database, statsdClient, metricsTicker, logger.Session("metrics"))

	members := grouper.Members{
		{"api-server", apiServer},
		{"conn-stopper", stopper},
		{"metrics", metricsReporter},
		{"route-register", routerRegister},
	}

	group := grouper.NewOrdered(os.Interrupt, members)
	process := ifrit.Invoke(sigmon.New(group))

	// This is used by testrunner to signal ready for tests.
	logger.Info("started", lager.Data{"port": *port})

	errChan := process.Wait()
	err = <-errChan
	if err != nil {
		logger.Error("shutdown-error", err)
		os.Exit(1)
	}
	logger.Info("exited")
}

func seedRouterGroups(cfg config.Config, logger lager.Logger, database db.DB) {
	// seed router groups from config
	if len(cfg.RouterGroups) > 0 {
		routerGroups, _ := database.ReadRouterGroups()
		// if config not empty and db is empty, seed
		if len(routerGroups) == 0 {
			for _, rg := range cfg.RouterGroups {
				guid, err := uuid.NewV4()
				if err != nil {
					logger.Error("failed to generate a guid for router group", err)
					os.Exit(1)
				}
				rg.Guid = guid.String()
				logger.Info("seeding", lager.Data{"router-group": rg})
				err = database.SaveRouterGroup(rg)
				if err != nil {
					logger.Error("failed to save router group from config", err)
					os.Exit(1)
				}
			}
		}
	}
}

func constructStopper(database db.DB) ifrit.Runner {
	return ifrit.RunFunc(func(signals <-chan os.Signal, ready chan<- struct{}) error {
		close(ready)
		select {
		case <-signals:
			database.CancelWatches()
		}

		return nil
	})
}

func constructRouteRegister(
	logGuid string,
	systemDomain string,
	maxTTL time.Duration,
	database db.DB,
	logger lager.Logger,
) ifrit.Runner {
	host := fmt.Sprintf("api.%s/routing", systemDomain)
	route := models.NewRoute(host, uint16(*port), *ip, logGuid, "", int(maxTTL.Seconds()))

	registerInterval := int(maxTTL.Seconds()) / 2
	ticker := time.NewTicker(time.Duration(registerInterval) * time.Second)

	return helpers.NewRouteRegister(database, route, ticker, logger)
}

func constructApiServer(cfg config.Config, database db.DB, statsdClient statsd.Statter, logger lager.Logger) ifrit.Runner {

	uaaClient, err := newUaaClient(logger, cfg)
	if err != nil {
		logger.Error("Failed to create uaa client", err)
		os.Exit(1)
	}

	_, err = uaaClient.FetchKey()
	if err != nil {
		logger.Error("Failed to get verification key from UAA", err)
		os.Exit(1)
	}

	validator := handlers.NewValidator()
	routesHandler := handlers.NewRoutesHandler(uaaClient, int(cfg.MaxTTL.Seconds()), validator, database, logger)
	eventStreamHandler := handlers.NewEventStreamHandler(uaaClient, database, logger, statsdClient)
	routerGroupsHandler := handlers.NewRouteGroupsHandler(uaaClient, logger, database)
	tcpMappingsHandler := handlers.NewTcpRouteMappingsHandler(uaaClient, validator, database, int(cfg.MaxTTL.Seconds()), logger)

	actions := rata.Handlers{
		routing_api.UpsertRoute:           route(routesHandler.Upsert),
		routing_api.DeleteRoute:           route(routesHandler.Delete),
		routing_api.ListRoute:             route(routesHandler.List),
		routing_api.EventStreamRoute:      route(eventStreamHandler.EventStream),
		routing_api.ListRouterGroups:      route(routerGroupsHandler.ListRouterGroups),
		routing_api.UpdateRouterGroup:     route(routerGroupsHandler.UpdateRouterGroup),
		routing_api.UpsertTcpRouteMapping: route(tcpMappingsHandler.Upsert),
		routing_api.DeleteTcpRouteMapping: route(tcpMappingsHandler.Delete),
		routing_api.ListTcpRouteMapping:   route(tcpMappingsHandler.List),
		routing_api.EventStreamTcpRoute:   route(eventStreamHandler.TcpEventStream),
	}

	handler, err := rata.NewRouter(routing_api.Routes(), actions)
	if err != nil {
		logger.Error("failed to create router", err)
		os.Exit(1)
	}

	handler = handlers.LogWrap(handler, logger)
	return http_server.New(":"+strconv.Itoa(int(*port)), handler)
}

func newUaaClient(logger lager.Logger, routingApiConfig config.Config) (uaaclient.Client, error) {
	if *devMode {
		return uaaclient.NewNoOpUaaClient(), nil
	}

	if routingApiConfig.OAuth.Port == -1 {
		logger.Fatal("tls-not-enabled", errors.New("GoRouter requires TLS enabled to get OAuth token"), lager.Data{"token-endpoint": routingApiConfig.OAuth.TokenEndpoint, "port": routingApiConfig.OAuth.Port})
	}

	scheme := "https"
	tokenURL := fmt.Sprintf("%s://%s:%d", scheme, routingApiConfig.OAuth.TokenEndpoint, routingApiConfig.OAuth.Port)

	cfg := &uaaconfig.Config{
		UaaEndpoint:      tokenURL,
		SkipVerification: routingApiConfig.OAuth.SkipSSLValidation,
		CACerts:          routingApiConfig.OAuth.CACerts,
	}
	return uaaclient.NewClient(logger, cfg, clock.NewClock())
}

func checkFlags() error {
	if *configPath == "" {
		return errors.New("No configuration file provided")
	}

	if *ip == "" {
		return errors.New("No ip address provided")
	}

	if *port > 65535 {
		return errors.New("Port must be in range 0 - 65535")
	}

	return nil
}
