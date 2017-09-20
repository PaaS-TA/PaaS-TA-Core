package main

import (
	"flag"
	"fmt"
	"net"
	"os"
	"time"

	"code.cloudfoundry.org/healthcheck"
)

var network = flag.String(
	"network",
	"tcp",
	"network type to dial with (e.g. unix, tcp)",
)

var uri = flag.String(
	"uri",
	"",
	"uri to healthcheck",
)

var port = flag.String(
	"port",
	"8080",
	"port to healthcheck",
)

var timeout = flag.Duration(
	"timeout",
	1*time.Second,
	"dial timeout",
)

func main() {
	flag.Parse()

	interfaces, err := net.Interfaces()
	if err != nil {
		fmt.Println(fmt.Sprintf("failure to get interfaces: %s", err))
		os.Exit(1)
		return
	}

	h := healthcheck.NewHealthCheck(*network, *uri, *port, *timeout)
	err = h.CheckInterfaces(interfaces)
	if err == nil {
		fmt.Println("healthcheck passed")
		os.Exit(0)
		return
	}

	failHealthCheck(err)
}

func failHealthCheck(err error) {
	if err, ok := err.(healthcheck.HealthCheckError); ok {
		fmt.Println("healthcheck failed: " + err.Message)
		os.Exit(err.Code)
	}

	fmt.Println("healthcheck failed(unknown error)" + err.Error())
	os.Exit(127)
}
