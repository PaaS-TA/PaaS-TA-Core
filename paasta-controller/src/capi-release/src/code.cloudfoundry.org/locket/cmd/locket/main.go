package main

import (
	"database/sql"
	"flag"
	"fmt"
	"net"
	"os"
	"strings"
	"time"

	"github.com/cloudfoundry/dropsonde"
	"github.com/hashicorp/consul/api"
	"github.com/tedsuo/ifrit"
	"github.com/tedsuo/ifrit/grouper"
	"github.com/tedsuo/ifrit/sigmon"

	"code.cloudfoundry.org/bbs/guidprovider"
	"code.cloudfoundry.org/cfhttp"
	"code.cloudfoundry.org/clock"
	"code.cloudfoundry.org/consuladapter"
	"code.cloudfoundry.org/debugserver"
	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/lager/lagerflags"
	"code.cloudfoundry.org/locket"
	"code.cloudfoundry.org/locket/cmd/locket/config"
	"code.cloudfoundry.org/locket/db"
	"code.cloudfoundry.org/locket/expiration"
	"code.cloudfoundry.org/locket/grpcserver"
	"code.cloudfoundry.org/locket/handlers"
	"code.cloudfoundry.org/locket/metrics"
)

const (
	dropsondeOrigin = "locket"
	metricsInterval = 10 * time.Second
)

var configFilePath = flag.String(
	"config",
	"",
	"Path to Locket JSON Configuration file",
)

func main() {
	flag.Parse()

	cfg, err := config.NewLocketConfig(*configFilePath)
	if err != nil {
		logger, _ := lagerflags.New("locket")
		logger.Fatal("invalid-config-file", err)
	}

	logger, reconfigurableSink := lagerflags.NewFromConfig("locket", cfg.LagerConfig)

	initializeDropsonde(logger, cfg.DropsondePort)

	clock := clock.NewClock()

	if cfg.DatabaseDriver == "postgres" && !strings.Contains(cfg.DatabaseConnectionString, "sslmode") {
		cfg.DatabaseConnectionString = fmt.Sprintf("%s?sslmode=disable", cfg.DatabaseConnectionString)
	}

	sqlConn, err := sql.Open(cfg.DatabaseDriver, cfg.DatabaseConnectionString)
	if err != nil {
		logger.Fatal("failed-to-open-sql", err)
	}
	defer sqlConn.Close()

	sqlConn.SetMaxIdleConns(cfg.MaxOpenDatabaseConnections)
	sqlConn.SetMaxOpenConns(cfg.MaxOpenDatabaseConnections)

	err = sqlConn.Ping()
	if err != nil {
		logger.Fatal("sql-failed-to-connect", err)
	}

	sqlDB := db.NewSQLDB(
		sqlConn,
		cfg.DatabaseDriver,
		guidprovider.DefaultGuidProvider,
	)

	err = sqlDB.CreateLockTable(logger)
	if err != nil {
		logger.Fatal("failed-to-create-lock-table", err)
	}

	consulClient, err := consuladapter.NewClientFromUrl(cfg.ConsulCluster)
	if err != nil {
		logger.Fatal("new-consul-client-failed", err)
	}

	_, portString, err := net.SplitHostPort(cfg.ListenAddress)
	if err != nil {
		logger.Fatal("failed-invalid-listen-address", err)
	}

	portNum, err := net.LookupPort("tcp", portString)
	if err != nil {
		logger.Fatal("failed-invalid-listen-port", err)
	}

	tlsConfig, err := cfhttp.NewTLSConfig(cfg.CertFile, cfg.KeyFile, cfg.CaFile)
	if err != nil {
		logger.Fatal("invalid-tls-config", err)
	}

	metricsNotifier := metrics.NewMetricsNotifier(logger, clock, metricsInterval, sqlDB)
	lockPick := expiration.NewLockPick(sqlDB, clock)
	burglar := expiration.NewBurglar(logger, sqlDB, lockPick, clock, locket.RetryInterval)
	handler := handlers.NewLocketHandler(logger, sqlDB, lockPick)
	server := grpcserver.NewGRPCServer(logger, cfg.ListenAddress, tlsConfig, handler)
	registrationRunner := initializeRegistrationRunner(logger, consulClient, portNum, clock)
	members := grouper.Members{
		{"server", server},
		{"burglar", burglar},
		{"metrics-notifier", metricsNotifier},
		{"registration-runner", registrationRunner},
	}

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
		logger.Error("exited-with-failure", err)
		os.Exit(1)
	}
}

func initializeRegistrationRunner(
	logger lager.Logger,
	consulClient consuladapter.Client,
	port int,
	clock clock.Clock,
) ifrit.Runner {
	registration := &api.AgentServiceRegistration{
		Name: "locket",
		Port: port,
		Check: &api.AgentServiceCheck{
			TTL: "20s",
		},
	}
	return locket.NewRegistrationRunner(logger, registration, consulClient, locket.RetryInterval, clock)
}

func initializeDropsonde(logger lager.Logger, dropsondePort int) {
	dropsondeDestination := fmt.Sprint("localhost:", dropsondePort)
	err := dropsonde.Initialize(dropsondeDestination, dropsondeOrigin)
	if err != nil {
		logger.Error("failed to initialize dropsonde: %v", err)
	}
}
