package main_test

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"io/ioutil"
	"math/rand"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os/exec"
	"strings"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"
)

const (
	caCertFilePath     = "fixtures/ca.crt"
	serverKeyFilePath  = "fixtures/server.key"
	serverCertFilePath = "fixtures/server.crt"
	clientKeyFilePath  = "fixtures/client.key"
	clientCertFilePath = "fixtures/client.crt"
)

func startMockETCDServer() *httptest.Server {
	var etcdServer *httptest.Server
	handler := http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		if req.URL.Path == "/v2/members" && req.Method == "GET" {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(fmt.Sprintf(`{
				  "members": [
					{
					  "clienturls": [
					  %[1]q
					  ],
					  "name": "etcd-z1-0",
					  "id": "1b8722e8a026db8e"
					},
					{
					  "clienturls": [
					  %[1]q
					  ],
					  "name": "etcd-z1-1",
					  "id": "2b8724e8a026db9e"
					}
				  ]
				}`, etcdServer.URL)))
			return
		}

		if req.URL.Path == "/v2/stats/self" && req.Method == "GET" {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{
				  "name": "etcd-z1-0",
				  "id": "1b8722e8a026db8e",
				  "state": "StateFollower",
				  "leaderInfo": {
					"leader": "2b8724e8a026db9e"
				  }
				}`))
			return
		}

		if req.URL.Path == "/v2/keys/some-key" && req.Method == "PUT" {
			body, err := ioutil.ReadAll(req.Body)
			Expect(err).NotTo(HaveOccurred())

			values := strings.Split(string(body), "value=")
			value := values[1]

			w.WriteHeader(http.StatusCreated)
			w.Write([]byte(fmt.Sprintf(`{
					"action": "set",
					"node": {
						"createdIndex": 3,
						"key": "/some-key",
						"modifiedIndex": 3,
						"value": %q
					}
				}`, value)))
			return
		}

		w.WriteHeader(http.StatusTeapot)
	})
	etcdServer = httptest.NewUnstartedServer(handler)

	tlsCert, err := tls.LoadX509KeyPair(serverCertFilePath, serverKeyFilePath)
	Expect(err).NotTo(HaveOccurred())

	tlsConfig := &tls.Config{
		Certificates:       []tls.Certificate{tlsCert},
		InsecureSkipVerify: false,
		ClientAuth:         tls.RequireAndVerifyClientCert,
	}

	certBytes, err := ioutil.ReadFile(caCertFilePath)
	Expect(err).NotTo(HaveOccurred())

	caCertPool := x509.NewCertPool()
	ok := caCertPool.AppendCertsFromPEM(certBytes)
	Expect(ok).To(BeTrue())

	tlsConfig.RootCAs = caCertPool
	tlsConfig.ClientCAs = caCertPool

	etcdServer.TLS = tlsConfig

	etcdServer.StartTLS()
	return etcdServer
}

var _ = Describe("provides an http proxy to an etcd cluster", func() {
	var (
		session *gexec.Session
		port    string
	)

	BeforeEach(func() {
		var err error
		port, err = openPort()
		Expect(err).NotTo(HaveOccurred())

	})

	AfterEach(func() {
		session.Terminate().Wait()
	})

	Context("main", func() {
		It("encrypts traffic to the etcd server", func() {
			etcdServer := startMockETCDServer()

			etcdServerURL, err := url.Parse(etcdServer.URL)
			Expect(err).NotTo(HaveOccurred())

			etcdServerHost := strings.Split(etcdServerURL.Host, ":")[0]
			etcdServerPort := strings.Split(etcdServerURL.Host, ":")[1]

			command := exec.Command(pathToEtcdProxy,
				"--etcd-dns-suffix", etcdServerHost,
				"--etcd-port", etcdServerPort,
				"--port", port,
				"--cacert", caCertFilePath,
				"--cert", clientCertFilePath,
				"--key", clientKeyFilePath,
			)

			session, err = gexec.Start(command, GinkgoWriter, GinkgoWriter)
			Expect(err).NotTo(HaveOccurred())

			waitForServerToStart(port)

			value := fmt.Sprintf("some-value-%d", rand.Int())
			statusCode, body, err := makeRequest("PUT", fmt.Sprintf("http://localhost:%s/v2/keys/some-key", port), fmt.Sprintf("value=%s", value))

			Expect(err).NotTo(HaveOccurred())
			Expect(statusCode).To(Equal(http.StatusCreated))
			Expect(body).To(MatchJSON(fmt.Sprintf(`{
		"action": "set",
		"node": {
		"createdIndex": 3,
		"key": "/some-key",
		"modifiedIndex": 3,
		"value": %q
		}
		}`, value)))

			Expect(string(session.Out.Contents())).To(ContainSubstring("RequestURI:/v2/keys/some-key"))
		})
	})

	It("returns the proxy ip in /v2/members", func() {
		etcdServer := startMockETCDServer()

		etcdServerURL, err := url.Parse(etcdServer.URL)
		Expect(err).NotTo(HaveOccurred())

		etcdServerHost := strings.Split(etcdServerURL.Host, ":")[0]
		etcdServerPort := strings.Split(etcdServerURL.Host, ":")[1]

		command := exec.Command(pathToEtcdProxy,
			"--etcd-dns-suffix", etcdServerHost,
			"--etcd-port", etcdServerPort,
			"--port", port,
			"--cacert", caCertFilePath,
			"--cert", clientCertFilePath,
			"--key", clientKeyFilePath,
			"--advertise-ip", "10.0.2.123",
		)

		session, err = gexec.Start(command, GinkgoWriter, GinkgoWriter)
		Expect(err).NotTo(HaveOccurred())

		waitForServerToStart(port)

		statusCode, body, err := makeRequest("GET", fmt.Sprintf("http://localhost:%s/v2/members", port), "")
		Expect(statusCode).To(Equal(http.StatusOK))
		Expect(err).NotTo(HaveOccurred())
		Expect(body).To(MatchJSON(fmt.Sprintf(`{
			"members": [
				{
					"id":"xxxxxxxxxxxxxxxx",
					"name":"proxy",
					"clientURLs": ["http://10.0.2.123:%s"]
				}
			]
		}`, port)))

		Eventually(func() string {
			return string(session.Out.Contents())
		}, "1m", "5s").Should(ContainSubstring("RequestURI:/v2/members"))
	})

	Context("failure cases", func() {
		It("returns an error when an unknown flag is provided", func() {
			var err error
			command := exec.Command(pathToEtcdProxy, "--some-unknown-flag")
			session, err = gexec.Start(command, GinkgoWriter, GinkgoWriter)
			Eventually(session).Should(gexec.Exit())

			Expect(err).NotTo(HaveOccurred())
			Expect(session.ExitCode()).To(Equal(2))
			Expect(session.Err.Contents()).To(ContainSubstring("flag provided but not defined: -some-unknown-flag"))
		})

		It("returns an error when a malformed etcd dns suffix is provided", func() {
			var err error
			command := exec.Command(pathToEtcdProxy, "--etcd-dns-suffix", "%%%%%")
			session, err = gexec.Start(command, GinkgoWriter, GinkgoWriter)
			Eventually(session).Should(gexec.Exit())

			Expect(err).NotTo(HaveOccurred())
			Expect(session.ExitCode()).To(Equal(1))
			Expect(session.Err.Contents()).To(ContainSubstring("failed to parse etcd-dns-suffix and etcd-port parse https://%%%%%:4001: invalid URL escape \"%%%\""))
		})

		It("returns an error when the cert file path does not exist", func() {
			var err error
			command := exec.Command(pathToEtcdProxy, "--cert", "/some/fake/path")
			session, err = gexec.Start(command, GinkgoWriter, GinkgoWriter)
			Eventually(session).Should(gexec.Exit())

			Expect(err).NotTo(HaveOccurred())
			Expect(session.ExitCode()).To(Equal(1))
			Expect(session.Err.Contents()).To(ContainSubstring("open /some/fake/path: no such file or directory"))
		})

		It("returns an error when the ca cert file path does not exist", func() {
			var err error
			args := []string{
				"--cert", clientCertFilePath,
				"--key", clientKeyFilePath,
				"--cacert", "/some/fake/path",
			}

			command := exec.Command(pathToEtcdProxy, args...)
			session, err = gexec.Start(command, GinkgoWriter, GinkgoWriter)
			Eventually(session).Should(gexec.Exit())

			Expect(err).NotTo(HaveOccurred())
			Expect(session.ExitCode()).To(Equal(1))
			Expect(session.Err.Contents()).To(ContainSubstring("open /some/fake/path: no such file or directory"))
		})

		It("returns an error when the ca cert is not PEM encoded", func() {
			file, err := ioutil.TempFile("", "")
			Expect(err).NotTo(HaveOccurred())

			args := []string{
				"--cert", clientCertFilePath,
				"--key", clientKeyFilePath,
				"--cacert", file.Name(),
			}

			command := exec.Command(pathToEtcdProxy, args...)
			session, err = gexec.Start(command, GinkgoWriter, GinkgoWriter)
			Eventually(session).Should(gexec.Exit())

			Expect(err).NotTo(HaveOccurred())
			Expect(session.ExitCode()).To(Equal(1))
			Expect(session.Err.Contents()).To(ContainSubstring("cacert is not a PEM encoded file"))
		})

		It("returns an error when the proxy fails to start", func() {
			var err error
			etcdServer := startMockETCDServer()
			etcdServerURL, err := url.Parse(etcdServer.URL)
			Expect(err).NotTo(HaveOccurred())

			etcdServerHost := strings.Split(etcdServerURL.Host, ":")[0]
			etcdServerPort := strings.Split(etcdServerURL.Host, ":")[1]
			command := exec.Command(pathToEtcdProxy,
				"--etcd-dns-suffix", etcdServerHost,
				"--etcd-port", etcdServerPort,
				"--cert", clientCertFilePath,
				"--key", clientKeyFilePath,
				"--cacert", caCertFilePath,
				"--port", "-1",
			)
			session, err = gexec.Start(command, GinkgoWriter, GinkgoWriter)
			Eventually(session).Should(gexec.Exit())

			Expect(err).NotTo(HaveOccurred())
			Expect(session.ExitCode()).To(Equal(1))
			Expect(session.Err.Contents()).To(ContainSubstring("invalid port"))
		})

		It("returns an error when its not able to talk to the cluster", func() {
			var err error
			command := exec.Command(pathToEtcdProxy,
				"--etcd-dns-suffix", "localhost",
				"--etcd-port", "9999",
				"--port", port,
				"--cacert", caCertFilePath,
				"--cert", clientCertFilePath,
				"--key", clientKeyFilePath,
			)
			session, err = gexec.Start(command, GinkgoWriter, GinkgoWriter)
			Eventually(session).Should(gexec.Exit())

			Expect(err).NotTo(HaveOccurred())
			Expect(session.ExitCode()).To(Equal(1))
			Expect(session.Err.Contents()).To(ContainSubstring("failed to reach etcd-cluster: "))
		})

		It("returns an error when it fails to bind to ip", func() {
			var err error
			etcdServer := startMockETCDServer()
			etcdServerURL, err := url.Parse(etcdServer.URL)
			Expect(err).NotTo(HaveOccurred())

			etcdServerHost := strings.Split(etcdServerURL.Host, ":")[0]
			etcdServerPort := strings.Split(etcdServerURL.Host, ":")[1]
			command := exec.Command(pathToEtcdProxy,
				"--etcd-dns-suffix", etcdServerHost,
				"--etcd-port", etcdServerPort,
				"--ip", "%%%",
				"--port", port,
				"--cacert", caCertFilePath,
				"--cert", clientCertFilePath,
				"--key", clientKeyFilePath,
			)
			session, err = gexec.Start(command, GinkgoWriter, GinkgoWriter)
			Eventually(session).Should(gexec.Exit())

			Expect(err).NotTo(HaveOccurred())
			Expect(session.ExitCode()).To(Equal(1))
			Expect(session.Err.Contents()).To(ContainSubstring("missing brackets in address"))
		})
	})
})

func makeRequest(method string, url string, body string) (int, string, error) {
	request, err := http.NewRequest(method, url, strings.NewReader(body))
	if err != nil {
		return 0, "", err
	}

	response, err := http.DefaultClient.Do(request)
	if err != nil {
		return 0, "", err
	}

	defer response.Body.Close()
	responseBody, err := ioutil.ReadAll(response.Body)
	if err != nil {
		return 0, "", err
	}

	return response.StatusCode, string(responseBody), nil
}

func openPort() (string, error) {
	l, err := net.Listen("tcp", "localhost:0")
	if err != nil {
		return "", err
	}

	defer l.Close()
	_, port, err := net.SplitHostPort(l.Addr().String())
	if err != nil {
		return "", err
	}

	return port, nil
}

func waitForServerToStart(port string) {
	timer := time.After(0 * time.Second)
	timeout := time.After(10 * time.Second)
	for {
		select {
		case <-timeout:
			panic("Failed to boot!")
		case <-timer:
			_, err := http.Get("http://localhost:" + port)
			if err == nil {
				return
			}

			timer = time.After(2 * time.Second)
		}
	}
}
