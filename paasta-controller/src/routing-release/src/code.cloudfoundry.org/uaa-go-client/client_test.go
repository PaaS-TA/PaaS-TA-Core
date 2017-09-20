package uaa_go_client_test

import (
	"crypto/tls"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"path"
	"path/filepath"
	"strings"
	"time"

	"code.cloudfoundry.org/clock/fakeclock"
	"code.cloudfoundry.org/lager/lagertest"
	"code.cloudfoundry.org/uaa-go-client"
	"code.cloudfoundry.org/uaa-go-client/config"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/ghttp"
)

var _ = Describe("UAA Client", func() {
	Context("non-TLS client", func() {

		BeforeEach(func() {
			forceUpdate = false
			cfg = &config.Config{
				MaxNumberOfRetries:    DefaultMaxNumberOfRetries,
				RetryInterval:         DefaultRetryInterval,
				ExpirationBufferInSec: DefaultExpirationBufferTime,
			}
			server = ghttp.NewServer()

			url, err := url.Parse(server.URL())
			Expect(err).ToNot(HaveOccurred())

			addr := strings.Split(url.Host, ":")

			cfg.UaaEndpoint = "http://" + addr[0] + ":" + addr[1]
			Expect(err).ToNot(HaveOccurred())

			cfg.ClientName = "client-name"
			cfg.ClientSecret = "client-secret"
			clock = fakeclock.NewFakeClock(time.Now())
			logger = lagertest.NewTestLogger("test")
		})

		AfterEach(func() {
			server.Close()
		})

		Describe("uaa_go_client.NewClient", func() {
			Context("when all values are valid", func() {
				It("returns a token fetcher instance", func() {
					client, err := uaa_go_client.NewClient(logger, cfg, clock)
					Expect(err).NotTo(HaveOccurred())
					Expect(client).NotTo(BeNil())
				})
			})

			Context("when values are invalid", func() {
				Context("when oauth config is nil", func() {
					It("returns error", func() {
						client, err := uaa_go_client.NewClient(logger, nil, clock)
						Expect(err).To(HaveOccurred())
						Expect(err.Error()).To(ContainSubstring("Configuration cannot be nil"))
						Expect(client).To(BeNil())
					})
				})

				Context("when oauth config client id is empty", func() {
					It("creates new client", func() {
						config := &config.Config{
							UaaEndpoint:  "http://some.url:80",
							ClientName:   "",
							ClientSecret: "client-secret",
						}
						client, err := uaa_go_client.NewClient(logger, config, clock)
						Expect(err).ToNot(HaveOccurred())
						Expect(client).ToNot(BeNil())
					})
				})

				Context("when oauth config client secret is empty", func() {
					It("creates a new client", func() {
						config := &config.Config{
							UaaEndpoint:  "http://some.url:80",
							ClientName:   "client-name",
							ClientSecret: "",
						}
						client, err := uaa_go_client.NewClient(logger, config, clock)
						Expect(err).ToNot(HaveOccurred())
						Expect(client).ToNot(BeNil())
					})
				})

				Context("when oauth config tokenendpoint is empty", func() {
					It("returns error", func() {
						config := &config.Config{
							UaaEndpoint:  "",
							ClientName:   "client-name",
							ClientSecret: "client-secret",
						}
						client, err := uaa_go_client.NewClient(logger, config, clock)
						Expect(err).To(HaveOccurred())
						Expect(err.Error()).To(ContainSubstring("UAA endpoint cannot be empty"))
						Expect(client).To(BeNil())
					})
				})

				Context("when token fetcher config's max number of retries is zero", func() {
					It("creates the client", func() {
						config := &config.Config{
							UaaEndpoint:           "http://some.url:80",
							MaxNumberOfRetries:    0,
							RetryInterval:         2 * time.Second,
							ExpirationBufferInSec: 30,
							ClientName:            "client-name",
							ClientSecret:          "client-secret",
						}
						client, err := uaa_go_client.NewClient(logger, config, clock)
						Expect(err).NotTo(HaveOccurred())
						Expect(client).NotTo(BeNil())
					})
				})

				Context("when token fetcher config's expiration buffer time is negative", func() {
					It("sets the expiration buffer time to the default value", func() {
						config := &config.Config{
							MaxNumberOfRetries:    3,
							RetryInterval:         2 * time.Second,
							ExpirationBufferInSec: -1,
							UaaEndpoint:           "http://some.url:80",
							ClientName:            "client-name",
							ClientSecret:          "client-secret",
						}
						client, err := uaa_go_client.NewClient(logger, config, clock)
						Expect(err).NotTo(HaveOccurred())
						Expect(client).NotTo(BeNil())
					})
				})
			})
		})
	})
	Context("secure (TLS) client", func() {

		var (
			tlsServer   *http.Server
			tlsListener net.Listener
		)
		BeforeEach(func() {
			forceUpdate = false
			cfg = &config.Config{
				MaxNumberOfRetries:    DefaultMaxNumberOfRetries,
				RetryInterval:         DefaultRetryInterval,
				ExpirationBufferInSec: DefaultExpirationBufferTime,
			}

			listener, err := net.Listen("tcp", "127.0.0.1:0")
			addr := strings.Split(listener.Addr().String(), ":")

			cfg.UaaEndpoint = "https://" + addr[0] + ":" + addr[1]
			Expect(err).NotTo(HaveOccurred())

			handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Write([]byte(fmt.Sprintf("{\"alg\":\"alg\", \"value\": \"%s\" }", ValidPemPublicKey)))
			})

			tlsListener = newTlsListener(listener)
			tlsServer = &http.Server{Handler: handler}

			go func() {
				err := tlsServer.Serve(tlsListener)
				Expect(err).ToNot(HaveOccurred())
			}()

			Expect(err).ToNot(HaveOccurred())

			cfg.ClientName = "client-name"
			cfg.ClientSecret = "client-secret"

			clock = fakeclock.NewFakeClock(time.Now())
			logger = lagertest.NewTestLogger("test")
		})

		Context("when CA cert provided", func() {
			var (
				tlsClient uaa_go_client.Client
			)

			BeforeEach(func() {
				caCertPath, err := filepath.Abs(path.Join("fixtures", "ca.pem"))
				Expect(err).ToNot(HaveOccurred())

				cfg.CACerts = caCertPath
				cfg.MaxNumberOfRetries = 0
				tlsClient, err = uaa_go_client.NewClient(logger, cfg, clock)
				Expect(err).ToNot(HaveOccurred())
				Expect(tlsClient).ToNot(BeNil())
			})

			It("can make uaa request with cert", func() {
				_, err := tlsClient.FetchToken(true)
				Expect(err).ToNot(HaveOccurred())
			})
		})

		Context("when secure uaa client skips verify", func() {
			var (
				tlsClient uaa_go_client.Client
			)

			BeforeEach(func() {
				cfg.SkipVerification = true
				var err error
				tlsClient, err = uaa_go_client.NewClient(logger, cfg, clock)
				Expect(err).ToNot(HaveOccurred())
				Expect(tlsClient).ToNot(BeNil())
			})

			It("logs fetching token", func() {
				_, err := tlsClient.FetchToken(true)
				Expect(err).ToNot(HaveOccurred())
				Expect(logger).To(gbytes.Say("uaa-client"))
				Expect(logger).To(gbytes.Say("started-fetching-token"))
				Expect(logger).To(gbytes.Say(cfg.UaaEndpoint))
				Expect(logger).To(gbytes.Say("successfully-fetched-token"))
			})

			It("logs fetching key", func() {
				_, err := tlsClient.FetchKey()
				Expect(err).ToNot(HaveOccurred())
				Expect(logger).To(gbytes.Say("uaa-client"))
				Expect(logger).To(gbytes.Say("fetch-key-starting"))
				Expect(logger).To(gbytes.Say(cfg.UaaEndpoint))
				Expect(logger).To(gbytes.Say("fetch-key-successful"))
			})
		})
	})
})

func newTlsListener(listener net.Listener) net.Listener {
	public := "fixtures/server.pem"
	private := "fixtures/server.key"
	cert, err := tls.LoadX509KeyPair(public, private)
	Expect(err).ToNot(HaveOccurred())

	tlsConfig := &tls.Config{
		Certificates: []tls.Certificate{cert},
		CipherSuites: []uint16{tls.TLS_RSA_WITH_AES_256_CBC_SHA},
	}

	return tls.NewListener(listener, tlsConfig)
}
