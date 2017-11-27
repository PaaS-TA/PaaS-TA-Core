package main

import (
	"bytes"
	"crypto/tls"
	"crypto/x509"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"

	"github.com/cloudfoundry-incubator/consul-release/src/acceptance-tests/testing/testconsumer/buffered"
	"github.com/cloudfoundry-incubator/consul-release/src/acceptance-tests/testing/testconsumer/handlers"
)

type commandLineFlags struct {
	port               string
	consulURL          string
	consulCACertPath   string
	consulCertPath     string
	consulKeyPath      string
	pathToCheckARecord string
}

func main() {
	commandLineFlags := parseCommandLineFlags()
	proxyURL, err := url.Parse(commandLineFlags.consulURL)
	if err != nil {
		log.Fatal(err)
	}

	if commandLineFlags.pathToCheckARecord == "" {
		log.Fatal("--path-to-check-a-record is required")
	}

	mux := http.NewServeMux()
	logBuffer := bytes.NewBuffer([]byte{})
	healthCheckHandler := handlers.NewHealthCheckHandler()
	dnsHandler := handlers.NewDNSHandler(commandLineFlags.pathToCheckARecord)

	proxy := httputil.NewSingleHostReverseProxy(proxyURL)

	if commandLineFlags.consulCertPath != "" && commandLineFlags.consulKeyPath != "" && commandLineFlags.consulCACertPath != "" {
		cert, err := tls.LoadX509KeyPair(commandLineFlags.consulCertPath, commandLineFlags.consulKeyPath)
		if err != nil {
			log.Fatal(err)
		}

		caCert, err := ioutil.ReadFile(commandLineFlags.consulCACertPath)
		if err != nil {
			log.Fatal(err)
		}
		caCertPool := x509.NewCertPool()
		caCertPool.AppendCertsFromPEM(caCert)

		tlsConfig := &tls.Config{
			Certificates: []tls.Certificate{cert},
			RootCAs:      caCertPool,
		}
		tlsConfig.BuildNameToCertificate()
		transport := &http.Transport{TLSClientConfig: tlsConfig}

		proxy.Transport = transport
	}

	director := proxy.Director
	proxy.Director = func(request *http.Request) {
		director(request)
		request.URL.Path = strings.TrimPrefix(request.URL.Path, "/consul")
		request.Host = request.URL.Host
	}
	proxy.ErrorLog = log.New(logBuffer, "", 0)

	mux.HandleFunc("/consul/", func(w http.ResponseWriter, req *http.Request) {
		bufferedRW := buffered.NewResponseWriter(w, logBuffer)
		proxy.ServeHTTP(bufferedRW, req)
		bufferedRW.Copy()
	})

	mux.HandleFunc("/health_check", func(w http.ResponseWriter, req *http.Request) {
		healthCheckHandler.ServeHTTP(w, req)
	})

	mux.HandleFunc("/dns", func(w http.ResponseWriter, req *http.Request) {
		dnsHandler.ServeHTTP(w, req)
	})

	log.Fatal(http.ListenAndServe(fmt.Sprintf("0.0.0.0:%s", commandLineFlags.port), mux))
}

func parseCommandLineFlags() commandLineFlags {
	var clFlags commandLineFlags

	flag.StringVar(&clFlags.port, "port", "", "port to use for test consumer server")
	flag.StringVar(&clFlags.consulURL, "consul-url", "", "url of local consul agent")
	flag.StringVar(&clFlags.consulCACertPath, "cacert", "", "path to cacert of local consul agent")
	flag.StringVar(&clFlags.consulCertPath, "cert", "", "path to cert of local consul agent")
	flag.StringVar(&clFlags.consulKeyPath, "key", "", "path to key of local consul agent")
	flag.StringVar(&clFlags.pathToCheckARecord, "path-to-check-a-record", "", "path to check-a-record binary")
	flag.Parse()

	return clFlags
}
