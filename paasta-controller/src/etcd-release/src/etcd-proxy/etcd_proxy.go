package main

import (
	"crypto/tls"
	"crypto/x509"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"time"

	"github.com/cloudfoundry-incubator/etcd-release/src/etcd-proxy/leaderfinder"
)

type Flags struct {
	EtcdDNSSuffix  string
	EtcdPort       string
	IP             string
	Port           string
	CACertFilePath string
	CertFilePath   string
	KeyFilePath    string
}

func main() {
	flags := Flags{}
	flag.StringVar(&flags.EtcdDNSSuffix, "etcd-dns-suffix", "", "domain of etcd cluster")
	flag.StringVar(&flags.EtcdPort, "etcd-port", "4001", "port that etcd server is running on")
	flag.StringVar(&flags.IP, "ip", "", "ip of the proxy server")
	flag.StringVar(&flags.Port, "port", "", "port of the proxy server")
	flag.StringVar(&flags.CACertFilePath, "cacert", "", "path to the etcd ca certificate")
	flag.StringVar(&flags.CertFilePath, "cert", "", "path to the etcd client certificate")
	flag.StringVar(&flags.KeyFilePath, "key", "", "path to the etcd client key")
	flag.Parse()

	etcdRawURL := fmt.Sprintf("https://%s:%s", flags.EtcdDNSSuffix, flags.EtcdPort)
	etcdURL, err := url.Parse(etcdRawURL)
	if err != nil {
		fail(fmt.Sprintf("failed to parse etcd-dns-suffix and etcd-port %s", err.Error()))
	}

	httpClient := &http.Client{Timeout: 10 * time.Second}
	httpClient.Transport = &http.Transport{
		TLSClientConfig: buildTLSConfig(flags.CACertFilePath, flags.CertFilePath, flags.KeyFilePath),
	}

	finder := leaderfinder.NewFinder(etcdURL.String(), httpClient)

	_, err = finder.Find()
	if err != nil {
		fail(fmt.Sprintf("failed to reach etcd-cluster: %s", err.Error()))
	}

	manager := leaderfinder.NewManager(etcdURL, finder)

	director := func(req *http.Request) {
		target := manager.LeaderOrDefault()
		req.URL.Scheme = target.Scheme
		req.URL.Host = target.Host
	}

	proxy := &httputil.ReverseProxy{Director: director}

	proxy.Transport = &http.Transport{
		TLSClientConfig: buildTLSConfig(flags.CACertFilePath, flags.CertFilePath, flags.KeyFilePath),
	}

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		log.Printf("%+v", r)
		proxy.ServeHTTP(w, r)
	})

	if err := http.ListenAndServe(flags.IP+":"+flags.Port, nil); err != nil {
		fail(err)
	}
}

func fail(message interface{}) {
	fmt.Fprint(os.Stderr, message)
	os.Exit(1)
}

func buildTLSConfig(caCertFilePath, certFilePath, keyFilePath string) *tls.Config {
	tlsCert, err := tls.LoadX509KeyPair(certFilePath, keyFilePath)
	if err != nil {
		fail(err)
	}

	tlsConfig := &tls.Config{
		Certificates:       []tls.Certificate{tlsCert},
		InsecureSkipVerify: false,
		ClientAuth:         tls.RequireAndVerifyClientCert,
	}

	certBytes, err := ioutil.ReadFile(caCertFilePath)
	if err != nil {
		fail(err)
	}

	caCertPool := x509.NewCertPool()
	if ok := caCertPool.AppendCertsFromPEM(certBytes); !ok {
		fail("cacert is not a PEM encoded file")
	}

	tlsConfig.RootCAs = caCertPool
	tlsConfig.ClientCAs = caCertPool

	return tlsConfig
}
