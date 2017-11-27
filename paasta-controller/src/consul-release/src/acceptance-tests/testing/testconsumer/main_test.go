package main_test

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"net/http/httptest"
	"os/exec"
	"strings"
	"time"

	"github.com/onsi/gomega/gexec"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Proxying consul requests", func() {
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
		It("returns 1 when the consul url is malformed", func() {
			command := exec.Command(pathToConsumer, "--port", port, "--consul-url", "%%%%%%%%%%", "--path-to-check-a-record", pathToCheckARecord)

			var err error
			session, err = gexec.Start(command, GinkgoWriter, GinkgoWriter)
			Expect(err).NotTo(HaveOccurred())

			Eventually(session).Should(gexec.Exit(1))
			Expect(session.Err.Contents()).To(ContainSubstring("invalid URL escape"))
		})

		It("returns 1 when path-to-check-a-record is not provided", func() {
			command := exec.Command(pathToConsumer, "--port", port, "--consul-url", "127.0.0.1")

			var err error
			session, err = gexec.Start(command, GinkgoWriter, GinkgoWriter)
			Expect(err).NotTo(HaveOccurred())

			Eventually(session).Should(gexec.Exit(1))
			Expect(session.Err.Contents()).To(ContainSubstring("--path-to-check-a-record is required"))
		})
	})

	Context("health_check", func() {
		BeforeEach(func() {
			consulServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
				w.WriteHeader(http.StatusTeapot)
			}))

			command := exec.Command(pathToConsumer, "--port", port, "--consul-url", consulServer.URL, "--path-to-check-a-record", pathToCheckARecord)

			var err error
			session, err = gexec.Start(command, GinkgoWriter, GinkgoWriter)
			Expect(err).NotTo(HaveOccurred())

			waitForServerToStart(port)
		})

		It("returns a 200 when the health check is alive", func() {
			status, _, err := makeRequest("GET", fmt.Sprintf("http://localhost:%s/health_check", port), "")
			Expect(err).NotTo(HaveOccurred())
			Expect(status).To(Equal(http.StatusOK))
		})

		It("returns a 503 when the health_check has been marked dead", func() {
			status, _, err := makeRequest("POST", fmt.Sprintf("http://localhost:%s/health_check", port), "false")
			Expect(err).NotTo(HaveOccurred())
			Expect(status).To(Equal(http.StatusOK))

			status, _, err = makeRequest("GET", fmt.Sprintf("http://localhost:%s/health_check", port), "")
			Expect(err).NotTo(HaveOccurred())
			Expect(status).To(Equal(http.StatusServiceUnavailable))
		})

	})

	Context("dns", func() {
		var pathToCheckARecord string

		BeforeEach(func() {
			var err error

			consulServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
				w.WriteHeader(http.StatusTeapot)
			}))

			args := []string{
				"-ldflags",
				"-X main.Addresses=127.0.0.2,127.0.0.3,127.0.0.4 -X main.ServiceName=something.service.cf.internal",
			}

			pathToCheckARecord, err = gexec.Build("github.com/cloudfoundry-incubator/consul-release/src/acceptance-tests/testing/testconsumer/fakes/checkarecord", args...)
			Expect(err).NotTo(HaveOccurred())

			command := exec.Command(pathToConsumer, "--port", port, "--consul-url", consulServer.URL, "--path-to-check-a-record", pathToCheckARecord)

			session, err = gexec.Start(command, GinkgoWriter, GinkgoWriter)
			Expect(err).NotTo(HaveOccurred())

			waitForServerToStart(port)
		})

		It("returns an array of ip addresses given the service", func() {
			status, body, err := makeRequest("GET", fmt.Sprintf("http://localhost:%s/dns?service=something.service.cf.internal", port), "")
			Expect(err).NotTo(HaveOccurred())
			Expect(status).To(Equal(http.StatusOK))
			Expect(body).To(Equal(`["127.0.0.2","127.0.0.3","127.0.0.4"]`))
		})
	})

	Context("with a functioning consul", func() {
		TestConsulProxy := func(enableTLS bool) {
			BeforeEach(func() {
				handlerFunc := http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
					if req.URL.Path == "/v1/kv/some-key" {
						if req.Method == "GET" {
							w.Write([]byte("some-value"))
							return
						}

						if req.Method == "PUT" {
							w.Write([]byte("true"))
							return
						}
					}

					w.WriteHeader(http.StatusTeapot)
				})

				var consulURL string
				var args []string
				if enableTLS {
					consulPort, err := openPort()
					Expect(err).NotTo(HaveOccurred())

					consulURL = fmt.Sprintf("https://127.0.0.1:%s", consulPort)
					startTLSServer(consulPort, "fixtures/server-ca.crt", "fixtures/agent.crt", "fixtures/agent.key", handlerFunc)
					args = append(args,
						"--cacert", "fixtures/server-ca.crt",
						"--cert", "fixtures/agent.crt",
						"--key", "fixtures/agent.key",
					)
				} else {
					consulServer := httptest.NewServer(handlerFunc)
					consulURL = consulServer.URL
				}
				args = append(args,
					"--port", port,
					"--consul-url", consulURL,
					"--path-to-check-a-record", pathToCheckARecord,
				)

				command := exec.Command(pathToConsumer, args...)

				var err error
				session, err = gexec.Start(command, GinkgoWriter, GinkgoWriter)
				Expect(err).NotTo(HaveOccurred())

				waitForServerToStart(port)
			})

			It("can set/get a key", func() {
				status, responseBody, err := makeRequest("PUT", fmt.Sprintf("http://localhost:%s/consul/v1/kv/some-key", port), "some-value")
				Expect(err).NotTo(HaveOccurred())
				Expect(status).To(Equal(http.StatusOK))
				Expect(responseBody).To(Equal("true"))

				status, responseBody, err = makeRequest("GET", fmt.Sprintf("http://localhost:%s/consul/v1/kv/some-key?raw", port), "")
				Expect(err).NotTo(HaveOccurred())
				Expect(status).To(Equal(http.StatusOK))
				Expect(responseBody).To(Equal("some-value"))
			})

			It("returns 418 for endpoints that are not supported", func() {
				status, _, err := makeRequest("OPTIONS", fmt.Sprintf("http://localhost:%s/consul/v1/kv/some-key", port), "")
				Expect(err).NotTo(HaveOccurred())
				Expect(status).To(Equal(http.StatusTeapot))

				status, _, err = makeRequest("GET", fmt.Sprintf("http://localhost:%s/consul/some/missing/path", port), "")
				Expect(err).NotTo(HaveOccurred())
				Expect(status).To(Equal(http.StatusTeapot))
			})
		}

		TestConsulProxy(true)
		TestConsulProxy(false)
	})

	Context("proxy errors", func() {
		It("returns a bad gateway status", func() {
			var err error
			command := exec.Command(pathToConsumer, "--port", port, "--consul-url", "http://localhost:999999", "--path-to-check-a-record", pathToCheckARecord)

			session, err = gexec.Start(command, GinkgoWriter, GinkgoWriter)
			Expect(err).NotTo(HaveOccurred())

			waitForServerToStart(port)

			status, _, err := makeRequest("GET", fmt.Sprintf("http://localhost:%s/consul/v1/kv/some-key?raw", port), "")
			Expect(err).NotTo(HaveOccurred())
			Expect(status).To(Equal(http.StatusBadGateway))
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
			_, err := http.Get("http://localhost:" + port + "/consul/v1/kv/banana")
			if err == nil {
				return
			}

			timer = time.After(1 * time.Second)
		}
	}
}

func startTLSServer(port, cert, key, caCert string, handlerFunc http.HandlerFunc) *http.Server {
	consulServer := &http.Server{
		Addr:    fmt.Sprintf(":%s", port),
		Handler: handlerFunc,
		TLSConfig: createTLSConfig(
			"fixtures/agent.crt",
			"fixtures/agent.key",
			"fixtures/server-ca.crt",
		),
	}
	go func() {
		log.Fatal(consulServer.ListenAndServeTLS("fixtures/agent.crt", "fixtures/agent.key"))
	}()

	return consulServer
}

func createTLSConfig(consulCertPath, consulKeyPath, consulCACertPath string) *tls.Config {
	cert, err := tls.LoadX509KeyPair(consulCertPath, consulKeyPath)
	if err != nil {
		log.Fatal(err)
	}

	caCert, err := ioutil.ReadFile(consulCACertPath)
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

	return tlsConfig
}
