package main_test

import (
	"crypto/tls"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os/exec"
	"time"

	"github.com/onsi/gomega/gexec"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
)

func newTLSServer(handler http.Handler) *httptest.Server {
	var err error
	etcdServer := httptest.NewUnstartedServer(handler)
	etcdServer.TLS = &tls.Config{}

	etcdServer.TLS.Certificates = make([]tls.Certificate, 1)
	etcdServer.TLS.Certificates[0], err = tls.LoadX509KeyPair("fixtures/server.crt", "fixtures/server.key")
	Expect(err).NotTo(HaveOccurred())

	etcdServer.StartTLS()
	return etcdServer
}

func executeEtcdCC(args []string, exitCode int) *gexec.Session {
	cmd := exec.Command(pathToEtcdCC, args...)
	session, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
	Expect(err).NotTo(HaveOccurred())
	Eventually(session, 10*time.Second).Should(gexec.Exit(exitCode))

	return session
}

var _ = Describe("etcd-consistency-checker", func() {
	DescribeTable("exits 1 when more than one leader exists",
		func(newServer func(http.Handler) *httptest.Server, ca, cert, key string) {
			etcdServer1Handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				switch r.URL.Path {
				case "/v2/stats/leader":
					if r.Method == "GET" {
						w.WriteHeader(http.StatusOK)
						w.Write([]byte(`{
							"leader": "XXXXXXXXXXXXXXXXXXXXXX"
						}`))
						return
					}
				}
				w.WriteHeader(http.StatusTeapot)
				return
			})

			etcdServer2Handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				switch r.URL.Path {
				case "/v2/stats/leader":
					if r.Method == "GET" {
						w.WriteHeader(http.StatusOK)
						w.Write([]byte(`{
							"leader": "YYYYYYYYYYYYYYYYYYYYY"
						}`))
						return
					}
				}
				w.WriteHeader(http.StatusTeapot)
				return
			})

			etcdServer1 := newServer(etcdServer1Handler)
			etcdServer2 := newServer(etcdServer2Handler)

			args := []string{
				"--cluster-members", fmt.Sprintf("%s,%s", etcdServer1.URL, etcdServer2.URL),
				"--ca-cert", ca,
				"--cert", cert,
				"--key", key,
			}

			session := executeEtcdCC(args, 1)
			Expect(session.Err.Contents()).To(ContainSubstring(fmt.Sprintf("more than one leader exists: [%s %s]", etcdServer1.URL, etcdServer2.URL)))
		},
		Entry("when tls is enabled", newTLSServer, "fixtures/ca.crt", "fixtures/client.crt", "fixtures/client.key"),
		Entry("when tls is not enabled", httptest.NewServer, "", "", ""),
	)
})
