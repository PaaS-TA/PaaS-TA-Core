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
	"code.cloudfoundry.org/diego-ssh/keys"
	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/locket"
	"code.cloudfoundry.org/runtimeschema/cc_messages/flags"
	"github.com/cloudfoundry/dropsonde"
	"github.com/nu7hatch/gouuid"
	"github.com/tedsuo/ifrit"
	"github.com/tedsuo/ifrit/grouper"
	"github.com/tedsuo/ifrit/sigmon"

	"code.cloudfoundry.org/nsync"
	"code.cloudfoundry.org/nsync/bulk"
	"code.cloudfoundry.org/nsync/recipebuilder"
)

var privilegedContainers = flag.Bool(
	"privilegedContainers",
	false,
	"Whether or not to use privileged containers for  buildpack based LRPs and tasks. Containers with a docker-image-based rootfs will continue to always be unprivileged and cannot be changed.",
)

var bbsAddress = flag.String(
	"bbsAddress",
	"",
	"Address to the BBS Server",
)

var consulCluster = flag.String(
	"consulCluster",
	"",
	"comma-separated list of consul server URLs (scheme://ip:port)",
)

var lockTTL = flag.Duration(
	"lockTTL",
	locket.LockTTL,
	"TTL for service lock",
)

var lockRetryInterval = flag.Duration(
	"lockRetryInterval",
	locket.RetryInterval,
	"interval to wait before retrying a failed lock acquisition",
)

var dropsondePort = flag.Int(
	"dropsondePort",
	3457,
	"port the local metron agent is listening on",
)

var ccBaseURL = flag.String(
	"ccBaseURL",
	"",
	"base URL of the cloud controller",
)

var ccUsername = flag.String(
	"ccUsername",
	"",
	"basic auth username for CC bulk API",
)

var ccPassword = flag.String(
	"ccPassword",
	"",
	"basic auth password for CC bulk API",
)

var communicationTimeout = flag.Duration(
	"communicationTimeout",
	30*time.Second,
	"Timeout applied to all HTTP requests.",
)

var pollingInterval = flag.Duration(
	"pollingInterval",
	30*time.Second,
	"interval at which to poll bulk API",
)

var domainTTL = flag.Duration(
	"domainTTL",
	2*time.Minute,
	"duration of the domain; bumped on every bulk sync",
)

var bulkBatchSize = flag.Uint(
	"bulkBatchSize",
	500,
	"number of apps to fetch at once from bulk API",
)

var skipCertVerify = flag.Bool(
	"skipCertVerify",
	false,
	"skip SSL certificate verification",
)

var fileServerURL = flag.String(
	"fileServerURL",
	"",
	"URL of the file server",
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

var updateLRPWorkers = flag.Int(
	"updateLRPWorkers",
	50,
	"Max concurrency for updating/creating lrps",
)

var failTaskPoolSize = flag.Int(
	"failTaskPoolSize",
	50,
	"Max concurrency for failing mismatched tasks",
)

var cancelTaskPoolSize = flag.Int(
	"cancelTaskPoolSize",
	50,
	"Max concurrency for canceling mismatched tasks",
)

const (
	dropsondeOrigin = "nsync_bulker"
)

func main() {
	debugserver.AddFlags(flag.CommandLine)
	cflager.AddFlags(flag.CommandLine)

	lifecycles := flags.LifecycleMap{}
	flag.Var(&lifecycles, "lifecycle", "app lifecycle binary bundle mapping (lifecycle[/stack]:bundle-filepath-in-fileserver)")
	flag.Parse()

	cfhttp.Initialize(*communicationTimeout)

	logger, reconfigurableSink := cflager.New("nsync-bulker")
	initializeDropsonde(logger)

	serviceClient := initializeServiceClient(logger)
	uuid, err := uuid.NewV4()
	if err != nil {
		logger.Fatal("Couldn't generate uuid", err)
	}
	lockMaintainer := serviceClient.NewNsyncBulkerLockRunner(logger, uuid.String(), *lockRetryInterval, *lockTTL)

	dockerRecipeBuilderConfig := recipebuilder.Config{
		Lifecycles:    lifecycles,
		FileServerURL: *fileServerURL,
		KeyFactory:    keys.RSAKeyPairFactory,
	}

	buildpackRecipeBuilderConfig := recipebuilder.Config{
		Lifecycles:           lifecycles,
		FileServerURL:        *fileServerURL,
		KeyFactory:           keys.RSAKeyPairFactory,
		PrivilegedContainers: *privilegedContainers,
	}

	recipeBuilders := map[string]recipebuilder.RecipeBuilder{
		"buildpack": recipebuilder.NewBuildpackRecipeBuilder(logger, buildpackRecipeBuilderConfig),
		"docker":    recipebuilder.NewDockerRecipeBuilder(logger, dockerRecipeBuilderConfig),
	}

	lrpRunner := bulk.NewLRPProcessor(
		logger,
		initializeBBSClient(logger),
		*pollingInterval,
		*domainTTL,
		*bulkBatchSize,
		*updateLRPWorkers,
		*skipCertVerify,
		&bulk.CCFetcher{
			BaseURI:   *ccBaseURL,
			BatchSize: int(*bulkBatchSize),
			Username:  *ccUsername,
			Password:  *ccPassword,
		},
		recipeBuilders,
		clock.NewClock(),
	)

	taskRunner := bulk.NewTaskProcessor(
		logger,
		initializeBBSClient(logger),
		&bulk.CCTaskClient{},
		*pollingInterval,
		*domainTTL,
		*failTaskPoolSize,
		*cancelTaskPoolSize,
		*skipCertVerify,
		&bulk.CCFetcher{
			BaseURI:   *ccBaseURL,
			BatchSize: int(*bulkBatchSize),
			Username:  *ccUsername,
			Password:  *ccPassword,
		},
		clock.NewClock(),
	)

	members := grouper.Members{
		{"lock-maintainer", lockMaintainer},
		{"lrp-runner", lrpRunner},
		{"task-runner", taskRunner},
	}

	if dbgAddr := debugserver.DebugAddress(flag.CommandLine); dbgAddr != "" {
		members = append(grouper.Members{
			{"debug-server", debugserver.Runner(dbgAddr, reconfigurableSink)},
		}, members...)
	}

	group := grouper.NewOrdered(os.Interrupt, members)

	logger.Info("waiting-for-lock")

	monitor := ifrit.Invoke(sigmon.New(group))

	logger.Info("started")

	err = <-monitor.Wait()
	if err != nil {
		logger.Error("exited-with-failure", err)
		os.Exit(1)
	}

	logger.Info("exited")
	os.Exit(0)
}

func initializeDropsonde(logger lager.Logger) {
	dropsondeDestination := fmt.Sprint("localhost:", *dropsondePort)
	err := dropsonde.Initialize(dropsondeDestination, dropsondeOrigin)
	if err != nil {
		logger.Error("failed to initialize dropsonde: %v", err)
	}
}

func initializeServiceClient(logger lager.Logger) nsync.ServiceClient {
	consulClient, err := consuladapter.NewClientFromUrl(*consulCluster)
	if err != nil {
		logger.Fatal("new-client-failed", err)
	}

	return nsync.NewServiceClient(consulClient, clock.NewClock())
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
