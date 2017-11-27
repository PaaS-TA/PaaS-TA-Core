package main

import (
	"errors"
	"flag"
	"fmt"
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
	"code.cloudfoundry.org/locket"
	"code.cloudfoundry.org/routing-api"
	"code.cloudfoundry.org/tcp-emitter/config"
	"code.cloudfoundry.org/tcp-emitter/emitter"
	"code.cloudfoundry.org/tcp-emitter/routing_table"
	"code.cloudfoundry.org/tcp-emitter/routing_table/schema"
	"code.cloudfoundry.org/tcp-emitter/syncer"
	"code.cloudfoundry.org/tcp-emitter/watcher"
	uaaclient "code.cloudfoundry.org/uaa-go-client"
	uaaconfig "code.cloudfoundry.org/uaa-go-client/config"
	"github.com/cloudfoundry/dropsonde"
	"github.com/nu7hatch/gouuid"
	"github.com/tedsuo/ifrit"
	"github.com/tedsuo/ifrit/grouper"
	"github.com/tedsuo/ifrit/sigmon"
)

const (
	tcpEmitterLockPath             = "tcp_emitter_lock"
	dropsondeOrigin                = "tcp_emitter"
	defaultTokenFetchRetryInterval = 5 * time.Second
	defaultTokenFetchNumRetries    = uint(3)
)

var bbsAddress = flag.String(
	"bbsAddress",
	"",
	"URL of BBS Server",
)

var communicationTimeout = flag.Duration(
	"communicationTimeout",
	10*time.Second,
	"Timeout applied to all HTTP requests.",
)

var syncInterval = flag.Duration(
	"syncInterval",
	20*time.Second,
	"The interval between syncs of the routing table from bbs.",
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

var configFile = flag.String(
	"config",
	"/var/vcap/jobs/tcp_emitter/config/tcp_emitter.yml",
	"The TCP emitter yml config.",
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

var sessionName = flag.String(
	"sessionName",
	"tcp-emitter",
	"consul session name",
)

var routeTTL = flag.Duration(
	"routeTTL",
	2*time.Minute,
	"TTL for TCP Routes",
)

var tokenFetchMaxRetries = flag.Uint(
	"tokenFetchMaxRetries",
	defaultTokenFetchNumRetries,
	"Maximum number of retries the Token Fetcher will use every time FetchToken is called",
)

var tokenFetchRetryInterval = flag.Duration(
	"tokenFetchRetryInterval",
	defaultTokenFetchRetryInterval,
	"interval to wait before TokenFetcher retries to fetch a token",
)

var tokenFetchExpirationBufferTime = flag.Uint64(
	"tokenFetchExpirationBufferTime",
	30,
	"Buffer time in seconds before the actual token expiration time, when TokenFetcher consider a token expired",
)

var dropsondePort = flag.Int(
	"dropsondePort",
	3457,
	"Port the local metron agent is listening on",
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

func main() {
	debugserver.AddFlags(flag.CommandLine)
	lagerflags.AddFlags(flag.CommandLine)
	flag.Parse()

	cfhttp.Initialize(*communicationTimeout)

	logger, reconfigurableSink := lagerflags.New("tcp-emitter")
	logger.Info("starting")

	clock := clock.NewClock()

	initializeDropsonde(logger)

	bbsURL, err := url.Parse(*bbsAddress)
	if err != nil {
		logger.Error("invalid-bbs-address", err)
		os.Exit(1)
	}

	var bbsClient bbs.Client

	logger.Debug("setting-up-bbs-client", lager.Data{"bbsURL": bbsURL.String()})

	if bbsURL.Scheme == "http" {
		bbsClient = bbs.NewClient(bbsURL.String())
	} else if bbsURL.Scheme == "https" {
		bbsClient, err = bbs.NewSecureClient(
			bbsURL.String(), *bbsCACert, *bbsClientCert,
			*bbsClientKey, *bbsClientSessionCacheSize, *bbsMaxIdleConnsPerHost,
		)
		if err != nil {
			logger.Error("failed-to-configure-bbs-client", err)
			os.Exit(1)
		}
	} else {
		logger.Error("invalid-scheme-in-bbs-address", err)
		os.Exit(1)
	}

	// Check BBS connectivity
	connected := bbsClient.Ping(logger)
	if !connected {
		logger.Error("failed-to-connect-to-bbs", nil)
		os.Exit(1)
	}

	cfg, err := config.New(*configFile)
	if err != nil {
		logger.Error("failed-to-unmarshal-config-file", err)
		os.Exit(1)
	}
	uaaClient := newUaaClient(logger, cfg, clock)

	// Check UAA connectivity
	_, err = uaaClient.FetchKey()
	if err != nil {
		logger.Error("failed-connecting-to-uaa", err)
		os.Exit(1)
	}

	routingAPIAddress := fmt.Sprintf("%s:%d", cfg.RoutingAPI.URI, cfg.RoutingAPI.Port)
	logger.Debug("creating-routing-api-client", lager.Data{"api-location": routingAPIAddress})
	routingAPIClient := routing_api.NewClient(routingAPIAddress, false)

	if routeTTL.Seconds() > 65535 {
		logger.Error("invalid-route-ttl", errors.New("route TTL value too large"))
		os.Exit(1)
	}

	emitter := emitter.NewEmitter(logger, routingAPIClient, uaaClient, int(routeTTL.Seconds()))
	routingTable := schema.NewTable(logger, nil)
	routingTableHandler := routing_table.NewRoutingTableHandler(logger, routingTable, emitter, bbsClient)
	syncChannel := make(chan struct{})
	syncRunner := syncer.New(clock, *syncInterval, syncChannel, logger)
	watcher := watcher.NewWatcher(bbsClient, clock, routingTableHandler, syncChannel, logger)

	lockMaintainer := initializeLockMaintainer(logger, *consulCluster, *sessionName,
		*lockTTL, *lockRetryInterval, clock)

	members := grouper.Members{
		grouper.Member{Name: "lock-maintainer", Runner: lockMaintainer},
		grouper.Member{Name: "watcher", Runner: watcher},
		grouper.Member{Name: "syncer", Runner: syncRunner},
	}

	if dbgAddr := debugserver.DebugAddress(flag.CommandLine); dbgAddr != "" {
		members = append(grouper.Members{
			grouper.Member{Name: "debug-server", Runner: debugserver.Runner(dbgAddr, reconfigurableSink)},
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

func newUaaClient(logger lager.Logger, c *config.Config, klok clock.Clock) uaaclient.Client {
	if c.RoutingAPI.AuthDisabled {
		logger.Debug("creating-noop-uaa-client")
		client := uaaclient.NewNoOpUaaClient()
		return client
	}
	logger.Debug("creating-uaa-client")

	if c.OAuth.Port == -1 {
		logger.Fatal("tls-not-enabled", errors.New("TcpEmitter requires to communicate with UAA over TLS"), lager.Data{"token-endpoint": c.OAuth.TokenEndpoint, "port": c.OAuth.Port})
	}

	tokenURL := fmt.Sprintf("https://%s:%d", c.OAuth.TokenEndpoint, c.OAuth.Port)

	cfg := &uaaconfig.Config{
		UaaEndpoint:           tokenURL,
		SkipVerification:      c.OAuth.SkipSSLValidation,
		ClientName:            c.OAuth.ClientName,
		ClientSecret:          c.OAuth.ClientSecret,
		MaxNumberOfRetries:    uint32(*tokenFetchMaxRetries),
		RetryInterval:         *tokenFetchRetryInterval,
		ExpirationBufferInSec: int64(*tokenFetchExpirationBufferTime),
		CACerts:               c.OAuth.CACerts,
	}

	uaaClient, err := uaaclient.NewClient(logger, cfg, klok)
	if err != nil {
		logger.Fatal("initialize-token-fetcher-error", err)
	}
	return uaaClient
}

func initializeDropsonde(logger lager.Logger) {
	dropsondeDestination := fmt.Sprintf("localhost:%d", *dropsondePort)
	err := dropsonde.Initialize(dropsondeDestination, dropsondeOrigin)
	if err != nil {
		logger.Error("failed-to-initialize-dropsonde", err)
	}
}

func initializeLockMaintainer(
	logger lager.Logger,
	consulCluster, sessionName string,
	lockTTL, lockRetryInterval time.Duration,
	clock clock.Clock,
) ifrit.Runner {
	client, err := consuladapter.NewClientFromUrl(consulCluster)
	if err != nil {
		logger.Fatal("new-client-failed", err)
	}

	return newLockRunner(logger, client, clock, lockRetryInterval, lockTTL)
}

func newLockRunner(
	logger lager.Logger,
	consulClient consuladapter.Client,
	clock clock.Clock,
	lockRetryInterval time.Duration,
	lockTTL time.Duration,
) ifrit.Runner {
	lockSchemaPath := locket.LockSchemaPath(tcpEmitterLockPath)

	tcpEmitterUUID, err := uuid.NewV4()
	if err != nil {
		logger.Fatal("Couldn't generate tcp Emitter UUID", err)
	}
	tcpEmitterID := []byte(tcpEmitterUUID.String())

	return locket.NewLock(logger, consulClient, lockSchemaPath,
		tcpEmitterID, clock, lockRetryInterval, lockTTL)
}
