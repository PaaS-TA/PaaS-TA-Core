package main

import (
	"flag"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"time"

	"code.cloudfoundry.org/cfhttp"
	"code.cloudfoundry.org/cflager"
	"code.cloudfoundry.org/debugserver"
	"code.cloudfoundry.org/lager"
	"github.com/cloudfoundry-incubator/etcd-metrics-server/instruments"
	"github.com/cloudfoundry-incubator/etcd-metrics-server/runners"
	"github.com/cloudfoundry/dropsonde"
	"github.com/tedsuo/ifrit"
	"github.com/tedsuo/ifrit/grouper"
	"github.com/tedsuo/ifrit/sigmon"
)

var jobName = flag.String(
	"jobName",
	"etcd",
	"component name for collector",
)

var etcdScheme = flag.String(
	"etcdScheme",
	"http",
	"scheme to use for etcd requests",
)

var etcdAddress = flag.String(
	"etcdAddress",
	"127.0.0.1:4001",
	"etcd host:port to instrument",
)

var index = flag.Uint(
	"index",
	0,
	"index of the etcd job",
)

var port = flag.Int(
	"port",
	5678,
	"port to listen on",
)

var metronAddress = flag.String(
	"metronAddress",
	"127.0.0.1:3457",
	"metron agent address",
)

var username = flag.String(
	"username",
	"",
	"basic auth username",
)

var password = flag.String(
	"password",
	"",
	"basic auth password",
)

var communicationTimeout = flag.Duration(
	"communicationTimeout",
	30*time.Second,
	"Timeout applied to all HTTP requests.",
)

var reportInterval = flag.Duration(
	"reportInterval",
	time.Minute,
	"interval on which to report metrics",
)

var caCertFilePath = flag.String(
	"caCert",
	"",
	"Path to the ETCD server CA",
)

var certFilePath = flag.String(
	"cert",
	"",
	"Path to the ETCD server cert",
)

var keyFilePath = flag.String(
	"key",
	"",
	"Path to the ETCD server key",
)

func main() {
	debugserver.AddFlags(flag.CommandLine)
	cflager.AddFlags(flag.CommandLine)
	flag.Parse()

	dropsonde.Initialize(*metronAddress, *jobName)
	cfhttp.Initialize(*communicationTimeout)

	client := cfhttp.NewClient()
	client.CheckRedirect = func(*http.Request, []*http.Request) error {
		return instruments.ErrRedirected
	}

	if *caCertFilePath != "" && *certFilePath != "" && *keyFilePath != "" {
		tlsConfig, err := cfhttp.NewTLSConfig(*certFilePath, *keyFilePath, *caCertFilePath)
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}
		client.Transport.(*http.Transport).TLSClientConfig = tlsConfig
	}

	componentName := fmt.Sprintf("%s-metrics-server", *jobName)

	logger, reconfigurableSink := cflager.New(componentName)

	members := grouper.Members{
		{"metron-notifier", initializeMetronNotifier(client, logger)},
	}

	if dbgAddr := debugserver.DebugAddress(flag.CommandLine); dbgAddr != "" {
		members = append(grouper.Members{
			{"debug-server", debugserver.Runner(dbgAddr, reconfigurableSink)},
		}, members...)
	}

	group := grouper.NewOrdered(os.Interrupt, members)
	monitorProcess := ifrit.Invoke(sigmon.New(group))

	err := <-monitorProcess.Wait()
	if err != nil {
		os.Exit(1)
	}
}

func createEtcdURL() *url.URL {
	return &url.URL{
		Scheme: *etcdScheme,
		Host:   *etcdAddress,
	}
}

func initializeMetronNotifier(client *http.Client, logger lager.Logger) *runners.PeriodicMetronNotifier {
	return runners.NewPeriodicMetronNotifier(client, createEtcdURL().String(), logger, *reportInterval)
}
