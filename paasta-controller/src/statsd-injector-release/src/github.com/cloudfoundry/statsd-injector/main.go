package main

import (
	"flag"

	"github.com/cloudfoundry/statsd-injector/app"
	"github.com/cloudfoundry/statsd-injector/profiler"
)

const defaultAPIVersion = "v1"

func main() {
	statsdHost := flag.String("statsd-host", "localhost", "The hostname the injector will listen on for statsd messages")
	statsdPort := flag.Uint("statsd-port", 8125, "The UDP port the injector will listen on for statsd messages")
	metronPort := flag.Uint("metron-port", 3458, "The GRPC port the injector will forward message to")

	ca := flag.String("ca", "", "File path to the CA certificate")
	cert := flag.String("cert", "", "File path to the client TLS cert")
	privateKey := flag.String("key", "", "File path to the client TLS private key")

	flag.Parse()

	p := profiler.New(0)
	go p.Start()

	injector := app.NewInjector(app.Config{
		StatsdHost: *statsdHost,
		StatsdPort: *statsdPort,
		MetronPort: *metronPort,
		CA:         *ca,
		Cert:       *cert,
		Key:        *privateKey,
	})
	injector.Start()
}
