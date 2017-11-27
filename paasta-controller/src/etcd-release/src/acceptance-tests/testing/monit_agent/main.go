package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/cloudfoundry-incubator/etcd-release/src/acceptance-tests/testing/monit_agent/handlers"
	"github.com/cloudfoundry-incubator/etcd-release/src/acceptance-tests/testing/monit_agent/utils"
)

type Flags struct {
	Port         string
	MonitCommand string
}

func main() {
	flags := parseCommandLineFlags()

	logger := log.New(os.Stdout, "[MonitAgent]", log.LUTC)
	startMonitHandler := handlers.NewMonitHandler("start", utils.NewMonitWrapper(flags.MonitCommand), utils.NewRemoveStore(), logger)
	stopMonitHandler := handlers.NewMonitHandler("stop", utils.NewMonitWrapper(flags.MonitCommand), utils.NewRemoveStore(), logger)
	restartMonitHandler := handlers.NewMonitHandler("restart", utils.NewMonitWrapper(flags.MonitCommand), utils.NewRemoveStore(), logger)
	monitJobStatusHandler := handlers.NewMonitJobStatusHandler(utils.NewMonitWrapper(flags.MonitCommand), logger)

	mux := http.NewServeMux()
	mux.HandleFunc("/start", func(w http.ResponseWriter, req *http.Request) {
		startMonitHandler.ServeHTTP(w, req)
	})
	mux.HandleFunc("/stop", func(w http.ResponseWriter, req *http.Request) {
		stopMonitHandler.ServeHTTP(w, req)
	})
	mux.HandleFunc("/restart", func(w http.ResponseWriter, req *http.Request) {
		restartMonitHandler.ServeHTTP(w, req)
	})
	mux.HandleFunc("/job_status", func(w http.ResponseWriter, req *http.Request) {
		monitJobStatusHandler.ServeHTTP(w, req)
	})

	log.Fatal(http.ListenAndServe(fmt.Sprintf("0.0.0.0:%s", flags.Port), mux))
}

func parseCommandLineFlags() Flags {
	flags := Flags{}
	flag.StringVar(&flags.Port, "port", "", "port to use for monit agent server")
	flag.StringVar(&flags.MonitCommand, "monitCommand", "/var/vcap/bosh/bin/monit", "command to use for monit agent server")
	flag.Parse()

	return flags
}
