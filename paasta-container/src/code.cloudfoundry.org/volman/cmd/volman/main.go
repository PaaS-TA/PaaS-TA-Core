package main

import (
	"flag"
	"os"

	"path/filepath"

	cf_lager "code.cloudfoundry.org/cflager"
	cf_debug_server "code.cloudfoundry.org/debugserver"
	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/volman/volhttp"
	"code.cloudfoundry.org/volman/vollocal"
	"github.com/tedsuo/ifrit"
	"github.com/tedsuo/ifrit/grouper"
	"github.com/tedsuo/ifrit/http_server"
	"github.com/tedsuo/ifrit/sigmon"
)

var atAddress = flag.String(
	"listenAddr",
	"0.0.0.0:8750",
	"host:port to serve volume management functions",
)

var driverPaths = flag.String(
	"volmanDriverPaths",
	"",
	"Path to the directory where drivers can be discovered.  Multiple paths may be specified using the OS-specific path separator; e.g. /path/to/somewhere:/path/to/somewhere-else",
)

func init() {
	// no command line parsing can happen here in go 1.6
}

func main() {
	parseCommandLine()
	logger, logSink := cf_lager.New("volman")
	defer logger.Info("ends")

	servers := createVolmanServer(logger, *atAddress, *driverPaths)

	if dbgAddr := cf_debug_server.DebugAddress(flag.CommandLine); dbgAddr != "" {
		servers = append(grouper.Members{
			{"debug-server", cf_debug_server.Runner(dbgAddr, logSink)},
		}, servers...)
	}
	process := ifrit.Invoke(processRunnerFor(servers))
	logger.Info("started")
	untilTerminated(logger, process)
}

func exitOnFailure(logger lager.Logger, err error) {
	if err != nil {
		logger.Error("Fatal err.. aborting", err)
		panic(err.Error())
	}
}

func untilTerminated(logger lager.Logger, process ifrit.Process) {
	err := <-process.Wait()
	exitOnFailure(logger, err)
}

func processRunnerFor(servers grouper.Members) ifrit.Runner {
	return sigmon.New(grouper.NewOrdered(os.Interrupt, servers))
}

func createVolmanServer(logger lager.Logger, atAddress string, driverPaths string) grouper.Members {
	if driverPaths == "" {
		panic("'-volmanDriverPaths' must be provided")
	}

	cfg := vollocal.NewDriverConfig()
	cfg.DriverPaths = filepath.SplitList(driverPaths)
	client, runner := vollocal.NewServer(logger, cfg)

	handler, err := volhttp.NewHandler(logger, client)
	exitOnFailure(logger, err)

	return grouper.Members{
		{"driver-syncer", runner},
		{"http-server", http_server.New(atAddress, handler)},
	}
}

func parseCommandLine() {
	cf_lager.AddFlags(flag.CommandLine)
	cf_debug_server.AddFlags(flag.CommandLine)
	flag.Parse()
}
