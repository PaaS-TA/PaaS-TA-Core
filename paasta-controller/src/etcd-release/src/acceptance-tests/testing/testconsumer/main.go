package main

import (
	"acceptance-tests/testing/testconsumer/handlers"
	"flag"
	"fmt"
	"log"
	"net/http"
)

type Flags struct {
	Port       string
	EtcdURL    stringSlice
	CACert     string
	ClientCert string
	ClientKey  string
}

type stringSlice []string

func (ss *stringSlice) String() string {
	return fmt.Sprintf("%s", *ss)
}

func (ss *stringSlice) Slice() []string {
	s := []string{}
	for _, v := range *ss {
		s = append(s, v)
	}
	return s
}

func (ss *stringSlice) Set(value string) error {
	*ss = append(*ss, value)

	return nil
}

func main() {
	flags := parseCommandLineFlags()

	kvHandler := handlers.NewKVHandler(flags.EtcdURL.Slice(), flags.CACert, flags.ClientCert, flags.ClientKey)
	leaderNameHandler := handlers.NewLeaderHandler(flags.EtcdURL.Slice()[0], flags.CACert, flags.ClientCert, flags.ClientKey)

	mux := http.NewServeMux()
	mux.HandleFunc("/kv/", func(w http.ResponseWriter, req *http.Request) {
		kvHandler.ServeHTTP(w, req)
	})

	mux.HandleFunc("/leader", func(w http.ResponseWriter, req *http.Request) {
		leaderNameHandler.ServeHTTP(w, req)
	})

	log.Fatal(http.ListenAndServe(fmt.Sprintf("0.0.0.0:%s", flags.Port), mux))
}

func parseCommandLineFlags() Flags {
	flags := Flags{}
	flag.StringVar(&flags.Port, "port", "", "port to use for test consumer server")
	flag.Var(&flags.EtcdURL, "etcd-service", "url of the etcd service")
	flag.StringVar(&flags.CACert, "ca-cert-file", "", "the file of the CA Certificate")
	flag.StringVar(&flags.ClientCert, "client-ssl-cert-file", "", "the file of the Client SSL Certificate")
	flag.StringVar(&flags.ClientKey, "client-ssl-key-file", "", "the file of the CLient SSL Key")
	flag.Parse()

	return flags
}
