package main

import (
	"github.com/cloudfoundry-incubator/etcd-release/src/acceptance-tests/testing/iptables_agent/handlers"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
)

type Flags struct {
	Port            string
	IPTablesCommand string
}

func main() {
	flags := parseCommandLineFlags()

	dropHandler := handlers.NewDropHandler(handlers.NewIPTables(flags.IPTablesCommand), os.Stdout)

	mux := http.NewServeMux()
	mux.HandleFunc("/drop", func(w http.ResponseWriter, req *http.Request) {
		dropHandler.ServeHTTP(w, req)
	})

	log.Fatal(http.ListenAndServe(fmt.Sprintf("0.0.0.0:%s", flags.Port), mux))
}

func parseCommandLineFlags() Flags {
	flags := Flags{}
	flag.StringVar(&flags.Port, "port", "", "port to use for iptables agent server")
	flag.StringVar(&flags.IPTablesCommand, "iptablesCommand", "iptables", "command to use for iptables agent server")
	flag.Parse()

	return flags
}
