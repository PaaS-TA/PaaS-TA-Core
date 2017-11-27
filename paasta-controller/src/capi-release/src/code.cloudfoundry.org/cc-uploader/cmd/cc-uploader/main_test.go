package main_test

import (
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"code.cloudfoundry.org/cc-uploader"
	"code.cloudfoundry.org/cc-uploader/ccclient/fake_cc"
	"code.cloudfoundry.org/cc-uploader/config"
	"code.cloudfoundry.org/cfhttp"
	"code.cloudfoundry.org/runtimeschema/cc_messages"
	"code.cloudfoundry.org/urljoiner"
	"github.com/hashicorp/consul/api"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/gexec"
)

type ByteEmitter struct {
	written int
	length  int
}

func NewEmitter(length int) *ByteEmitter {
	return &ByteEmitter{
		length:  length,
		written: 0,
	}
}

func (emitter *ByteEmitter) Read(p []byte) (n int, err error) {
	if emitter.written >= emitter.length {
		return 0, io.EOF
	}
	time.Sleep(time.Millisecond)
	p[0] = 0xF1
	emitter.written++
	return 1, nil
}

var _ = Describe("CC Uploader", func() {
	var (
		uploaderConfig  config.UploaderConfig
		httpListenPort  int
		httpsListenPort int
		session         *gexec.Session
		configFile      *os.File
		appGuid         = "app-guid"
		fakeCCServer    *httptest.Server
	)

	dropletUploadRequest := func(appGuid string, body io.Reader, contentLength int, address string) *http.Request {
		ccUrl, err := url.Parse(fakeCCServer.URL)
		Expect(err).NotTo(HaveOccurred())
		ccUrl.Path = urljoiner.Join("staging", "droplets", appGuid, "upload")
		v := url.Values{"async": []string{"true"}}
		ccUrl.RawQuery = v.Encode()

		route, ok := ccuploader.Routes.FindRouteByName(ccuploader.UploadDropletRoute)
		Expect(ok).To(BeTrue())

		path, err := route.CreatePath(map[string]string{"guid": appGuid})
		Expect(err).NotTo(HaveOccurred())

		u, err := url.Parse(urljoiner.Join(address, path))
		Expect(err).NotTo(HaveOccurred())
		v = url.Values{cc_messages.CcDropletUploadUriKey: []string{ccUrl.String()}}
		u.RawQuery = v.Encode()

		postRequest, err := http.NewRequest("POST", u.String(), body)
		Expect(err).NotTo(HaveOccurred())
		postRequest.ContentLength = int64(contentLength)
		postRequest.Header.Set("Content-Type", "application/octet-stream")

		return postRequest
	}

	BeforeEach(func() {
		httpListenPort = 8182 + GinkgoParallelNode()
		httpsListenPort = 9192 + GinkgoParallelNode()

		uploaderConfig = config.DefaultUploaderConfig()
		uploaderConfig.ConsulCluster = consulRunner.URL()
		uploaderConfig.ListenAddress = fmt.Sprintf("localhost:%d", httpListenPort)
		uploaderConfig.CCCACert = filepath.Join("..", "..", "fixtures", "cc_uploader_ca_cn.crt")
		uploaderConfig.CCClientCert = filepath.Join("..", "..", "fixtures", "cc_uploader_cn.crt")
		uploaderConfig.CCClientKey = filepath.Join("..", "..", "fixtures", "cc_uploader_cn.key")

		uploaderConfig.ListenAddress = fmt.Sprintf("localhost:%d", httpListenPort)
		uploaderConfig.MutualTLS = config.MutualTLS{
			ListenAddress: fmt.Sprintf("localhost:%d", httpsListenPort),
			CACert:        filepath.Join("..", "..", "fixtures", "certs", "ca.crt"),
			ServerCert:    filepath.Join("..", "..", "fixtures", "certs", "server.crt"),
			ServerKey:     filepath.Join("..", "..", "fixtures", "certs", "server.key"),
		}

		var err error
		configFile, err = ioutil.TempFile("", "uploader_config")
		Expect(err).NotTo(HaveOccurred())
		configJson, err := json.Marshal(uploaderConfig)
		Expect(err).NotTo(HaveOccurred())
		err = ioutil.WriteFile(configFile.Name(), configJson, 0644)
		Expect(err).NotTo(HaveOccurred())

		args := []string{
			"-configPath", configFile.Name(),
		}
		session, err = gexec.Start(exec.Command(ccUploaderBinary, args...), GinkgoWriter, GinkgoWriter)
		Expect(err).NotTo(HaveOccurred())

		Eventually(session).Should(gbytes.Say("cc-uploader.ready"))
	})

	AfterEach(func() {
		os.Remove(configFile.Name())
		session.Kill().Wait()
	})

	Describe("Uploading a file", func() {
		var (
			contentLength     = 100
			postRequest       *http.Request
			ccUploaderAddress string
		)

		JustBeforeEach(func() {
			emitter := NewEmitter(contentLength)
			postRequest = dropletUploadRequest(appGuid, emitter, contentLength, ccUploaderAddress)
		})

		Context("when the HTTP endpoint of the cc-uploader is used", func() {
			BeforeEach(func() {
				fakeCC = fake_cc.New()
				fakeCCServer = httptest.NewUnstartedServer(fakeCC)
				fakeCCServer.Start()

				ccUploaderAddress = fmt.Sprintf("http://localhost:%d", httpListenPort)
			})

			AfterEach(func() {
				fakeCCServer.Close()
			})

			It("should upload the file", func() {
				resp, err := http.DefaultClient.Do(postRequest)
				Expect(err).NotTo(HaveOccurred())
				defer resp.Body.Close()

				Expect(resp.StatusCode).To(Equal(http.StatusCreated))
				Expect(len(fakeCC.UploadedDroplets[appGuid])).To(Equal(contentLength))
			})
		})

		Context("when the HTTPS endpoint of the cc-uploader is used", func() {
			var httpClient *http.Client

			BeforeEach(func() {
				ccUploaderAddress = fmt.Sprintf("https://localhost:%d", httpsListenPort)
				httpClient = cfhttp.NewClient()
				cellTLSConfig, err := cfhttp.NewTLSConfig(
					filepath.Join("..", "..", "fixtures", "certs", "client.crt"),
					filepath.Join("..", "..", "fixtures", "certs", "client.key"),
					filepath.Join("..", "..", "fixtures", "certs", "ca.crt"),
				)
				Expect(err).NotTo(HaveOccurred())
				httpClient.Transport = &http.Transport{TLSClientConfig: cellTLSConfig}
			})

			Context("when the CC callback URI is HTTP", func() {
				BeforeEach(func() {
					fakeCC = fake_cc.New()
					fakeCCServer = httptest.NewUnstartedServer(fakeCC)
					fakeCCServer.Start()
				})

				AfterEach(func() {
					fakeCCServer.Close()
				})

				It("should upload the file using an HTTP client", func() {
					resp, err := httpClient.Do(postRequest)
					Expect(err).NotTo(HaveOccurred())
					defer resp.Body.Close()

					Expect(resp.StatusCode).To(Equal(http.StatusCreated))
					Expect(len(fakeCC.UploadedDroplets[appGuid])).To(Equal(contentLength))
				})
			})

			Context("when the CC callback URI is HTTPS", func() {
				BeforeEach(func() {
					fakeCC = fake_cc.New()
					fakeCCServer = httptest.NewUnstartedServer(fakeCC)

					cert, err := tls.LoadX509KeyPair(uploaderConfig.CCClientCert, uploaderConfig.CCClientKey)
					if err != nil {
						log.Fatalln("Unable to load cert", err)
					}
					caCert, err := ioutil.ReadFile(uploaderConfig.CCCACert)
					if err != nil {
						log.Fatal("Unable to open cert", err)
					}

					clientCertPool := x509.NewCertPool()
					clientCertPool.AppendCertsFromPEM(caCert)
					tlsConfig := &tls.Config{
						InsecureSkipVerify: false,
						Certificates:       []tls.Certificate{cert},
						ClientAuth:         tls.RequireAndVerifyClientCert,
						ClientCAs:          clientCertPool,
						RootCAs:            clientCertPool,
						CipherSuites: []uint16{
							tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,
							tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,
						},
					}

					fakeCCServer.TLS = tlsConfig
					fakeCCServer.StartTLS()
				})

				AfterEach(func() {
					fakeCCServer.Close()
				})

				It("should upload the file using an HTTPS client with mTLS", func() {
					resp, err := httpClient.Do(postRequest)
					Expect(err).NotTo(HaveOccurred())
					defer resp.Body.Close()

					Expect(resp.StatusCode).To(Equal(http.StatusCreated))
					Expect(len(fakeCC.UploadedDroplets[appGuid])).To(Equal(contentLength))
				})
			})
		})
	})

	Describe("Initialization", func() {
		It("registers itself with consul", func() {
			services, err := consulRunner.NewClient().Agent().Services()
			Expect(err).NotTo(HaveOccurred())
			Expect(services).Should(HaveKeyWithValue("cc-uploader",
				&api.AgentService{
					Service: "cc-uploader",
					ID:      "cc-uploader",
					Port:    httpListenPort,
				}))
		})

		It("registers a TTL healthcheck", func() {
			checks, err := consulRunner.NewClient().Agent().Checks()
			Expect(err).NotTo(HaveOccurred())
			Expect(checks).Should(HaveKeyWithValue("service:cc-uploader",
				&api.AgentCheck{
					Node:        "0",
					CheckID:     "service:cc-uploader",
					Name:        "Service 'cc-uploader' check",
					Status:      "passing",
					ServiceID:   "cc-uploader",
					ServiceName: "cc-uploader",
				}))
		})
	})
})
